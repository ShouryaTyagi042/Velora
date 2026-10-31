package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/crypto/bcrypt"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

const (
	testUser     = "tester"
	testPassword = "correct horse"
)

// newTestAuth returns an auth service holding testUser, and a live token for them.
// bcrypt.MinCost keeps the tests fast; production uses auth.DefaultCost.
func newTestAuth() (*auth.Service, string) {
	ctx := context.Background()
	a := auth.New(memstore.NewUsers(), auth.Options{Cost: bcrypt.MinCost})
	if _, err := a.CreateUser(ctx, testUser, testPassword); err != nil {
		panic(err)
	}
	token, _, err := a.Login(ctx, testUser, testPassword)
	if err != nil {
		panic(err)
	}
	return a, token
}

// signedInHandler is NewHandler with every request that has no Authorization header sent as
// testUser, so the endpoint tests test endpoints rather than repeat the header.
func signedInHandler(repo media.Repository, js Jobs, files, thumbs fs.FS) http.Handler {
	a, token := newTestAuth()
	h := NewHandler(repo, js, files, thumbs, a)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		h.ServeHTTP(w, r)
	})
}

// asUser runs h as a signed-in user without a session lookup: for tests that mount a single
// handler on their own mux.
func asUser(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.User{ID: 1, Username: testUser}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

// rawHandler is the API without the automatic session, and a valid token to send by hand.
func rawHandler() (http.Handler, string) {
	a, token := newTestAuth()
	return NewHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/The Matrix.mp4"},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{}, a), token
}

func do(h http.Handler, method, path, body, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("not an error envelope: %s", rec.Body)
	}
	return env.Error.Code
}

func TestEveryRouteNeedsASession(t *testing.T) {
	h, _ := rawHandler()
	routes := []struct{ method, path, body string }{
		{"GET", "/api/media", ""},
		{"GET", "/api/media/v1", ""},
		{"GET", "/api/media/v1/stream", ""},
		{"GET", "/api/media/v1/thumbnail", ""},
		{"GET", "/api/media/v1/pages/0", ""},
		{"GET", "/api/actors", ""},
		{"GET", "/api/continue", ""},
		{"GET", "/api/auth/me", ""},
		{"POST", "/api/auth/logout", ""},
		// Every write that was open on the LAN until now:
		{"POST", "/api/scan", ""},
		{"POST", "/api/media/v1/actors", `{"name":"Carrie-Anne Moss"}`},
		{"DELETE", "/api/media/v1/actors/Keanu%20Reeves", ""},
		{"PUT", "/api/media/v1/progress", `{"positionMs":1,"durationMs":2,"clientTs":"2026-10-01T00:00:00Z"}`},
		// An unknown path doesn't reveal that it's unknown before you sign in.
		{"GET", "/api/nope", ""},
	}
	for _, rt := range routes {
		for _, authz := range []string{"", "Bearer not-a-real-token", "Basic dGVzdGVyOnB3", "Bearer"} {
			rec := do(h, rt.method, rt.path, rt.body, authz)
			if rec.Code != 401 || errorCode(t, rec) != "unauthorized" {
				t.Errorf("%s %s with %q: got %d %s, want 401 unauthorized", rt.method, rt.path, authz, rec.Code, rec.Body)
			}
		}
	}
	if rec := do(h, "GET", "/api/media", "", ""); rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("a 401 must carry WWW-Authenticate")
	}
}

func TestHealthNeedsNoSession(t *testing.T) {
	h, _ := rawHandler()
	if rec := do(h, "GET", "/health", "", ""); rec.Code != 200 {
		t.Errorf("health: %d", rec.Code)
	}
}

