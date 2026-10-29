// Package mediatest is the behaviour every media.Repository must have, as one test suite.
// memstore and postgres both run it, so the test double can't drift from the real thing.
package mediatest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Store is a repository a scan can write to.
type Store interface {
	media.Repository
	media.Syncer
}

// t0 is the first scan's time; later scans add to it.
var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// Library is what a scanner would return for a small library:
//
//	movies/Keanu Reeves/The Matrix.mp4
//	movies/Keanu Reeves/John Wick.mp4
//	movies/Matthew McConaughey/Interstellar.mp4
//	movies/Loose Clip.mp4
//	comics/Watchmen/{page-1,page-2}.png
func Library() []media.Media {
	return []media.Media{
		video("movies/Keanu Reeves/The Matrix.mp4", "The Matrix", "Keanu Reeves"),
		video("movies/Keanu Reeves/John Wick.mp4", "John Wick", "Keanu Reeves"),
		video("movies/Matthew McConaughey/Interstellar.mp4", "Interstellar", "Matthew McConaughey"),
		video("movies/Loose Clip.mp4", "Loose Clip", ""),
		{
			Fingerprint: "fp:comics/Watchmen", ID: "fp:comics/Watchmen", Title: "Watchmen", Kind: media.KindComic,
			RelPath: "comics/Watchmen", SizeBytes: 300, ModTime: t0.Add(-time.Hour),
			Comic: &media.ComicMeta{PageCount: 2, Pages: []string{"page-1.png", "page-2.png"}},
		},
	}
}

func video(relPath, title, actor string) media.Media {
	m := media.Media{
		Fingerprint: "fp:" + relPath, ID: "fp:" + relPath, Title: title, Kind: media.KindVideo,
		RelPath: relPath, MimeType: "video/mp4", SizeBytes: 1000, ModTime: t0.Add(-time.Hour),
	}
	if actor != "" {
		m.Actors = []media.ActorTag{{Name: actor, Source: media.ActorFromFolder}}
	}
	return m
}

