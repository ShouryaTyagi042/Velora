// Package memstore is an in-memory media.Repository.
package memstore

import (
	"context"
	"sort"
	"sync"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Store holds media in a map guarded by a read/write mutex,
// so readers (handlers) and a future writer (the phase 3 scanner) can share it.
type Store struct {
	mu    sync.RWMutex
	items map[string]media.Media
}

// New returns a Store seeded with the given media.
func New(seed []media.Media) *Store {
	s := &Store{items: make(map[string]media.Media, len(seed))}
	for _, m := range seed {
		s.items[m.ID] = m
	}
	return s
}

// List returns a copy of every item, sorted by title.
func (s *Store) List(ctx context.Context) ([]media.Media, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]media.Media, 0, len(s.items))
	for _, m := range s.items {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// Get returns the item with the given id, or media.ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (media.Media, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.items[id]
	if !ok {
		return media.Media{}, media.ErrNotFound
	}
	return m, nil
}
