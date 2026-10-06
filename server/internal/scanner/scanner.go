// Package scanner turns a library folder into media records.
// It works on an fs.FS, so tests pass an in-memory fstest.MapFS and main passes os.DirFS.
package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Result is what one scan found. Warnings are problems that didn't stop the scan.
type Result struct {
	Items    []media.Media
	Warnings []string
}

// ctxCheckEvery is how many entries the walk visits between cancellation checks.
const ctxCheckEvery = 256

// Scan walks fsys and returns every video under movies/ and every comic under comics/.
// Only an unreadable root or a cancelled ctx fails the scan; any other problem
// becomes a warning, so one bad file can't hide the rest of the library.
func Scan(ctx context.Context, fsys fs.FS) (Result, error) {
	var res Result
	visited := 0

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == "." {
				return err
			}
			res.warn(p, err.Error())
			return nil // keep walking; a directory that couldn't be listed is skipped
		}
		if visited%ctxCheckEvery == 0 { // the first entry, then every 256th
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		visited++
		if p == "." {
			return nil
		}
		if hidden(d.Name()) {
			return skip(d) // .DS_Store, ._x.png, .git/ …: skipped silently everywhere
		}

		top, rest, _ := strings.Cut(p, "/")
		switch {
		case top == moviesDir && rest == "":
			return mustBeDir(&res, p, d)
		case top == comicsDir && rest == "":
			return mustBeDir(&res, p, d)
		case top == moviesDir:
			scanVideo(&res, p, d)
			return nil
		case top == comicsDir && !strings.Contains(rest, "/"):
			if d.IsDir() {
				scanComic(&res, fsys, p)
				return fs.SkipDir // the comic was handled as a unit; don't descend into it
			}
			return nil // loose file directly in comics/: ignored
		default:
			res.warn(p, "not under movies/ or comics/, ignored")
			return skip(d)
		}
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// scanVideo records one file under movies/, if it's a video.
func scanVideo(res *Result, p string, d fs.DirEntry) {
	if d.IsDir() {
		return
	}
	mimeType, ok := videoTypes[ext(d.Name())]
	if !ok {
		return // poster.jpg, notes.txt …: not a video, ignored silently
	}
	if !d.Type().IsRegular() {
		res.warn(p, "not a regular file (symlinks are not followed), ignored")
		return
	}
	info, err := d.Info()
	if err != nil {
		res.warn(p, err.Error())
		return
	}

	m := media.Media{
		ID:        idFor(p),
		Title:     strings.TrimSuffix(d.Name(), path.Ext(d.Name())),
		Kind:      media.KindVideo,
		RelPath:   p,
		MimeType:  mimeType,
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
		AddedAt:   info.ModTime(), // no database yet; phase 7 records when it was first seen
	}
	if actor := folderActor(p); actor != "" {
		m.Actors = []media.ActorTag{{Name: actor, Source: media.ActorFromFolder}}
	}
	res.Items = append(res.Items, m)
}

// scanComic records the folder dir ("comics/<title>") as one comic made of its images.
func scanComic(res *Result, fsys fs.FS, dir string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		res.warn(dir, err.Error())
		return
	}

	var (
		pages   []string
		size    int64
		newest  time.Time
		skipped bool
	)
	for _, e := range entries {
		name := e.Name()
		switch {
		case hidden(name):
			continue
		case e.IsDir():
			res.warn(dir+"/"+name, "subfolder inside a comic, ignored")
			continue
		}
		if _, ok := imageTypes[ext(name)]; !ok || !e.Type().IsRegular() {
			continue // notes.txt, a symlinked page …: not a page
		}
		info, err := e.Info()
		if err != nil {
			res.warn(dir+"/"+name, err.Error())
			skipped = true
			continue
		}
		pages = append(pages, name)
		size += info.Size()
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}

	if len(pages) == 0 {
		if !skipped {
			res.warn(dir, "no images, skipped")
		}
		return
	}
	slices.SortFunc(pages, naturalCompare)

	res.Items = append(res.Items, media.Media{
		ID:        idFor(dir),
		Title:     path.Base(dir),
		Kind:      media.KindComic,
		RelPath:   dir,
		SizeBytes: size,
		ModTime:   newest, // a folder's own mtime ignores rewritten pages; the newest page doesn't
		AddedAt:   newest,
		Comic:     &media.ComicMeta{PageCount: len(pages), Pages: pages},
	})
}

// mustBeDir handles "movies" and "comics" at the root: they must be folders.
func mustBeDir(res *Result, p string, d fs.DirEntry) error {
	if !d.IsDir() {
		res.warn(p, "expected a folder, ignored")
	}
	return nil
}

// skip returns the WalkDir result that skips d: its whole subtree if it's a directory.
func skip(d fs.DirEntry) error {
	if d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

func (r *Result) warn(p, msg string) {
	r.Warnings = append(r.Warnings, fmt.Sprintf("%s: %s", p, msg))
}
