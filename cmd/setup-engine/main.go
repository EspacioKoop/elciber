// setup-engine is an installer-only downloader. It never starts EasyTier,
// installs drivers/services, or configures the host network.
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	engineVersion        = "2.6.4"
	maxBytes       int64 = 160 * 1024 * 1024
	installTimeout       = 120 * time.Second
	manifestName         = "manifest.json"
)

type artifact struct {
	URL, SHA256, Base, Platform string
	Files                       []string
}

func officialArtifact(platform string) (artifact, error) {
	a := artifact{Platform: platform}
	switch platform {
	case "windows":
		a.Base = "easytier-windows-x86_64"
		a.SHA256 = "27af91e270e554709b048bd32327fefd2dfce5062ae1e8701af7550c6f525f84"
		a.Files = []string{"easytier-core.exe", "easytier-cli.exe", "wintun.dll"}
	case "linux":
		a.Base = "easytier-linux-x86_64"
		a.SHA256 = "61b659eaedba658fa66fe47d17e1426cdd77e5d02fa15fed447bb4357c09dfd6"
		a.Files = []string{"easytier-core", "easytier-cli"}
	default:
		return artifact{}, errors.New("plataforma no válida; utiliza windows o linux")
	}
	a.URL = "https://github.com/EasyTier/EasyTier/releases/download/v" + engineVersion + "/" + a.Base + "-v" + engineVersion + ".zip"
	return a, nil
}

// No ProxyFromEnvironment, custom roots, insecure TLS, or user-supplied URLs.
func downloadClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:    true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   installTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil {
				return errors.New("redirección de descarga no permitida")
			}
			return nil
		},
	}
}

type manifest struct {
	Version       string            `json:"version"`
	Platform      string            `json:"platform"`
	ArchiveSHA256 string            `json:"archive_sha256"`
	Files         map[string]string `json:"files"`
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func absent(path string) bool {
	_, err := os.Lstat(path) // Also refuse dangling symlinks.
	return errors.Is(err, os.ErrNotExist)
}

// The artifact/client seams are internal and permit fully offline HTTP tests.
// Public CLI callers can only select the two pinned official artifacts.
func install(ctx context.Context, destination string, a artifact, client *http.Client, progress io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	if destination == "" {
		return errors.New("debes indicar --destination")
	}
	dest, err := filepath.Abs(destination)
	if err != nil {
		return errors.New("destino no válido")
	}
	if !absent(dest) {
		return errors.New("el destino ya existe o no es accesible; no se sobrescribe ni repara")
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return errors.New("no se puede preparar el directorio padre")
	}
	stage, err := os.MkdirTemp(parent, ".elciber-engine-") // 0700, same filesystem.
	if err != nil {
		return errors.New("no se puede crear el directorio temporal")
	}
	defer os.RemoveAll(stage)

	fmt.Fprintln(progress, "Descargando EasyTier 2.6.4 desde su publicación oficial…")
	archivePath := filepath.Join(stage, "download.zip")
	archiveHash, err := download(ctx, client, a, archivePath)
	if err != nil {
		return err
	}
	fmt.Fprintln(progress, "SHA-256 del paquete verificado. Preparando los archivos del motor…")
	fileHashes, err := extract(ctx, archivePath, stage, a)
	if err != nil {
		return err
	}
	if err := os.Remove(archivePath); err != nil {
		return errors.New("no se puede limpiar el paquete temporal")
	}
	data, err := json.MarshalIndent(manifest{engineVersion, a.Platform, archiveHash, fileHashes}, "", "  ")
	if err != nil {
		return errors.New("no se puede crear el manifiesto")
	}
	if err := os.WriteFile(filepath.Join(stage, manifestName), append(data, '\n'), 0600); err != nil {
		return errors.New("no se puede guardar el manifiesto")
	}
	if ctx.Err() != nil {
		return errors.New("se ha agotado el tiempo de instalación")
	}

	// Claim the destination exclusively. Renaming a directory directly would
	// replace an existing empty directory on Unix, violating refuse-existing.
	// Publish the manifest LAST as the completion marker. This is deliberately
	// not crash-atomic: interrupted installs are refused on retry, not repaired.
	if err := os.Mkdir(dest, 0700); err != nil {
		return errors.New("el destino ya existe o no se puede crear; no se sobrescribe")
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(dest)
		}
	}()
	for _, name := range append(append([]string{}, a.Files...), manifestName) {
		if ctx.Err() != nil {
			return errors.New("se ha agotado el tiempo de instalación")
		}
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(dest, name)); err != nil {
			return errors.New("no se pueden publicar los archivos del motor")
		}
	}
	committed = true
	fmt.Fprintln(progress, "Motor preparado y manifiesto guardado. No se ha ejecutado el motor ni instalado controladores.")
	return nil
}

