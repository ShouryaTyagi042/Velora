package media

import (
	"context"
	"time"
)

// Repository is what the rest of Velora needs from media storage.
// Methods are added only when a caller needs one.
//
// Progress is per user, so every method that returns Media takes the user whose progress to
// attach. It's a parameter, not a context value: a forgotten user is a compile error, not
// silently missing progress. User 0 is nobody: no progress attached.
type Repository interface {
	// List returns the media matching f, in f.Sort order, starting after f.After, at most f.Limit.
	List(ctx context.Context, user int64, f ListFilter) ([]Media, error)
	// Get returns the media with the given id, or ErrNotFound.
	Get(ctx context.Context, user int64, id string) (Media, error)
	// ListActors returns every actor with at least one available video, sorted by name.
	ListActors(ctx context.Context) ([]ActorCount, error)
	// AddActor links a manual actor to a video and returns the updated video. Adding an actor
	// the video already has is a no-op. ErrNotFound for an unknown id, ErrNotVideo for a comic.
	// Actors are shared: every user sees them.
	AddActor(ctx context.Context, user int64, id, name string) (Media, error)
	// RemoveActor removes a manual actor from a video. ErrNotFound when the video has no actor
	// by that name, ErrFolderActor when the link comes from the folder.
	RemoveActor(ctx context.Context, id, name string) error

	// SaveProgress stores user's progress p for media id unless the stored progress is newer (by
	// UpdatedAt), and returns what is stored afterwards. A stale save is not an error: two saves
	// can arrive out of order, and the newer one must win. ErrNotFound for an unknown id.
	SaveProgress(ctx context.Context, user int64, id string, p Progress) (Progress, error)
	// Continue returns up to limit items of kind with user's unfinished progress, most recent first.
	Continue(ctx context.Context, user int64, kind Kind, limit int) ([]Media, error)
}

// Syncer is what a scan needs: make the stored library match what's on disk.
type Syncer interface {
	// Sync upserts items and marks everything not among them as missing, all at once:
	// readers see the library before the scan or after it, never half of it.
	// A video's folder actor is replaced; its manual actors are kept.
	Sync(ctx context.Context, items []Media, scannedAt time.Time) (SyncResult, error)
}

// ListFilter narrows and pages List. Zero values match everything, sorted by title, unpaged.
type ListFilter struct {
	Kind  Kind   // "" for all kinds
	Actor string // exact name, any source; "" for all
	// Query matches a title or an actor name containing it, ignoring case and accents
	// ("amelie" finds "Amélie"). "" for all.
	Query string
	Sort  Sort
	// After is the id of the last item of the previous page: the list continues right after it
	// in Sort order. "" starts at the beginning. Each store finds that item's sort key itself, so
	// the key is never recomputed outside the database (Go's ToLower and Postgres's lower() can
	// disagree on non-ASCII titles). Media are never deleted, so the id always resolves.
	After string
	Limit int // at most this many items; 0 for all
}

// Sort is the order List returns items in. Every order ends with the id, so it is total:
// two items never tie, and a page boundary can't skip or repeat one.
type Sort string

const (
	SortTitle  Sort = ""       // by title, case-insensitive, then id
	SortRecent Sort = "recent" // newest AddedAt first, then id descending
)

// Valid reports whether s is a known order.
func (s Sort) Valid() bool { return s == SortTitle || s == SortRecent }

// ActorCount is one row of GET /api/actors.
type ActorCount struct {
	Name       string `json:"name"`
	VideoCount int    `json:"videoCount"`
}

// SyncResult says what one Sync changed.
type SyncResult struct {
	Upserted int
	Missing  int // items newly marked missing by this sync
}
