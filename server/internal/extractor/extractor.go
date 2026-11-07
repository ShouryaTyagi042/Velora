// Package extractor fills in each video's metadata and thumbnail outside the request path,
// several videos at a time, and then its long-press preview (phase 14).
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
	"sync/atomic"
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

	// PendingPreviews returns up to limit videos with metadata whose preview is missing or older
	// than the file (by ModTime), oldest first, with Video.DurationMs and ModTime filled in.
	// Videos whose preview failed MaxAttempts times are not pending.
	PendingPreviews(ctx context.Context, limit int) ([]media.Media, error)
	// SetPreview records that id's preview was made from the file as it was at modTime.
	SetPreview(ctx context.Context, id string, modTime time.Time) error
	RecordPreviewFailure(ctx context.Context, id string, reason string) error

	// PendingShorts, SetShort and RecordShortFailure are the same for the Shorts feed's clips.
	PendingShorts(ctx context.Context, limit int) ([]media.Media, error)
	SetShort(ctx context.Context, id string, modTime time.Time) error
	RecordShortFailure(ctx context.Context, id string, reason string) error
}

const (
	// MaxAttempts is how often a video is tried before the extractor gives up on it. Without a
	// bound it would spend its life retrying one corrupt file.
	MaxAttempts = 3
	// roundSize bounds one round's list, so a 100k-video library isn't read into memory at once.
	roundSize = 500
	// previewsPerWorker is how many previews a preview round makes per worker. Previews are slow
	// (seconds each), so they go in small batches: between two batches, the loop looks for
	// metadata work first, and a newly scanned video gets its thumbnail without waiting for a
	// whole library's previews.
	previewsPerWorker = 2
)

// Extractor probes pending videos, Workers at a time, and writes their thumbnails to ThumbDir,
// then their previews to PreviewDir.
type Extractor struct {
	Store      Store
	MediaDir   string         // the library root; a video's file is MediaDir/RelPath
	ThumbDir   string         // thumbnails are ThumbDir/{id}.jpg
	PreviewDir string         // previews are PreviewDir/{id}.mp4; "" makes none
	ShortDir   string         // Shorts clips are ShortDir/{id}.mp4; "" makes none
	Workers    int            // how many videos are extracted at once; at least 1
	Idle       time.Duration  // how long to wait for a Wake before looking again anyway
	Jobs       *jobs.Registry // where each round is reported; nil reports nowhere

	// The FFmpeg calls; tests replace them.
	Probe     func(ctx context.Context, path string) (media.VideoMeta, error)
	Thumbnail func(ctx context.Context, in, out string, at time.Duration) error
	Preview   func(ctx context.Context, in, out string, durationMs int) error
	Short     func(ctx context.Context, in, out string, durationMs int) error

	wakeOnce sync.Once
	wake     chan struct{}
}

// New returns an Extractor that runs the real ffprobe and ffmpeg, workers at a time.
func New(store Store, mediaDir, thumbDir, previewDir, shortDir string, workers int, reg *jobs.Registry) *Extractor {
	return &Extractor{
		Store: store, MediaDir: mediaDir, ThumbDir: thumbDir, PreviewDir: previewDir, ShortDir: shortDir, Workers: workers, Jobs: reg,
		Idle: time.Minute, Probe: ffmpeg.Probe, Thumbnail: ffmpeg.Thumbnail, Preview: ffmpeg.Preview, Short: ffmpeg.Short,
	}
}

