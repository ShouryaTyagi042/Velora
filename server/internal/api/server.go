// Package api is Velora's HTTP layer.
package api

import (
	"io/fs"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// NewHandler returns the HTTP handler for the whole API: routes wrapped in middleware.
// It depends on interfaces (media.Repository, Jobs, fs.FS, Authenticator), never on a
// concrete store, scanner or directory. files is the media library; media RelPaths are opened
// inside it. thumbs holds the extractor's video thumbnails, {id}.jpg.
func NewHandler(repo media.Repository, js Jobs, files, thumbs fs.FS, authn Authenticator) http.Handler {
	// Everything on this mux needs a session: it's mounted below behind requireUser, so a new
	// route is protected by default and can't be forgotten.
	mux := http.NewServeMux()
	mux.Handle("GET /api/auth/me", HandlerE(me))
	mux.Handle("POST /api/auth/logout", logout(authn))
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
	mux.Handle("POST /api/scan", scanLibrary(js))
	mux.Handle("GET /api/jobs", listJobs(js))
	mux.Handle("GET /api/jobs/{id}", getJob(js))

	// The only routes open without a session: the health check, and signing in.
	// "POST /api/auth/login" is more specific than "/api/", so the mux picks it first.
	public := http.NewServeMux()
	public.Handle("GET /health", HandlerE(health))
	public.Handle("POST /api/auth/login", login(authn, newLoginLimiter()))
	public.Handle("/api/", requireUser(authn, mux))

	// Outermost first: Recover catches panics from everything inside it, auth included.
	return Recover(RequestID(Logging(public)))
}

func health(w http.ResponseWriter, r *http.Request) error {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
