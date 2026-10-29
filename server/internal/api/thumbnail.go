package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// getThumbnail handles GET /api/media/{id}/thumbnail.
//
// A video's thumbnail is a frame the extractor wrote to thumbs/{id}.jpg; until it has run, the
// answer is 202 Accepted: the work is queued, ask again. A comic needs no job at all: its cover
// is its first page, which exists the moment the scan does.
func getThumbnail(repo media.Repository, files, thumbs fs.FS) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}

		if m.Kind == media.KindComic {
			return servePage(w, r, files, m, 0)
		}

		f, err := thumbs.Open(m.ID + ".jpg")
		if errors.Is(err, fs.ErrNotExist) {
			// Not cached: a pending thumbnail must not be remembered as "no thumbnail".
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Retry-After", "5")
			respondJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
			return nil
		}
		if err != nil {
			return fmt.Errorf("open thumbnail %s: %w", m.ID, err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		content, ok := f.(io.ReadSeeker)
		if !ok {
			return fmt.Errorf("thumbnail %s: file does not support seeking", m.ID)
		}
		// A day, then revalidate with If-Modified-Since: a re-extracted thumbnail keeps its URL.
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeContent(w, r, m.ID+".jpg", info.ModTime(), content)
		return nil
	}
}