// PreviewPath is where the preview (or, in ShortDir, the Shorts clip) of media id lives.
func PreviewPath(previewDir, id string) string {
	return filepath.Join(previewDir, id+".mp4")
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

// Run extracts until ctx is cancelled: a metadata round whenever there is metadata work, else a
// small preview round, then waits for a Wake, or Idle as a fallback. Metadata always comes first:
// it is quick and the grid needs it (thumbnails), while previews are slow and only a long-press
// needs them. A failing database is logged and retried after Idle, like an empty queue.
func (e *Extractor) Run(ctx context.Context) {
	log.Printf("extractor: started, %d workers", e.workers())
	for {
		n, err := e.Round(ctx)
		if err == nil && n == 0 && ctx.Err() == nil {
			n, err = e.PreviewRound(ctx)
		}
		if err == nil && n == 0 && ctx.Err() == nil {
			n, err = e.ShortRound(ctx) // last: the biggest files, and only the Shorts tab needs them
		}
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
	return e.pool(ctx, "extract", pending, e.one)
}

// PreviewRound makes previews for up to previewsPerWorker × Workers videos, Workers at a time,
// and returns how many it tried. It makes none without a PreviewDir.
func (e *Extractor) PreviewRound(ctx context.Context) (int, error) {
	if e.PreviewDir == "" || e.Preview == nil {
		return 0, nil
	}
	pending, err := e.Store.PendingPreviews(ctx, previewsPerWorker*e.workers())
	if err != nil || len(pending) == 0 {
		return 0, err
	}
	return e.pool(ctx, "preview", pending, e.onePreview)
}

// ShortRound makes Shorts clips like PreviewRound makes previews: a small batch, Workers at a
// time, only once every preview is made. It makes none without a ShortDir.
func (e *Extractor) ShortRound(ctx context.Context) (int, error) {
	if e.ShortDir == "" || e.Short == nil {
		return 0, nil
	}
	pending, err := e.Store.PendingShorts(ctx, previewsPerWorker*e.workers())
	if err != nil || len(pending) == 0 {
		return 0, err
	}
	return e.pool(ctx, "short", pending, e.oneShort)
}

// pool runs fn on every video in pending, Workers at a time, reported as a job called name.
func (e *Extractor) pool(ctx context.Context, name string, pending []media.Media, fn func(context.Context, media.Media) error) (int, error) {
	var task *jobs.Task
	if e.Jobs != nil {
		task = e.Jobs.Start(name)
		task.SetTotal(int64(len(pending)))
	}
	start := time.Now()

	videos := make(chan media.Media)
	var wg sync.WaitGroup
	var dbErr error
	var errOnce sync.Once
	// started counts videos a worker began while ctx was live. Not the producer's sends: once ctx
	// is cancelled, a worker freed by its killed ffmpeg and ctx.Done can be ready at the same
	// moment, and select picks between them at random, so one more send can slip through. That
	// video is skipped here, not started, and not counted.
	var started atomic.Int64
	for range e.workers() {
		wg.Go(func() {
			for m := range videos { // ends when the producer closes videos
				if ctx.Err() != nil {
					continue // handed out after shutdown began: drain, don't start
				}
				started.Add(1)
				if err := fn(ctx, m); err != nil {
					errOnce.Do(func() { dbErr = err }) // the first database error; the round goes on
				}
				if task != nil {
					task.Add(1)
				}
			}
		})
	}

send:
	for _, m := range pending {
		select {
		case videos <- m: // blocks until a worker is free: this is the backpressure
		case <-ctx.Done(): // shutting down: hand out no more
			break send
		}
	}
	close(videos) // no more work: each worker's range loop ends once it finishes its current video
	wg.Wait()
	sent := int(started.Load())

	err := dbErr
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if task != nil {
		task.Finish(map[string]int{"videos": sent}, err)
	}
	took := time.Since(start).Round(time.Millisecond)
	if ctx.Err() != nil {
		// The ones in flight were stopped, not failed: they're still pending and run next time.
		log.Printf("extractor: %s round stopped by shutdown after %s; %d videos handed out, unfinished ones stay pending", name, took, sent)
	} else {
		log.Printf("extractor: %s round of %d videos with %d workers in %s", name, sent, e.workers(), took)
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

// onePreview makes m's preview. Like one, it returns only database errors.
func (e *Extractor) onePreview(ctx context.Context, m media.Media) error {
	return e.oneClip(ctx, m, clip{"preview", e.PreviewDir, e.Preview, e.Store.SetPreview, e.Store.RecordPreviewFailure})
}

// oneShort makes m's Shorts clip.
func (e *Extractor) oneShort(ctx context.Context, m media.Media) error {
	return e.oneClip(ctx, m, clip{"short", e.ShortDir, e.Short, e.Store.SetShort, e.Store.RecordShortFailure})
}

// clip is one kind of montage the extractor cuts from each video: where it goes, how it's made,
// and how its progress is recorded.
type clip struct {
	name string
	dir  string
	make func(ctx context.Context, in, out string, durationMs int) error
	done func(ctx context.Context, id string, modTime time.Time) error
	fail func(ctx context.Context, id string, reason string) error
}

func (e *Extractor) oneClip(ctx context.Context, m media.Media, c clip) error {
	if m.Video == nil {
		return c.fail(ctx, m.ID, "no duration")
	}
	start := time.Now()
	path := filepath.Join(e.MediaDir, filepath.FromSlash(m.RelPath))
	err := os.MkdirAll(c.dir, 0o755)
	if err == nil {
		err = c.make(ctx, path, PreviewPath(c.dir, m.ID), m.Video.DurationMs)
	}
	if ctx.Err() != nil {
		return nil // shutting down: not the file's fault
	}
	if err != nil {
		log.Printf("extractor: %s %s (%s) failed: %v", c.name, m.ID, m.Title, err)
		return c.fail(ctx, m.ID, err.Error())
	}
	// The ModTime the clip was made from, read before it was made: if the file changed while
	// ffmpeg ran, the clip is already stale and is made again.
	if err := c.done(ctx, m.ID, m.ModTime); err != nil {
		return err
	}
	log.Printf("extractor: %s %s (%s) in %s", c.name, m.ID, m.Title, time.Since(start).Round(time.Millisecond))
	return nil
}
