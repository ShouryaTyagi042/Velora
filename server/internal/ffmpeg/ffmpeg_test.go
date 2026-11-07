package ffmpeg

import (
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// testVideo generates a 2 s, 320×240, 24 fps H.264 clip with ffmpeg's built-in test source, so
// no binary fixture lives in git. Its name starts with "-" on purpose: it must not become a flag.
func testVideo(t *testing.T) string {
	t.Helper()
	if err := Check(); err != nil {
		t.Skip(err)
	}
	p := filepath.Join(t.TempDir(), "-i clip.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=24",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "file:"+p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate a test video: %v: %s", err, out)
	}
	return p
}

func TestProbe(t *testing.T) {
	v, err := Probe(context.Background(), testVideo(t))
	if err != nil {
		t.Fatal(err)
	}
	if v.Width != 320 || v.Height != 240 || v.Codec != "h264" || v.FPS != 24 {
		t.Errorf("Probe = %+v, want 320×240 h264 at 24 fps", v)
	}
	if v.DurationMs < 1900 || v.DurationMs > 2100 {
		t.Errorf("DurationMs = %d, want about 2000", v.DurationMs)
	}
}

func TestProbeNotAVideo(t *testing.T) {
	if err := Check(); err != nil {
		t.Skip(err)
	}
	p := filepath.Join(t.TempDir(), "fake.mp4")
	os.WriteFile(p, []byte("not a video"), 0o644)
	if _, err := Probe(context.Background(), p); err == nil {
		t.Error("Probe(garbage) = nil error")
	}
}

func TestThumbnail(t *testing.T) {
	in := testVideo(t)
	out := filepath.Join(t.TempDir(), "thumb.jpg")
	if err := Thumbnail(context.Background(), in, out, time.Second); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 240 { // narrower than 960: kept, not enlarged
		t.Errorf("thumbnail is %v, want 320×240", b.Size())
	}
}

func TestParseRate(t *testing.T) {
	for in, want := range map[string]float64{"24/1": 24, "24000/1001": 23.976, "0/0": 0, "30": 30, "": 0} {
		if got := parseRate(in); got != want {
			t.Errorf("parseRate(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestPreview cuts a preview from a generated 60 s, 640×480 video with an audio track: four 3 s
// cuts at 24 fps, scaled to 360 high, silent, and nothing left behind in the folder but the result.
func TestPreview(t *testing.T) {
	if err := Check(); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "-film.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=60:size=640x480:rate=30",
		"-f", "lavfi", "-i", "sine=duration=60",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "file:"+in)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate a test video: %v: %s", err, out)
	}

	out := filepath.Join(dir, "preview.mp4")
	if err := Preview(context.Background(), in, out, 60_000); err != nil {
		t.Fatal(err)
	}
	v, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if v.Width != 480 || v.Height != 360 || v.Codec != "h264" || v.FPS != 24 {
		t.Errorf("preview = %+v, want 480×360 h264 at 24 fps", v)
	}
	if v.DurationMs < 11_500 || v.DurationMs > 12_500 {
		t.Errorf("preview DurationMs = %d, want about 12000 (4 × 3 s)", v.DurationMs)
	}
	streams, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", "file:"+out).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(streams); got != "video\n" {
		t.Errorf("preview streams = %q, want only one video stream", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("folder holds %d entries, want 2 (the film and the preview): the temp folder leaked", len(entries))
	}
}

// TestShort cuts the Shorts clip from a generated 120 s video with sound: eight 8 s cuts, audio
// kept, never scaled up (480 high stays 480).
func TestShort(t *testing.T) {
	if err := Check(); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "film.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=120:size=640x480:rate=30",
		"-f", "lavfi", "-i", "sine=duration=120",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "file:"+in)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("can't generate a test video: %v: %s", err, out)
	}
	out := filepath.Join(dir, "short.mp4")
	if err := Short(context.Background(), in, out, 120_000); err != nil {
		t.Fatal(err)
	}
	v, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if v.Width != 640 || v.Height != 480 || v.FPS != 24 {
		t.Errorf("short = %+v, want 640×480 (not enlarged) at 24 fps", v)
	}
	if v.DurationMs < 63_000 || v.DurationMs > 65_000 {
		t.Errorf("short DurationMs = %d, want about 64000 (8 × 8 s)", v.DurationMs)
	}
	streams, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", "file:"+out).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(streams); got != "video\naudio\n" {
		t.Errorf("short streams = %q, want video and audio", got)
	}
}

func TestPreviewShortVideoAndNoUpscale(t *testing.T) {
	in := testVideo(t) // 2 s, 320×240
	out := filepath.Join(t.TempDir(), "preview.mp4")
	if err := Preview(context.Background(), in, out, 2000); err != nil {
		t.Fatal(err)
	}
	v, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if v.Height != 240 {
		t.Errorf("height = %d, want 240: a preview must never scale up", v.Height)
	}
}

func TestPreviewFailureLeavesNothing(t *testing.T) {
	if err := Check(); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "fake.mp4")
	os.WriteFile(in, []byte("not a video"), 0o644)
	if err := Preview(context.Background(), in, filepath.Join(dir, "preview.mp4"), 60_000); err == nil {
		t.Fatal("Preview(garbage) = nil error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("folder holds %d entries after a failure, want only the input", len(entries))
	}
}
