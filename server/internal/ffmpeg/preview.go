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

// ShortMoments are where a short's cuts come from: more of them, spread across the whole video,
// since a short is watched, not glanced at.
var ShortMoments = []float64{0.08, 0.20, 0.32, 0.44, 0.56, 0.68, 0.80, 0.92}

// montage is the shape of a cut-together clip of a video.
type montage struct {
	name      string        // for errors and the temp folder
	moments   []float64     // where each cut starts, as fractions of the duration
	cut       time.Duration // how long each cut is (shorter if the video is too short for them all)
	maxHeight int           // never scaled up
	crf       int
	audio     bool
	timeout   time.Duration
}

// preview is the long-press preview: four 3 s cuts, small and silent, so it starts at once.
var preview = montage{name: "preview", moments: PreviewMoments, cut: 3 * time.Second, maxHeight: 360, crf: 28, timeout: 3 * time.Minute}

// short is what the Shorts feed plays full screen: eight 8 s cuts (about a minute), sharp enough
// for a phone's whole width (720 px high at most), with sound (the feed has a mute button).
var short = montage{name: "short", moments: ShortMoments, cut: 8 * time.Second, maxHeight: 720, crf: 26, audio: true, timeout: 10 * time.Minute}

// Preview writes a short silent montage of the video in to out: a cut at each of PreviewMoments,
// concatenated into one small MP4 that starts playing at once. durationMs is the video's
// duration (from Probe), which places the cuts.
func Preview(ctx context.Context, in, out string, durationMs int) error {
	return preview.write(ctx, in, out, durationMs)
}

// Short writes the Shorts feed's clip of the video in to out: a cut at each of ShortMoments, with
// its audio, if it has any.
func Short(ctx context.Context, in, out string, durationMs int) error {
	return short.write(ctx, in, out, durationMs)
}

// write makes the montage. Each cut is re-encoded (24 fps, H.264) with a keyframe every second,
// so a player can start anywhere without waiting; audio, when kept, as AAC. Subtitles, data
// streams and metadata are all dropped: a camera's timecode tag would otherwise come through as a
// data track. The cuts are then joined without re-encoding, with the moov atom moved to the front
// (+faststart): one round trip to the first frame.
//
// The work happens in a temporary folder next to out, removed afterwards whatever happens, and
// out appears only once complete (a rename), so a half-written clip is never served.
func (mt montage) write(ctx context.Context, in, out string, durationMs int) error {
	if durationMs <= 0 {
		return fmt.Errorf("%s: unknown duration", mt.name)
	}
	ctx, cancel := context.WithTimeout(ctx, mt.timeout)
	defer cancel()

	dir, err := os.MkdirTemp(filepath.Dir(out), "."+mt.name+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	duration := time.Duration(durationMs) * time.Millisecond
	cut := min(mt.cut, duration/time.Duration(len(mt.moments)))
	audio := []string{"-an"}
	if mt.audio {
		// 0:a:0? : the first audio track if there is one; a silent file gives a silent clip.
		audio = []string{"-map", "0:a:0?", "-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000"}
	}
	var list strings.Builder
	for i, at := range mt.moments {
		seg := filepath.Join(dir, fmt.Sprintf("cut%d.mp4", i))
		args := []string{
			"-v", "error", "-y",
			"-ss", fmt.Sprintf("%.3f", (time.Duration(float64(duration) * at)).Seconds()), // before -i: keyframe seek, fast
			"-t", fmt.Sprintf("%.3f", cut.Seconds()),
			"-i", "file:" + in,
			"-map", "0:v:0", "-sn", "-dn", "-map_metadata", "-1",
			// -2: keep the aspect ratio with an even width; min(…, ih): never scale up.
			"-vf", fmt.Sprintf("scale=-2:'trunc(min(%d,ih)/2)*2',fps=24", mt.maxHeight),
			"-c:v", "libx264", "-preset", "veryfast", "-crf", fmt.Sprint(mt.crf), "-g", "24", "-pix_fmt", "yuv420p",
		}
		args = append(append(args, audio...), "file:"+seg)
		if _, err := run(ctx, "ffmpeg", args...); err != nil {
			return fmt.Errorf("%s cut %d: %w", mt.name, i+1, err)
		}
		fmt.Fprintf(&list, "file '%s'\n", filepath.Base(seg)) // relative to the list file: no quoting of the real path
	}
	listPath := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}

	joined := filepath.Join(dir, mt.name+".mp4")
	if _, err := run(ctx, "ffmpeg",
		"-v", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", "file:"+listPath,
		"-c", "copy", "-map_metadata", "-1", "-movflags", "+faststart",
		"file:"+joined,
	); err != nil {
		return fmt.Errorf("%s concat: %w", mt.name, err)
	}
	return os.Rename(joined, out)
}
