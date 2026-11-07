// Package memstore is an in-memory media.Repository: the test double for the Postgres one.
//
// It follows the same rules as internal/postgres (the shared suite in mediatest checks both),
// but nothing survives the process: manual actors added here are gone on restart. Only the
// Postgres repository makes them durable.
package memstore

import (
	"cmp"
	"context"
	"math/rand/v2"
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
	mu         sync.RWMutex
	items      map[string]*entry              // by ID
	progress   map[progressKey]media.Progress // per user and item
	favourites map[progressKey]time.Time      // per user and item: when it was added
	views      map[string]int                 // per item
	lastView   map[progressKey]time.Time      // per user and item: their latest counted view
}

type progressKey struct {
	user int64
	id   string
}

// New returns a Store seeded with the given media.
func New(seed []media.Media) *Store {
	s := &Store{items: make(map[string]*entry, len(seed)), progress: map[progressKey]media.Progress{}, favourites: map[progressKey]time.Time{},
		views: map[string]int{}, lastView: map[progressKey]time.Time{}}
	for _, m := range seed {
		m.Section = media.SectionOf(m.Kind, m.RelPath)
		m.Series = media.SeriesOf(m.Kind, m.RelPath)
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
		m.Section = media.SectionOf(m.Kind, m.RelPath)
		m.Series = media.SeriesOf(m.Kind, m.RelPath)
		seen[m.ID] = true
		m.Actors = folderActors(m.Actors) // the scanner only owns folder links
		if old, ok := s.items[m.ID]; ok {
			m.AddedAt = old.m.AddedAt
			m.Actors = mergeActors(m.Actors, old.m.Actors)
			m.Tags, m.InBin, m.BinnedAt = old.m.Tags, old.m.InBin, old.m.BinnedAt // the scanner doesn't own these
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
func (s *Store) List(ctx context.Context, user int64, f media.ListFilter) ([]media.Media, error) {
	order := byTitle
	switch f.Sort {
	case media.SortBinned:
		order = byBinned
	case media.SortRecent:
		order = byRecent
	case media.SortTitleDesc:
		order = func(a, b media.Media) int { return byTitle(b, a) }
	}
	q := media.Fold(f.Query)

	s.mu.RLock()
	// The cursor item, found even if it has since gone missing: its sort key still places it.
	var after *media.Media
	if f.After != "" {
		e, ok := s.items[f.After]
		if !ok {
			s.mu.RUnlock()
			return []media.Media{}, nil // an id that never existed: nothing comes after it
		}
		after = &e.m
	}
	out := make([]media.Media, 0, len(s.items))
	for _, e := range s.items {
		if e.missing || (f.Kind != "" && e.m.Kind != f.Kind) || (f.Section != "" && e.m.Section != f.Section) ||
			(f.Series != "" && e.m.Series != f.Series) || (f.Actor != "" && !e.m.HasActor(f.Actor)) || !hasClip(e.m, f.Clip) ||
			e.m.InBin != f.InBin || (f.Tag != "" && !slices.Contains(e.m.Tags, f.Tag)) ||
			(f.Favourites && !s.isFavourite(user, e.m.ID)) ||
			(q != "" && !matches(e.m, q)) || (after != nil && order(e.m, *after) <= 0) {
			continue
		}
		out = append(out, s.withProgress(user, e.m))
	}
	s.mu.RUnlock()

	if f.Sort == media.SortRandom {
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	} else {
		slices.SortFunc(out, order)
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// hasClip reports whether m's clip of kind c is ready ("" matches everything).
func hasClip(m media.Media, c media.Clip) bool {
	switch c {
	case media.ClipPreview:
		return m.Video != nil && m.Video.Preview
	case media.ClipShort:
		return m.Video != nil && m.Video.Short
	}
	return true
}

// byTitle and byRecent are List's orders (media.SortTitle, media.SortRecent).
func byTitle(a, b media.Media) int {
	return cmp.Or(strings.Compare(media.TitleKey(a.Title), media.TitleKey(b.Title)), strings.Compare(a.ID, b.ID))
}

func byBinned(a, b media.Media) int {
	if a.BinnedAt != nil && b.BinnedAt != nil {
		if c := b.BinnedAt.Compare(*a.BinnedAt); c != 0 {
			return c
		}
	}
	return strings.Compare(b.ID, a.ID)
}

func byRecent(a, b media.Media) int {
	return cmp.Or(b.AddedAt.Compare(a.AddedAt), strings.Compare(b.ID, a.ID))
}

// matches reports whether m's title, one of its actors or its series contains q, which is already
// folded.
func matches(m media.Media, q string) bool {
	if strings.Contains(media.Fold(m.Title), q) || (m.Series != "" && strings.Contains(media.Fold(m.Series), q)) {
		return true
	}
	for _, a := range m.Actors {
		if strings.Contains(media.Fold(a.Name), q) {
			return true
		}
	}
	for _, t := range m.Tags {
		if strings.Contains(media.Fold(t), q) {
			return true
		}
	}
	return false
}

// Get returns the available item with the given id, or media.ErrNotFound.
func (s *Store) Get(ctx context.Context, user int64, id string) (media.Media, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	return s.withProgress(user, e.m), nil
}

// withProgress returns m with user's saved progress, if any, and whether it's their favourite.
// Callers hold s.mu.
func (s *Store) withProgress(user int64, m media.Media) media.Media {
	if p, ok := s.progress[progressKey{user, m.ID}]; ok {
		m.Progress = &p
	}
	m.Favourite = s.isFavourite(user, m.ID)
	m.Views = s.views[m.ID]
	return m
}

// RecordView implements media.Repository.
func (s *Store) RecordView(ctx context.Context, user int64, id string, at time.Time) (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.items[id]; !ok || e.missing {
		return 0, false, media.ErrNotFound
	}
	k := progressKey{user, id}
	if last, ok := s.lastView[k]; ok && at.Sub(last) < media.ViewWindow {
		return s.views[id], false, nil
	}
	s.lastView[k] = at
	s.views[id]++
	return s.views[id], true, nil
}

func (s *Store) isFavourite(user int64, id string) bool {
	_, ok := s.favourites[progressKey{user, id}]
	return ok
}

// SaveProgress implements media.Repository: the newer save wins.
func (s *Store) SaveProgress(ctx context.Context, user int64, id string, p media.Progress) (media.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.items[id]; !ok || e.missing {
		return media.Progress{}, media.ErrNotFound
	}
	k := progressKey{user, id}
	if old, ok := s.progress[k]; ok && !old.UpdatedAt.Before(p.UpdatedAt) {
		return old, nil // stale: an older save arrived after a newer one
	}
	s.progress[k] = p
	return p, nil
}

// Continue implements media.Repository.
func (s *Store) Continue(ctx context.Context, user int64, kind media.Kind, limit int) ([]media.Media, error) {
	s.mu.RLock()
	out := []media.Media{}
	for k, p := range s.progress {
		e := s.items[k.id]
		if k.user != user || e == nil || e.missing || e.m.InBin || e.m.Kind != kind || p.Completed {
			continue
		}
		out = append(out, s.withProgress(user, e.m))
	}
	s.mu.RUnlock()

	slices.SortFunc(out, func(a, b media.Media) int {
		return cmp.Or(b.Progress.UpdatedAt.Compare(a.Progress.UpdatedAt), strings.Compare(a.ID, b.ID))
	})
	return out[:min(limit, len(out))], nil
}

// ListActors counts available videos per actor, both sources.
func (s *Store) ListActors(ctx context.Context, section media.Section) ([]media.ActorCount, error) {
	s.mu.RLock()
	counts := map[string]int{}
	covers := map[string]media.Media{} // the newest video with a thumbnail, as in Postgres
	for _, e := range s.items {
		if e.missing || e.m.InBin || e.m.Kind != media.KindVideo || (section != "" && e.m.Section != section) {
			continue
		}
		for _, a := range e.m.Actors {
			counts[a.Name]++
			if c, ok := covers[a.Name]; e.m.Video != nil && (!ok || byRecent(e.m, c) < 0) {
				covers[a.Name] = e.m
			}
		}
	}
	s.mu.RUnlock()

	out := make([]media.ActorCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, media.ActorCount{Name: name, VideoCount: n, Cover: coverOf(covers, name)})
	}
	slices.SortFunc(out, func(a, b media.ActorCount) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// ListSeries counts available episodes per anime series, in natural order.
func (s *Store) ListSeries(ctx context.Context) ([]media.SeriesCount, error) {
	s.mu.RLock()
	counts := map[string]int{}
	covers := map[string]media.Media{} // the first episode by title with a thumbnail
	for _, e := range s.items {
		if !e.missing && !e.m.InBin && e.m.Series != "" {
			counts[e.m.Series]++
			if c, ok := covers[e.m.Series]; e.m.Video != nil && (!ok || byTitle(e.m, c) < 0) {
				covers[e.m.Series] = e.m
			}
		}
	}
	s.mu.RUnlock()

	out := make([]media.SeriesCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, media.SeriesCount{Name: name, EpisodeCount: n, Cover: coverOf(covers, name)})
	}
	slices.SortFunc(out, func(a, b media.SeriesCount) int {
		return cmp.Or(strings.Compare(media.TitleKey(a.Name), media.TitleKey(b.Name)), strings.Compare(a.Name, b.Name))
	})
	return out, nil
}

// coverOf is the cover chosen for name, or nil when none of its videos has a thumbnail.
func coverOf(covers map[string]media.Media, name string) *media.Cover {
	m, ok := covers[name]
	if !ok {
		return nil
	}
	return &media.Cover{ID: m.ID, ModTime: m.ModTime}
}

// AddActor implements media.Repository.
func (s *Store) AddActor(ctx context.Context, user int64, id, name string) (media.Media, error) {
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
	return s.withProgress(user, e.m), nil
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

// SetInBin implements media.Repository.
func (s *Store) SetInBin(ctx context.Context, user int64, id string, inBin bool) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	switch {
	case inBin && !e.m.InBin:
		now := time.Now()
		e.m.InBin, e.m.BinnedAt = true, &now
	case !inBin:
		e.m.InBin, e.m.BinnedAt = false, nil
	}
	return s.withProgress(user, e.m), nil
}

// Delete implements media.Repository.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	switch {
	case !ok:
		return media.ErrNotFound
	case !e.m.InBin:
		return media.ErrNotInBin
	}
	delete(s.items, id)
	for k := range s.progress {
		if k.id == id {
			delete(s.progress, k)
		}
	}
	for k := range s.favourites {
		if k.id == id {
			delete(s.favourites, k)
		}
	}
	for k := range s.lastView {
		if k.id == id {
			delete(s.lastView, k)
		}
	}
	delete(s.views, id)
	return nil
}

// SetFavourite implements media.Repository.
func (s *Store) SetFavourite(ctx context.Context, user int64, id string, favourite bool) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	k := progressKey{user, id}
	if _, has := s.favourites[k]; favourite && !has {
		s.favourites[k] = time.Now()
	} else if !favourite {
		delete(s.favourites, k)
	}
	return s.withProgress(user, e.m), nil
}

// AddTag implements media.Repository.
func (s *Store) AddTag(ctx context.Context, user int64, id, name string) (media.Media, error) {
	return s.changeTags(user, id, func(tags []string) []string {
		if slices.Contains(tags, name) {
			return tags
		}
		out := append(slices.Clone(tags), name)
		slices.Sort(out)
		return out
	})
}

// RemoveTag implements media.Repository.
func (s *Store) RemoveTag(ctx context.Context, user int64, id, name string) (media.Media, error) {
	return s.changeTags(user, id, func(tags []string) []string {
		return slices.DeleteFunc(slices.Clone(tags), func(t string) bool { return t == name })
	})
}

func (s *Store) changeTags(user int64, id string, fn func([]string) []string) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok || e.missing {
		return media.Media{}, media.ErrNotFound
	}
	if e.m.Kind != media.KindVideo {
		return media.Media{}, media.ErrNotVideo
	}
	e.m.Tags = fn(e.m.Tags)
	if len(e.m.Tags) == 0 {
		e.m.Tags = nil
	}
	return s.withProgress(user, e.m), nil
}

// ListTags implements media.Repository.
func (s *Store) ListTags(ctx context.Context, section media.Section) ([]media.TagCount, error) {
	s.mu.RLock()
	counts := map[string]int{}
	for _, e := range s.items {
		if e.missing || e.m.InBin || (section != "" && e.m.Section != section) {
			continue
		}
		for _, t := range e.m.Tags {
			counts[t]++
		}
	}
	s.mu.RUnlock()
	out := make([]media.TagCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, media.TagCount{Name: name, VideoCount: n})
	}
	slices.SortFunc(out, func(a, b media.TagCount) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
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
