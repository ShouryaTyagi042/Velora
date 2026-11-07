package trash

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func TestRemove(t *testing.T) {
	lib, cache := t.TempDir(), t.TempDir()
	write := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(lib, "movies/A/Clip.mp4"))
	write(filepath.Join(lib, "movies/A/Keep.mp4"))
	write(filepath.Join(lib, "comics/Saga/001.png"))
	write(filepath.Join(cache, "7.jpg"))
	write(filepath.Join(cache, "7.mp4"))
	write(filepath.Join(cache, "8.jpg"))
	write(filepath.Join(t.TempDir(), "outside.txt"))

	root, err := os.OpenRoot(lib)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	r := Remover{Library: root, Derived: []string{cache}}

	if err := r.Remove(media.Media{ID: "7", Kind: media.KindVideo, RelPath: "movies/A/Clip.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(media.Media{ID: "9", Kind: media.KindComic, RelPath: "comics/Saga"}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]bool{
		filepath.Join(lib, "movies/A/Clip.mp4"): false,
		filepath.Join(lib, "movies/A/Keep.mp4"): true, // its neighbour stays
		filepath.Join(lib, "comics/Saga"):       false,
		filepath.Join(cache, "7.jpg"):           false,
		filepath.Join(cache, "7.mp4"):           false,
		filepath.Join(cache, "8.jpg"):           true,
	} {
		if _, err := os.Stat(p); (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", p, err == nil, want)
		}
	}

	// Already gone: still a success.
	if err := r.Remove(media.Media{ID: "7", Kind: media.KindVideo, RelPath: "movies/A/Clip.mp4"}); err != nil {
		t.Errorf("removing it again = %v, want nil", err)
	}
	// A path that escapes the library is refused by the os.Root, whatever the database says.
	if err := r.Remove(media.Media{ID: "x", Kind: media.KindComic, RelPath: "../"}); err == nil {
		t.Error("a path outside the library was accepted")
	}
}
