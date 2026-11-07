package sources

import (
	"strings"
	"testing"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func TestRead(t *testing.T) {
	history := `[
	  {"path": "/lib/movies/A/Clip.mp4", "pageUrl": "https://example.com/watch/1", "videoUrl": "https://cdn/x.mp4"},
	  {"path": "/lib/movies/B.mp4", "pageUrl": null},
	  {"path": "/lib/movies/C.mp4", "pageUrl": "javascript:alert(1)"}
	]`
	got, err := Read(strings.NewReader(history))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Record{Path: "/lib/movies/A/Clip.mp4", PageURL: "https://example.com/watch/1"}) {
		t.Errorf("history = %+v, want only the entry with a web page", got)
	}

	// pending.json: no path, the downloader's root + series + filename instead; cookies ignored.
	pending := `[{"root": "/dl/Comics", "series": "Ep 1", "filename": "003.png", "pageUrl": "https://example.com/ep-1/", "cookies": "secret"}]`
	got, err = Read(strings.NewReader(pending))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/dl/Comics/Ep 1/003.png" {
		t.Errorf("pending = %+v, want the path joined from root, series and filename", got)
	}

	if _, err := Read(strings.NewReader(`{"not": "a list"}`)); err == nil {
		t.Error("Read(object) = nil error")
	}
}

func TestMatchAll(t *testing.T) {
	items := []media.Media{
		{ID: "1", Kind: media.KindVideo, RelPath: "movies/Keanu Reeves/The Matrix.mp4"},
		{ID: "2", Kind: media.KindVideo, RelPath: "movies/Keanu Reeves/John Wick.mp4"}, // moved after download
		{ID: "3", Kind: media.KindComic, RelPath: "comics/Watchmen"},
		{ID: "4", Kind: media.KindComic, RelPath: "comics/Saga"}, // downloaded to ~/Downloads, moved in
		{ID: "5", Kind: media.KindVideo, RelPath: "movies/A/Intro.mp4"},
		{ID: "6", Kind: media.KindVideo, RelPath: "movies/B/Intro.mp4"}, // same name as 5
		{ID: "7", Kind: media.KindVideo, RelPath: "anime/Trigun/Episode 1.mkv"},
	}
	records := []Record{
		{Path: "/lib/movies/Keanu Reeves/The Matrix.mp4", PageURL: "https://a/matrix"},
		{Path: "/lib/movies/John Wick.mp4", PageURL: "https://a/wick"},
		{Path: "/lib/comics/Watchmen/002.png", PageURL: "https://a/watchmen-new"}, // newest first: wins
		{Path: "/lib/comics/Watchmen/001.png", PageURL: "https://a/watchmen-old"},
		{Path: "/Users/me/Downloads/Saga/001.jpg", PageURL: "https://a/saga"},
		{Path: "/elsewhere/Intro.mp4", PageURL: "https://a/intro"},      // fits 5 and 6: left alone
		{Path: "/lib/movies/Deleted.mp4", PageURL: "https://a/deleted"}, // not in the library
		{Path: "/lib/anime/Trigun/Episode 1.mkv", PageURL: "https://a/trigun-1"},
	}

	res := MatchAll(records, items, "/lib/")
	got := map[string]Match{}
	for _, m := range res.Matches {
		got[m.ID] = m
	}
	want := map[string]Match{
		"1": {ID: "1", PageURL: "https://a/matrix", How: ByPath},
		"2": {ID: "2", PageURL: "https://a/wick", How: ByName},
		"3": {ID: "3", PageURL: "https://a/watchmen-new", How: ByPath},
		"4": {ID: "4", PageURL: "https://a/saga", How: ByName},
		"7": {ID: "7", PageURL: "https://a/trigun-1", How: ByPath},
	}
	if len(got) != len(want) {
		t.Errorf("matched %d items, want %d: %+v", len(got), len(want), res.Matches)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("item %s = %+v, want %+v", id, got[id], w)
		}
	}
	if res.Ambiguous != 1 || res.Unmatched != 1 {
		t.Errorf("ambiguous %d, unmatched %d; want 1 and 1", res.Ambiguous, res.Unmatched)
	}
}
