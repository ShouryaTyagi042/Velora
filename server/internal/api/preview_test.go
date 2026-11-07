package api

import (
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

func TestPreview(t *testing.T) {
	a, token := newTestAuth()
	h := NewHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "Ready", Kind: media.KindVideo, RelPath: "movies/Ready.mp4",
			Video: &media.VideoMeta{DurationMs: 60_000, Preview: true}},
		{ID: "v2", Title: "Stale", Kind: media.KindVideo, RelPath: "movies/Stale.mp4",
			Video: &media.VideoMeta{DurationMs: 60_000}}, // a file exists, but not for this version
		{ID: "v3", Title: "Lost", Kind: media.KindVideo, RelPath: "movies/Lost.mp4",
			Video: &media.VideoMeta{DurationMs: 60_000, Preview: true}}, // the cache was cleared
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, RelPath: "comics/Watchmen",
			Comic: &media.ComicMeta{PageCount: 1, Pages: []string{"1.png"}}},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{}, fstest.MapFS{
		"v1.mp4": {Data: []byte("0123456789 an mp4")},
		"v2.mp4": {Data: []byte("an old preview")},
	}, fstest.MapFS{}, nil, a)
	bearer := "Bearer " + token

	rec := get(t, h, "/api/media/v1/preview?v=1", "Authorization", bearer)
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789 an mp4" {
		t.Fatalf("preview = %d %q", rec.Code, rec.Body.String())
	}
	if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "video/mp4" || cc != cacheForever {
		t.Errorf("Content-Type %q, Cache-Control %q", ct, cc)
	}
	if cc := get(t, h, "/api/media/v1/preview", "Authorization", bearer).Header().Get("Cache-Control"); cc != "private, no-cache" {
		t.Errorf("unversioned Cache-Control = %q", cc)
	}
	if rec := get(t, h, "/api/media/v1/preview", "Authorization", bearer, "Range", "bytes=0-3"); rec.Code != http.StatusPartialContent || rec.Body.String() != "0123" {
		t.Errorf("range = %d %q, want 206 \"0123\"", rec.Code, rec.Body.String())
	}
	for _, id := range []string{"v2", "v3", "c1", "nope"} {
		if rec := get(t, h, "/api/media/"+id+"/preview", "Authorization", bearer); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", id, rec.Code)
		}
	}
	if rec := get(t, h, "/api/media/v1/preview"); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a token: %d, want 401", rec.Code)
	}
}
