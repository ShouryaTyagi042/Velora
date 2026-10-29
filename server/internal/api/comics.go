package api

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"

	"github.com/ShouryaTyagi042/Velora/server/internal/comics"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

type pagesResponse struct {
	PageCount int `json:"pageCount"`
}

// getComic returns the comic with the id in the path, or a 404 for anything else.
func getComic(repo media.Repository, r *http.Request) (media.Media, error) {
	id := r.PathValue("id")
	m, err := repo.Get(r.Context(), id)
	if errors.Is(err, media.ErrNotFound) {
		return m, notFoundError("media %s not found", id)
	}
	if err != nil {
		return m, err
	}
	if m.Kind != media.KindComic || m.Comic == nil {
		return m, notFoundError("media %s is not a comic", id)
	}
	return m, nil
}

// listPages handles GET /api/media/{id}/pages.
func listPages(repo media.Repository) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		m, err := getComic(repo, r)
		if err != nil {
			return err
		}
		respondJSON(w, http.StatusOK, pagesResponse{PageCount: m.Comic.PageCount})
		return nil
	}
}

// getPage handles GET /api/media/{id}/pages/{n}: page n (0-based) of a comic, as the image file.
//
// The client names a page by number, never by filename: the filename comes from the scan, which
// only ever recorded names it found inside that comic's folder. And files is rooted at the
// library (an os.Root in main), so even a bad name can't open anything outside it.
func getPage(repo media.Repository, files fs.FS) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		n, err := strconv.Atoi(r.PathValue("n"))
		if err != nil {
			return validationError("page must be a number")
		}
		m, err := getComic(repo, r)
		if err != nil {
			return err
		}
		if n < 0 || n >= len(m.Comic.Pages) {
			return notFoundError("page %d not found: the comic has %d pages", n, len(m.Comic.Pages))
		}
		return servePage(w, r, files, m, n)
	}
}

// servePage serves page n of comic m with http.ServeContent: Last-Modified and 304s, ranges,
// HEAD, constant memory. Also the comic's cover (phase 9's thumbnail endpoint).
func servePage(w http.ResponseWriter, r *http.Request, files fs.FS, m media.Media, n int) error {
	name := m.Comic.Pages[n]
	p := path.Join(m.RelPath, name) // fs.FS paths are slash-separated: path, not filepath
	f, err := files.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		log.Printf("page: %s (%s) is gone since the last scan; rescan", m.ID, p)
		return notFoundError("page %d is no longer available", n)
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", p, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", p, err)
	}
	content, ok := f.(io.ReadSeeker)
	if !ok {
		return fmt.Errorf("open %s: file does not support seeking", p)
	}

	// immutable only on a versioned URL: ?v= is the comic's modTime, which a rescan changes when
	// any page does, so a changed page gets a new URL. A bare URL must revalidate (Last-Modified).
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.Header().Set("Content-Type", comics.ContentType(name))
	http.ServeContent(w, r, name, info.ModTime(), content)
	return nil
}
