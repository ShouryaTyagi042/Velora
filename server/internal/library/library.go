// Package library keeps the stored library in step with the folder: a scan walks the folder and
// syncs what it found into the store, in the background, one scan at a time.
package library

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sync"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
	"github.com/ShouryaTyagi042/Velora/server/internal/media"
	"github.com/ShouryaTyagi042/Velora/server/internal/scanner"
)

// Result is what a finished scan reports.
type Result struct {
	Found    int      `json:"found"`
	Missing  int      `json:"missing"` // items newly marked missing
	Warnings []string `json:"warnings"`
}

// Library connects the scanner to the store.
type Library struct {
	FS    fs.FS
	Store media.Syncer
	Jobs  *jobs.Runner
	// Scanned runs after every successful scan: the server wakes the extractor with it, so new
	// videos get metadata now rather than at its next idle check.
	Scanned func()

	scanMu sync.Mutex // one scan at a time, whoever started it

	mu      sync.Mutex // guards current
	current *jobs.Task // the latest scan job started by StartScan
}

// StartScan starts a scan in the background and returns its job at once. If a scan job is
// already running, that one is returned instead: two scans of the same folder would do the same
// work twice, and the caller only wants to know when the library is current.
//
// The scan runs under the server's context, not the request's. Under the request's (as before
// phase 13), a phone that gave up on a long scan, or was put away, cancelled it, and the whole
// scan's transaction rolled back.
func (l *Library) StartScan() jobs.Job {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil {
		if j, ok := l.Jobs.Registry.Get(l.current.ID()); ok && j.Status == jobs.Running {
			return j
		}
	}
	l.current = l.Jobs.Go("scan", func(ctx context.Context, t *jobs.Task) (any, error) {
		return l.Rescan(ctx)
	})
	j, _ := l.Jobs.Registry.Get(l.current.ID())
	return j
}

// Rescan walks the library and makes the store match what it found, in one transaction, and
// waits for it. The server's startup scan uses it directly; requests go through StartScan.
func (l *Library) Rescan(ctx context.Context) (Result, error) {
	l.scanMu.Lock()
	defer l.scanMu.Unlock()

	start := time.Now()
	res, err := scanner.Scan(ctx, l.FS)
	if err != nil {
		return Result{}, err
	}
	sync, err := l.Store.Sync(ctx, res.Items, start)
	if err != nil {
		return Result{}, fmt.Errorf("save scan: %w", err)
	}

	log.Printf("scan: %d upserted, %d newly missing, %d warnings in %s",
		sync.Upserted, sync.Missing, len(res.Warnings), time.Since(start).Round(time.Millisecond))
	for _, w := range res.Warnings {
		log.Printf("scan warning: %s", w)
	}
	if l.Scanned != nil {
		l.Scanned()
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{} // JSON [] rather than null
	}
	return Result{Found: len(res.Items), Missing: sync.Missing, Warnings: warnings}, nil
}
