package cli

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The dashboard serves its page only to its own address (not a name that resolves to it: DNS
// rebinding), the page carries the token, and an API call without it is refused before the daemon
// is asked.
func TestWebHostAndToken(t *testing.T) {
	s := &webServer{e: &env{dir: t.TempDir(), noStart: true, admin: true}, token: "t0k"}
	h := s.handler("127.0.0.1:4125")
	get := func(host, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = host
		if token != "" {
			r.Header.Set(tokenHeader, token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := get("evil.example:4125", "/", ""); w.Code != http.StatusForbidden {
		t.Fatalf("another host: %d", w.Code)
	}
	for _, host := range []string{"127.0.0.1:4125", "localhost:4125"} {
		w := get(host, "/", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `const TOKEN = "t0k"`) {
			t.Fatalf("%s: %d, token in page: %v", host, w.Code, strings.Contains(w.Body.String(), "t0k"))
		}
	}
	for _, tok := range []string{"", "wrong"} {
		if w := get("127.0.0.1:4125", "/api/view", tok); w.Code != http.StatusForbidden {
			t.Fatalf("token %q: %d", tok, w.Code)
		}
	}
	// mail needs a participant or a team: refused before the daemon is asked
	if w := get("127.0.0.1:4125", "/api/mail", "t0k"); w.Code != http.StatusBadRequest {
		t.Fatalf("mail without a scope: %d", w.Code)
	}
	if w := get("127.0.0.1:4125", "/api/mail?participant=bob&pins=1", "t0k"); w.Code != http.StatusBadRequest {
		t.Fatalf("pins without a team: %d", w.Code)
	}
	// with the token it reaches the daemon, which is not running here
	if w := get("127.0.0.1:4125", "/api/view", "t0k"); w.Code != http.StatusBadGateway {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("with the token: %d %s", w.Code, b)
	}
}

// The dashboard acts as admin, so it refuses to listen beyond loopback.
func TestWebLoopbackOnly(t *testing.T) {
	e := &env{dir: t.TempDir(), stdout: io.Discard}
	for _, addr := range []string{"0.0.0.0:4125", "192.168.1.2:4125", ":4125"} {
		if err := e.web([]string{"--addr", addr}); !errors.Is(err, errUsage) {
			t.Fatalf("%s: %v", addr, err)
		}
	}
}
