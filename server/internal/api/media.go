package api

import (
	"errors"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// listResponse wraps the list in an object so it can grow fields (e.g. pagination) later.
type listResponse struct {
	Items []media.Media `json:"items"`
}

// listMedia handles GET /api/media?kind=video|comic.
func listMedia(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		kind := media.Kind(r.URL.Query().Get("kind"))
		if kind != "" && !kind.Valid() {
			return validationError("kind must be %q or %q", media.KindVideo, media.KindComic)
		}

		all, err := repo.List(r.Context())
		if err != nil {
			return err
		}

		items := make([]media.Media, 0, len(all))
		for _, m := range all {
			if kind == "" || m.Kind == kind {
				items = append(items, m)
			}
		}
		respondJSON(w, http.StatusOK, listResponse{Items: items})
		return nil
	}
}

// getMedia handles GET /api/media/{id}.
func getMedia(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

// getThumbnail handles GET /api/media/{id}/thumbnail. Phase 9 generates thumbnails.
func getThumbnail(w http.ResponseWriter, r *http.Request) error {
	return notImplementedError("thumbnails are not implemented yet")
}
