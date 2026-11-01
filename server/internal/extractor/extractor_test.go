package extractor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// fakeStore is a queue in memory. Workers call it at the same time, so, like the real database,
// it has to be safe for that: one mutex around everything. (Remove it and run -race.)
type fakeStore struct {
	mu       sync.Mutex
	videos   []media.Media
	meta     map[string]media.VideoMeta
	failures map[string]int

	previews        map[string]time.Time // id → the ModTime its preview was made from
	previewFailures map[string]int
}

func newStore(videos ...media.Media) *fakeStore {
	return &fakeStore{videos: videos, meta: map[string]media.VideoMeta{}, failures: map[string]int{},
		previews: map[string]time.Time{}, previewFailures: map[string]int{}}
}

func (s *fakeStore) PendingPreviews(ctx context.Context, limit int) ([]media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []media.Media
	for _, m := range s.videos {
		v, extracted := s.meta[m.ID]
		made, ok := s.previews[m.ID]
		if extracted && (!ok || !made.Equal(m.ModTime)) && s.previewFailures[m.ID] < MaxAttempts && len(out) < limit {
			m.Video = &v
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *fakeStore) SetPreview(ctx context.Context, id string, modTime time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previews[id] = modTime
	s.previewFailures[id] = 0
	return nil
}

func (s *fakeStore) RecordPreviewFailure(ctx context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previewFailures[id]++
	return nil
}

func (s *fakeStore) setModTime(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.videos {
		if s.videos[i].ID == id {
			s.videos[i].ModTime = t
		}
	}
}

func (s *fakeStore) add(m media.Media) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos = append(s.videos, m)
}

func (s *fakeStore) Pending(ctx context.Context, limit int) ([]media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []media.Media
	for _, m := range s.videos {
		if _, done := s.meta[m.ID]; !done && s.failures[m.ID] < MaxAttempts && len(out) < limit {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *fakeStore) SetVideoMeta(ctx context.Context, id string, v media.VideoMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[id] = v
	return nil
}

func (s *fakeStore) RecordFailure(ctx context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[id]++
	return nil
}

func (s *fakeStore) counts() (done, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.meta), len(s.failures)
}

func videos(n int) []media.Media {
	var out []media.Media
	for i := range n {
		out = append(out, media.Media{ID: fmt.Sprint(i + 1), RelPath: fmt.Sprintf("movies/v%d.mp4", i+1)})
	}
	return out
}

func writeThumb(ctx context.Context, in, out string, at time.Duration) error {
	return os.WriteFile(out, []byte("jpeg"), 0o644)
}

func TestRoundRetriesThenGivesUp(t *testing.T) {
	store := newStore(media.Media{ID: "1", RelPath: "movies/good.mp4"}, media.Media{ID: "2", RelPath: "movies/corrupt.mp4"})
	var thumbAt atomic.Int64
	e := &Extractor{
		Store: store, MediaDir: "/lib", ThumbDir: t.TempDir(), Workers: 2,
		Probe: func(ctx context.Context, path string) (media.VideoMeta, error) {
			if path == filepath.FromSlash("/lib/movies/corrupt.mp4") {
				return media.VideoMeta{}, errors.New("moov atom not found")
			}
			return media.VideoMeta{DurationMs: 600_000, Width: 1920, Height: 1080, Codec: "h264", FPS: 24}, nil
		},
		Thumbnail: func(ctx context.Context, in, out string, at time.Duration) error {
			thumbAt.Store(int64(at))
			return writeThumb(ctx, in, out, at)
		},
	}

	// Round 1 tries both; rounds 2 and 3 only the corrupt one; then nothing is pending.
	var tried []int
	for range 10 {
		n, err := e.Round(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		tried = append(tried, n)
	}
	if fmt.Sprint(tried) != "[2 1 1]" {
		t.Errorf("videos per round = %v, want [2 1 1]", tried)
	}
	if store.meta["1"].Width != 1920 || store.failures["2"] != MaxAttempts {
		t.Errorf("meta = %v, failures = %v", store.meta, store.failures)
	}
	if time.Duration(thumbAt.Load()) != time.Minute {
		t.Errorf("thumbnail taken at %v, want 10%% of 10 min = 1m", time.Duration(thumbAt.Load()))
	}
	if _, err := os.Stat(ThumbPath(e.ThumbDir, "1")); err != nil {
		t.Errorf("no thumbnail: %v", err)
	}
}

// TestBoundIsRespected: 20 videos, 3 workers. At no moment do more than 3 probes run, and 3 do
// run together (so the pool is concurrent at all).
func TestBoundIsRespected(t *testing.T) {
	var running, peak atomic.Int64
	e := &Extractor{
		Store: newStore(videos(20)...), ThumbDir: t.TempDir(), Workers: 3, Thumbnail: writeThumb,
		Probe: func(ctx context.Context, path string) (media.VideoMeta, error) {
			n := running.Add(1)
			for { // peak = max(peak, n), without a lock: retry if another worker changed it first
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
			return media.VideoMeta{DurationMs: 1000}, nil
		},
	}
	reg := jobs.NewRegistry()
	e.Jobs = reg
	if n, err := e.Round(context.Background()); n != 20 || err != nil {
		t.Fatalf("Round = %d, %v", n, err)
	}
	if p := peak.Load(); p != 3 {
		t.Errorf("peak concurrent probes = %d, want 3", p)
	}
	// The round is reported as a job, with its progress complete.
	j, ok := reg.Latest("extract")
	if !ok || j.Status != jobs.Done || j.Progress == nil || j.Progress.Done != 20 || j.Progress.Total != 20 {
		t.Errorf("extract job = %+v (progress %+v)", j, j.Progress)
	}
}

// TestCancelStopsARound: shutdown in the middle of a round. Workers' probes see the cancel (as
// exec.CommandContext would kill ffprobe), the producer hands out nothing more, Round returns
// promptly, and no video is blamed for it.
func TestCancelStopsARound(t *testing.T) {
	store := newStore(videos(50)...)
	started := make(chan struct{}, 50)
	e := &Extractor{
		Store: store, ThumbDir: t.TempDir(), Workers: 4, Thumbnail: writeThumb,
		Probe: func(ctx context.Context, path string) (media.VideoMeta, error) {
			started <- struct{}{}
			<-ctx.Done() // a long probe that only ends when killed
			return media.VideoMeta{}, ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		n   int
		err error
	}
	done := make(chan result)
	go func() { n, err := e.Round(ctx); done <- result{n, err} }()
	for range 4 {
		<-started // all four workers busy
	}
	cancel()
	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) || r.n != 4 {
			t.Errorf("Round = %d, %v; want 4 handed out and context.Canceled", r.n, r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Round kept running after its context was cancelled")
	}
	if done, failed := store.counts(); done != 0 || failed != 0 {
		t.Errorf("after cancel: %d extracted, %d failures recorded; want none (not the files' fault)", done, failed)
	}
}

func TestWakeStartsARound(t *testing.T) {
	store := newStore()
	e := &Extractor{
		Store: store, ThumbDir: t.TempDir(), Workers: 2, Idle: time.Hour, Thumbnail: writeThumb,
		Probe: func(ctx context.Context, path string) (media.VideoMeta, error) {
			return media.VideoMeta{DurationMs: 1000}, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { e.Run(ctx); close(stopped) }()

	store.add(media.Media{ID: "1", RelPath: "movies/new.mp4"}) // a scan found a new video
	e.Wake()
	e.Wake() // a second wake while the first is pending must not block
	deadline := time.Now().Add(time.Second)
	for done, _ := store.counts(); done == 0; done, _ = store.counts() {
		if time.Now().After(deadline) {
			t.Fatal("Wake didn't start a round (Idle is an hour)")
		}
		time.Sleep(time.Millisecond)
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Run kept running after its context was cancelled")
	}
}

func probeOK(ctx context.Context, path string) (media.VideoMeta, error) {
	return media.VideoMeta{DurationMs: 600_000, Width: 1920, Height: 1080, Codec: "h264", FPS: 24}, nil
}

// TestPreviewRound: previews come only after metadata, in batches of previewsPerWorker per
// worker, failures are retried up to MaxAttempts, and a changed file gets a new preview.
func TestPreviewRound(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var vids []media.Media
	for _, m := range videos(5) {
		m.ModTime = t0
		vids = append(vids, m)
	}
	vids[4].RelPath = "movies/corrupt.mp4"
	store := newStore(vids...)
	dir := t.TempDir()
	var made atomic.Int64
	e := &Extractor{
		Store: store, MediaDir: "/lib", ThumbDir: t.TempDir(), PreviewDir: dir, Workers: 1,
		Probe: probeOK, Thumbnail: writeThumb,
		Preview: func(ctx context.Context, in, out string, durationMs int) error {
			if durationMs != 600_000 {
				t.Errorf("Preview got durationMs %d, want 600000", durationMs)
			}
			if in == filepath.FromSlash("/lib/movies/corrupt.mp4") {
				return errors.New("invalid data found")
			}
			made.Add(1)
			return os.WriteFile(out, []byte("mp4"), 0o644)
		},
	}
	ctx := context.Background()

	// No metadata yet: nothing to preview.
	if n, err := e.PreviewRound(ctx); n != 0 || err != nil {
		t.Fatalf("PreviewRound before metadata = %d, %v; want 0", n, err)
	}
	if _, err := e.Round(ctx); err != nil {
		t.Fatal(err)
	}

	// One worker: batches of 2.
	var tried []int
	for range 10 {
		n, err := e.PreviewRound(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		tried = append(tried, n)
	}
	// 5 pending: [2 2 1], then the corrupt one alone until it has failed MaxAttempts times.
	if fmt.Sprint(tried) != "[2 2 1 1 1]" {
		t.Errorf("previews per round = %v, want [2 2 1 1 1]", tried)
	}
	if made.Load() != 4 || store.previewFailures["5"] != MaxAttempts {
		t.Errorf("made %d, corrupt failures %d; want 4 and %d", made.Load(), store.previewFailures["5"], MaxAttempts)
	}
	if _, err := os.Stat(PreviewPath(dir, "1")); err != nil {
		t.Errorf("preview file: %v", err)
	}

	// The file is replaced: its preview is stale and made again, once.
	store.setModTime("2", t0.Add(time.Hour))
	if n, _ := e.PreviewRound(ctx); n != 1 {
		t.Errorf("PreviewRound after a change = %d, want 1", n)
	}
	if !store.previews["2"].Equal(t0.Add(time.Hour)) {
		t.Errorf("preview of 2 made for %v, want the new modTime", store.previews["2"])
	}
}

func TestPreviewRoundWithoutDirMakesNone(t *testing.T) {
	store := newStore(videos(1)...)
	e := &Extractor{Store: store, MediaDir: "/lib", ThumbDir: t.TempDir(), Probe: probeOK, Thumbnail: writeThumb,
		Preview: func(ctx context.Context, in, out string, durationMs int) error {
			t.Error("Preview called without a PreviewDir")
			return nil
		}}
	e.Round(context.Background())
	if n, err := e.PreviewRound(context.Background()); n != 0 || err != nil {
		t.Errorf("PreviewRound = %d, %v; want 0", n, err)
	}
}
