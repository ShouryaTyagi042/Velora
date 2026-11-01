package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// getPreview handles GET /api/media/{id}/preview?v=<modTime>: the video's long-press preview, a
// short silent montage the extractor made (phase 14, ADR 011).
//
// 404 unless the database says the preview matches the file as it is now (Video.Preview). Not a
// 202 like a pending thumbnail: a long-press can't wait, so the app simply doesn't preview. And
// checking the database, not just the file, means a stale preview of a replaced file is never
// served under the new file's ?v=, where it would then be cached forever.
func getPreview(repo media.Repository, previews fs.FS) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), currentUser(r).ID, id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		if m.Kind != media.KindVideo || m.Video == nil || !m.Video.Preview {
			return notFoundError("media %s has no preview", id)
		}

		f, err := previews.Open(m.ID + ".mp4")
		if errors.Is(err, fs.ErrNotExist) {
			// The database says yes but the file is gone (the cache dir was cleared).
			return notFoundError("media %s has no preview", id)
		}
		if err != nil {
			return fmt.Errorf("open preview %s: %w", m.ID, err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		content, ok := f.(io.ReadSeeker)
		if !ok {
			return fmt.Errorf("preview %s: file does not support seeking", m.ID)
		}
		// Versioned by the video's modTime, like the thumbnail: a new file is a new URL.
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", cacheForever)
		} else {
			w.Header().Set("Cache-Control", "private, no-cache")
		}
		w.Header().Set("Content-Type", "video/mp4")
		// ServeContent answers Range requests (players probe with them) and If-Modified-Since.
		http.ServeContent(w, r, m.ID+".mp4", info.ModTime(), content)
		return nil
	}
}