func TestLoginFlow(t *testing.T) {
	h, _ := rawHandler()

	rec := do(h, "POST", "/api/auth/login", `{"username":"tester","password":"correct horse"}`, "")
	var got loginResponse
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Token == "" || got.User.Username != testUser {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the login response must not be cached")
	}
	if strings.Contains(rec.Body.String(), `"id"`) {
		t.Errorf("login exposes the user id: %s", rec.Body)
	}
	bearer := "Bearer " + got.Token

	if rec := do(h, "GET", "/api/auth/me", "", bearer); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"username":"tester"}` {
		t.Errorf("me: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/media", "", "bearer "+got.Token); rec.Code != 200 {
		t.Errorf("lowercase scheme: %d, want 200 (the scheme is case-insensitive)", rec.Code)
	}
	if rec := do(h, "POST", "/api/auth/logout", "", bearer); rec.Code != 204 {
		t.Errorf("logout: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/media", "", bearer); rec.Code != 401 {
		t.Errorf("after logout: %d, want 401", rec.Code)
	}
}

func TestLoginFailures(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
		wantCode   string
	}{
		{"wrong password", `{"username":"tester","password":"wrong horse"}`, 401, "unauthorized"},
		{"unknown user", `{"username":"nobody","password":"correct horse"}`, 401, "unauthorized"},
		{"missing password", `{"username":"tester"}`, 400, "validation"},
		{"not json", `username=tester`, 400, "validation"},
	}
	for _, tt := range tests {
		h, _ := rawHandler() // a fresh limiter per case
		rec := do(h, "POST", "/api/auth/login", tt.body, "")
		if rec.Code != tt.wantStatus || errorCode(t, rec) != tt.wantCode {
			t.Errorf("%s: got %d %s, want %d %s", tt.name, rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
		}
	}
	// The two 401s read the same: the answer doesn't say which usernames exist.
	h, _ := rawHandler()
	a := do(h, "POST", "/api/auth/login", tests[0].body, "").Body.String()
	b := do(h, "POST", "/api/auth/login", tests[1].body, "").Body.String()
	if a != b {
		t.Errorf("wrong password and unknown user differ:\n%s\n%s", a, b)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	h, _ := rawHandler()
	wrong := `{"username":"tester","password":"wrong horse"}`
	for i := range 5 {
		if rec := do(h, "POST", "/api/auth/login", wrong, ""); rec.Code != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, rec.Code)
		}
	}
	// The sixth, even with the right password, waits.
	rec := do(h, "POST", "/api/auth/login", `{"username":"tester","password":"correct horse"}`, "")
	if rec.Code != 429 || errorCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("sixth attempt: %d %v %s, want 429 rate_limited with Retry-After", rec.Code, rec.Header(), rec.Body)
	}

	// Another address has its own budget.
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(wrong))
	req.RemoteAddr = "192.0.2.99:5555"
	other := httptest.NewRecorder()
	h.ServeHTTP(other, req)
	if other.Code != 401 {
		t.Errorf("another address: %d, want 401", other.Code)
	}
}

func TestProgressBelongsToTheSignedInUser(t *testing.T) {
	ctx := context.Background()
	a := auth.New(memstore.NewUsers(), auth.Options{Cost: bcrypt.MinCost})
	tokens := map[string]string{}
	for _, name := range []string{"ana", "ben"} {
		if _, err := a.CreateUser(ctx, name, testPassword); err != nil {
			t.Fatal(err)
		}
		tok, _, err := a.Login(ctx, name, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = "Bearer " + tok
	}
	h := NewHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/The Matrix.mp4"},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{}, a)

	save := `{"positionMs":60000,"durationMs":600000,"clientTs":"2026-10-01T00:00:00Z"}`
	if rec := do(h, "PUT", "/api/media/v1/progress", save, tokens["ana"]); rec.Code != 200 {
		t.Fatalf("ana's save: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/media/v1/progress", "", tokens["ben"]); rec.Code != 404 {
		t.Errorf("ben reads ana's progress: %d %s, want 404", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/continue", "", tokens["ben"]); !strings.Contains(rec.Body.String(), `"watching":[]`) {
		t.Errorf("ben's continue: %s", rec.Body)
	}
	if rec := do(h, "GET", "/api/continue", "", tokens["ana"]); !strings.Contains(rec.Body.String(), `"The Matrix"`) {
		t.Errorf("ana's continue: %s", rec.Body)
	}
}
