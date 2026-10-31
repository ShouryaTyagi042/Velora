package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestOutcomes(t *testing.T) {
	reg := NewRegistry()
	r := NewRunner(context.Background(), reg)
	ok := r.Go("ok", func(ctx context.Context, t *Task) (any, error) { return map[string]int{"found": 3}, nil })
	bad := r.Go("bad", func(ctx context.Context, t *Task) (any, error) { return nil, errors.New("disk gone") })
	stopped := r.Go("stopped", func(ctx context.Context, t *Task) (any, error) {
		return nil, fmt.Errorf("scan: %w", context.Canceled) // wrapped, as real errors are
	})
	boom := r.Go("boom", func(ctx context.Context, t *Task) (any, error) { panic("nil map") })
	r.Wait()

	for _, c := range []struct {
		task *Task
		want Status
	}{{ok, Done}, {bad, Failed}, {stopped, Canceled}, {boom, Failed}} {
		j, found := reg.Get(c.task.ID())
		if !found || j.Status != c.want || j.FinishedAt == nil {
			t.Errorf("%s: %+v, want %s and finished", j.Name, j, c.want)
		}
	}
	if j, _ := reg.Get(ok.ID()); fmt.Sprint(j.Result) != "map[found:3]" {
		t.Errorf("result = %v", j.Result)
	}
	// A failure's details stay in the log: they can name paths.
	if j, _ := reg.Get(bad.ID()); strings.Contains(j.Error, "disk gone") || !strings.Contains(j.Error, bad.ID()) {
		t.Errorf("failed job's error = %q, want a pointer to the log, not the cause", j.Error)
	}
	if names := fmt.Sprint(names(reg.List())); names != "[boom stopped bad ok]" {
		t.Errorf("List = %s, want newest first", names)
	}
}

// TestProgressFromManyGoroutines is the counter from chapter 3: 8 goroutines × 1000 Add(1).
// With a plain int64 and done++, -race reports it and the total comes up short.
func TestProgressFromManyGoroutines(t *testing.T) {
	reg := NewRegistry()
	task := reg.Start("extract")
	task.SetTotal(8000)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 1000 {
				task.Add(1)
				reg.Get(task.ID()) // and someone polling the API meanwhile
			}
		})
	}
	wg.Wait()
	j, _ := reg.Get(task.ID())
	if j.Progress == nil || j.Progress.Done != 8000 || j.Progress.Total != 8000 {
		t.Errorf("progress = %+v, want 8000 of 8000", j.Progress)
	}
}

func TestFinishedJobsAreForgottenRunningOnesAreNot(t *testing.T) {
	reg := NewRegistry()
	reg.keep = 3
	running := reg.Start("long")
	for i := range 5 {
		reg.Start(fmt.Sprint(i)).Finish(nil, nil)
	}
	if got := fmt.Sprint(names(reg.List())); got != "[4 3 long]" {
		t.Errorf("List = %s, want the 2 newest finished and the running one", got)
	}
	if _, ok := reg.Get(running.ID()); !ok {
		t.Error("a running job was forgotten")
	}
	if _, ok := reg.Latest("0"); ok {
		t.Error("the oldest finished job is still remembered")
	}
}

func TestFinishTwiceKeepsTheFirst(t *testing.T) {
	reg := NewRegistry()
	task := reg.Start("scan")
	task.Finish("first", nil)
	task.Finish(nil, errors.New("second"))
	if j, _ := reg.Get(task.ID()); j.Status != Done || j.Result != "first" {
		t.Errorf("job = %+v, want the first outcome", j)
	}
}

func names(js []Job) []string {
	var out []string
	for _, j := range js {
		out = append(out, j.Name)
	}
	return out
}
