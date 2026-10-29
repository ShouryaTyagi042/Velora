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
	// List returns the media matching f, sorted by title.
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

// ListFilter narrows List. Zero values match everything.
type ListFilter struct {
	Kind  Kind   // "" for all kinds
	Actor string // exact name, any source; "" for all
}

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
