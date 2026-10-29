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
	if b := img.Bounds(); b.Dx() != 480 || b.Dy() != 360 {
		t.Errorf("thumbnail is %v, want 480×360", b.Size())
	}
}

func TestParseRate(t *testing.T) {
	for in, want := range map[string]float64{"24/1": 24, "24000/1001": 23.976, "0/0": 0, "30": 30, "": 0} {
		if got := parseRate(in); got != want {
			t.Errorf("parseRate(%q) = %v, want %v", in, got, want)
		}
	}
}
