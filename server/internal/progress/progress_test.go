package progress

import (
	"errors"
	"testing"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

func ptr(n int) *int { return &n }

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-time.Second)
	tests := []struct {
		name          string
		kind          media.Kind
		s             Save
		wantErr       bool
		wantCompleted bool
	}{
		{"video halfway", media.KindVideo, Save{PositionMs: ptr(50_000), DurationMs: ptr(100_000), ClientTs: ts}, false, false},
		{"video at 95%", media.KindVideo, Save{PositionMs: ptr(95_000), DurationMs: ptr(100_000), ClientTs: ts}, false, true},
		{"video just past the end", media.KindVideo, Save{PositionMs: ptr(103_000), DurationMs: ptr(100_000), ClientTs: ts}, false, true},
		{"video far past the end", media.KindVideo, Save{PositionMs: ptr(200_000), DurationMs: ptr(100_000), ClientTs: ts}, true, false},
		{"video negative", media.KindVideo, Save{PositionMs: ptr(-1), DurationMs: ptr(100_000), ClientTs: ts}, true, false},
		{"video no duration", media.KindVideo, Save{PositionMs: ptr(1), ClientTs: ts}, true, false},
		{"video with a page", media.KindVideo, Save{PositionMs: ptr(1), DurationMs: ptr(10), Page: ptr(1), ClientTs: ts}, true, false},
		{"comic first page", media.KindComic, Save{Page: ptr(0), PageCount: ptr(24), ClientTs: ts}, false, false},
		{"comic last page", media.KindComic, Save{Page: ptr(23), PageCount: ptr(24), ClientTs: ts}, false, true},
		{"comic page out of range", media.KindComic, Save{Page: ptr(24), PageCount: ptr(24), ClientTs: ts}, true, false},
		{"comic with a position", media.KindComic, Save{Page: ptr(1), PageCount: ptr(2), PositionMs: ptr(1), ClientTs: ts}, true, false},
		{"no clientTs", media.KindComic, Save{Page: ptr(0), PageCount: ptr(2)}, true, false},
		{"clientTs from the future", media.KindComic, Save{Page: ptr(0), PageCount: ptr(2), ClientTs: now.Add(time.Hour)}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Validate(tt.kind, tt.s, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tt.wantErr)
			}
			if err != nil {
				if !errors.Is(err, ErrInvalid) {
					t.Errorf("err = %v, want it to wrap ErrInvalid", err)
				}
				return
			}
			if p.Completed != tt.wantCompleted {
				t.Errorf("Completed = %v, want %v", p.Completed, tt.wantCompleted)
			}
		})
	}
}
