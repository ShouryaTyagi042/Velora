package api

import (
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Remover deletes an item's files for good: the video file or comic folder in the library, and
// everything the server made from it (thumbnail, preview, Shorts clip). A file already gone is not
// an error: the goal is that it isn't there.
type Remover interface {
	Remove(m media.Media) error
}

// moveToBin handles POST /api/media/{id}/bin: the item disappears from every list but the bin.
// Its files are untouched. Binning what's already binned is a 200 too.
func moveToBin(repo media.Repository) HandlerE {
	return setInBin(repo, true)
}

// restoreFromBin handles DELETE /api/media/{id}/bin: back where it was, as it was.
func restoreFromBin(repo media.Repository) HandlerE {
	return setInBin(repo, false)
}

func setInBin(repo media.Repository, inBin bool) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		m, err := repo.SetInBin(r.Context(), currentUser(r).ID, r.PathValue("id"), inBin)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

// deleteMedia handles DELETE /api/media/{id}: deletes an item in the bin for good, files first.
// 409 for an item not in the bin: deleting is a two-step act on purpose.
func deleteMedia(repo media.Repository, rm Remover) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		m, err := repo.Get(r.Context(), currentUser(r).ID, r.PathValue("id"))
		if err != nil {
			return err
		}
		if err := deleteForGood(r, repo, rm, m); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

func deleteForGood(r *http.Request, repo media.Repository, rm Remover, m media.Media) error {
	if !m.InBin {
		return media.ErrNotInBin
	}
	// Files first: if removing them fails, the item stays in the bin and can be tried again. The
	// other order could leave files on disk that nothing knows about (until a scan adds them back).
	if rm != nil {
		if err := rm.Remove(m); err != nil {
			return err
		}
	}
	return repo.Delete(r.Context(), m.ID)
}

type emptyBinResponse struct {
	Deleted int `json:"deleted"`
	Failed  int `json:"failed"` // left in the bin; the server's log says why
}

// emptyBin handles DELETE /api/bin?kind=video|comic&section=movies|anime: deletes everything in
// the bin for good, or only what's of that kind or section. One item failing (a file the server
// may not remove) doesn't stop the rest; it stays in the bin and is counted.
func emptyBin(repo media.Repository, rm Remover) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		user := currentUser(r).ID
		kind := media.Kind(r.URL.Query().Get("kind"))
		if kind != "" && !kind.Valid() {
			return validationError("kind must be %q or %q", media.KindVideo, media.KindComic)
		}
		section, err := parseSection(r.URL.Query().Get("section"))
		if err != nil {
			return err
		}
		var res emptyBinResponse
		failed := map[string]bool{}
		for {
			// Always the first page: what was deleted is gone, and failures are skipped below.
			items, err := repo.List(r.Context(), user, media.ListFilter{
				InBin: true, Kind: kind, Section: section, Sort: media.SortBinned, Limit: 200 + len(failed),
			})
			if err != nil {
				return err
			}
			progressed := false
			for _, m := range items {
				if failed[m.ID] {
					continue
				}
				progressed = true
				if err := deleteForGood(r, repo, rm, m); err != nil {
					log.Printf("empty bin: %s (%s): %v", m.ID, m.Title, err)
					failed[m.ID] = true
					res.Failed++
					continue
				}
				res.Deleted++
			}
			if !progressed {
				break
			}
		}
		respondJSON(w, http.StatusOK, res)
		return nil
	}
}

// addFavourite and removeFavourite handle PUT and DELETE /api/media/{id}/favourite, for the
// signed-in user only. Both answer with the item, and both are idempotent.
func addFavourite(repo media.Repository) HandlerE    { return setFavourite(repo, true) }
func removeFavourite(repo media.Repository) HandlerE { return setFavourite(repo, false) }

func setFavourite(repo media.Repository, on bool) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		m, err := repo.SetFavourite(r.Context(), currentUser(r).ID, r.PathValue("id"), on)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

// maxTag bounds a tag, in characters.
const maxTag = 40

type tagRequest struct {
	Name string `json:"name"`
}

// addTag handles POST /api/media/{id}/tags {"name": "..."}: tags a video (stored lowercase,
// spaces collapsed) and answers with it. A tag it already has is a 200 no-op.
func addTag(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[tagRequest](w, r)
		if err != nil {
			return err
		}
		name, err := parseTag(req.Name)
		if err != nil {
			return err
		}
		m, err := repo.AddTag(r.Context(), currentUser(r).ID, r.PathValue("id"), name)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

// removeTag handles DELETE /api/media/{id}/tags/{name}, answering with the video.
func removeTag(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		m, err := repo.RemoveTag(r.Context(), currentUser(r).ID, r.PathValue("id"), media.NormalizeTag(r.PathValue("name")))
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, m)
		return nil
	}
}

func parseTag(s string) (string, error) {
	name := media.NormalizeTag(s)
	switch {
	case name == "":
		return "", validationError("name must be non-empty")
	case utf8.RuneCountInString(name) > maxTag:
		return "", validationError("name must be at most %d characters", maxTag)
	case strings.ContainsAny(name, "/#?"):
		return "", validationError("name can't contain / # or ?")
	}
	return name, nil
}

type tagsResponse struct {
	Items []media.TagCount `json:"items"`
}

// listTags handles GET /api/tags?section=movies|anime: every tag in use, with how many videos
// have it, counting only that section's videos when one is given.
func listTags(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		section, err := parseSection(r.URL.Query().Get("section"))
		if err != nil {
			return err
		}
		items, err := repo.ListTags(r.Context(), section)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, tagsResponse{Items: items})
		return nil
	}
}

type viewResponse struct {
	Views   int  `json:"views"`
	Counted bool `json:"counted"` // false: the same person's view of it within the last half hour
}

// recordView handles POST /api/media/{id}/views: the app watched a video past its first 30 s (or
// to the end of a shorter one) or read a comic to its last page. A repeat within half an hour by
// the same person is the same view: a 200 with counted false.
func recordView(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		views, counted, err := repo.RecordView(r.Context(), currentUser(r).ID, r.PathValue("id"), time.Now())
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, viewResponse{Views: views, Counted: counted})
		return nil
	}
}
