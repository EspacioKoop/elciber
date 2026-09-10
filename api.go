package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFiles embed.FS

const appVersion = "0.1.0-alpha.1"

type API struct {
	control  sync.Mutex
	store    *Store
	engine   *Engine
	token    string
	host     string
	web      fs.FS
	shutdown context.CancelFunc // Set once, before serving any request.
}

func NewAPI(store *Store, engine *Engine, host string) (*API, error) {
	if _, err := validateListen(host); err != nil {
		return nil, err
	}
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return nil, errors.New("No se encontró la interfaz integrada.")
	}
	return &API{store: store, engine: engine, token: token, host: host, web: web}, nil
}
func validateListen(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.New("--listen requiere IP loopback numérica y puerto.")
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() || ip.Zone() != "" {
		return "", errors.New("--listen solo permite una IP loopback numérica.")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 || strconv.Itoa(p) != port {
		return "", errors.New("Puerto local no válido.")
	}
	if net.JoinHostPort(ip.String(), port) != addr {
		return "", errors.New("Usa la forma canónica de la IP loopback.")
	}
	return addr, nil
}
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	ip, ipErr := netip.ParseAddr(remote)
	if err != nil || ipErr != nil || !ip.IsLoopback() || r.Host != a.host {
		apiError(w, http.StatusForbidden, "Petición local no autorizada.")
		return
	}
	origins := r.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && origins[0] != "http://"+a.host) {
		apiError(w, http.StatusForbidden, "Origen no autorizado.")
		return
	}
	site := r.Header.Get("Sec-Fetch-Site")
	if site != "" && site != "same-origin" && site != "none" {
		apiError(w, http.StatusForbidden, "Origen no autorizado.")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
		tokens := r.Header.Values("X-Elciber-Token")
		if len(tokens) != 1 || subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(a.token)) != 1 {
			apiError(w, http.StatusUnauthorized, "Sesión local no válida. Recarga la página.")
			return
		}
		a.serveAPI(w, r)
		return
	}
	a.serveWeb(w, r)
}
func apiJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, code int, message string) {
	apiJSON(w, code, map[string]string{"error": message})
}
func replyError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, errDisk):
		code = http.StatusInternalServerError
	case errors.Is(err, errConflict) || errors.Is(err, errActive) || errors.Is(err, errLimit):
		code = http.StatusConflict
	case errors.Is(err, errMissing):
		code = http.StatusNotFound
	case errors.Is(err, errEngine) || errors.Is(err, errEngineMissing) || errors.Is(err, errEngineVersion):
		code = http.StatusServiceUnavailable
	}
	// All callers return predefined errors. Raw disk, process and JSON errors
	// never reach this boundary.
	apiError(w, code, err.Error())
}
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	values := r.Header.Values("Content-Type")
	if len(values) != 1 {
		apiError(w, 415, "Se requiere Content-Type: application/json.")
		return false
	}
	media, params, err := mime.ParseMediaType(values[0])
	if err != nil || media != "application/json" || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
		apiError(w, 415, "Se requiere Content-Type: application/json.")
		return false
	}
	if r.Header.Get("Content-Encoding") != "" {
		apiError(w, 415, "No se aceptan cuerpos comprimidos.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		apiError(w, 413, "La petición supera el límite permitido.")
		return false
	}
	if err := decodeStrict(b, dst); err != nil {
		apiError(w, 400, errInvalid.Error())
		return false
	}
	return true
}
func (a *API) serveAPI(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/api/state" {
		if r.Method != http.MethodGet {
			apiError(w, 405, "Método no permitido.")
			return
		}
		apiJSON(w, 200, struct {
			Version string      `json:"version"`
			Rooms   []Room      `json:"rooms"`
			Engine  EngineState `json:"engine"`
		}{appVersion, a.store.Rooms(), a.engine.Snapshot()})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		apiError(w, 405, "Método no permitido.")
		return
	}
	a.control.Lock()
	defer a.control.Unlock()
	switch {
	case p == "/api/rooms" && r.Method == http.MethodPost:
		var in roomInput
		if !readJSON(w, r, &in) {
			return
		}
		room, err := createRoom(in)
		if err != nil {
			replyError(w, err)
			return
		}
		pub, err := a.store.Add(room)
		if err != nil {
			replyError(w, err)
			return
		}
		apiJSON(w, 201, pub)
	case p == "/api/rooms/import" && r.Method == http.MethodPost:
		var in struct {
			Invite string `json:"invite"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		room, err := decodeInvite(in.Invite)
		if err != nil {
			replyError(w, err)
			return
		}
		pub, err := a.store.Add(room)
		if err != nil {
			replyError(w, err)
			return
		}
		apiJSON(w, 201, pub)
	case p == "/api/connect" && r.Method == http.MethodPost:
		var in struct {
			RoomID      string `json:"roomId"`
			Acknowledge bool   `json:"acknowledge"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		if !in.Acknowledge {
			apiError(w, 400, "Debes confirmar que confías en los invitados y en el destino.")
			return
		}
		if !idPattern.MatchString(in.RoomID) {
			replyError(w, errInvalid)
			return
		}
		room, err := a.store.Get(in.RoomID)
		if err != nil {
			replyError(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := a.engine.Connect(ctx, room); err != nil {
			replyError(w, err)
			return
		}
		apiJSON(w, 200, map[string]bool{"ok": true})
	case p == "/api/quit" && r.Method == http.MethodPost:
		if !readJSON(w, r, &struct{}{}) {
			return
		}
		if a.shutdown == nil {
			apiError(w, 503, "El cierre no está disponible.")
			return
		}
		apiJSON(w, 200, map[string]bool{"ok": true})
		if flush, ok := w.(http.Flusher); ok {
			flush.Flush()
		}
		go a.shutdown()
	case p == "/api/disconnect" && r.Method == http.MethodPost:
		if !readJSON(w, r, &struct{}{}) {
			return
		}
		a.engine.Disconnect()
		apiJSON(w, 200, map[string]bool{"ok": true})
	case strings.HasPrefix(p, "/api/rooms/"):
		parts := strings.Split(strings.TrimPrefix(p, "/api/rooms/"), "/")
		if len(parts) == 0 || !idPattern.MatchString(parts[0]) {
			apiError(w, 404, "Ruta no encontrada.")
			return
		}
		if len(parts) == 2 && parts[1] == "invite" && r.Method == http.MethodPost {
			if !readJSON(w, r, &struct{}{}) {
				return
			}
			room, err := a.store.Get(parts[0])
			if err != nil {
				replyError(w, err)
				return
			}
			inv, err := encodeInvite(room)
			if err != nil {
				replyError(w, err)
				return
			}
			apiJSON(w, 200, map[string]string{"invite": inv})
			return
		}
		if len(parts) == 1 && r.Method == http.MethodDelete {
			if !readJSON(w, r, &struct{}{}) {
				return
			}
			if a.engine.ActiveRoom() == parts[0] {
				apiError(w, 409, "No puedes borrar una sala activa. Desconecta primero.")
				return
			}
			if err := a.store.Delete(parts[0]); err != nil {
				replyError(w, err)
				return
			}
			apiJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		apiError(w, 404, "Ruta no encontrada.")
	default:
		apiError(w, 404, "Ruta no encontrada.")
	}
}
func (a *API) serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		apiError(w, 405, "Método no permitido.")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		apiError(w, 404, "Ruta no encontrada.")
		return
	}
	switch path.Ext(name) {
	case ".html":
		if name != "index.html" {
			apiError(w, 404, "Ruta no encontrada.")
			return
		}
	case ".js", ".css", ".svg", ".png", ".ico", ".woff2":
	default:
		apiError(w, 404, "Ruta no encontrada.")
		return
	}
	b, err := fs.ReadFile(a.web, name)
	if err != nil {
		apiError(w, 404, "Ruta no encontrada.")
		return
	}
	if name == "index.html" {
		if bytes.Count(b, []byte("__ELCIBER_TOKEN__")) != 1 {
			apiError(w, 500, "La interfaz integrada no es válida.")
			return
		}
		b = bytes.Replace(b, []byte("__ELCIBER_TOKEN__"), []byte(a.token), 1)
	}
	typ := mime.TypeByExtension(path.Ext(name))
	if typ == "" {
		typ = "application/octet-stream"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(200)
	if r.Method == http.MethodGet {
		_, _ = w.Write(b)
	}
}
