package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// The library contract (ADR 013):
//
//	movies/<actor>/<title>.<video-ext>     one video; folder actor = <actor>
//	movies/<actor>/<sub>/…/<title>.<ext>   still <actor> (first segment under movies/)
//	movies/<title>.<video-ext>             one video, no actors
//	anime/<series>/…/<title>.<video-ext>   one episode of <series> (a show, not an actor: no actors)
//	anime/<title>.<video-ext>              one episode, no series
//	comics/<title>/<page>.<image-ext>      the pages of ONE comic called <title>
//	anything else at the root              ignored, with a warning
const (
	moviesDir = "movies"
	animeDir  = "anime"
	comicsDir = "comics"
)

// folderActor returns the actor for a video at relPath ("movies/<actor>/…/x.mp4"), or "" when the
// file sits directly in movies/, or isn't under movies/ at all: anime/<series>/ is a series
// (media.SeriesOf), not an actor.
// fs.FS paths always use "/", whatever the OS, so splitting on "/" is correct.
func folderActor(relPath string) string {
	segs := strings.Split(relPath, "/")
	if len(segs) < 3 || segs[0] != moviesDir {
		return ""
	}
	return segs[1]
}

// idFor derives a stable id from a path under the library root: the file for a video,
// the folder for a comic. The same path gives the same id on every scan and restart;
// a rename gives a new one (ADR 004).
func idFor(relPath string) string {
	sum := sha256.Sum256([]byte(relPath))
	return hex.EncodeToString(sum[:8])
}
