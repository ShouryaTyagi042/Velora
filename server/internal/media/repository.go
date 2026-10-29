package media

import (
	"context"
	"time"
)

// Repository is what the rest of Velora needs from media storage.
// Methods are added only when a caller needs one.
type Repository interface {
	// List returns the media matching f, sorted by title.
	List(ctx context.Context, f ListFilter) ([]Media, error)
	// Get returns the media with the given id, or ErrNotFound.
	Get(ctx context.Context, id string) (Media, error)
	// ListActors returns every actor with at least one available video, sorted by name.
	ListActors(ctx context.Context) ([]ActorCount, error)
	// AddActor links a manual actor to a video and returns the updated video. Adding an actor
	// the video already has is a no-op. ErrNotFound for an unknown id, ErrNotVideo for a comic.
	AddActor(ctx context.Context, id, name string) (Media, error)
	// RemoveActor removes a manual actor from a video. ErrNotFound when the video has no actor
	// by that name, ErrFolderActor when the link comes from the folder.
	RemoveActor(ctx context.Context, id, name string) error
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
