package api

import (
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/jobs"
)

// Jobs is the background work the API starts and reports on.
// Declared here, by its consumer, so api depends on neither the scanner nor the store.
type Jobs interface {
	// StartScan starts a library scan in the background, or returns the one already running.
	StartScan() jobs.Job
	Get(id string) (jobs.Job, bool)
	List() []jobs.Job // newest first
}

// scanLibrary handles POST /api/scan: 202 Accepted with the scan's job, at once. The scan runs
// in the background; GET /api/jobs/{id} says when it's done. (Before phase 13 the request waited
// for the whole scan: 8 s for 10,000 files, and a phone that gave up cancelled the scan.)
func scanLibrary(js Jobs) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		j := js.StartScan()
		w.Header().Set("Location", "/api/jobs/"+j.ID)
		respondJSON(w, http.StatusAccepted, j)
		return nil
	}
}

// getJob handles GET /api/jobs/{id}.
func getJob(js Jobs) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		j, ok := js.Get(id)
		if !ok {
			// Finished jobs are remembered for a while and lost on restart: a 404 means "ask
			// the library itself", not that the work didn't happen.
			return notFoundError("job %s not found: it may have finished before a restart", id)
		}
		w.Header().Set("Cache-Control", "no-store") // a poller must see the change, never a copy
		respondJSON(w, http.StatusOK, j)
		return nil
	}
}

// listJobs handles GET /api/jobs: recent jobs, newest first.
func listJobs(js Jobs) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusOK, map[string][]jobs.Job{"items": js.List()})
		return nil
	}
}
