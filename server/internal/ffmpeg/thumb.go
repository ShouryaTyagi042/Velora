package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Thumbnail writes one frame of the video at in, taken at "at", to out as a 480-px-wide JPEG.
// It writes to a temporary file and renames it into place, so a reader never sees half a JPEG
// and a failed run leaves no file behind.
func Thumbnail(ctx context.Context, in, out string, at time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	tmp, err := os.CreateTemp(filepath.Dir(out), ".thumb-*.jpg")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name()) // a no-op after the rename

	_, err = run(ctx, "ffmpeg",
		"-v", "error",
		"-y", // the temp file exists already
		// -ss before -i seeks by keyframe, which is fast; after -i, ffmpeg decodes every frame up
		// to the exact time.
		"-ss", fmt.Sprintf("%.3f", at.Seconds()),
		"-i", "file:"+in,
		"-frames:v", "1",
		"-vf", "scale=480:-2", // -2: keep the aspect ratio, rounded to an even height
		"-q:v", "3",
		"file:"+tmp.Name(),
	)
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
}
