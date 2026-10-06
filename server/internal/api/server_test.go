package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // keep request logs out of test output
	os.Exit(m.Run())
}

func newTestHandler() http.Handler {
	return NewHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/secret-dir/The Matrix.mp4",
			Actors: []media.ActorTag{{Name: "Keanu Reeves", Source: media.ActorFromFolder}}},
		{ID: "v2", Title: "Interstellar", Kind: media.KindVideo, RelPath: "movies/secret-dir/Interstellar.mp4",
			Actors: []media.ActorTag{{Name: "Matthew McConaughey", Source: media.ActorFromFolder}}},
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, RelPath: "comics/secret-dir/Watchmen",
			Comic: &media.ComicMeta{PageCount: 2, Pages: []string{"secret-page-1.png", "secret-page-2.png"}}},
	}), fakeRescanner{found: 3, warnings: nil})
}

// fakeRescanner stands in for the real scanner + store.
type fakeRescanner struct {
	found    int
	warnings []string
	err      error
}

func (f fakeRescanner) Rescan(ctx context.Context) (int, []string, error) {
	return f.found, f.warnings, f.err
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string // error envelope code; "" for success responses
	}{
		{"health", "/health", 200, ""},
		{"list all", "/api/media", 200, ""},
		{"list videos", "/api/media?kind=video", 200, ""},
		{"list bad kind", "/api/media?kind=audio", 400, "validation"},
		{"list by actor", "/api/media?actor=Keanu%20Reeves", 200, ""},
		{"list unknown actor", "/api/media?actor=Nobody", 200, ""},
		{"list empty actor", "/api/media?actor=", 400, "validation"},
		{"list huge actor", "/api/media?actor=" + strings.Repeat("a", 201), 400, "validation"},
		{"get existing", "/api/media/v1", 200, ""},
		{"get missing", "/api/media/nope", 404, "not_found"},
		{"thumbnail stub", "/api/media/v1/thumbnail", 501, "not_implemented"},
	}

	h := newTestHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			if rec.Header().Get("X-Request-ID") == "" {
				t.Error("missing X-Request-ID header")
			}
			if tt.wantCode == "" {
				return
			}
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("body is not an error envelope: %v; body: %s", err, rec.Body)
			}
			if env.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if env.Error.Message == "" {
				t.Error("error message is empty")
			}
		})
	}
}

func TestListFilters(t *testing.T) {
	tests := []struct {
		query   string
		wantIDs []string
	}{
		{"", []string{"v2", "v1", "c1"}}, // sorted by title: Interstellar, The Matrix, Watchmen
		{"?kind=video", []string{"v2", "v1"}},
		{"?kind=comic", []string{"c1"}},
		{"?actor=Keanu%20Reeves", []string{"v1"}},
		{"?actor=Keanu%20Reeves&kind=comic", []string{}},
		{"?actor=keanu%20reeves", []string{}}, // exact match: case matters
		{"?actor=Nobody", []string{}},
	}
	h := newTestHandler()
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media"+tt.query, nil))

			var body listResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, m := range body.Items {
				got = append(got, m.ID)
			}
			if body.Items == nil || !slices.Equal(got, tt.wantIDs) { // items must be [] (not null) even when empty
				t.Errorf("ids = %v (items nil: %v), want %v", got, body.Items == nil, tt.wantIDs)
			}
		})
	}
}

func TestResponsesHideFilesystemDetails(t *testing.T) {
	h := newTestHandler()
	for _, path := range []string{"/api/media", "/api/media/v1", "/api/media/c1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if body := rec.Body.String(); strings.Contains(body, "secret") {
			t.Errorf("%s leaks a path or page filename: %s", path, body)
		}
	}
}

func TestMediaJSONShape(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/v1", nil))
	if !strings.Contains(rec.Body.String(), `"actors":[{"name":"Keanu Reeves","source":"folder"}]`) {
		t.Errorf("video actors not in the expected shape: %s", rec.Body)
	}

	rec = httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/c1", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `"comic":{"pageCount":2}`) || strings.Contains(body, `"actors"`) {
		t.Errorf("comic JSON wrong: %s", body)
	}
}

func TestScanEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scan", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"found":3,"warnings":[]}` {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	NewHandler(failingRepo{}, fakeRescanner{err: errors.New("open /Users/x/media: permission denied")}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scan", nil))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "/Users/") {
		t.Errorf("scan failure: got %d %s, want a 500 that hides the path", rec.Code, rec.Body)
	}
}

// failingRepo returns an error that must never reach the client.
type failingRepo struct{}

func (failingRepo) List(ctx context.Context) ([]media.Media, error) {
	return nil, errors.New(`pq: relation "media" does not exist`)
}

func (failingRepo) Get(ctx context.Context, id string) (media.Media, error) {
	return media.Media{}, errors.New("disk on fire")
}

func TestInternalErrorsAreNotLeaked(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(failingRepo{}, fakeRescanner{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "pq:") {
		t.Errorf("internal error leaked to client: %s", rec.Body)
	}
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	Recover(panicky).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != "internal" {
		t.Errorf("body = %s, want internal error envelope", rec.Body)
	}
}
