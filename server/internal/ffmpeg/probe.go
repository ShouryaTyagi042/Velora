// Package ffmpeg runs ffprobe and ffmpeg as child processes. Go doesn't parse video containers
// itself: MP4, MKV and WebM are each their own deep format, and FFmpeg already reads them all.
//
// Every call follows the same rules: exec.CommandContext with a timeout (a hung process is
// killed, not leaked), arguments as a slice (no shell, so no quoting or injection), the input
// as a file: URL (so "-i.mp4" isn't a flag and "http:x.mp4" isn't a protocol), stderr captured
// into the error, and JSON output rather than the human-readable text.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// Check reports whether ffprobe and ffmpeg are installed: called at startup, so a missing binary
// fails the server loudly instead of every extraction quietly.
func Check() error {
	for _, bin := range []string{"ffprobe", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found on PATH (brew install ffmpeg): %w", bin, err)
		}
	}
	return nil
}

// probeOutput is the part of ffprobe's JSON Velora reads.
type probeOutput struct {
	Format struct {
		Duration string `json:"duration"` // seconds, as a decimal string
	} `json:"format"`
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"` // a fraction: "24000/1001"
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
}

// Probe reads a video's duration, size, codec and frame rate.
func Probe(ctx context.Context, path string) (media.VideoMeta, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := run(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format", "-show_streams",
		"-i", "file:"+path,
	)
	if err != nil {
		return media.VideoMeta{}, err
	}

	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return media.VideoMeta{}, fmt.Errorf("ffprobe output: %w", err)
	}
	// The container's duration covers every stream; a video stream's can be shorter than the
	// audio's. The format's is what a player shows.
	secs, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil {
		return media.VideoMeta{}, fmt.Errorf("ffprobe duration %q: %w", p.Format.Duration, err)
	}
	for _, s := range p.Streams {
		if s.CodecType != "video" {
			continue
		}
		fps := parseRate(s.AvgFrameRate)
		if fps == 0 {
			fps = parseRate(s.RFrameRate)
		}
		return media.VideoMeta{
			DurationMs: int(secs * 1000),
			Width:      s.Width,
			Height:     s.Height,
			Codec:      s.CodecName,
			FPS:        fps,
		}, nil
	}
	return media.VideoMeta{}, fmt.Errorf("ffprobe %s: no video stream", path)
}

// parseRate turns "24000/1001" into 23.976. "0/0" (unknown) is 0.
func parseRate(r string) float64 {
	num, den, ok := strings.Cut(r, "/")
	if !ok {
		f, _ := strconv.ParseFloat(r, 64)
		return f
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return float64(int(n/d*1000+0.5)) / 1000 // three decimals, like the column
}

// run executes name with args and returns its stdout. A failure carries the start of stderr,
// which is where FFmpeg explains itself.
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("%w (%w)", err, ctx.Err()) // killed by the timeout, not a bad file
		}
		return nil, fmt.Errorf("%s: %w: %s", name, err, truncate(stderr.String(), 500))
	}
	return stdout.Bytes(), nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
