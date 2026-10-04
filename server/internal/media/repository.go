package media

import "context"

// Repository is what the rest of Velora needs from media storage.
// Methods are added only when a caller needs one.
type Repository interface {
	// List returns every media item, sorted by title.
	List(ctx context.Context) ([]Media, error)
	// Get returns the media with the given id, or ErrNotFound.
	Get(ctx context.Context, id string) (Media, error)
}
