// Package progress holds the rules about progress that don't depend on storage or HTTP:
// when a save is valid, and when an item counts as finished. The server decides "finished",
// never the client, so the rule lives in one place.
package progress

import (
	"errors"
	"fmt"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

const (
	// FinishedAt is how far into a video counts as watched: the credits don't need watching.
	FinishedAt = 0.95
	// durationSlack allows a reported position a little past the reported duration: players
	// round, and the last timeUpdate can land after the end.
	durationSlack = 5 * time.Second
	// futureSlack is how far ahead of the server's clock a client's timestamp may be. A phone
	// whose clock runs far ahead would otherwise win every save for days.
	futureSlack = 10 * time.Minute
)

// Save is one PUT /api/media/{id}/progress body: the whole current state, not a change.
// Pointers tell "missing" from zero.
type Save struct {
	PositionMs *int      `json:"positionMs"` // videos
	DurationMs *int      `json:"durationMs"`
	Page       *int      `json:"page"` // comics
	PageCount  *int      `json:"pageCount"`
	ClientTs   time.Time `json:"clientTs"`
}

// ErrInvalid wraps every validation failure; its message is safe to show to the client.
var ErrInvalid = errors.New("invalid progress")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Validate checks s for an item of kind and returns it as Progress, with Completed computed.
func Validate(kind media.Kind, s Save, now time.Time) (media.Progress, error) {
	if s.ClientTs.IsZero() {
		return media.Progress{}, invalid("clientTs is required")
	}
	if s.ClientTs.After(now.Add(futureSlack)) {
		return media.Progress{}, invalid("clientTs is in the future; check the device's clock")
	}
	p := media.Progress{UpdatedAt: s.ClientTs.UTC()}

	switch kind {
	case media.KindVideo:
		if s.Page != nil || s.PageCount != nil {
			return media.Progress{}, invalid("a video's progress is positionMs and durationMs, not pages")
		}
		if s.PositionMs == nil || s.DurationMs == nil {
			return media.Progress{}, invalid("positionMs and durationMs are required")
		}
		pos, dur := *s.PositionMs, *s.DurationMs
		switch {
		case dur <= 0:
			return media.Progress{}, invalid("durationMs must be positive")
		case pos < 0:
			return media.Progress{}, invalid("positionMs must not be negative")
		case pos > dur+int(durationSlack.Milliseconds()):
			return media.Progress{}, invalid("positionMs is past the end of the video")
		}
		p.PositionMs, p.DurationMs = min(pos, dur), dur
		p.Completed = float64(p.PositionMs) >= FinishedAt*float64(dur)

	case media.KindComic:
		if s.PositionMs != nil || s.DurationMs != nil {
			return media.Progress{}, invalid("a comic's progress is page and pageCount, not a position")
		}
		if s.Page == nil || s.PageCount == nil {
			return media.Progress{}, invalid("page and pageCount are required")
		}
		page, count := *s.Page, *s.PageCount
		switch {
		case count <= 0:
			return media.Progress{}, invalid("pageCount must be positive")
		case page < 0 || page >= count:
			return media.Progress{}, invalid("page must be between 0 and pageCount-1")
		}
		p.Page, p.PageCount = page, count
		p.Completed = page >= count-1 // the last page is open

	default:
		return media.Progress{}, invalid("unknown kind %q", kind)
	}
	return p, nil
}
