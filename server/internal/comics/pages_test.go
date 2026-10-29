package comics

import (
	"slices"
	"testing"
	"testing/fstest"
	"time"
)

func TestListPages(t *testing.T) {
	old, newer := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	fsys := fstest.MapFS{
		"comics/W/page-10.png":      {Data: make([]byte, 10), ModTime: old},
		"comics/W/page-2.PNG":       {Data: make([]byte, 20), ModTime: newer},
		"comics/W/page-1.jpg":       {Data: make([]byte, 30), ModTime: old},
		"comics/W/.DS_Store":        {Data: []byte("x")},
		"comics/W/._page-1.jpg":     {Data: []byte("x")},
		"comics/W/notes.txt":        {Data: []byte("x")},
		"comics/W/extras/cover.jpg": {Data: []byte("x")},
		"comics/Empty/notes.txt":    {Data: []byte("x")},
	}

	l, err := ListPages(fsys, "comics/W")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"page-1.jpg", "page-2.PNG", "page-10.png"}; !slices.Equal(l.Pages, want) {
		t.Errorf("Pages = %v, want %v", l.Pages, want)
	}
	if l.Size != 60 || !l.ModTime.Equal(newer) {
		t.Errorf("Size = %d, ModTime = %v; want 60, %v", l.Size, l.ModTime, newer)
	}
	if want := []string{"comics/W/extras: subfolder inside a comic, ignored"}; !slices.Equal(l.Warnings, want) {
		t.Errorf("Warnings = %v, want %v", l.Warnings, want)
	}

	l, err = ListPages(fsys, "comics/Empty")
	if err != nil || len(l.Pages) != 0 {
		t.Errorf("Empty: %+v, %v; want no pages, no error", l, err)
	}
	if _, err := ListPages(fsys, "comics/Nope"); err == nil {
		t.Error("missing folder: want an error")
	}
}

func TestIsPage(t *testing.T) {
	for name, want := range map[string]bool{
		"001.jpg": true, "a.JPEG": true, "p.webp": true, "p.png": true,
		"._001.jpg": false, ".DS_Store": false, "notes.txt": false, "cover.gif": false,
	} {
		if got := IsPage(name); got != want {
			t.Errorf("IsPage(%q) = %v, want %v", name, got, want)
		}
	}
}
