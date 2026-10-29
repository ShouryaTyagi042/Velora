// Package comics decides what a comic's pages are and in which order. It is the one owner of
// "what is a page": the scanner calls it once per comic folder, at scan time.
package comics

import (
	"io/fs"
	"path"
	"slices"
	"time"
)

// Listing is one comic folder's pages, in reading order.
type Listing struct {
	Pages    []string  // filenames directly in the folder; page n is Pages[n]
	Size     int64     // sum of the pages' sizes
	ModTime  time.Time // the newest page's: a folder's own mtime misses a rewritten page
	Warnings []string  // problems that didn't stop the listing
}

// ListPages lists the pages of the comic folder dir in fsys: regular image files directly
// inside it, naturally sorted. Subfolders and unreadable pages become warnings; other files
// (notes.txt, .DS_Store) are ignored silently. Only an unreadable dir is an error.
func ListPages(fsys fs.FS, dir string) (Listing, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return Listing{}, err
	}

	var l Listing
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir() && name[0] != '.':
			l.warn(dir, name, "subfolder inside a comic, ignored")
			continue
		case !IsPage(name) || !e.Type().IsRegular():
			continue // notes.txt, ._001.jpg, a symlinked page …: not a page
		}
		info, err := e.Info()
		if err != nil {
			l.warn(dir, name, err.Error())
			continue
		}
		l.Pages = append(l.Pages, name)
		l.Size += info.Size()
		if info.ModTime().After(l.ModTime) {
			l.ModTime = info.ModTime()
		}
	}
	// ReadDir's order is lexical (page-10 before page-2); reading order is natural.
	slices.SortFunc(l.Pages, NaturalCompare)
	return l, nil
}

func (l *Listing) warn(dir, name, msg string) {
	l.Warnings = append(l.Warnings, path.Join(dir, name)+": "+msg)
}
