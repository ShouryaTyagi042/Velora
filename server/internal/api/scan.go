package api

import (
	"context"
	"net/http"
)

// Rescanner rescans the media library and swaps in the result.
// Declared here, by its consumer, so api depends on neither the scanner nor the store.
type Rescanner interface {
	Rescan(ctx context.Context) (found int, warnings []string, err error)
}

type scanResponse struct {
	Found    int      `json:"found"`
	Warnings []string `json:"warnings"`
}

// scanLibrary handles POST /api/scan. It blocks until the scan finishes;
// phase 13 moves scanning into the background.
func scanLibrary(lib Rescanner) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		found, warnings, err := lib.Rescan(r.Context())
		if err != nil {
			return err
		}
		if warnings == nil {
			warnings = []string{} // JSON [] rather than null
		}
		respondJSON(w, http.StatusOK, scanResponse{Found: found, Warnings: warnings})
		return nil
	}
}
