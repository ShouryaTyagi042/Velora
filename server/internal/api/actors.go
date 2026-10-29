package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// maxActorName bounds a manual actor's name, in characters (runes), not bytes: a 40-character
// Japanese name is ~120 bytes.
const maxActorName = 100

type actorsResponse struct {
	Items []media.ActorCount `json:"items"`
}

// listActors handles GET /api/actors: every actor with a count of their available videos.
func listActors(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := repo.ListActors(r.Context())
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, actorsResponse{Items: items})
		return nil
	}
}

type addActorRequest struct {
	Name string `json:"name"`
}

// addActor handles POST /api/media/{id}/actors {"name": "..."}. It answers with the updated
// media, so the app needs no refetch. Adding an actor the video already has is a 200 too: the
// goal is already true, and a phone retrying after a timeout gets the same answer.
func addActor(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[addActorRequest](w, r)
		if err != nil {
			return err
		}
		// "  Keanu Reeves " and "Keanu Reeves" must be the same actor, or name unique lets in two.
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return validationError("name must be non-empty")
		}
		if utf8.RuneCountInString(name) > maxActorName {
			return validationError("name must be at most %d characters", maxActorName)
		}
		m, err := repo.AddActor(r.Context(), r.PathValue("id"), name)
		if err != nil {
			return err // ErrNotFound → 404, ErrNotVideo → 400 (writeError)
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

// removeActor handles DELETE /api/media/{id}/actors/{name}. PathValue is already URL-decoded:
// /actors/Carrie-Anne%20Moss → "Carrie-Anne Moss", and AC%2FDC stays one segment.
func removeActor(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := repo.RemoveActor(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
			return err // ErrNotFound → 404, ErrFolderActor → 409
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}
