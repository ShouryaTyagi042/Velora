package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // keep request logs out of test output
	os.Exit(m.Run())
}

// newTestHandler is the API over a small library, with every request signed in as testUser
// (see signedInHandler). auth_test.go tests what happens without a session.
func newTestHandler() http.Handler {
	return signedInHandler(memstore.New([]media.Media{
		{ID: "v1", Title: "The Matrix", Kind: media.KindVideo, RelPath: "movies/secret-dir/The Matrix.mp4",
			Actors: []media.ActorTag{{Name: "Keanu Reeves", Source: media.ActorFromFolder}}},
		{ID: "v2", Title: "Interstellar", Kind: media.KindVideo, RelPath: "movies/secret-dir/Interstellar.mp4",
			Actors: []media.ActorTag{{Name: "Matthew McConaughey", Source: media.ActorFromFolder}}},
		{ID: "c1", Title: "Watchmen", Kind: media.KindComic, RelPath: "comics/secret-dir/Watchmen",
			Comic: &media.ComicMeta{PageCount: 2, Pages: []string{"secret-page-1.png", "secret-page-2.png"}}},
	}), newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{
		"v2.jpg": {Data: []byte("\xff\xd8 a jpeg")}, // Interstellar has a thumbnail; The Matrix is pending
	})
}

// fakeJobs stands in for the real library: a real registry and runner, and a scan that returns
// result or err. Wait waits for the scans it started.
type fakeJobs struct {
	*jobs.Registry
	runner *jobs.Runner
	result any
	err    error
}

func newFakeJobs(result any, err error) *fakeJobs {
	reg := jobs.NewRegistry()
	return &fakeJobs{Registry: reg, runner: jobs.NewRunner(context.Background(), reg), result: result, err: err}
}

func (f *fakeJobs) StartScan() jobs.Job {
	t := f.runner.Go("scan", func(ctx context.Context, t *jobs.Task) (any, error) { return f.result, f.err })
	j, _ := f.Get(t.ID())
	return j
}

func (f *fakeJobs) Wait() { f.runner.Wait() }

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
		{"list huge q", "/api/media?q=" + strings.Repeat("a", 201), 400, "validation"},
		{"list bad sort", "/api/media?sort=size", 400, "validation"},
		{"list limit 0", "/api/media?limit=0", 400, "validation"},
		{"list limit too big", "/api/media?limit=201", 400, "validation"},
		{"list limit not a number", "/api/media?limit=ten", 400, "validation"},
		{"list cursor not base64", "/api/media?after=%25%25", 400, "validation"},
		{"list cursor from the other sort", "/api/media?sort=recent&after=" + encodeCursor(media.SortTitle, "v1"), 400, "validation"},
		{"get existing", "/api/media/v1", 200, ""},
		{"get missing", "/api/media/nope", 404, "not_found"},
		{"thumbnail pending", "/api/media/v1/thumbnail", 202, ""},
		{"thumbnail unknown", "/api/media/nope/thumbnail", 404, "not_found"},
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
		{"?q=STELL", []string{"v2"}},              // search: title, any case
		{"?q=keanu", []string{"v1"}},              // search: actor
		{"?q=%20%20", []string{"v2", "v1", "c1"}}, // a blank search is no search
		{"?limit=2", []string{"v2", "v1"}},
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

