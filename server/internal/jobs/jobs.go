// Package jobs runs Velora's background work and remembers how it went: scans and metadata
// extraction run outside the request that asked for them, and the API reports their status.
//
// The state is in memory: a restart forgets finished jobs, which is fine for a personal server
// (ADR 010). What the work itself depends on lives in the database ("which videos still need
// metadata" is a query), so a restart loses no work, only the history.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Status is where a job is in its life.
type Status string

const (
	Running  Status = "running"
	Done     Status = "done"
	Failed   Status = "failed"
	Canceled Status = "canceled" // stopped by shutdown: not the job's fault, so not "failed"
)

// Job is a snapshot of one job, as the API sends it.
type Job struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Status     Status     `json:"status"`
	Progress   *Progress  `json:"progress,omitempty"`
	Result     any        `json:"result,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// Progress is how far a job is: Done of Total units (files, videos).
type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// Task is a running job's handle: the code doing the work reports progress through it.
//
// done and total are atomics, not fields under the registry's lock: eight workers each finishing
// a video call Add(1), and an atomic add is one instruction where a lock is a queue. The rest of
// the job (status, result) changes once or twice in its life and stays under the mutex.
type Task struct {
	reg         *Registry
	id          string
	done, total atomic.Int64
}

// ID is the job's id, for the API to hand back.
func (t *Task) ID() string { return t.id }

// SetTotal says how many units the job has; until it's called the job has no progress to show.
func (t *Task) SetTotal(n int64) { t.total.Store(n) }

// Add records n more units done. Safe from any number of goroutines at once.
func (t *Task) Add(n int64) { t.done.Add(n) }

// Finish records the outcome. err == nil is done; a context error is canceled; anything else
// is failed, and logged. Finishing twice keeps the first outcome.
func (t *Task) Finish(result any, err error) {
	r := t.reg
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[t.id]
	if j == nil || j.Status != Running {
		return
	}
	now := r.now()
	j.FinishedAt = &now
	switch {
	case err == nil:
		j.Status, j.Result = Done, result
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		j.Status, j.Error = Canceled, err.Error()
	default:
		// The full error can name paths and database details: it goes to the log, and the API
		// says only where to look, as writeError does for a failed request.
		log.Printf("job %s (%s) failed: %v", t.id, j.Name, err)
		j.Status, j.Error = Failed, "failed; the server log has the details (job "+t.id+")"
	}
}

// Registry remembers the most recent jobs.
type Registry struct {
	mu    sync.Mutex
	jobs  map[string]*Job
	tasks map[string]*Task // the running ones, for their live progress
	order []string         // ids, oldest first
	keep  int
	now   func() time.Time
}

// keepJobs is how many finished jobs the registry remembers.
const keepJobs = 50

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{jobs: map[string]*Job{}, tasks: map[string]*Task{}, keep: keepJobs, now: time.Now}
}

// Start records a new running job called name and returns its handle.
func (r *Registry) Start(name string) *Task {
	b := make([]byte, 6)
	rand.Read(b) // random rather than a counter: after a restart, an old id can't name a new job
	t := &Task{reg: r, id: "j_" + hex.EncodeToString(b)}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[t.id] = &Job{ID: t.id, Name: name, Status: Running, StartedAt: r.now()}
	r.tasks[t.id] = t
	r.order = append(r.order, t.id)
	r.trim()
	return t
}

// trim forgets the oldest finished jobs beyond keep. A running job is never forgotten. Callers hold r.mu.
func (r *Registry) trim() {
	for i := 0; len(r.order) > r.keep && i < len(r.order); {
		id := r.order[i]
		if r.jobs[id].Status == Running {
			i++
			continue
		}
		delete(r.jobs, id)
		delete(r.tasks, id)
		r.order = slices.Delete(r.order, i, i+1)
	}
}

// Get returns a snapshot of the job with id.
func (r *Registry) Get(id string) (Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return Job{}, false
	}
	return r.snapshot(j), true
}

// List returns snapshots of the remembered jobs, newest first.
func (r *Registry) List() []Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Job, 0, len(r.order))
	for _, id := range slices.Backward(r.order) {
		out = append(out, r.snapshot(r.jobs[id]))
	}
	return out
}

// Latest returns the newest job called name, if any is remembered.
func (r *Registry) Latest(name string) (Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range slices.Backward(r.order) {
		if j := r.jobs[id]; j.Name == name {
			return r.snapshot(j), true
		}
	}
	return Job{}, false
}

// snapshot copies j with its current progress. A copy, so the caller can't race with Finish.
// Callers hold r.mu.
func (r *Registry) snapshot(j *Job) Job {
	c := *j
	if t := r.tasks[j.ID]; t != nil {
		if total := t.total.Load(); total > 0 {
			c.Progress = &Progress{Done: t.done.Load(), Total: total}
		}
	}
	return c
}

// Runner starts jobs in goroutines it can wait for: shutdown cancels ctx, then waits, so no job
// is still writing when the database closes.
type Runner struct {
	Registry *Registry
	ctx      context.Context
	wg       sync.WaitGroup
}

// NewRunner returns a Runner whose jobs run under ctx.
func NewRunner(ctx context.Context, reg *Registry) *Runner {
	return &Runner{Registry: reg, ctx: ctx}
}

// Go starts fn as a job called name and returns its handle at once. fn's return value is the
// job's result or error; a panic in fn fails the job instead of crashing the server.
func (r *Runner) Go(name string, fn func(ctx context.Context, t *Task) (any, error)) *Task {
	t := r.Registry.Start(name)
	// wg.Go does the Add before the goroutine starts. An Add inside the goroutine could run after
	// Wait had already seen zero, and Wait would return with the job still running.
	r.wg.Go(func() {
		var (
			result any
			err    error
		)
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("panic: %v", p) // Finish logs it
			}
			t.Finish(result, err)
		}()
		result, err = fn(r.ctx, t)
	})
	return t
}

// Wait blocks until every job started with Go has returned.
func (r *Runner) Wait() { r.wg.Wait() }
