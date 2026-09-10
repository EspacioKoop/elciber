package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

var errLocked = errors.New("El directorio de datos está abierto en otra instancia de El Ciber.")

func main() {
	if err := run(os.Args[1:]); err != nil {
		reportStartupError(err)
		os.Exit(1)
	}
}
func run(args []string) error {
	flags := flag.NewFlagSet("elciber", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:37963", "Dirección HTTP loopback numérica")
	dataDir := flags.String("data-dir", "", "Directorio privado de salas")
	engineDir := flags.String("engine-dir", "", "Directorio de EasyTier 2.6.4 (sin descarga automática)")
	vpn := flags.Bool("enable-vpn", false, "Permitir adaptador VPN al conectar explícitamente")
	noBrowser := flags.Bool("no-browser", false, "No abrir el navegador")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errors.New("Argumentos no válidos; consulta --help.")
	}
	if flags.NArg() != 0 {
		return errors.New("No se aceptan argumentos posicionales.")
	}
	addr, err := validateListen(*listen)
	if err != nil {
		return err
	}
	if *dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return errors.New("Indica un directorio privado mediante --data-dir.")
		}
		*dataDir = filepath.Join(base, "elciber")
	}
	if err := privateDir(*dataDir); err != nil {
		return err
	}
	unlock, err := acquireDataLock(*dataDir)
	if err != nil {
		return err
	}
	defer unlock()
	store, err := NewStore(*dataDir)
	if err != nil {
		return err
	}
	engine, err := NewEngine(*engineDir, store.dir, *vpn)
	if err != nil {
		return err
	}
	defer engine.Close()
	api, err := NewAPI(store, engine, addr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("No se pudo abrir el puerto local. Comprueba si El Ciber ya está abierto.")
	}
	defer ln.Close()
	root, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	api.shutdown = cancel
	server := &http.Server{Handler: api, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0), BaseContext: func(net.Listener) context.Context { return root }}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	fmt.Printf("El Ciber: http://%s · modo %s\n", addr, engine.Snapshot().Mode)
	if !*noBrowser {
		openBrowser("http://" + addr)
	}
	select {
	case <-root.Done():
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(ctx)
		_ = server.Close()
		engine.Close()
		return nil
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("El servidor local se ha detenido.")
		}
		return nil
	}
}
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}
