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
	return getClip(repo, previews, "preview", func(v *media.VideoMeta) bool { return v.Preview })
}

// getShort handles GET /api/media/{id}/short?v=<modTime>: the Shorts feed's clip, served exactly
// like the preview.
func getShort(repo media.Repository, shorts fs.FS) HandlerE {
	return getClip(repo, shorts, "short", func(v *media.VideoMeta) bool { return v.Short })
}

// getClip serves a clip from dir, {id}.mp4, when ready says the database has a current one.
func getClip(repo media.Repository, previews fs.FS, name string, ready func(*media.VideoMeta) bool) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), currentUser(r).ID, id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		if m.Kind != media.KindVideo || m.Video == nil || !ready(m.Video) {
			return notFoundError("media %s has no %s", id, name)
		}

		f, err := previews.Open(m.ID + ".mp4")
		if errors.Is(err, fs.ErrNotExist) {
			// The database says yes but the file is gone (the cache dir was cleared).
			return notFoundError("media %s has no %s", id, name)
		}
		if err != nil {
			return fmt.Errorf("open %s %s: %w", name, m.ID, err)
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		content, ok := f.(io.ReadSeeker)
		if !ok {
			return fmt.Errorf("%s %s: file does not support seeking", name, m.ID)
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
