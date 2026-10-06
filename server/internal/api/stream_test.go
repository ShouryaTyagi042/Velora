package api

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

const videoSize = 1000

var videoModTime = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// videoBytes is 1000 bytes where byte i is i%256, so any range's content is predictable.
func videoBytes() []byte {
	b := make([]byte, videoSize)
	for i := range b {
		b[i] = byte(i % 256)
	}
	return b
}

// impls are the two ways to serve the bytes. Every test below runs against both.
var impls = []struct {
	name  string
	serve contentServer
}{
	{"ServeContent", http.ServeContent},
	{"hand-written", serveRange},
}

func newStreamHandler(serve contentServer) http.Handler {
	repo := memstore.New([]media.Media{
		{ID: "v1", Title: "Clip", Kind: media.KindVideo, RelPath: "movies/A/clip.mkv", MimeType: "video/x-matroska"},
		{ID: "gone", Title: "Gone", Kind: media.KindVideo, RelPath: "movies/A/deleted.mp4", MimeType: "video/mp4"},
		{ID: "c1", Title: "Comic", Kind: media.KindComic, RelPath: "comics/W"},
	})
	files := fstest.MapFS{
		"movies/A/clip.mkv": {Data: videoBytes(), ModTime: videoModTime},
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/media/{id}/stream", streamVideo(repo, files, serve))
	return mux
}

func stream(t *testing.T, serve contentServer, method string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/media/v1/stream", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newStreamHandler(serve).ServeHTTP(rec, req)
	return rec
}

type rangeWant struct {
	status       int
	contentRange string
	body         []byte // nil: don't check the body
}

func TestStreamRanges(t *testing.T) {
	all := videoBytes()
	tests := []struct {
		name     string
		rangeHdr string
		want     rangeWant
		mine     *rangeWant // set where the hand-written version behaves differently
	}{
		{"no range: whole file", "", rangeWant{200, "", all}, nil},
		{"first 100 bytes", "bytes=0-99", rangeWant{206, "bytes 0-99/1000", all[0:100]}, nil},
		{"middle", "bytes=500-599", rangeWant{206, "bytes 500-599/1000", all[500:600]}, nil},
		{"open-ended: from 900 to the end", "bytes=900-", rangeWant{206, "bytes 900-999/1000", all[900:]}, nil},
		{"suffix: last 100 bytes", "bytes=-100", rangeWant{206, "bytes 900-999/1000", all[900:]}, nil},
		{"suffix longer than the file", "bytes=-5000", rangeWant{206, "bytes 0-999/1000", all}, nil},
		{"end past the file is clamped", "bytes=990-5000", rangeWant{206, "bytes 990-999/1000", all[990:]}, nil},
		{"start past the end: unsatisfiable", "bytes=5000-", rangeWant{416, "bytes */1000", nil}, nil},
		// RFC 9110 says a server SHOULD ignore a Range it can't use and send the whole file.
		// The hand-written version does; Go's ServeContent answers 416. Players only send
		// valid bytes= ranges, so the difference never shows up in practice.
		{"unknown range unit", "items=0-10",
			rangeWant{416, "", nil}, &rangeWant{200, "", all}},
		{"garbage byte range", "bytes=abc",
			rangeWant{416, "", nil}, &rangeWant{200, "", all}},
		// Two ranges: ServeContent sends both as a multipart/byteranges body (no
		// Content-Range header; each part has its own). The hand-written one serves the first.
		{"two ranges", "bytes=0-9,20-29",
			rangeWant{206, "", nil}, &rangeWant{206, "bytes 0-9/1000", all[0:10]}},
	}
	for _, impl := range impls {
		for _, tt := range tests {
			t.Run(impl.name+"/"+tt.name, func(t *testing.T) {
				want := tt.want
				if impl.name == "hand-written" && tt.mine != nil {
					want = *tt.mine
				}
				hdr := map[string]string{}
				if tt.rangeHdr != "" {
					hdr["Range"] = tt.rangeHdr
				}
				rec := stream(t, impl.serve, http.MethodGet, hdr)

				if rec.Code != want.status {
					t.Fatalf("status = %d, want %d; body %q", rec.Code, want.status, rec.Body.String())
				}
				if got := rec.Header().Get("Content-Range"); got != want.contentRange {
					t.Errorf("Content-Range = %q, want %q", got, want.contentRange)
				}
				if want.body != nil {
					if !bytes.Equal(rec.Body.Bytes(), want.body) {
						t.Errorf("body: got %d bytes, want %d bytes of the right range", rec.Body.Len(), len(want.body))
					}
					if got, w := rec.Header().Get("Content-Length"), fmt.Sprint(len(want.body)); got != w {
						t.Errorf("Content-Length = %s, want %s", got, w)
					}
				}
				if rec.Code != 416 && rec.Header().Get("Accept-Ranges") != "bytes" {
					t.Error("missing Accept-Ranges: bytes (players check it before seeking)")
				}
			})
		}
	}
}

func TestStreamHeaders(t *testing.T) {
	for _, impl := range impls {
		rec := stream(t, impl.serve, http.MethodGet, nil)
		if ct := rec.Header().Get("Content-Type"); ct != "video/x-matroska" {
			t.Errorf("%s: Content-Type = %q, want video/x-matroska from Media.MimeType (sniffing would get .mkv wrong)", impl.name, ct)
		}
		if lm := rec.Header().Get("Last-Modified"); lm != videoModTime.Format(http.TimeFormat) {
			t.Errorf("%s: Last-Modified = %q", impl.name, lm)
		}
	}
}

func TestStreamHEAD(t *testing.T) {
	for _, impl := range impls {
		rec := stream(t, impl.serve, http.MethodHead, nil)
		if rec.Code != 200 || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") != "1000" {
			t.Errorf("%s HEAD: status %d, body %d bytes, Content-Length %q; want 200, empty, 1000",
				impl.name, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Length"))
		}
	}
}

// Conditional requests are where the hand-written version falls short: it ignores both
// headers. These cases pin down that gap rather than hide it.
func TestStreamConditional(t *testing.T) {
	current := videoModTime.Format(http.TimeFormat)
	stale := videoModTime.Add(-time.Hour).Format(http.TimeFormat)
	tests := []struct {
		name              string
		headers           map[string]string
		wantStd, wantMine int
		bodyStd, bodyMine int
	}{
		// The player already has the file from this modification time: nothing to send.
		// Mine sends the whole file again.
		{"If-Modified-Since current", map[string]string{"If-Modified-Since": current},
			304, 200, 0, videoSize},
		// The file changed since the player cached its first part, so a range would splice
		// old and new bytes together. ServeContent sends the whole new file; mine sends the
		// range anyway, which is a real bug if a file is replaced while someone watches.
		{"If-Range stale", map[string]string{"Range": "bytes=0-99", "If-Range": stale},
			200, 206, videoSize, 100},
	}
	for _, tt := range tests {
		for _, impl := range impls {
			wantStatus, wantBody := tt.wantStd, tt.bodyStd
			if impl.name == "hand-written" {
				wantStatus, wantBody = tt.wantMine, tt.bodyMine
			}
			rec := stream(t, impl.serve, http.MethodGet, tt.headers)
			if rec.Code != wantStatus || rec.Body.Len() != wantBody {
				t.Errorf("%s/%s: got %d with %d bytes, want %d with %d",
					impl.name, tt.name, rec.Code, rec.Body.Len(), wantStatus, wantBody)
			}
		}
	}
}

func TestParseRange(t *testing.T) {
	const size = 1000
	tests := []struct {
		header     string
		start, end int64
		wantErr    error
	}{
		{"bytes=0-99", 0, 99, nil},
		{"bytes=900-", 900, 999, nil},
		{"bytes=-100", 900, 999, nil},
		{"bytes=-5000", 0, 999, nil},
		{"bytes=990-5000", 990, 999, nil},
		{"bytes=999-999", 999, 999, nil},
		{"bytes= 0-9 , 20-29", 0, 9, nil}, // first of several; spaces allowed around commas
		{"bytes=1000-", 0, 0, errUnsatisfiable},
		{"bytes=-0", 0, 0, errUnsatisfiable},
		{"items=0-10", 0, 0, errMalformed},
		{"bytes=abc", 0, 0, errMalformed},
		{"bytes=-", 0, 0, errMalformed},
		{"bytes=500-100", 0, 0, errMalformed},
		{"bytes=+5-10", 0, 0, errMalformed},
		{"bytes=1--5", 0, 0, errMalformed},
		{"bytes=99999999999999999999-", 0, 0, errMalformed}, // overflows int64
	}
	for _, tt := range tests {
		start, end, err := parseRange(tt.header, size)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("parseRange(%q) error = %v, want %v", tt.header, err, tt.wantErr)
			continue
		}
		if err == nil && (start != tt.start || end != tt.end) {
			t.Errorf("parseRange(%q) = %d-%d, want %d-%d", tt.header, start, end, tt.start, tt.end)
		}
	}

	// An empty file has no byte to start at, so every range is unsatisfiable.
	if _, _, err := parseRange("bytes=0-", 0); !errors.Is(err, errUnsatisfiable) {
		t.Errorf("parseRange on an empty file: error = %v, want errUnsatisfiable", err)
	}
}

func TestStreamErrors(t *testing.T) {
	tests := []struct {
		path       string
		wantStatus int
		wantMsg    string
	}{
		{"/api/media/nope/stream", 404, "media nope not found"},
		{"/api/media/c1/stream", 404, "media c1 is not a video"},
		{"/api/media/gone/stream", 404, "media gone is no longer available"},
	}
	h := newStreamHandler(http.ServeContent)
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantMsg) {
			t.Errorf("%s: got %d %s, want %d %q", tt.path, rec.Code, rec.Body, tt.wantStatus, tt.wantMsg)
		}
		if strings.Contains(rec.Body.String(), "movies/") {
			t.Errorf("%s leaks a path: %s", tt.path, rec.Body)
		}
	}
}
