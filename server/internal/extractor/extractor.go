// Package extractor fills in each video's metadata and thumbnail outside the request path,
// several videos at a time.
//
// Its queue is the database: "which videos still need metadata" is a query, so a crash or
// restart loses nothing, and the API never waits for ffprobe. Phase 13 made it concurrent: a
// round takes every pending video and runs Workers of them at once.
package extractor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/ffmpeg"
	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Store is what the extractor needs from storage. Declared here, by its consumer.
type Store interface {
	// Pending returns up to limit videos that still need metadata, oldest first. Videos that
	// failed MaxAttempts times are not pending.
	Pending(ctx context.Context, limit int) ([]media.Media, error)
	SetVideoMeta(ctx context.Context, id string, v media.VideoMeta) error
	RecordFailure(ctx context.Context, id string, reason string) error
}

const (
	// MaxAttempts is how often a video is tried before the extractor gives up on it. Without a
	// bound it would spend its life retrying one corrupt file.
	MaxAttempts = 3
	// roundSize bounds one round's list, so a 100k-video library isn't read into memory at once.
	roundSize = 500
)

// Extractor probes pending videos, Workers at a time, and writes their thumbnails to ThumbDir.
type Extractor struct {
	Store    Store
	MediaDir string         // the library root; a video's file is MediaDir/RelPath
	ThumbDir string         // thumbnails are ThumbDir/{id}.jpg
	Workers  int            // how many videos are extracted at once; at least 1
	Idle     time.Duration  // how long to wait for a Wake before looking again anyway
	Jobs     *jobs.Registry // where each round is reported; nil reports nowhere

	// The FFmpeg calls; tests replace them.
	Probe     func(ctx context.Context, path string) (media.VideoMeta, error)
	Thumbnail func(ctx context.Context, in, out string, at time.Duration) error

	wakeOnce sync.Once
	wake     chan struct{}
}

// New returns an Extractor that runs the real ffprobe and ffmpeg, workers at a time.
func New(store Store, mediaDir, thumbDir string, workers int, reg *jobs.Registry) *Extractor {
	return &Extractor{
		Store: store, MediaDir: mediaDir, ThumbDir: thumbDir, Workers: workers, Jobs: reg,
		Idle: time.Minute, Probe: ffmpeg.Probe, Thumbnail: ffmpeg.Thumbnail,
	}
}

// ThumbPath is where the thumbnail of media id lives.
func ThumbPath(thumbDir, id string) string {
	return filepath.Join(thumbDir, id+".jpg")
}

func (e *Extractor) wakeCh() chan struct{} {
	// Buffered 1: a Wake while a round runs is remembered, and a hundred Wakes are still one.
	e.wakeOnce.Do(func() { e.wake = make(chan struct{}, 1) })
	return e.wake
}

// Wake tells the extractor there may be new work (a scan just finished), so it looks now
// instead of after Idle. It never blocks.
func (e *Extractor) Wake() {
	select {
	case e.wakeCh() <- struct{}{}:
	default: // a wake is already pending
	}
}

// Run extracts until ctx is cancelled: a round whenever there is work, then waits for a Wake,
// or Idle as a fallback. A failing database is logged and retried after Idle, like an empty queue.
func (e *Extractor) Run(ctx context.Context) {
	log.Printf("extractor: started, %d workers", e.workers())
	for {
		n, err := e.Round(ctx)
		if ctx.Err() != nil {
			log.Printf("extractor: stopped")
			return
		}
		if err != nil {
			log.Printf("extractor: %v", err)
		}
		if n > 0 && err == nil {
			continue // failures are retried at once, up to MaxAttempts; new videos may have come in
		}
		select {
		case <-ctx.Done():
			log.Printf("extractor: stopped")
			return
		case <-e.wakeCh():
		case <-time.After(e.Idle):
		}
	}
}

// Round extracts every pending video (up to roundSize), Workers at a time, and returns how many
// it tried. A video's own failure is recorded against it and isn't an error of Round.
//
// The shape is a worker pool: one producer (this goroutine) hands videos to Workers goroutines
// over an unbuffered channel. A send blocks until a worker is free to take it, so at most Workers
// ffmpeg processes run at once, and the rest wait in the list, not as goroutines. Only the
// producer reads the queue, and it reads it again only after the round is over, so two workers
// can never be given the same video: no row locks, no "claimed" column.
func (e *Extractor) Round(ctx context.Context) (int, error) {
	pending, err := e.Store.Pending(ctx, roundSize)
	if err != nil || len(pending) == 0 {
		return 0, err
	}

	var task *jobs.Task
	if e.Jobs != nil {
		task = e.Jobs.Start("extract")
		task.SetTotal(int64(len(pending)))
	}
	start := time.Now()

	videos := make(chan media.Media)
	var wg sync.WaitGroup
	var dbErr error
	var errOnce sync.Once
	for range e.workers() {
		wg.Go(func() {
			for m := range videos { // ends when the producer closes videos
				if err := e.one(ctx, m); err != nil {
					errOnce.Do(func() { dbErr = err }) // the first database error; the round goes on
				}
				if task != nil {
					task.Add(1)
				}
			}
		})
	}

	sent := 0
send:
	for _, m := range pending {
		select {
		case videos <- m: // blocks until a worker is free: this is the backpressure
			sent++
		case <-ctx.Done(): // shutting down: hand out no more
			break send
		}
	}
	close(videos) // no more work: each worker's range loop ends once it finishes its current video
	wg.Wait()

	err = dbErr
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if task != nil {
		task.Finish(map[string]int{"videos": sent}, err)
	}
	took := time.Since(start).Round(time.Millisecond)
	if ctx.Err() != nil {
		// The ones in flight were stopped, not failed: they're still pending and run next time.
		log.Printf("extractor: round stopped by shutdown after %s; %d videos handed out, unfinished ones stay pending", took, sent)
	} else {
		log.Printf("extractor: round of %d videos with %d workers in %s", sent, e.workers(), took)
	}
	return sent, err
}

func (e *Extractor) workers() int { return max(1, e.Workers) }

// one extracts m. It returns only database errors: the file's own failure is recorded against it.
func (e *Extractor) one(ctx context.Context, m media.Media) error {
	start := time.Now()
	v, err := e.extract(ctx, m)
	if ctx.Err() != nil {
		return nil // shutting down: not the file's fault, so no attempt is recorded
	}
	if err != nil {
		log.Printf("extractor: %s (%s) failed: %v", m.ID, m.Title, err)
		return e.Store.RecordFailure(ctx, m.ID, err.Error())
	}
	if err := e.Store.SetVideoMeta(ctx, m.ID, v); err != nil {
		return err
	}
	log.Printf("extractor: %s (%s) %dx%d %s %.3gfps %s in %s", m.ID, m.Title, v.Width, v.Height, v.Codec,
		v.FPS, time.Duration(v.DurationMs)*time.Millisecond, time.Since(start).Round(time.Millisecond))
	return nil
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
