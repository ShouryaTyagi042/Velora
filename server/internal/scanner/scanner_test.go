package scanner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

var (
	older = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
)

// testLibrary mirrors the phase 3 test library, including the parts that must be skipped.
func testLibrary() fstest.MapFS {
	return fstest.MapFS{
		"movies/Test Actor A/Big Buck Bunny.mp4":         {Data: make([]byte, 100)},
		"movies/Test Actor A/Sintel.mkv":                 {Data: make([]byte, 50)},
		"movies/Test Actor A/poster.jpg":                 {}, // not a video
		"movies/Test Actor B/Extras/Tears of Steel.webm": {}, // nested
		"movies/Loose Clip.mp4":                          {}, // no actor
		"movies/Test Actor A/.DS_Store":                  {}, // hidden
		"comics/Sample Comic/page-10.png":                {Data: make([]byte, 3), ModTime: newer},
		"comics/Sample Comic/page-2.png":                 {Data: make([]byte, 2), ModTime: older},
		"comics/Sample Comic/page-1.png":                 {Data: make([]byte, 1), ModTime: older},
		"comics/Sample Comic/._page-1.png":               {}, // macOS resource fork
		"comics/Sample Comic/notes.txt":                  {}, // not an image
		"comics/Sample Comic/extras/sketch.png":          {}, // subfolder: warned, ignored
		"comics/Zero Padded/002.jpg":                     {},
		"comics/Zero Padded/001.jpg":                     {},
		"comics/Empty Folder/readme.txt":                 {}, // no images: warned, skipped
		"comics/loose.png":                               {}, // loose file in comics/: ignored
		"notes.txt":                                      {}, // root junk: warned
		"Movies/Wrong Case/x.mp4":                        {}, // case matters: warned
		".DS_Store":                                      {},
	}
}

func scanTestLibrary(t *testing.T) (Result, map[string]media.Media) {
	t.Helper()
	res, err := Scan(context.Background(), testLibrary())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	byTitle := map[string]media.Media{}
	for _, m := range res.Items {
		byTitle[m.Title] = m
	}
	return res, byTitle
}

func TestScanFindsExactlyTheLibrary(t *testing.T) {
	_, byTitle := scanTestLibrary(t)

	var got []string
	for title := range byTitle {
		got = append(got, title)
	}
	slices.Sort(got)
	want := []string{"Big Buck Bunny", "Loose Clip", "Sample Comic", "Sintel", "Tears of Steel", "Zero Padded"}
	if !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestScanVideos(t *testing.T) {
	_, byTitle := scanTestLibrary(t)

	tests := []struct {
		title     string
		wantActor string // "" = no actors
		wantMime  string
		wantPath  string
	}{
		{"Big Buck Bunny", "Test Actor A", "video/mp4", "movies/Test Actor A/Big Buck Bunny.mp4"},
		{"Sintel", "Test Actor A", "video/x-matroska", "movies/Test Actor A/Sintel.mkv"},
		{"Tears of Steel", "Test Actor B", "video/webm", "movies/Test Actor B/Extras/Tears of Steel.webm"},
		{"Loose Clip", "", "video/mp4", "movies/Loose Clip.mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			m := byTitle[tt.title]
			if m.Kind != media.KindVideo || m.MimeType != tt.wantMime || m.RelPath != tt.wantPath {
				t.Errorf("got kind=%s mime=%s path=%q", m.Kind, m.MimeType, m.RelPath)
			}
			if tt.wantActor == "" {
				if len(m.Actors) != 0 {
					t.Errorf("actors = %v, want none", m.Actors)
				}
				return
			}
			want := []media.ActorTag{{Name: tt.wantActor, Source: media.ActorFromFolder}}
			if !slices.Equal(m.Actors, want) {
				t.Errorf("actors = %v, want %v", m.Actors, want)
			}
		})
	}
	if got := byTitle["Big Buck Bunny"].SizeBytes; got != 100 {
		t.Errorf("Big Buck Bunny size = %d, want 100", got)
	}
}

