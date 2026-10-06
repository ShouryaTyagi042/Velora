// Package api is Velora's HTTP layer.
package api

import (
	"io/fs"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// NewHandler returns the HTTP handler for the whole API: routes wrapped in middleware.
// It depends on interfaces (media.Repository, Rescanner, fs.FS), never on a concrete store,
// scanner or directory. files is the media library; media RelPaths are opened inside it.
func NewHandler(repo media.Repository, lib Rescanner, files fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /health", HandlerE(health))
	mux.Handle("GET /api/media", listMedia(repo))
	mux.Handle("GET /api/media/{id}", getMedia(repo))
	mux.Handle("GET /api/media/{id}/thumbnail", HandlerE(getThumbnail))
	mux.Handle("GET /api/media/{id}/stream", streamVideo(repo, files, http.ServeContent)) // GET patterns also match HEAD
	mux.Handle("POST /api/scan", scanLibrary(lib))

	// Outermost first: Recover catches panics from everything inside it.
	return Recover(RequestID(Logging(mux)))
}

func health(w http.ResponseWriter, r *http.Request) error {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
