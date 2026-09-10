package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func testAPI(t *testing.T) *API {
	t.Helper()
	s, err := NewStore(privateTestDir(t))
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(privateTestDir(t), s.dir, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	a, err := NewAPI(s, e, "127.0.0.1:37963")
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func request(a *API, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Elciber-Token", a.token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestAPITrustBoundary(t *testing.T) {
	a := testAPI(t)
	for _, p := range []string{"/api/state", "/api/unknown", "/api/rooms", "/api"} {
		r := httptest.NewRequest("GET", "http://"+a.host+p, nil)
		r.RemoteAddr = "127.0.0.1:1"
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Errorf("missing token %s: %d", p, w.Code)
		}
	}
	mutations := map[string]func(*http.Request){"host": func(r *http.Request) { r.Host = "evil.example:37963" }, "origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, "null origin": func(r *http.Request) { r.Header.Set("Origin", "null") }, "duplicate origin": func(r *http.Request) {
		r.Header.Add("Origin", "http://"+a.host)
		r.Header.Add("Origin", "http://"+a.host)
	}, "fetch site": func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, "remote": func(r *http.Request) { r.RemoteAddr = "192.168.1.2:12" }, "token": func(r *http.Request) { r.Header.Set("X-Elciber-Token", "wrong") }, "duplicate token": func(r *http.Request) { r.Header.Add("X-Elciber-Token", a.token) }}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+a.host+"/api/state", nil)
			r.RemoteAddr = "127.0.0.1:1"
			r.Header.Set("X-Elciber-Token", a.token)
			mutate(r)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != 401 && w.Code != 403 {
				t.Fatal(w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("CORS enabled")
			}
		})
	}
	r := httptest.NewRequest("GET", "http://"+a.host+"/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 || strings.Contains(w.Body.String(), a.token) {
		t.Fatal("HTML token exposed cross-origin")
	}
}
func TestAPIWorkflowWithoutEngine(t *testing.T) {
	a := testAPI(t)
	state := request(a, "GET", "/api/state", "")
	if state.Code != 200 || strings.Contains(state.Body.String(), "secret") {
		t.Fatal("state invalid")
	}
	created := request(a, "POST", "/api/rooms", `{"name":"Sala LAN","game":"Juego","rendezvous":"","rendezvousKey":""}`)
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var pub Room
	if err := json.Unmarshal(created.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.store.Get(pub.ID)
	if strings.Contains(created.Body.String(), saved.Secret) {
		t.Fatal("create leaks secret")
	}
	invite := request(a, "POST", "/api/rooms/"+pub.ID+"/invite", `{}`)
	if invite.Code != 200 {
		t.Fatal(invite.Code)
	}
	var data map[string]string
	_ = json.Unmarshal(invite.Body.Bytes(), &data)
	body, _ := json.Marshal(map[string]string{"invite": data["invite"]})
	imported := request(a, "POST", "/api/rooms/import", string(body))
	if imported.Code != 201 || len(a.store.Rooms()) != 1 {
		t.Fatal("duplicate import mutated identity")
	}
	connect := request(a, "POST", "/api/connect", `{"roomId":"`+pub.ID+`","acknowledge":false}`)
	if connect.Code != 400 {
		t.Fatal("acknowledge not required")
	}
	connect = request(a, "POST", "/api/connect", `{"roomId":"`+pub.ID+`","acknowledge":true}`)
	if connect.Code != 503 {
		t.Fatal("missing engine not visible", connect.Code)
	}
	if strings.Contains(connect.Body.String(), a.store.dir) || strings.Contains(connect.Body.String(), saved.Secret) {
		t.Fatal("private path or secret leak")
	}
	if a.engine.proc != nil {
		t.Fatal("unexpected child")
	}
	for i := 0; i < 2; i++ {
		if w := request(a, "POST", "/api/disconnect", `{}`); w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	if w := request(a, "DELETE", "/api/rooms/"+pub.ID, `{}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if len(a.store.Rooms()) != 0 {
		t.Fatal("delete failed")
	}
}
func TestAPIBodyAndMethodLimits(t *testing.T) {
	a := testAPI(t)
	for _, body := range []string{`{"name":"x","engineDir":"evil"}`, `{"Name":"x"}`, `{"name":"x","name":"y"}`, `null`, `{} {}`, `{"name":"x","routes":[]}`} {
		if w := request(a, "POST", "/api/rooms", body); w.Code != 400 {
			t.Errorf("body accepted %q: %d", body, w.Code)
		}
	}
	if w := request(a, "POST", "/api/rooms", strings.Repeat("x", maxBody+1)); w.Code != 413 {
		t.Fatal("body unbounded", w.Code)
	}
	for _, typ := range []string{"text/plain", "application/x-www-form-urlencoded", "application/json; junk=yes", ""} {
		r := httptest.NewRequest("POST", "http://"+a.host+"/api/rooms", strings.NewReader(`{"name":"x"}`))
		r.RemoteAddr = "127.0.0.1:1"
		r.Header.Set("X-Elciber-Token", a.token)
		if typ != "" {
			r.Header.Set("Content-Type", typ)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 415 {
			t.Fatal("content-type accepted", typ, w.Code)
		}
	}
	if w := request(a, "OPTIONS", "/api/rooms", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := request(a, "POST", "/api/state", `{}`); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := request(a, "POST", "/api/disconnect", `{"force":true}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
func TestEmbeddedWebAndHeaders(t *testing.T) {
	a := testAPI(t)
	w := request(a, "GET", "/", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if bytes.Count(w.Body.Bytes(), []byte(a.token)) != 1 || strings.Contains(w.Body.String(), "__ELCIBER_TOKEN__") {
		t.Fatal("token injection failed")
	}
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "Cache-Control", "Referrer-Policy", "X-Content-Type-Options"} {
		if w.Header().Get(h) == "" {
			t.Fatal("missing header", h)
		}
	}
	for _, p := range []string{"/../rooms.json", "/rooms.json", "/.git/config", "/api.go", "/web/index.html"} {
		if w := request(a, "GET", p, ""); w.Code != 404 {
			t.Fatal("private file served", p, w.Code)
		}
	}
}
func TestListenValidation(t *testing.T) {
	for _, v := range []string{"127.0.0.1:37963", "[::1]:37963"} {
		if _, err := validateListen(v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"localhost:37963", "0.0.0.0:37963", ":37963", "192.168.1.2:37963", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:037963", "[::]:37963", "http://127.0.0.1:37963"} {
		if _, err := validateListen(v); err == nil {
			t.Fatal("unsafe listen", v)
		}
	}
}
func TestAPIConcurrentStateAndMutations(t *testing.T) {
	a := testAPI(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			for j := 0; j < 5; j++ {
				w := request(a, "GET", "/api/state", "")
				if w.Code != 200 {
					t.Error(w.Code)
				}
			}
		})
	}
	wg.Go(func() {
		for i := 0; i < 5; i++ {
			w := request(a, "POST", "/api/rooms", `{"name":"Sala"}`)
			if w.Code != 201 {
				t.Error(w.Code)
			}
		}
	})
	wg.Wait()
}
