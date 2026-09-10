package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type zipEntry struct {
	name, content string
	mode          os.FileMode
}

func checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func makeZIP(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, e.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func fixture(t *testing.T, platform string) (artifact, []zipEntry) {
	t.Helper()
	a, err := officialArtifact(platform)
	if err != nil {
		t.Fatal(err)
	}
	var entries []zipEntry
	for _, name := range a.Files {
		entries = append(entries, zipEntry{a.Base + "/" + name, "synthetic test bytes: " + name, 0700})
	}
	return a, entries
}

func serveZIP(t *testing.T, a artifact, data []byte, before func()) (artifact, *http.Client) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if before != nil {
			before()
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(s.Close)
	a.URL, a.SHA256 = s.URL, checksum(data)
	return a, s.Client()
}

func assertClean(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".elciber-engine-") {
			t.Errorf("staging leaked: %s", entry.Name())
		}
	}
}

func TestInstallManifestAndExactAllowlist(t *testing.T) {
	for _, platform := range []string{"linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			a, entries := fixture(t, platform)
			entries = append(entries,
				zipEntry{"../escaped", "never extract", 0600},
				zipEntry{a.Base + "/../../escaped", "never extract", 0600},
				zipEntry{"/absolute/escaped", "never extract", 0600},
				zipEntry{a.Base + "\\" + a.Files[0], "never extract", 0600},
				zipEntry{a.Base + "/install.ps1", "never execute", 0600},
				zipEntry{a.Base + "/link", "../../escaped", os.ModeSymlink | 0777},
			)
			data := makeZIP(t, entries)
			parent := t.TempDir()
			dest := filepath.Join(parent, "engine")
			a, client := serveZIP(t, a, data, func() {
				stages, _ := filepath.Glob(filepath.Join(parent, ".elciber-engine-*"))
				if len(stages) != 1 {
					t.Errorf("want one sibling staging directory, got %v", stages)
					return
				}
				st, err := os.Stat(stages[0])
				if err != nil {
					t.Error(err)
					return
				}
				if runtime.GOOS != "windows" && st.Mode().Perm() != 0700 {
					t.Errorf("staging mode %o", st.Mode().Perm())
				}
			})
			var progress bytes.Buffer
			if err := install(context.Background(), dest, a, client, &progress); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadDir(dest)
			if err != nil {
				t.Fatal(err)
			}
			if len(contents) != len(a.Files)+1 {
				t.Fatalf("unexpected destination contents: %v", contents)
			}
			var m manifest
			manifestData, err := os.ReadFile(filepath.Join(dest, manifestName))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(manifestData, &m); err != nil {
				t.Fatal(err)
			}
			if m.Version != engineVersion || m.Platform != platform || m.ArchiveSHA256 != checksum(data) || len(m.Files) != len(a.Files) {
				t.Fatalf("bad manifest: %+v", m)
			}
			for _, name := range a.Files {
				p := filepath.Join(dest, name)
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != "synthetic test bytes: "+name || m.Files[name] != checksum(data) {
					t.Fatalf("bad file/hash: %s", name)
				}
				st, err := os.Lstat(p)
				if err != nil {
					t.Fatal(err)
				}
				if !st.Mode().IsRegular() {
					t.Fatal("extracted non-regular file")
				}
				if platform == "linux" && runtime.GOOS != "windows" && st.Mode().Perm() != 0700 {
					t.Fatalf("not executable/private: %o", st.Mode().Perm())
				}
			}
			if _, err := os.Lstat(filepath.Join(parent, "escaped")); !os.IsNotExist(err) {
				t.Fatal("traversal escaped")
			}
			assertClean(t, parent)
			if !strings.Contains(progress.String(), "No se ha ejecutado") {
				t.Fatal("missing completion message")
			}
		})
	}
}

func TestRejectedArchivesCleanStaging(t *testing.T) {
	for _, kind := range []string{"wrong-hash", "missing", "traversal-only", "symlink", "duplicate", "empty", "bad-zip", "expanded-limit"} {
		t.Run(kind, func(t *testing.T) {
			a, entries := fixture(t, "linux")
			switch kind {
			case "missing":
				entries = entries[:1]
			case "traversal-only":
				entries[0].name = a.Base + "/../" + a.Files[0]
			case "symlink":
				entries[0].mode = os.ModeSymlink | 0777
			case "duplicate":
				entries = append(entries, entries[0])
			case "empty":
				entries[0].content = ""
			}
			data := makeZIP(t, entries)
			if kind == "bad-zip" {
				data = []byte("not a zip; private remote payload")
			}
			if kind == "expanded-limit" {
				var b bytes.Buffer
				w := zip.NewWriter(&b)
				_, err := w.CreateRaw(&zip.FileHeader{Name: entries[0].name, Method: zip.Store, UncompressedSize64: uint64(maxBytes + 1)})
				if err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				data = b.Bytes()
			}
			a, client := serveZIP(t, a, data, nil)
			if kind == "wrong-hash" {
				a.SHA256 = strings.Repeat("0", 64)
			}
			parent := t.TempDir()
			dest := filepath.Join(parent, "engine")
			if err := install(context.Background(), dest, a, client, io.Discard); err == nil {
				t.Fatal("accepted bad archive")
			}
			if !absent(dest) {
				t.Fatal("failed install created destination")
			}
			assertClean(t, parent)
		})
	}
}

