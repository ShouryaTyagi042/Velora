package memstore

import (
	"context"
	"errors"
	"testing"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func TestGet(t *testing.T) {
	s := New([]media.Media{{ID: "m1", Title: "The Matrix", Kind: media.KindVideo}})

	t.Run("seeded id", func(t *testing.T) {
		m, err := s.Get(context.Background(), "m1")
		if err != nil {
			t.Fatalf("Get(m1) error = %v, want nil", err)
		}
		if m.Title != "The Matrix" {
			t.Errorf("Get(m1).Title = %q, want %q", m.Title, "The Matrix")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, err := s.Get(context.Background(), "nope")
		if !errors.Is(err, media.ErrNotFound) {
			t.Errorf("Get(nope) error = %v, want media.ErrNotFound", err)
		}
	})
}

func TestListSortedByTitle(t *testing.T) {
	s := New([]media.Media{
		{ID: "a", Title: "Watchmen"},
		{ID: "b", Title: "Interstellar"},
		{ID: "c", Title: "Saga"},
	})

	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	want := []string{"Interstellar", "Saga", "Watchmen"}
	if len(got) != len(want) {
		t.Fatalf("List() returned %d items, want %d", len(got), len(want))
	}
	for i, m := range got {
		if m.Title != want[i] {
			t.Errorf("List()[%d].Title = %q, want %q", i, m.Title, want[i])
		}
	}
}

func TestListEmptyIsNotNil(t *testing.T) {
	got, _ := New(nil).List(context.Background())
	if got == nil {
		t.Error("List on empty store returned nil; JSON would be null instead of []")
	}
}

func TestReplaceSwapsWholeLibrary(t *testing.T) {
	s := New([]media.Media{{ID: "old", Title: "Gone"}})
	s.Replace(context.Background(), []media.Media{{ID: "a", Title: "zebra"}, {ID: "b", Title: "Apple"}})

	if _, err := s.Get(context.Background(), "old"); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("old item still present after Replace")
	}
	got, _ := s.List(context.Background())
	if len(got) != 2 || got[0].Title != "Apple" || got[1].Title != "zebra" {
		t.Errorf("List = %+v, want Apple then zebra (case-insensitive)", got)
	}
}

func TestReplaceWhileReading(t *testing.T) {
	// Run with -race: readers and a writer share the map only through the mutex.
	s := New(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			s.Replace(context.Background(), []media.Media{{ID: "a", Title: "A"}})
		}
	}()
	for range 1000 {
		s.List(context.Background())
	}
	<-done
}