func TestScanComics(t *testing.T) {
	_, byTitle := scanTestLibrary(t)

	sample := byTitle["Sample Comic"]
	if sample.Kind != media.KindComic || sample.RelPath != "comics/Sample Comic" {
		t.Fatalf("got kind=%s path=%q", sample.Kind, sample.RelPath)
	}
	wantPages := []string{"page-1.png", "page-2.png", "page-10.png"} // natural order, junk dropped
	if sample.Comic == nil || !slices.Equal(sample.Comic.Pages, wantPages) || sample.Comic.PageCount != 3 {
		t.Errorf("comic = %+v, want pages %v", sample.Comic, wantPages)
	}
	if sample.SizeBytes != 6 {
		t.Errorf("size = %d, want 6 (sum of pages)", sample.SizeBytes)
	}
	if !sample.ModTime.Equal(newer) {
		t.Errorf("modTime = %v, want newest page's %v", sample.ModTime, newer)
	}
	if len(sample.Actors) != 0 {
		t.Errorf("comic has actors %v", sample.Actors)
	}

	if got := byTitle["Zero Padded"].Comic.Pages; !slices.Equal(got, []string{"001.jpg", "002.jpg"}) {
		t.Errorf("Zero Padded pages = %v", got)
	}
}

func TestScanWarnings(t *testing.T) {
	res, _ := scanTestLibrary(t)
	all := strings.Join(res.Warnings, "\n")

	for _, want := range []string{
		"notes.txt: not under movies/ or comics/",
		"Movies: not under movies/ or comics/",
		"comics/Empty Folder: no images, skipped",
		"comics/Sample Comic/extras: subfolder inside a comic",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing warning %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "DS_Store") || strings.Contains(all, "._page") {
		t.Errorf("hidden files should be skipped silently, got:\n%s", all)
	}
	if len(res.Warnings) != 4 {
		t.Errorf("got %d warnings, want 4:\n%s", len(res.Warnings), all)
	}
}

func TestScanIDsAreStable(t *testing.T) {
	first, _ := scanTestLibrary(t)
	second, _ := scanTestLibrary(t)

	ids := map[string]string{}
	for _, m := range first.Items {
		ids[m.RelPath] = m.ID
	}
	seen := map[string]bool{}
	for _, m := range second.Items {
		if ids[m.RelPath] != m.ID {
			t.Errorf("%s: id changed between scans", m.RelPath)
		}
		if seen[m.ID] {
			t.Errorf("duplicate id %s", m.ID)
		}
		seen[m.ID] = true
	}
	if idFor("comics/Sample Comic") != byPath(first, "comics/Sample Comic").ID {
		t.Error("comic id should come from its folder path")
	}
}

func TestScanStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fsys := fstest.MapFS{}
	for i := range 1000 {
		fsys[fmt.Sprintf("movies/A/%d.mp4", i)] = &fstest.MapFile{}
	}
	if _, err := Scan(ctx, fsys); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestScanFailsOnlyWhenRootUnreadable(t *testing.T) {
	if _, err := Scan(context.Background(), fstest.MapFS{}); err != nil {
		t.Errorf("empty library should scan fine, got %v", err)
	}
}

func TestFolderActor(t *testing.T) {
	tests := map[string]string{
		"movies/A/x.mp4":            "A",
		"movies/A/extras/x.mp4":     "A",
		"movies/x.mp4":              "",
		"movies/Keanu Reeves/x.mp4": "Keanu Reeves",
	}
	for p, want := range tests {
		if got := folderActor(p); got != want {
			t.Errorf("folderActor(%q) = %q, want %q", p, got, want)
		}
	}
}

func byPath(res Result, p string) media.Media {
	for _, m := range res.Items {
		if m.RelPath == p {
			return m
		}
	}
	return media.Media{}
}
