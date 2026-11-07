// Package trash deletes an item's files when it is deleted from the bin: the media itself, inside
// the library, and what the server made from it, in its cache.
package trash

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Remover implements api.Remover.
type Remover struct {
	// Library is the media folder as an os.Root: a RelPath can never reach outside it (no "..",
	// no absolute paths, no symlinks out), so a bad row can't delete anything but library files.
	Library *os.Root
	// Derived are the cache folders holding {id}.jpg / {id}.mp4 files made from each video.
	Derived []string
}

// Remove deletes m's file (a video) or folder (a comic) from the library, then its derived files.
// Anything already gone counts as removed.
func (r Remover) Remove(m media.Media) error {
	if m.RelPath == "" || m.RelPath == "." {
		return fmt.Errorf("trash: %s has no path", m.ID)
	}
	var err error
	if m.Kind == media.KindComic {
		err = r.Library.RemoveAll(m.RelPath)
	} else {
		err = r.Library.Remove(m.RelPath)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("trash: remove %s: %w", m.RelPath, err)
	}
	for _, dir := range r.Derived {
		for _, ext := range []string{".jpg", ".mp4"} {
			if err := os.Remove(filepath.Join(dir, m.ID+ext)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("trash: remove %s%s from %s: %w", m.ID, ext, dir, err)
			}
		}
	}
	return nil
}
