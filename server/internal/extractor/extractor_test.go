package extractor

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// fakeStore hands out its pending videos in order and records what the extractor reports.
type fakeStore struct {
	pending  []media.Media
	meta     map[string]media.VideoMeta
	failures map[string]int
}

func (s *fakeStore) NextPending(ctx context.Context) (media.Media, bool, error) {
	for len(s.pending) > 0 {
		m := s.pending[0]
		if s.failures[m.ID] < MaxAttempts && s.meta[m.ID] == (media.VideoMeta{}) {
			return m, true, nil
		}
		s.pending = s.pending[1:]
	}
	return media.Media{}, false, nil
}

func (s *fakeStore) SetVideoMeta(ctx context.Context, id string, v media.VideoMeta) error {
	s.meta[id] = v
	return nil
}

func (s *fakeStore) RecordFailure(ctx context.Context, id, reason string) error {
	s.failures[id]++
	return nil
}

func TestOnce(t *testing.T) {
	store := &fakeStore{
		pending:  []media.Media{{ID: "1", RelPath: "movies/good.mp4"}, {ID: "2", RelPath: "movies/corrupt.mp4"}},
		meta:     map[string]media.VideoMeta{},
		failures: map[string]int{},
	}
	var thumbAt time.Duration
	e := &Extractor{
		Store: store, MediaDir: "/lib", ThumbDir: t.TempDir(),
		Probe: func(ctx context.Context, path string) (media.VideoMeta, error) {
			if path == filepath.FromSlash("/lib/movies/corrupt.mp4") {
				return media.VideoMeta{}, errors.New("moov atom not found")
			}
			return media.VideoMeta{DurationMs: 600_000, Width: 1920, Height: 1080, Codec: "h264", FPS: 24}, nil
		},
		Thumbnail: func(ctx context.Context, in, out string, at time.Duration) error {
			thumbAt = at
			return os.WriteFile(out, []byte("jpeg"), 0o644)
		},
	}

	// The corrupt file is tried MaxAttempts times, then never again: 1 good + 3 attempts = 4 runs.
	runs := 0
	for {
		did, err := e.Once(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !did {
			break
		}
		if runs++; runs > 10 {
			t.Fatal("extractor never stops retrying")
		}
	}
	if runs != 1+MaxAttempts {
		t.Errorf("runs = %d, want %d", runs, 1+MaxAttempts)
	}
	if store.meta["1"].Width != 1920 || store.failures["2"] != MaxAttempts {
		t.Errorf("meta = %v, failures = %v", store.meta, store.failures)
	}
	if thumbAt != time.Minute {
		t.Errorf("thumbnail taken at %v, want 10%% of 10 min = 1m", thumbAt)
	}
	if _, err := os.Stat(ThumbPath(e.ThumbDir, "1")); err != nil {
		t.Errorf("no thumbnail: %v", err)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	e := &Extractor{Store: &fakeStore{meta: map[string]media.VideoMeta{}, failures: map[string]int{}}, Idle: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run kept running after its context was cancelled")
	}
}
