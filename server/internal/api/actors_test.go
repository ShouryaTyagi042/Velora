package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActorEndpoints(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantCode                 string // error envelope code; "" for success
	}{
		{"add", "POST", "/api/media/v1/actors", `{"name":"  Carrie-Anne Moss "}`, 200, ""},
		{"add again is a no-op", "POST", "/api/media/v1/actors", `{"name":"Carrie-Anne Moss"}`, 200, ""},
		{"add empty name", "POST", "/api/media/v1/actors", `{"name":"   "}`, 400, "validation"},
		{"add 101 runes", "POST", "/api/media/v1/actors", `{"name":"` + strings.Repeat("é", 101) + `"}`, 400, "validation"},
		{"add 100 runes", "POST", "/api/media/v1/actors", `{"name":"` + strings.Repeat("é", 100) + `"}`, 200, ""},
		{"add unknown field", "POST", "/api/media/v1/actors", `{"nmae":"Keanu"}`, 400, "validation"},
		{"add not json", "POST", "/api/media/v1/actors", `name=Keanu`, 400, "validation"},
		{"add empty body", "POST", "/api/media/v1/actors", ``, 400, "validation"},
		{"add two values", "POST", "/api/media/v1/actors", `{"name":"a"}{"name":"b"}`, 400, "validation"},
		{"add to comic", "POST", "/api/media/c1/actors", `{"name":"Alan Moore"}`, 400, "validation"},
		{"add to unknown", "POST", "/api/media/nope/actors", `{"name":"Keanu"}`, 404, "not_found"},
		{"remove manual", "DELETE", "/api/media/v1/actors/Carrie-Anne%20Moss", ``, 204, ""},
		{"remove it again", "DELETE", "/api/media/v1/actors/Carrie-Anne%20Moss", ``, 404, "not_found"},
		{"remove folder actor", "DELETE", "/api/media/v1/actors/Keanu%20Reeves", ``, 409, "conflict"},
		{"remove from unknown", "DELETE", "/api/media/nope/actors/Keanu%20Reeves", ``, 404, "not_found"},
	}

	h := newTestHandler() // shared: the cases run in order against one store
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantCode == "" {
				return
			}
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != tt.wantCode {
				t.Errorf("body = %s, want error code %q", rec.Body, tt.wantCode)
			}
		})
	}
}

func TestAddActorReturnsTheVideo(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/media/v1/actors", strings.NewReader(`{"name":"Carrie-Anne Moss"}`))
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)

	want := `"actors":[{"name":"Keanu Reeves","source":"folder"},{"name":"Carrie-Anne Moss","source":"manual"}]`
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
		t.Errorf("got %d %s, want the video with %s", rec.Code, rec.Body, want)
	}
}

func TestListActors(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/actors", nil))
	want := `{"items":[{"name":"Keanu Reeves","videoCount":1},{"name":"Matthew McConaughey","videoCount":1}]}`
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("got %d %s, want %s", rec.Code, rec.Body, want)
	}
}