// Run runs the suite. open must return an empty store each time it's called.
func Run(t *testing.T, open func(t *testing.T) Store) {
	ctx := context.Background()

	// seeded opens a store holding Library().
	seeded := func(t *testing.T) Store {
		t.Helper()
		s := open(t)
		res, err := s.Sync(ctx, Library(), t0)
		if err != nil {
			t.Fatalf("Sync: %v", err)
		}
		if res.Upserted != 5 || res.Missing != 0 {
			t.Fatalf("first Sync = %+v, want 5 upserted, 0 missing", res)
		}
		return s
	}

	t.Run("list sorted by title", func(t *testing.T) {
		s := seeded(t)
		got := titles(list(t, s, media.ListFilter{}))
		want := []string{"Interstellar", "John Wick", "Loose Clip", "The Matrix", "Watchmen"}
		if !slices.Equal(got, want) {
			t.Errorf("List = %v, want %v", got, want)
		}
	})

	t.Run("empty list is not nil", func(t *testing.T) {
		if got := list(t, open(t), media.ListFilter{}); got == nil {
			t.Error("List on an empty store returned nil; JSON would be null instead of []")
		}
	})

	t.Run("filters", func(t *testing.T) {
		s := seeded(t)
		tests := []struct {
			f    media.ListFilter
			want []string
		}{
			{media.ListFilter{Kind: media.KindComic}, []string{"Watchmen"}},
			{media.ListFilter{Kind: media.KindVideo, Actor: "Keanu Reeves"}, []string{"John Wick", "The Matrix"}},
			{media.ListFilter{Actor: "keanu reeves"}, nil}, // exact match
			{media.ListFilter{Actor: "Nobody"}, nil},
		}
		for _, tt := range tests {
			if got := titles(list(t, s, tt.f)); !slices.Equal(got, tt.want) {
				t.Errorf("List(%+v) = %v, want %v", tt.f, got, tt.want)
			}
		}
	})

	t.Run("get", func(t *testing.T) {
		s := seeded(t)
		m := byTitle(t, s, "Watchmen")
		got, err := s.Get(ctx, m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Comic == nil || got.Comic.PageCount != 2 || !slices.Equal(got.Comic.Pages, []string{"page-1.png", "page-2.png"}) {
			t.Errorf("comic = %+v", got.Comic)
		}
		if got.RelPath != "comics/Watchmen" || got.AddedAt.IsZero() {
			t.Errorf("RelPath = %q, AddedAt = %v", got.RelPath, got.AddedAt)
		}
		if _, err := s.Get(ctx, "999999"); !errors.Is(err, media.ErrNotFound) {
			t.Errorf("Get(unknown) = %v, want ErrNotFound", err)
		}
	})

	t.Run("sync is idempotent", func(t *testing.T) {
		s := seeded(t)
		before := list(t, s, media.ListFilter{})
		if _, err := s.Sync(ctx, Library(), t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		after := list(t, s, media.ListFilter{})
		if !slices.Equal(ids(before), ids(after)) {
			t.Errorf("ids changed on rescan: %v → %v", ids(before), ids(after))
		}
		if !after[0].AddedAt.Equal(before[0].AddedAt) {
			t.Errorf("AddedAt changed on rescan: %v → %v", before[0].AddedAt, after[0].AddedAt)
		}
		actors(t, s, []media.ActorCount{count("Keanu Reeves", 2), count("Matthew McConaughey", 1)})
	})

	t.Run("manual actor survives a rescan", func(t *testing.T) {
		s := seeded(t)
		matrix := byTitle(t, s, "The Matrix")
		got, err := s.AddActor(ctx, matrix.ID, "Carrie-Anne Moss")
		if err != nil {
			t.Fatal(err)
		}
		wantTags := []media.ActorTag{folder("Keanu Reeves"), manual("Carrie-Anne Moss")}
		if !slices.Equal(got.Actors, wantTags) {
			t.Errorf("after AddActor: %v, want %v", got.Actors, wantTags)
		}

		if _, err := s.Sync(ctx, Library(), t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if got := byTitle(t, s, "The Matrix").Actors; !slices.Equal(got, wantTags) {
			t.Errorf("after rescan: %v, want %v", got, wantTags)
		}
		if got := titles(list(t, s, media.ListFilter{Actor: "Carrie-Anne Moss"})); !slices.Equal(got, []string{"The Matrix"}) {
			t.Errorf("?actor= manual actor: %v", got)
		}
		actors(t, s, []media.ActorCount{count("Carrie-Anne Moss", 1), count("Keanu Reeves", 2), count("Matthew McConaughey", 1)})
	})

	t.Run("add actor is idempotent", func(t *testing.T) {
		s := seeded(t)
		matrix := byTitle(t, s, "The Matrix")
		for range 2 {
			if _, err := s.AddActor(ctx, matrix.ID, "Carrie-Anne Moss"); err != nil {
				t.Fatal(err)
			}
		}
		// Adding the folder actor by hand changes nothing: the folder link stays.
		got, err := s.AddActor(ctx, matrix.ID, "Keanu Reeves")
		if err != nil {
			t.Fatal(err)
		}
		want := []media.ActorTag{folder("Keanu Reeves"), manual("Carrie-Anne Moss")}
		if !slices.Equal(got.Actors, want) {
			t.Errorf("actors = %v, want %v", got.Actors, want)
		}
	})

	t.Run("the folder wins over an earlier manual link", func(t *testing.T) {
		s := open(t)
		lib := Library()
		loose := lib[3] // Loose Clip: no folder actor yet
		if _, err := s.Sync(ctx, []media.Media{loose}, t0); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddActor(ctx, byTitle(t, s, "Loose Clip").ID, "Keanu Reeves"); err != nil {
			t.Fatal(err)
		}
		// The same file now reports a folder actor of the same name (same fingerprint).
		loose.Actors = []media.ActorTag{{Name: "Keanu Reeves", Source: media.ActorFromFolder}}
		if _, err := s.Sync(ctx, []media.Media{loose}, t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		got := byTitle(t, s, "Loose Clip")
		if want := []media.ActorTag{folder("Keanu Reeves")}; !slices.Equal(got.Actors, want) {
			t.Errorf("actors = %v, want %v", got.Actors, want)
		}
		if err := s.RemoveActor(ctx, got.ID, "Keanu Reeves"); !errors.Is(err, media.ErrFolderActor) {
			t.Errorf("RemoveActor(folder) = %v, want ErrFolderActor", err)
		}
	})

	t.Run("add actor errors", func(t *testing.T) {
		s := seeded(t)
		if _, err := s.AddActor(ctx, byTitle(t, s, "Watchmen").ID, "Alan Moore"); !errors.Is(err, media.ErrNotVideo) {
			t.Errorf("AddActor(comic) = %v, want ErrNotVideo", err)
		}
		if _, err := s.AddActor(ctx, "999999", "Keanu Reeves"); !errors.Is(err, media.ErrNotFound) {
			t.Errorf("AddActor(unknown) = %v, want ErrNotFound", err)
		}
	})

	t.Run("remove actor", func(t *testing.T) {
		s := seeded(t)
		matrix := byTitle(t, s, "The Matrix")
		if _, err := s.AddActor(ctx, matrix.ID, "Carrie-Anne Moss"); err != nil {
			t.Fatal(err)
		}
		if err := s.RemoveActor(ctx, matrix.ID, "Carrie-Anne Moss"); err != nil {
			t.Fatalf("RemoveActor(manual) = %v", err)
		}
		if err := s.RemoveActor(ctx, matrix.ID, "Carrie-Anne Moss"); !errors.Is(err, media.ErrNotFound) {
			t.Errorf("second RemoveActor = %v, want ErrNotFound", err)
		}
		if err := s.RemoveActor(ctx, matrix.ID, "Keanu Reeves"); !errors.Is(err, media.ErrFolderActor) {
			t.Errorf("RemoveActor(folder) = %v, want ErrFolderActor", err)
		}
		if err := s.RemoveActor(ctx, "999999", "Keanu Reeves"); !errors.Is(err, media.ErrNotFound) {
			t.Errorf("RemoveActor(unknown media) = %v, want ErrNotFound", err)
		}
		actors(t, s, []media.ActorCount{count("Keanu Reeves", 2), count("Matthew McConaughey", 1)})
	})

	t.Run("missing files drop out but keep their id", func(t *testing.T) {
		s := seeded(t)
		matrix := byTitle(t, s, "The Matrix")
		lib := Library()
		res, err := s.Sync(ctx, slices.Delete(lib, 0, 1), t0.Add(time.Minute)) // The Matrix is gone
		if err != nil {
			t.Fatal(err)
		}
		if res.Missing != 1 {
			t.Errorf("Missing = %d, want 1", res.Missing)
		}
		if _, err := s.Get(ctx, matrix.ID); !errors.Is(err, media.ErrNotFound) {
			t.Errorf("Get(missing) = %v, want ErrNotFound", err)
		}
		actors(t, s, []media.ActorCount{count("Keanu Reeves", 1), count("Matthew McConaughey", 1)})

		// It comes back (a drive reconnected): same id, available again.
		if _, err := s.Sync(ctx, Library(), t0.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if got := byTitle(t, s, "The Matrix"); got.ID != matrix.ID {
			t.Errorf("id after return = %s, want %s", got.ID, matrix.ID)
		}
	})

	t.Run("actor folder rename", func(t *testing.T) {
		s := seeded(t)
		old := byTitle(t, s, "The Matrix")
		if _, err := s.AddActor(ctx, old.ID, "Carrie-Anne Moss"); err != nil {
			t.Fatal(err)
		}
		lib := Library()
		lib[0] = video("movies/Keanu/The Matrix.mp4", "The Matrix", "Keanu") // folder renamed
		if _, err := s.Sync(ctx, lib, t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		renamed := byTitle(t, s, "The Matrix")
		if renamed.ID == old.ID {
			t.Error("a renamed path kept its id; the fingerprint is a path hash, so it can't")
		}
		if want := []media.ActorTag{folder("Keanu")}; !slices.Equal(renamed.Actors, want) {
			t.Errorf("renamed actors = %v, want %v (manual actors stay on the old record, ADR 013)", renamed.Actors, want)
		}
	})
}

func list(t *testing.T, s Store, f media.ListFilter) []media.Media {
	t.Helper()
	got, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatalf("List(%+v): %v", f, err)
	}
	return got
}

func byTitle(t *testing.T, s Store, title string) media.Media {
	t.Helper()
	for _, m := range list(t, s, media.ListFilter{}) {
		if m.Title == title {
			return m
		}
	}
	t.Fatalf("no media titled %q", title)
	return media.Media{}
}

func actors(t *testing.T, s Store, want []media.ActorCount) {
	t.Helper()
	got, err := s.ListActors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("ListActors = %v, want %v", got, want)
	}
}

func titles(ms []media.Media) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Title)
	}
	return out
}

func ids(ms []media.Media) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func folder(name string) media.ActorTag {
	return media.ActorTag{Name: name, Source: media.ActorFromFolder}
}
func manual(name string) media.ActorTag { return media.ActorTag{Name: name, Source: media.ActorManual} }
func count(name string, n int) media.ActorCount {
	return media.ActorCount{Name: name, VideoCount: n}
}
