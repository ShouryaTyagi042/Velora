package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	ts := func(d time.Duration) string { return time.Now().Add(-time.Hour + d).UTC().Format(time.RFC3339Nano) }
	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantBody                 string // a substring of the response
	}{
		{"get before any save", "GET", "/api/media/v1/progress", "", 404, `"not_found"`},
		{"save video", "PUT", "/api/media/v1/progress", `{"positionMs":60000,"durationMs":600000,"clientTs":"` + ts(2*time.Second) + `"}`, 200, `"positionMs":60000,"durationMs":600000,"completed":false`},
		{"same save again", "PUT", "/api/media/v1/progress", `{"positionMs":60000,"durationMs":600000,"clientTs":"` + ts(2*time.Second) + `"}`, 200, `"positionMs":60000`},
		{"stale save is ignored", "PUT", "/api/media/v1/progress", `{"positionMs":50000,"durationMs":600000,"clientTs":"` + ts(time.Second) + `"}`, 200, `"positionMs":60000`},
		{"get", "GET", "/api/media/v1/progress", "", 200, `"positionMs":60000`},
		{"in the media JSON", "GET", "/api/media/v1", "", 200, `"progress":{"positionMs":60000`},
		{"finished", "PUT", "/api/media/v2/progress", `{"positionMs":590000,"durationMs":600000,"clientTs":"` + ts(0) + `"}`, 200, `"completed":true`},
		{"save comic", "PUT", "/api/media/c1/progress", `{"page":1,"pageCount":2,"clientTs":"` + ts(0) + `"}`, 200, `"page":1,"pageCount":2,"completed":true`},
		{"client sends completed", "PUT", "/api/media/v1/progress", `{"positionMs":1,"durationMs":2,"completed":true,"clientTs":"` + ts(3*time.Second) + `"}`, 400, `unknown field`},
		{"page on a video", "PUT", "/api/media/v1/progress", `{"page":1,"pageCount":2,"clientTs":"` + ts(3*time.Second) + `"}`, 400, `not pages`},
		{"page out of range", "PUT", "/api/media/c1/progress", `{"page":2,"pageCount":2,"clientTs":"` + ts(3*time.Second) + `"}`, 400, `page must be`},
		{"missing clientTs", "PUT", "/api/media/c1/progress", `{"page":0,"pageCount":2}`, 400, `clientTs is required`},
		{"unknown media", "PUT", "/api/media/nope/progress", `{"page":0,"pageCount":2,"clientTs":"` + ts(0) + `"}`, 404, `"not_found"`},
	}

	h := newTestHandler() // shared: the cases run in order against one store
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("got %d %s, want %d containing %s", rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
		})
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/continue", nil))
	var body struct {
		Watching []struct{ ID string } `json:"watching"`
		Reading  []struct{ ID string } `json:"reading"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// v2 is finished and c1 is on its last page: neither is "continue".
	if len(body.Watching) != 1 || body.Watching[0].ID != "v1" || body.Reading == nil || len(body.Reading) != 0 {
		t.Errorf("continue = %s", rec.Body)
	}
}