func TestExistingDestinationPreserved(t *testing.T) {
	for _, kind := range []string{"directory", "empty-directory", "file", "dangling-symlink", "created-during-download"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "dangling-symlink" && runtime.GOOS == "windows" {
				t.Skip("Windows symlink privileges not assumed")
			}
			parent := t.TempDir()
			dest := filepath.Join(parent, "engine")
			create := func() {
				switch kind {
				case "file":
					if err := os.WriteFile(dest, []byte("preserve me"), 0600); err != nil {
						t.Error(err)
					}
				case "dangling-symlink":
					if err := os.Symlink(filepath.Join(parent, "missing"), dest); err != nil {
						t.Error(err)
					}
				default:
					if err := os.Mkdir(dest, 0700); err != nil {
						t.Error(err)
					}
					if kind == "directory" {
						if err := os.WriteFile(filepath.Join(dest, "sentinel"), []byte("preserve me"), 0600); err != nil {
							t.Error(err)
						}
					}
				}
			}
			a, entries := fixture(t, "linux")
			called := false
			a, client := serveZIP(t, a, makeZIP(t, entries), func() {
				called = true
				if kind == "created-during-download" {
					create()
				}
			})
			if kind != "created-during-download" {
				create()
			}
			if err := install(context.Background(), dest, a, client, io.Discard); err == nil {
				t.Fatal("accepted existing destination")
			}
			if kind != "created-during-download" && called {
				t.Error("downloaded before rejecting existing destination")
			}
			st, err := os.Lstat(dest)
			if err != nil {
				t.Fatal("removed existing destination:", err)
			}
			switch kind {
			case "directory", "file":
				p := dest
				if kind == "directory" {
					p = filepath.Join(dest, "sentinel")
				}
				data, err := os.ReadFile(p)
				if err != nil || string(data) != "preserve me" {
					t.Fatal("modified existing data")
				}
			case "dangling-symlink":
				if st.Mode()&os.ModeSymlink == 0 {
					t.Fatal("replaced symlink")
				}
			default:
				children, err := os.ReadDir(dest)
				if err != nil || len(children) != 0 {
					t.Fatal("modified empty existing destination")
				}
			}
			assertClean(t, parent)
		})
	}
}

func TestHTTPFailuresAndCancellation(t *testing.T) {
	for _, kind := range []string{"status", "too-large", "cancelled", "tls"} {
		t.Run(kind, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "too-large" {
					w.Header().Set("Content-Length", "167772161")
					return
				}
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, "PRIVATE_SERVER_PAYLOAD")
			})
			var s *httptest.Server
			if kind == "tls" {
				s = httptest.NewTLSServer(handler)
			} else {
				s = httptest.NewServer(handler)
			}
			defer s.Close()
			a, _ := fixture(t, "linux")
			a.URL = s.URL
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancelled" {
				cancel()
			}
			parent := t.TempDir()
			dest := filepath.Join(parent, "engine")
			client := downloadClient() // Intentionally reject self-signed TLS.
			defer client.CloseIdleConnections()
			err := install(ctx, dest, a, client, io.Discard)
			if err == nil {
				t.Fatal("accepted HTTP failure")
			}
			if strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), s.URL) {
				t.Fatal("leaked network payload/URL")
			}
			if !absent(dest) {
				t.Fatal("created failed destination")
			}
			assertClean(t, parent)
		})
	}
}

func TestTransportIgnoresProxiesAndRejectsDowngrade(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	client := downloadClient()
	defer client.CloseIdleConnections()
	tr := client.Transport.(*http.Transport)
	if tr.Proxy != nil {
		t.Fatal("inherits proxy")
	}
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification disabled")
	}
	if client.Timeout != installTimeout {
		t.Fatal("missing timeout")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if client.CheckRedirect(req, nil) == nil {
		t.Fatal("accepted HTTPS downgrade")
	}
}

func TestCLIValidationIsSanitized(t *testing.T) {
	for _, args := range [][]string{nil, {"--platform", "linux"}, {"--destination", "x", "--platform", "PRIVATE_TOKEN"}, {"--PRIVATE_TOKEN"}} {
		var stdout, stderr bytes.Buffer
		if got := run(args, &stdout, &stderr); got != 2 {
			t.Fatalf("code %d for %v", got, args)
		}
		if strings.Contains(stderr.String(), "PRIVATE_TOKEN") {
			t.Fatal("argument leaked")
		}
	}
	var stdout, stderr bytes.Buffer
	if run([]string{"--help"}, &stdout, &stderr) != 0 || !strings.Contains(stdout.String(), "predeterminada: windows") {
		t.Fatal("missing default/help")
	}
}
