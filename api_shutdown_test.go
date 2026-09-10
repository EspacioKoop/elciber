package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQuitRequiresAuthenticatedExplicitPost(t *testing.T) {
	a := testAPI(t)
	called := make(chan struct{}, 1)
	a.shutdown = func() { called <- struct{}{} }
	for _, method := range []string{"GET", "DELETE"} {
		if w := request(a, method, "/api/quit", "{}"); w.Code == 200 {
			t.Fatal("non-POST quit accepted")
		}
	}
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/quit", strings.NewReader("{}"))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("unauthenticated quit accepted")
	}
	select {
	case <-called:
		t.Fatal("quit called without authorization")
	default:
	}
	if w := request(a, "POST", "/api/quit", "{}"); w.Code != 200 {
		t.Fatal("authenticated quit failed")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("shutdown not requested")
	}
}
