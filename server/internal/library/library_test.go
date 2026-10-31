package library

import (
	"context"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

var folder = fstest.MapFS{
	"movies/Keanu Reeves/The Matrix.mp4": {Data: []byte("x")},
	"movies/Loose Clip.mp4":              {Data: []byte("x")},
}

// slowStore holds every Sync until release is closed, so a test can act while a scan runs.
type slowStore struct {
	media.Syncer
	release chan struct{}
	syncs   atomic.Int64
}

func (s *slowStore) Sync(ctx context.Context, items []media.Media, at time.Time) (media.SyncResult, error) {
	s.syncs.Add(1)
	select {
	case <-s.release:
	case <-ctx.Done():
		return media.SyncResult{}, ctx.Err()
	}
	return s.Syncer.Sync(ctx, items, at)
}

func TestStartScanRunsInTheBackground(t *testing.T) {
	store := &slowStore{Syncer: memstore.New(nil), release: make(chan struct{})}
	runner := jobs.NewRunner(context.Background(), jobs.NewRegistry())
	var woke atomic.Int64
	lib := &Library{FS: folder, Store: store, Jobs: runner, Scanned: func() { woke.Add(1) }}

	first := lib.StartScan() // returns while the scan is still blocked in Sync
	if first.Status != jobs.Running {
		t.Fatalf("StartScan = %+v, want a running job", first)
	}
	// Ten more requests while it runs: all get the same job, and no second scan starts.
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if j := lib.StartScan(); j.ID != first.ID {
				t.Errorf("concurrent StartScan = %s, want the running %s", j.ID, first.ID)
			}
		})
	}
	wg.Wait()

	close(store.release)
	runner.Wait()
	j, _ := runner.Registry.Get(first.ID)
	res, _ := j.Result.(Result)
	if j.Status != jobs.Done || res.Found != 2 || store.syncs.Load() != 1 || woke.Load() != 1 {
		t.Errorf("job %+v, %d syncs, %d wakes; want done with 2 found, 1 sync, 1 wake", j, store.syncs.Load(), woke.Load())
	}

	// Once it has finished, the next request starts a new scan.
	if next := lib.StartScan(); next.ID == first.ID {
		t.Error("StartScan after the scan finished returned the old job")
	}
	runner.Wait()
}

func TestShutdownCancelsAScan(t *testing.T) {
	store := &slowStore{Syncer: memstore.New(nil), release: make(chan struct{})} // never released
	ctx, cancel := context.WithCancel(context.Background())
	runner := jobs.NewRunner(ctx, jobs.NewRegistry())
	lib := &Library{FS: folder, Store: store, Jobs: runner}

	j := lib.StartScan()
	cancel()
	runner.Wait() // returns: the scan saw the cancel
	if got, _ := runner.Registry.Get(j.ID); got.Status != jobs.Canceled {
		t.Errorf("scan after shutdown = %s, want canceled", got.Status)
	}
}
