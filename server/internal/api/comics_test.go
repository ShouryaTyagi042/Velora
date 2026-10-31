package api

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

// comicHandler serves comic c1 from a temp dir holding two PNGs drawn with image/png
// (no binary fixtures in git) and a file outside the comic folder.
func comicHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	comic := filepath.Join(dir, "comics", "Watchmen")
	if err := os.MkdirAll(comic, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"page-1.png", "page-2.png"} {
		f, err := os.Create(filepath.Join(comic, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })

	repo := memstore.New([]media.Media{
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, RelPath: "comics/Watchmen",
			Comic: &media.ComicMeta{PageCount: 3, Pages: []string{"page-1.png", "page-2.png", "page-3.png"}}}, // page-3 deleted since the scan
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/The Matrix.mp4"},
	})
	return signedInHandler(repo, newFakeJobs(nil, nil), root.FS(), fstest.MapFS{}), dir
}

func TestPages(t *testing.T) {
	h, _ := comicHandler(t)
	tests := []struct {
		path        string
		wantStatus  int
		wantType    string
		wantForever bool // Cache-Control: immutable
	}{
		{"/api/media/c1/pages", 200, "application/json; charset=utf-8", false},
		{"/api/media/v1/pages", 404, "application/json; charset=utf-8", false},
		{"/api/media/c1/pages/0", 200, "image/png", false},
		{"/api/media/c1/pages/1?v=2026-10-01T00:00:00Z", 200, "image/png", true},
		{"/api/media/c1/pages/2", 404, "application/json; charset=utf-8", false},  // gone since the scan
		{"/api/media/c1/pages/3", 404, "application/json; charset=utf-8", false},  // out of range
		{"/api/media/c1/pages/-1", 404, "application/json; charset=utf-8", false}, // out of range
		{"/api/media/c1/pages/one", 400, "application/json; charset=utf-8", false},
		{"/api/media/c1/pages/..%2F..%2Fsecret.txt", 400, "application/json; charset=utf-8", false},
		{"/api/media/v1/pages/0", 404, "application/json; charset=utf-8", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tt.wantType {
				t.Errorf("Content-Type = %q, want %q", ct, tt.wantType)
			}
			if forever := strings.Contains(rec.Header().Get("Cache-Control"), "immutable"); forever != tt.wantForever {
				t.Errorf("Cache-Control = %q, want immutable: %v", rec.Header().Get("Cache-Control"), tt.wantForever)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Errorf("leaked a file outside the comic: %s", rec.Body)
			}
		})
	}
}

func TestPageCount(t *testing.T) {
	h, _ := comicHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/c1/pages", nil))
	if got := strings.TrimSpace(rec.Body.String()); got != `{"pageCount":3}` {
		t.Errorf("body = %s", got)
	}
}

func TestPageConditionalAndRange(t *testing.T) {
	h, _ := comicHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/c1/pages/0", nil))
	lastMod := rec.Header().Get("Last-Modified")
	if lastMod == "" {
		t.Fatal("no Last-Modified")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/c1/pages/0", nil)
	req.Header.Set("If-Modified-Since", lastMod)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("If-Modified-Since: got %d with %d bytes, want 304 and no body", rec.Code, rec.Body.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/media/c1/pages/0", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "\x89PNG" {
		t.Errorf("Range: got %d %q, want 206 and the PNG signature", rec.Code, rec.Body.String())
	}
}

func TestComicThumbnailIsFirstPage(t *testing.T) {
	h, _ := comicHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/c1/thumbnail", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("got %d %s, want 200 image/png", rec.Code, rec.Header().Get("Content-Type"))
	}
}
