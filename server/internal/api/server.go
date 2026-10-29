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
// thumbs holds the extractor's video thumbnails, {id}.jpg.
func NewHandler(repo media.Repository, lib Rescanner, files, thumbs fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /health", HandlerE(health))
	mux.Handle("GET /api/media", listMedia(repo))
	mux.Handle("GET /api/media/{id}", getMedia(repo))
	mux.Handle("POST /api/media/{id}/actors", addActor(repo))
	mux.Handle("DELETE /api/media/{id}/actors/{name}", removeActor(repo))
	mux.Handle("GET /api/actors", listActors(repo))
	mux.Handle("PUT /api/media/{id}/progress", putProgress(repo))
	mux.Handle("GET /api/media/{id}/progress", getProgress(repo))
	mux.Handle("GET /api/continue", getContinue(repo))
	mux.Handle("GET /api/media/{id}/thumbnail", getThumbnail(repo, files, thumbs))
	mux.Handle("GET /api/media/{id}/stream", streamVideo(repo, files, http.ServeContent)) // GET patterns also match HEAD
	mux.Handle("GET /api/media/{id}/pages", listPages(repo))
	mux.Handle("GET /api/media/{id}/pages/{n}", getPage(repo, files))
	mux.Handle("POST /api/scan", scanLibrary(lib))

	// Outermost first: Recover catches panics from everything inside it.
	return Recover(RequestID(Logging(mux)))
}

func health(w http.ResponseWriter, r *http.Request) error {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
