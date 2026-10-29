package memstore

import (
	"context"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/media/mediatest"
)

func TestRepository(t *testing.T) {
	mediatest.Run(t, func(t *testing.T) mediatest.Store { return New(nil) })
}

func TestSyncWhileReading(t *testing.T) {
	// Run with -race: readers and a writer share the map only through the mutex.
	s := New(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			s.Sync(context.Background(), mediatest.Library(), time.Now())
		}
	}()
	for range 1000 {
		s.List(context.Background(), media.ListFilter{})
	}
	<-done
}