func download(ctx context.Context, client *http.Client, a artifact, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", errors.New("no se puede preparar la descarga")
	}
	response, err := client.Do(req)
	if err != nil {
		return "", errors.New("no se ha podido descargar el motor; comprueba la conexión y vuelve a intentarlo")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("la publicación oficial no está disponible; vuelve a intentarlo más tarde")
	}
	if response.ContentLength > maxBytes {
		return "", errors.New("el paquete supera el límite de 160 MiB")
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", errors.New("no se puede guardar el paquete temporal")
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(contextReader{ctx, response.Body}, maxBytes+1))
	closeErr := out.Close()
	if n > maxBytes {
		return "", errors.New("el paquete supera el límite de 160 MiB")
	}
	if copyErr != nil || closeErr != nil || ctx.Err() != nil {
		return "", errors.New("descarga interrumpida o fuera del tiempo permitido")
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != a.SHA256 {
		return "", errors.New("SHA-256 incorrecto; no se ha instalado el motor")
	}
	return actual, nil
}

func extract(ctx context.Context, archivePath, stage string, a artifact) (map[string]string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, errors.New("el paquete no es un ZIP válido")
	}
	defer archive.Close()
	hashes := make(map[string]string, len(a.Files))
	remaining := maxBytes // Bound aggregate expanded size as well as download.
	for _, name := range a.Files {
		var entry *zip.File
		for _, candidate := range archive.File {
			// Do not clean/join archive paths: ONLY exact upstream names match.
			if candidate.Name == a.Base+"/"+name {
				if entry != nil {
					return nil, errors.New("el paquete contiene archivos duplicados")
				}
				entry = candidate
			}
		}
		if entry == nil {
			return nil, errors.New("faltan archivos obligatorios en el paquete")
		}
		if !entry.Mode().IsRegular() || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > uint64(remaining) {
			return nil, errors.New("tipo o tamaño de archivo no permitido en el paquete")
		}
		source, err := entry.Open()
		if err != nil {
			return nil, errors.New("no se puede leer un archivo del paquete")
		}
		mode := os.FileMode(0600)
		if a.Platform == "linux" {
			mode = 0700
		}
		// Output names come from compiled-in allowlists, never from the ZIP.
		out, err := os.OpenFile(filepath.Join(stage, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			source.Close()
			return nil, errors.New("no se puede extraer un archivo del motor")
		}
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(contextReader{ctx, source}, remaining+1))
		sourceErr, closeErr := source.Close(), out.Close()
		if copyErr != nil || sourceErr != nil || closeErr != nil || ctx.Err() != nil || n > remaining || uint64(n) != entry.UncompressedSize64 {
			return nil, errors.New("archivo del motor dañado o fuera de los límites permitidos")
		}
		remaining -= n
		hashes[name] = hex.EncodeToString(h.Sum(nil))
	}
	return hashes, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("setup-engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // Never echo untrusted arguments or local paths.
	destination := flags.String("destination", "", "directorio nuevo para el motor")
	platform := flags.String("platform", "windows", "windows o linux (x86-64)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, "Uso: setup-engine --destination DIRECTORIO_NUEVO [--platform windows|linux]\nPlataforma predeterminada: windows (x86-64). Solo para el instalador; no actualiza ni repara destinos existentes.")
			return 0
		}
		fmt.Fprintln(stderr, "Error: argumentos no válidos; utiliza --help.")
		return 2
	}
	if *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Error: --destination es obligatorio; utiliza --help.")
		return 2
	}
	a, err := officialArtifact(*platform)
	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 2
	}
	client := downloadClient()
	defer client.CloseIdleConnections()
	if err := install(context.Background(), *destination, a, client, stdout); err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
