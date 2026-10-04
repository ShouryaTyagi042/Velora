// Package api is Velora's HTTP layer.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// NewHandler returns the HTTP handler for the whole API.
// It depends on the media.Repository interface, never on a concrete store.
func NewHandler(repo media.Repository) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/media", func(w http.ResponseWriter, r *http.Request) {
		items, err := repo.List(r.Context())
		if err != nil {
			log.Printf("list media: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)
	})

	mux.HandleFunc("GET /api/media/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			http.Error(w, "media not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("get media %q: %v", id, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, m)
	})

	return mux
}

// writeJSON sends v as a 200 JSON response.
// Content-Type must be set before the first write, because writing sends the headers.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}
