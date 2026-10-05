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
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, Path: "/secret/the-matrix.mp4"},
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, Path: "/secret/watchmen.cbz"},
	}))
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

func TestListFiltersByKindAndHidesPath(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media?kind=video", nil))

	var body listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != "v1" {
		t.Errorf("items = %+v, want only v1", body.Items)
	}
	if strings.Contains(rec.Body.String(), "/secret/") {
		t.Errorf("response leaks a filesystem path: %s", rec.Body)
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
	NewHandler(failingRepo{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media", nil))

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
