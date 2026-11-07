package memstore

import (
	"context"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/media/mediatest"
)

func TestRepository(t *testing.T) {
	mediatest.Run(t, func(t *testing.T) mediatest.Store { return New(nil) })
}

func TestSyncWhileReading(t *testing.T) {
	// Run with -race: readers and a writer share the map only through the mutex.
	s := New(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			s.Sync(context.Background(), mediatest.Library(), time.Now())
		}
	}()
	for range 1000 {
		s.List(context.Background(), 1, media.ListFilter{})
	}
	<-done
}

func TestCovers(t *testing.T) {
	ctx := context.Background()
	lib := mediatest.Library()
	meta := &media.VideoMeta{DurationMs: 1000, Width: 1920, Height: 1080}
	for i := range lib {
		if lib[i].Title == "John Wick" {
			lib[i].Video = meta
		}
	}
	ep1 := media.Media{ID: "e1", Title: "Episode 1", Kind: media.KindVideo, RelPath: "anime/Trigun/Episode 1.mkv"}
	ep2 := media.Media{ID: "e2", Title: "Episode 2", Kind: media.KindVideo, RelPath: "anime/Trigun/Episode 2.mkv", Video: meta}
	s := New(append(lib, ep1, ep2))

	actors, err := s.ListActors(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actors {
		switch {
		case a.Name == "Keanu Reeves" && (a.Cover == nil || a.Cover.ID != "fp:movies/Keanu Reeves/John Wick.mp4"):
			t.Errorf("Keanu Reeves cover = %+v, want John Wick, the one with a thumbnail", a.Cover)
		case a.Name == "Matthew McConaughey" && a.Cover != nil:
			t.Errorf("Matthew McConaughey cover = %+v, want none: no thumbnail yet", a.Cover)
		}
	}
	series, err := s.ListSeries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].Cover == nil || series[0].Cover.ID != "e2" {
		t.Errorf("ListSeries = %+v, want Trigun covered by Episode 2", series)
	}
}
