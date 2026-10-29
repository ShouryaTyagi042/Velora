package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/progress"
)

// continueLimit is how many items each Continue row shows.
const continueLimit = 10

// putProgress handles PUT /api/media/{id}/progress. The body is the whole current state
// ({positionMs, durationMs} or {page, pageCount}, plus clientTs), so sending it twice is
// harmless: PUT means "make it look like this". A save older than the stored one is accepted
// and ignored; the answer is always the progress as stored.
func putProgress(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		save, err := decode[progress.Save](w, r)
		if err != nil {
			return err
		}
		p, err := progress.Validate(m.Kind, save, time.Now())
		if errors.Is(err, progress.ErrInvalid) {
			return validationError("%s", strings.TrimPrefix(err.Error(), progress.ErrInvalid.Error()+": "))
		}
		if err != nil {
			return err
		}
		stored, err := repo.SaveProgress(r.Context(), id, p)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, stored)
		return nil
	}
}

// getProgress handles GET /api/media/{id}/progress: 404 when it was never started.
func getProgress(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		if m.Progress == nil {
			return notFoundError("media %s has no progress yet", id)
		}
		respondJSON(w, http.StatusOK, m.Progress)
		return nil
	}
}

type continueResponse struct {
	Watching []media.Media `json:"watching"`
	Reading  []media.Media `json:"reading"`
}

// getContinue handles GET /api/continue: unfinished videos and comics, most recent first.
func getContinue(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		watching, err := repo.Continue(r.Context(), media.KindVideo, continueLimit)
		if err != nil {
			return err
		}
		reading, err := repo.Continue(r.Context(), media.KindComic, continueLimit)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, continueResponse{Watching: watching, Reading: reading})
		return nil
	}
}