// scanAndWait starts a scan through h, waits for it, and returns the job as GET /api/jobs/{id} reports it.
func scanAndWait(t *testing.T, h http.Handler, js *fakeJobs) jobs.Job {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scan", nil))
	var started jobs.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || rec.Code != http.StatusAccepted || started.ID == "" {
		t.Fatalf("POST /api/scan: %d %s, want 202 and a job", rec.Code, rec.Body)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/jobs/"+started.ID {
		t.Errorf("Location = %q", loc)
	}
	js.Wait()
	rec = get(t, h, "/api/jobs/"+started.ID)
	var j jobs.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil || rec.Code != 200 {
		t.Fatalf("GET /api/jobs/%s: %d %s", started.ID, rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a job's status must not be cached: the phone polls it")
	}
	return j
}

func TestScanEndpoint(t *testing.T) {
	js := newFakeJobs(map[string]int{"found": 3}, nil)
	h := signedInHandler(memstore.New(nil), js, fstest.MapFS{}, fstest.MapFS{})
	if j := scanAndWait(t, h, js); j.Status != jobs.Done || fmt.Sprint(j.Result) != "map[found:3]" {
		t.Errorf("scan job = %+v", j)
	}
	if rec := get(t, h, "/api/jobs"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"scan"`) {
		t.Errorf("GET /api/jobs: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "/api/jobs/j_nope"); rec.Code != 404 {
		t.Errorf("unknown job: %d, want 404", rec.Code)
	}

	// A failed scan says so, without the error's details: they name paths.
	js = newFakeJobs(nil, errors.New("open /Users/x/media: permission denied"))
	h = signedInHandler(memstore.New(nil), js, fstest.MapFS{}, fstest.MapFS{})
	if j := scanAndWait(t, h, js); j.Status != jobs.Failed || strings.Contains(j.Error, "/Users/") {
		t.Errorf("failed scan job = %+v, want failed and no path", j)
	}
}

// failingRepo returns an error that must never reach the client. Methods it doesn't
// override panic (nil embedded interface), which Recover turns into a 500.
type failingRepo struct{ media.Repository }

func (failingRepo) List(ctx context.Context, user int64, f media.ListFilter) ([]media.Media, error) {
	return nil, errors.New(`pq: relation "media" does not exist`)
}

func (failingRepo) Get(ctx context.Context, user int64, id string) (media.Media, error) {
	return media.Media{}, errors.New("disk on fire")
}

func TestInternalErrorsAreNotLeaked(t *testing.T) {
	rec := httptest.NewRecorder()
	signedInHandler(failingRepo{}, newFakeJobs(nil, nil), fstest.MapFS{}, fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media", nil))

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

func TestVideoThumbnail(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/v2/thumbnail", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("Cache-Control") != "private, max-age=86400" {
		t.Errorf("ready: got %d %v", rec.Code, rec.Header())
	}
	// Versioned, as the app asks for it: the URL changes with the video, so it's kept forever.
	rec = get(t, newTestHandler(), "/api/media/v2/thumbnail?v=2026-10-01T12:00:00Z")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != cacheForever {
		t.Errorf("versioned: got %d %v, want 200 and %q", rec.Code, rec.Header(), cacheForever)
	}

	rec = httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/media/v1/thumbnail", nil))
	if rec.Code != 202 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("pending: got %d %v, want 202 and no-store", rec.Code, rec.Header())
	}
}

// get is one signed-in GET through h, with optional request headers as name, value pairs.
func get(t *testing.T, h http.Handler, path string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListPaging(t *testing.T) {
	h := newTestHandler()
	var ids []string
	path := "/api/media?limit=1"
	for range 10 { // 3 items, one per page
		rec := get(t, h, path)
		var body listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		for _, m := range body.Items {
			ids = append(ids, m.ID)
		}
		if body.NextCursor == nil {
			break
		}
		path = "/api/media?limit=1&after=" + *body.NextCursor
	}
	if want := []string{"v2", "v1", "c1"}; !slices.Equal(ids, want) {
		t.Errorf("paged ids = %v, want %v", ids, want)
	}

	// A page that ends exactly at the last item says so: no cursor to an empty page.
	rec := get(t, h, "/api/media?limit=3")
	if !strings.Contains(rec.Body.String(), `"nextCursor":null`) {
		t.Errorf("full last page: %s, want nextCursor null", rec.Body)
	}
}

func TestListRevalidation(t *testing.T) {
	h := newTestHandler()
	for _, path := range []string{"/api/media?kind=video", "/api/continue"} {
		first := get(t, h, path)
		etag := first.Header().Get("ETag")
		if first.Code != 200 || etag == "" || first.Header().Get("Cache-Control") != cacheRevalidate {
			t.Fatalf("%s: %d, ETag %q, Cache-Control %q", path, first.Code, etag, first.Header().Get("Cache-Control"))
		}
		for _, inm := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
			rec := get(t, h, path, "If-None-Match", inm)
			if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 || rec.Header().Get("ETag") != etag {
				t.Errorf("%s If-None-Match %s: %d, %d bytes, ETag %q; want 304, empty, same ETag",
					path, inm, rec.Code, rec.Body.Len(), rec.Header().Get("ETag"))
			}
		}
		if rec := get(t, h, path, "If-None-Match", `"stale"`); rec.Code != 200 || rec.Body.String() != first.Body.String() {
			t.Errorf("%s with a stale ETag: %d, want 200 and the full body", path, rec.Code)
		}
	}

	// The tag follows the content: a different page has a different one.
	if get(t, h, "/api/media?kind=video").Header().Get("ETag") == get(t, h, "/api/media?kind=comic").Header().Get("ETag") {
		t.Error("two different lists share an ETag")
	}
}
