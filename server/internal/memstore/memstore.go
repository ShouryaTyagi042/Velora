// Package memstore is an in-memory media.Repository: the test double for the Postgres one.
//
// It follows the same rules as internal/postgres (the shared suite in mediatest checks both),
// but nothing survives the process: manual actors added here are gone on restart. Only the
// Postgres repository makes them durable.
package memstore

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// entry is one stored item. Missing items stay (Postgres never deletes them either) but are
// invisible to readers.
type entry struct {
	m       media.Media
	missing bool
}

// Store holds media in a map guarded by a read/write mutex,
// so readers (handlers) and the writer (a scan) can share it.
type Store struct {
	mu       sync.RWMutex
	items    map[string]*entry          // by ID
	progress map[string]media.Progress // by media ID; one user for now
}

// New returns a Store seeded with the given media.
func New(seed []media.Media) *Store {
	s := &Store{items: make(map[string]*entry, len(seed)), progress: map[string]media.Progress{}}
	for _, m := range seed {
		s.items[m.ID] = &entry{m: m}
	}
	return s
}

// Sync implements media.Syncer. Items are matched by Fingerprint, which is also their ID here.
// An item seen before keeps its AddedAt and its manual actors; its folder actor is replaced.
func (s *Store) Sync(ctx context.Context, items []media.Media, scannedAt time.Time) (media.SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]bool, len(items))
	for _, m := range items {
		m.ID = m.Fingerprint
		seen[m.ID] = true
		m.Actors = folderActors(m.Actors) // the scanner only owns folder links
		if old, ok := s.items[m.ID]; ok {
			m.AddedAt = old.m.AddedAt
			m.Actors = mergeActors(m.Actors, old.m.Actors)
		} else {
			m.AddedAt = scannedAt
		}
		s.items[m.ID] = &entry{m: m}
	}

	missing := 0
	for id, e := range s.items {
		if !seen[id] && !e.missing {
			e.missing = true
			missing++
		}
	}
	return media.SyncResult{Upserted: len(items), Missing: missing}, nil
}

// List returns a copy of every available item matching f, sorted by title (case-insensitive,
// then by id so equal titles keep a stable order).
func (s *Store) List(ctx context.Context, f media.ListFilter) ([]media.Media, error) {
	s.mu.RLock()
	out := make([]media.Media, 0, len(s.items))
	for _, e := range s.items {
		if e.missing || (f.Kind != "" && e.m.Kind != f.Kind) || (f.Actor != "" && !e.m.HasActor(f.Actor)) {
			continue
		}
		out = append(out, s.withProgress(e.m))
	}
	s.mu.RUnlock()

	slices.SortFunc(out, func(a, b media.Media) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)),
			strings.Compare(a.ID, b.ID),
		)
	})
	return out, nil
}

// Get returns the available item with the given id, or media.ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (media.Media, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	return s.withProgress(e.m), nil
}

// withProgress returns m with its saved progress, if any. Callers hold s.mu.
func (s *Store) withProgress(m media.Media) media.Media {
	if p, ok := s.progress[m.ID]; ok {
		m.Progress = &p
	}
	return m
}

// SaveProgress implements media.Repository: the newer save wins.
func (s *Store) SaveProgress(ctx context.Context, id string, p media.Progress) (media.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.items[id]; !ok || e.missing {
		return media.Progress{}, media.ErrNotFound
	}
	if old, ok := s.progress[id]; ok && !old.UpdatedAt.Before(p.UpdatedAt) {
		return old, nil // stale: an older save arrived after a newer one
	}
	s.progress[id] = p
	return p, nil
}

// Continue implements media.Repository.
func (s *Store) Continue(ctx context.Context, kind media.Kind, limit int) ([]media.Media, error) {
	s.mu.RLock()
	out := []media.Media{}
	for id, p := range s.progress {
		e := s.items[id]
		if e == nil || e.missing || e.m.Kind != kind || p.Completed {
			continue
		}
		out = append(out, s.withProgress(e.m))
	}
	s.mu.RUnlock()

	slices.SortFunc(out, func(a, b media.Media) int {
		return cmp.Or(b.Progress.UpdatedAt.Compare(a.Progress.UpdatedAt), strings.Compare(a.ID, b.ID))
	})
	return out[:min(limit, len(out))], nil
}

// ListActors counts available videos per actor, both sources.
func (s *Store) ListActors(ctx context.Context) ([]media.ActorCount, error) {
	s.mu.RLock()
	counts := map[string]int{}
	for _, e := range s.items {
		if e.missing || e.m.Kind != media.KindVideo {
			continue
		}
		for _, a := range e.m.Actors {
			counts[a.Name]++
		}
	}
	s.mu.RUnlock()

	out := make([]media.ActorCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, media.ActorCount{Name: name, VideoCount: n})
	}
	slices.SortFunc(out, func(a, b media.ActorCount) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// AddActor implements media.Repository.
func (s *Store) AddActor(ctx context.Context, id, name string) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	if e.m.Kind != media.KindVideo {
		return media.Media{}, media.ErrNotVideo
	}
	if !e.m.HasActor(name) { // an existing link, folder or manual, is left as it is
		e.m.Actors = sortActors(append(slices.Clone(e.m.Actors), media.ActorTag{Name: name, Source: media.ActorManual}))
	}
	return s.withProgress(e.m), nil
}

// RemoveActor implements media.Repository.
func (s *Store) RemoveActor(ctx context.Context, id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[id]
	if !ok || e.missing {
		return media.ErrNotFound
	}
	i := slices.IndexFunc(e.m.Actors, func(a media.ActorTag) bool { return a.Name == name })
	switch {
	case i < 0:
		return media.ErrNotFound
	case e.m.Actors[i].Source == media.ActorFromFolder:
		return media.ErrFolderActor
	}
	e.m.Actors = slices.Delete(slices.Clone(e.m.Actors), i, i+1)
	if len(e.m.Actors) == 0 {
		e.m.Actors = nil
	}
	return nil
}

func folderActors(tags []media.ActorTag) []media.ActorTag {
	var out []media.ActorTag
	for _, a := range tags {
		if a.Source == media.ActorFromFolder {
			out = append(out, a)
		}
	}
	return out
}

// mergeActors keeps old's manual tags next to the new folder tags. The folder wins: a manual
// tag for the folder actor is dropped, since the folder tag says the same thing.
func mergeActors(folder, old []media.ActorTag) []media.ActorTag {
	out := slices.Clone(folder)
	for _, a := range old {
		if a.Source == media.ActorManual && !slices.ContainsFunc(out, func(b media.ActorTag) bool { return b.Name == a.Name }) {
			out = append(out, a)
		}
	}
	return sortActors(out)
}

// sortActors orders tags like the Postgres query: the folder actor first, then manual ones by name.
func sortActors(tags []media.ActorTag) []media.ActorTag {
	slices.SortFunc(tags, func(a, b media.ActorTag) int {
		return cmp.Or(
			cmp.Compare(rank(a.Source), rank(b.Source)),
			strings.Compare(a.Name, b.Name),
		)
	})
	return tags
}

func rank(s media.ActorSource) int {
	if s == media.ActorFromFolder {
		return 0
	}
	return 1
}
