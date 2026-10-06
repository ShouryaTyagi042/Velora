package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// contentServer is the signature of http.ServeContent.
type contentServer func(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, content io.ReadSeeker)

// streamVideo handles GET (and HEAD) /api/media/{id}/stream.
//
// http.ServeContent does the HTTP work: Range and 206 Partial Content, 416 for an
// unsatisfiable range, multi-range responses, HEAD, and conditional requests
// (If-Modified-Since, If-Range). It reads only the requested bytes, so memory stays
// constant however large the file is. What stays ours: finding the file by id,
// opening it safely, and setting Content-Type.
//
// serve is http.ServeContent in the route. Tests also pass serveRange, the hand-written
// version, to compare the two on the same requests.
func streamVideo(repo media.Repository, files fs.FS, serve contentServer) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		m, err := repo.Get(r.Context(), id)
		if errors.Is(err, media.ErrNotFound) {
			return notFoundError("media %s not found", id)
		}
		if err != nil {
			return err
		}
		if m.Kind != media.KindVideo {
			return notFoundError("media %s is not a video", id)
		}

		// files is rooted at the library folder (an os.Root in main), so even a bad
		// RelPath can't open anything outside it.
		f, err := files.Open(m.RelPath)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("stream: %s (%s) is gone since the last scan; rescan", id, m.RelPath)
			return notFoundError("media %s is no longer available", id)
		}
		if err != nil {
			return fmt.Errorf("open %s: %w", m.RelPath, err)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("stat %s: %w", m.RelPath, err)
		}
		content, ok := f.(io.ReadSeeker)
		if !ok {
			return fmt.Errorf("open %s: file does not support seeking", m.RelPath)
		}

		// Set Content-Type ourselves: ServeContent would otherwise sniff the first bytes,
		// and sniffing gets .mkv and .mov wrong.
		w.Header().Set("Content-Type", m.MimeType)
		serve(w, r, path.Base(m.RelPath), info.ModTime(), content)
		return nil
	}
}
