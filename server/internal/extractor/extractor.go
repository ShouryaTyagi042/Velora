// Package extractor is Velora's first background job: it fills in each video's metadata and
// thumbnail outside the request path.
//
// A job, at this size, is a loop in a goroutine whose state lives in the database: "which videos
// still need metadata" is a query, so a crash or restart loses nothing, and the API never waits
// for ffprobe. Phase 13 makes it concurrent and observable.
package extractor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/ffmpeg"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Store is what the extractor needs from storage. Declared here, by its consumer.
type Store interface {
	// NextPending returns a video that still needs metadata, or ok == false when none does.
	// Videos that failed MaxAttempts times are not pending.
	NextPending(ctx context.Context) (m media.Media, ok bool, err error)
	SetVideoMeta(ctx context.Context, id string, v media.VideoMeta) error
	RecordFailure(ctx context.Context, id string, reason string) error
}

// MaxAttempts is how often a video is tried before the extractor gives up on it. Without a bound
// the worker would spend its life retrying one corrupt file.
const MaxAttempts = 3

// Extractor probes pending videos one at a time and writes their thumbnails to ThumbDir.
type Extractor struct {
	Store    Store
	MediaDir string        // the library root; a video's file is MediaDir/RelPath
	ThumbDir string        // thumbnails are ThumbDir/{id}.jpg
	Idle     time.Duration // how long to sleep when nothing is pending

	// The FFmpeg calls; tests replace them.
	Probe     func(ctx context.Context, path string) (media.VideoMeta, error)
	Thumbnail func(ctx context.Context, in, out string, at time.Duration) error
}

// New returns an Extractor that runs the real ffprobe and ffmpeg.
func New(store Store, mediaDir, thumbDir string) *Extractor {
	return &Extractor{
		Store: store, MediaDir: mediaDir, ThumbDir: thumbDir, Idle: 5 * time.Second,
		Probe: ffmpeg.Probe, Thumbnail: ffmpeg.Thumbnail,
	}
}

// ThumbPath is where the thumbnail of media id lives.
func ThumbPath(thumbDir, id string) string {
	return filepath.Join(thumbDir, id+".jpg")
}

// Run extracts until ctx is cancelled. It never returns early: a failing database is logged and
// retried after Idle, like an empty queue.
func (e *Extractor) Run(ctx context.Context) {
	log.Printf("extractor: started")
	for {
		did, err := e.Once(ctx)
		if ctx.Err() != nil {
			log.Printf("extractor: stopped")
			return
		}
		if err != nil {
			log.Printf("extractor: %v", err)
		}
		if did {
			continue // there may be more; don't sleep between videos
		}
		select {
		case <-ctx.Done():
			log.Printf("extractor: stopped")
			return
		case <-time.After(e.Idle):
		}
	}
}

// Once extracts one pending video, if there is one, and reports whether there was.
// A video's own failure is recorded against it and isn't an error of Once.
func (e *Extractor) Once(ctx context.Context) (bool, error) {
	m, ok, err := e.Store.NextPending(ctx)
	if err != nil || !ok {
		return false, err
	}

	start := time.Now()
	v, err := e.extract(ctx, m)
	if ctx.Err() != nil {
		return true, ctx.Err() // shutting down: not the file's fault, so no attempt is recorded
	}
	if err != nil {
		log.Printf("extractor: %s (%s) failed: %v", m.ID, m.Title, err)
		return true, e.Store.RecordFailure(ctx, m.ID, err.Error())
	}
	if err := e.Store.SetVideoMeta(ctx, m.ID, v); err != nil {
		return true, err
	}
	log.Printf("extractor: %s (%s) %dx%d %s %.3gfps %s in %s", m.ID, m.Title, v.Width, v.Height, v.Codec,
		v.FPS, time.Duration(v.DurationMs)*time.Millisecond, time.Since(start).Round(time.Millisecond))
	return true, nil
}

func (e *Extractor) extract(ctx context.Context, m media.Media) (media.VideoMeta, error) {
	path := filepath.Join(e.MediaDir, filepath.FromSlash(m.RelPath))
	v, err := e.Probe(ctx, path)
	if err != nil {
		return v, err
	}
	// 10% in rather than at a fixed second: films often open on black.
	at := time.Duration(v.DurationMs) * time.Millisecond / 10
	if err := os.MkdirAll(e.ThumbDir, 0o755); err != nil {
		return v, err
	}
	// The thumbnail is written before the metadata: once a video has metadata, its thumbnail exists.
	if err := e.Thumbnail(ctx, path, ThumbPath(e.ThumbDir, m.ID), at); err != nil {
		return v, fmt.Errorf("thumbnail: %w", err)
	}
	return v, nil
}
