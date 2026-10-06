// Package api is Velora's HTTP layer.
package api

import (
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// NewHandler returns the HTTP handler for the whole API: routes wrapped in middleware.
// It depends on interfaces (media.Repository, Rescanner), never on a concrete store or scanner.
func NewHandler(repo media.Repository, lib Rescanner) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /health", HandlerE(health))
	mux.Handle("GET /api/media", listMedia(repo))
	mux.Handle("GET /api/media/{id}", getMedia(repo))
	mux.Handle("GET /api/media/{id}/thumbnail", HandlerE(getThumbnail))
	mux.Handle("POST /api/scan", scanLibrary(lib))

	// Outermost first: Recover catches panics from everything inside it.
	return Recover(RequestID(Logging(mux)))
}

func health(w http.ResponseWriter, r *http.Request) error {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
