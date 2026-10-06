// Package memstore is an in-memory media.Repository.
package memstore

import (
	"cmp"
	"context"
	"slices"
	"strings"
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

// Replace swaps the whole library for items, e.g. after a rescan.
// The new map is built before taking the lock, so readers wait only for the swap itself,
// and they see either the old library or the new one, never a mix.
func (s *Store) Replace(ctx context.Context, items []media.Media) {
	next := make(map[string]media.Media, len(items))
	for _, m := range items {
		next[m.ID] = m
	}

	s.mu.Lock()
	s.items = next
	s.mu.Unlock()
}

// List returns a copy of every item, sorted by title (case-insensitive, then by id so
// equal titles keep a stable order).
func (s *Store) List(ctx context.Context) ([]media.Media, error) {
	s.mu.RLock()
	out := make([]media.Media, 0, len(s.items))
	for _, m := range s.items {
		out = append(out, m)
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
