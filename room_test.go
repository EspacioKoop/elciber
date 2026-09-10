package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func privateTestDir(t testing.TB) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-data")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestRejectPublicDataDirWithoutChmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix mode regression")
	}
	dir := privateTestDir(t)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(dir); err == nil {
		t.Fatal("public directory accepted")
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0755 {
		t.Fatal("silently modified existing directory permissions")
	}
}
func testRoom(t *testing.T) savedRoom {
	t.Helper()
	r, err := createRoom(roomInput{Name: "Partida amigos", Game: "LAN"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestRoomAndInviteRoundTrip(t *testing.T) {
	r := testRoom(t)
	other := testRoom(t)
	if r.ID == other.ID || r.Secret == other.Secret {
		t.Fatal("random identities reused")
	}
	inv, err := encodeInvite(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeInvite(inv)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID || got.Secret != r.Secret || got.Name != r.Name {
		t.Fatal("roundtrip failed")
	}
	b, _ := json.Marshal(r.Room)
	if strings.Contains(string(b), r.Secret) || strings.Contains(string(b), "secret") {
		t.Fatal("public room leaks secret")
	}
}
func TestStrictInviteRejection(t *testing.T) {
	r := testRoom(t)
	inv, _ := encodeInvite(r)
	payload, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(inv, invitePrefix))
	var good map[string]any
	_ = json.Unmarshal(payload, &good)
	cases := map[string]func(map[string]any){
		"version": func(v map[string]any) { v["version"] = 2 }, "version string": func(v map[string]any) { v["version"] = "1" },
		"missing id": func(v map[string]any) { delete(v, "id") }, "short secret": func(v map[string]any) { v["secret"] = "weak" },
		"uppercase id": func(v map[string]any) { v["id"] = strings.ToUpper(r.ID) }, "uuid id": func(v map[string]any) { v["id"] = "00000000-0000-0000-0000-000000000000" },
		"newline": func(v map[string]any) { v["name"] = "room\nsecret" }, "bidi": func(v map[string]any) { v["name"] = "room\u202eexe" },
		"large name": func(v map[string]any) { v["name"] = strings.Repeat("a", 81) }, "bad scheme": func(v map[string]any) { v["rendezvous"] = "file:///tmp/secret" },
		"userinfo": func(v map[string]any) { v["rendezvous"] = "tcp://u:p@127.0.0.1:1234" }, "args": func(v map[string]any) { v["args"] = []string{"--daemon"} },
		"engine path": func(v map[string]any) { v["engineDir"] = "/tmp/evil" }, "key": func(v map[string]any) { v["rendezvousKey"] = "not a key" },
		"case alias": func(v map[string]any) { v["Name"] = "alias" }, "null": func(v map[string]any) { v["game"] = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := map[string]any{}
			for k, x := range good {
				v[k] = x
			}
			mutate(v)
			b, _ := json.Marshal(v)
			if _, err := decodeInvite(invitePrefix + base64.RawURLEncoding.EncodeToString(b)); err == nil {
				t.Fatal("accepted malformed invitation")
			}
		})
	}
	for _, s := range []string{"", inv + "=", inv + "\n", strings.Repeat("a", maxInvite+1), "elciber2:" + strings.TrimPrefix(inv, invitePrefix), invitePrefix + base64.RawURLEncoding.EncodeToString(append(payload, payload...)), invitePrefix + base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"version":1}`))} {
		if _, err := decodeInvite(s); err == nil {
			t.Fatal("accepted malformed encoding/JSON")
		}
	}
}
func TestStrictJSON(t *testing.T) {
	for _, b := range []string{`null`, `[]`, `{"name":"x","name":"y"}`, `{"Name":"x"}`, `{"name":"x"} {}`, `{"name":null}`, `{"name":"x","unknown":1}`, "{\"name\":\"\xff\"}", strings.Repeat("[", 18) + strings.Repeat("]", 18)} {
		var in roomInput
		if decodeStrict([]byte(b), &in) == nil {
			t.Fatalf("accepted %q", b)
		}
	}
	var in roomInput
	if err := decodeStrict([]byte(`{"name":"Salón","game":"","rendezvous":""}`), &in); err != nil {
		t.Fatal(err)
	}
}
func TestEndpointValidation(t *testing.T) {
	for _, s := range []string{"", "tcp://127.0.0.1:1234", "udp://[::1]:1234", "tcp://friends.example:11010", "udp://192.168.1.10:11010"} {
		if validateEndpoint(s, false) != nil {
			t.Errorf("rejected valid %q", s)
		}
	}
	for _, s := range []string{"tcp://localhost:1234", "tcp://friends.example:1234", "udp://192.168.1.10:1234"} {
		if validateEndpoint(s, true) == nil {
			t.Errorf("inspection accepts nonnumeric/nonloopback %q", s)
		}
	}
	for _, s := range []string{"tcp://127.0.0.1", "udp://127.0.0.1:0", "tcp://127.0.0.1:65536", "tcp://127.0.0.1:01", "tcp://127.0.0.1:+2", "TCP://127.0.0.1:12", "tcp://u@127.0.0.1:12", "tcp://127.0.0.1:12/", "tcp://127.0.0.1:12?", "tcp://127.0.0.1:12#", "tcp://127.0.0.1:12\n", "tcp://127.1:1234", "udp://0.0.0.0:12", "udp://224.0.0.1:12", "udp://[fe80::1%eth0]:12", "tcp://bad_host:12", "tcp://bad..host:12", "tcp://a\\b:12", "tcp://0x7f000001:12"} {
		if validateEndpoint(s, false) == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestStoreAtomicRoundTripAndFailure(t *testing.T) {
	dir := privateTestDir(t)
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := testRoom(t)
	if _, err := s.Add(r); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(r.ID)
	if err != nil || loaded != r {
		t.Fatal("persistence roundtrip")
	}
	before, _ := os.ReadFile(filepath.Join(dir, "rooms.json"))
	s.write = func(string, []byte) error { return errors.New("private /secret/path token") }
	if _, err := s.Add(testRoom(t)); !errors.Is(err, errDisk) {
		t.Fatal(err)
	}
	if err := s.Delete(r.ID); !errors.Is(err, errDisk) {
		t.Fatal(err)
	}
	if len(s.Rooms()) != 1 {
		t.Fatal("memory mutated despite disk failure")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "rooms.json"))
	if string(before) != string(after) {
		t.Fatal("disk changed on failure")
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(filepath.Join(dir, "rooms.json"))
		if st.Mode().Perm() != 0600 {
			t.Fatal("public file permissions")
		}
		st, _ = os.Stat(dir)
		if st.Mode().Perm() != 0700 {
			t.Fatal("public dir permissions")
		}
	}
}
func TestStoreIdentityLimitAndConcurrentAccess(t *testing.T) {
	s, err := NewStore(privateTestDir(t))
	if err != nil {
		t.Fatal(err)
	}
	r := testRoom(t)
	_, _ = s.Add(r)
	if _, err := s.Add(r); err != nil || len(s.Rooms()) != 1 {
		t.Fatal("identical import not idempotent")
	}
	changed := r
	changed.Secret = testRoom(t).Secret
	if _, err := s.Add(changed); !errors.Is(err, errConflict) {
		t.Fatal("secret overwrite allowed")
	}
	changed = r
	changed.Rendezvous = "tcp://127.0.0.1:11010"
	if _, err := s.Add(changed); !errors.Is(err, errConflict) {
		t.Fatal("destination overwrite allowed")
	}
	var wg sync.WaitGroup
	for i := 1; i < maxRooms; i++ {
		rr := testRoom(t)
		wg.Go(func() {
			_, err := s.Add(rr)
			if err != nil {
				t.Error(err)
			}
			_ = s.Rooms()
			_, _ = s.Get(r.ID)
		})
	}
	wg.Wait()
	if len(s.Rooms()) != maxRooms {
		t.Fatal("lost concurrent write")
	}
	if _, err := s.Add(testRoom(t)); !errors.Is(err, errLimit) {
		t.Fatal("limit not enforced")
	}
	if err := s.Delete(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(r.ID); !errors.Is(err, errMissing) {
		t.Fatal("delete not applied")
	}
}
func TestStoreCorruptionFailsClosed(t *testing.T) {
	for _, data := range []string{`{"version":2,"rooms":[]}`, `{"version":1,"rooms":null}`, `{"version":1,"rooms":[],"engine":"evil"}`, `{"version":1,"rooms":[]}junk`, strings.Repeat("x", (1<<20)+1)} {
		dir := privateTestDir(t)
		p := filepath.Join(dir, "rooms.json")
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewStore(dir); err == nil {
			t.Fatal("corrupt store accepted")
		}
		b, _ := os.ReadFile(p)
		if string(b) != data {
			t.Fatal("corrupt store overwritten")
		}
	}
}
func TestStoreRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Windows privilege")
	}
	dir := privateTestDir(t)
	outside := filepath.Join(privateTestDir(t), "private")
	_ = os.WriteFile(outside, []byte("unchanged"), 0600)
	if err := os.Symlink(outside, filepath.Join(dir, "rooms.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(dir); err == nil {
		t.Fatal("followed rooms symlink")
	}
}
func TestAtomicWriteFailureAndReplace(t *testing.T) {
	dir := privateTestDir(t)
	p := filepath.Join(dir, "rooms.json")
	if err := atomicWrite(p, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(p, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "new" {
		t.Fatal("replacement failed")
	}
	targetDir := filepath.Join(dir, "not-a-file")
	_ = os.Mkdir(targetDir, 0700)
	if atomicWrite(targetDir, []byte("x")) == nil {
		t.Fatal("invalid rename accepted")
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".rooms-*"))
	if len(temps) != 0 {
		t.Fatal("temporary files leaked")
	}
}
func TestDataLock(t *testing.T) {
	dir := privateTestDir(t)
	unlock, err := acquireDataLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if next, err := acquireDataLock(dir); err == nil {
		next()
		unlock()
		t.Fatal("second writer admitted")
	}
	unlock()
	next, err := acquireDataLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	next()
}
func FuzzDecodeInvite(f *testing.F) {
	f.Add("elciber1:e30")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		r, err := decodeInvite(s)
		if err == nil {
			if err := validateRoom(r); err != nil {
				t.Fatal("invalid decoded invite")
			}
			_, err := encodeInvite(r)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}
