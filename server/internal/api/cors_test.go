package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := CORS([]string{"http://localhost:8081"}, ok)

	tests := []struct {
		name, method, origin, preflight string
		wantStatus                      int
		wantAllow                       string
	}{
		{"allowed origin", "GET", "http://localhost:8081", "", 200, "http://localhost:8081"},
		{"other origin: served, but the browser won't let the page read it", "GET", "https://evil.example", "", 200, ""},
		{"no Origin (the phone app)", "GET", "", "", 200, ""},
		{"preflight from allowed origin", "OPTIONS", "http://localhost:8081", "POST", 204, "http://localhost:8081"},
		{"preflight from other origin", "OPTIONS", "https://evil.example", "POST", 204, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/media", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflight != "" {
				req.Header.Set("Access-Control-Request-Method", tt.preflight)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantAllow)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
				t.Error("never answer *")
			}
		})
	}

	t.Run("no origins configured: untouched", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/media", nil)
		req.Header.Set("Origin", "http://localhost:8081")
		rec := httptest.NewRecorder()
		CORS(nil, ok).ServeHTTP(rec, req)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Header().Get("Vary") != "" {
			t.Errorf("headers set with CORS off: %v", rec.Header())
		}
	})
}
