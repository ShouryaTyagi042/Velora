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
}

func newStore(videos ...media.Media) *fakeStore {
	return &fakeStore{videos: videos, meta: map[string]media.VideoMeta{}, failures: map[string]int{}}
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
