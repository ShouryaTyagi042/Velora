package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PreviewMoments are where a preview's cuts come from, as fractions of the duration: past the
// opening credits, before the ending.
var PreviewMoments = []float64{0.15, 0.35, 0.55, 0.75}

// previewCut is how long each cut is. A video too short for four of them gets shorter cuts.
const previewCut = 3 * time.Second

// Preview writes a short silent montage of the video in to out: a cut at each of PreviewMoments,
// concatenated into one small MP4 that starts playing at once. durationMs is the video's
// duration (from Probe), which places the cuts.
//
// Each cut is re-encoded small (at most 360 px high, never scaled up; 24 fps; CRF 28) with a
// keyframe every second, so a player can start anywhere without waiting. Audio, subtitles, data
// streams and metadata are all dropped: a camera's timecode tag would otherwise come through as a
// data track. The cuts are then joined without re-encoding, with the moov atom moved to the front
// (+faststart): one round trip to the first frame.
//
// The work happens in a temporary folder next to out, removed afterwards whatever happens, and
// out appears only once complete (a rename), so a half-written preview is never served.
func Preview(ctx context.Context, in, out string, durationMs int) error {
	if durationMs <= 0 {
		return fmt.Errorf("preview: unknown duration")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	dir, err := os.MkdirTemp(filepath.Dir(out), ".preview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	duration := time.Duration(durationMs) * time.Millisecond
	cut := min(previewCut, duration/time.Duration(len(PreviewMoments)))
	var list strings.Builder
	for i, at := range PreviewMoments {
		seg := filepath.Join(dir, fmt.Sprintf("cut%d.mp4", i))
		_, err := run(ctx, "ffmpeg",
			"-v", "error", "-y",
			"-ss", fmt.Sprintf("%.3f", (time.Duration(float64(duration)*at)).Seconds()), // before -i: keyframe seek, fast
			"-t", fmt.Sprintf("%.3f", cut.Seconds()),
			"-i", "file:"+in,
			"-map", "0:v:0", "-an", "-sn", "-dn", "-map_metadata", "-1",
			// -2: keep the aspect ratio with an even width; min(…, ih): never scale up.
			"-vf", "scale=-2:'trunc(min(360,ih)/2)*2',fps=24",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "28", "-g", "24", "-pix_fmt", "yuv420p",
			"file:"+seg,
		)
		if err != nil {
			return fmt.Errorf("preview cut %d: %w", i+1, err)
		}
		fmt.Fprintf(&list, "file '%s'\n", filepath.Base(seg)) // relative to the list file: no quoting of the real path
	}
	listPath := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}

	joined := filepath.Join(dir, "preview.mp4")
	if _, err := run(ctx, "ffmpeg",
		"-v", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", "file:"+listPath,
		"-c", "copy", "-map_metadata", "-1", "-movflags", "+faststart",
		"file:"+joined,
	); err != nil {
		return fmt.Errorf("preview concat: %w", err)
	}
	return os.Rename(joined, out)
}
