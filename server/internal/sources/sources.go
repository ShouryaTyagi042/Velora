// Package sources matches the downloader's records (nomedia-downloader: history.json of finished
// downloads, pending.json of queued ones) to library items, so each can keep the web page it was
// downloaded from.
//
// A record is one downloaded file: a video, or one page of a comic. Its path is where the
// downloader saved it, which is not always where the file is now (comics were saved under
// ~/Downloads and moved to the library later), so matching tries the exact place first and then
// the name.
package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Record is one downloaded file and the page it came from.
type Record struct {
	Path    string // absolute, where the downloader saved the file
	PageURL string
}

// record is either file's entry. history.json has path; pending.json has root, series and filename
// (or title), from which the downloader computes the path the same way (series_host.target_path).
type record struct {
	Path     string `json:"path"`
	PageURL  string `json:"pageUrl"`
	Root     string `json:"root"`
	Series   string `json:"series"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
}

// Read parses a history.json or pending.json: a JSON list, newest first in history. Entries
// without a page, or with one that isn't http(s), are skipped. Cookies and the like are ignored.
func Read(r io.Reader) ([]Record, error) {
	var raw []record
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("not a downloader list: %w", err)
	}
	out := make([]Record, 0, len(raw))
	for _, e := range raw {
		p := e.Path
		if p == "" && e.Root != "" {
			name := e.Filename
			if name == "" {
				name = e.Title
			}
			p = filepath.Join(e.Root, e.Series, name)
		}
		if p == "" || !isWebPage(e.PageURL) {
			continue
		}
		out = append(out, Record{Path: p, PageURL: e.PageURL})
	}
	return out, nil
}

func isWebPage(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// How a record was matched to an item.
const (
	ByPath = "path" // it is where the downloader saved it, inside the library
	ByName = "name" // same file name (a video) or folder name (a comic), and only one item has it
)

// Match is a library item and the page it came from.
type Match struct {
	ID      string
	PageURL string
	How     string
}

// Result is what matching found.
type Result struct {
	Matches   []Match
	Unmatched int // records with no item: a file since deleted, renamed, or never moved to the library
	Ambiguous int // records whose name fits more than one item, left alone rather than guessed
}

// MatchAll finds each item's page. items are the library (ID, Kind and RelPath are used); root is
// the library folder records' paths are compared under. When several records point to one item
// (a comic's pages), the first wins: history lists the newest first.
func MatchAll(records []Record, items []media.Media, root string) Result {
	byRel := map[string]media.Media{}
	videosByName := map[string][]media.Media{}
	comicsByName := map[string][]media.Media{}
	for _, m := range items {
		byRel[m.RelPath] = m
		base := path.Base(m.RelPath)
		if m.Kind == media.KindComic {
			comicsByName[base] = append(comicsByName[base], m)
		} else {
			videosByName[base] = append(videosByName[base], m)
		}
	}

	var res Result
	seen := map[string]bool{}
	add := func(m media.Media, r Record, how string) {
		if seen[m.ID] {
			return
		}
		seen[m.ID] = true
		res.Matches = append(res.Matches, Match{ID: m.ID, PageURL: r.PageURL, How: how})
	}

	for _, r := range records {
		// By place: the file itself is a video's rel_path; its folder is a comic's.
		if rel, ok := relTo(root, r.Path); ok {
			if m, ok := byRel[rel]; ok && m.Kind == media.KindVideo {
				add(m, r, ByPath)
				continue
			}
			if m, ok := byRel[path.Dir(rel)]; ok && m.Kind == media.KindComic {
				add(m, r, ByPath)
				continue
			}
		}
		// By name: the file's name among videos, else its folder's among comics.
		file := filepath.Base(r.Path)
		folder := filepath.Base(filepath.Dir(r.Path))
		switch v, c := videosByName[file], comicsByName[folder]; {
		case len(v) == 1:
			add(v[0], r, ByName)
		case len(v) > 1:
			res.Ambiguous++
		case len(c) == 1:
			add(c[0], r, ByName)
		case len(c) > 1:
			res.Ambiguous++
		default:
			res.Unmatched++
		}
	}
	return res
}

// relTo is p relative to root, slash-separated like media.RelPath, if p is inside root.
func relTo(root, p string) (string, bool) {
	if root == "" {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
