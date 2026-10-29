// Package media defines Velora's domain types. It knows nothing about HTTP or storage.
package media

import (
	"errors"
	"time"
)

// Kind says which kind of media a record is.
type Kind string

const (
	KindVideo Kind = "video"
	KindComic Kind = "comic"
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	return k == KindVideo || k == KindComic
}

// ActorSource says who owns an actor tag: the folder layout or a person using the app.
type ActorSource string

const (
	ActorFromFolder ActorSource = "folder" // from movies/<actor>/; changes only by renaming the folder
	ActorManual     ActorSource = "manual" // added in the app; the scanner never touches it
)

// ActorTag is one actor on a video.
type ActorTag struct {
	Name   string      `json:"name"`
	Source ActorSource `json:"source"`
}

// ComicMeta is what only comics have.
type ComicMeta struct {
	PageCount int      `json:"pageCount"`
	Pages     []string `json:"-"` // page filenames in reading order; never sent to clients
}

// VideoMeta is what ffprobe found in a video (phase 9). Absent until the extractor has run.
type VideoMeta struct {
	DurationMs int     `json:"durationMs"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Codec      string  `json:"codec"`
	FPS        float64 `json:"fps"`
}

// Progress is where you are in one item: a position for a video, a page for a comic.
// Fields of the other kind are zero and omitted.
type Progress struct {
	PositionMs int       `json:"positionMs,omitempty"` // videos
	DurationMs int       `json:"durationMs,omitempty"`
	Page       int       `json:"page,omitempty"` // comics, 0-based; absent means page 0
	PageCount  int       `json:"pageCount,omitempty"`
	Completed  bool      `json:"completed"`
	UpdatedAt  time.Time `json:"updatedAt"` // the client's clock when it saved
}

// Media is one item in the library: a video file or a comic folder.
type Media struct {
	ID          string     `json:"id"`
	Fingerprint string     `json:"-"` // hash of RelPath: how a scan recognises an item it has seen before
	Title       string     `json:"title"`
	Kind        Kind       `json:"kind"`
	RelPath     string     `json:"-"` // path under the library root (file for a video, folder for a comic); never sent to clients
	MimeType    string     `json:"mimeType,omitempty"`
	SizeBytes   int64      `json:"sizeBytes"`
	ModTime     time.Time  `json:"modTime"`
	AddedAt     time.Time  `json:"addedAt"`
	Actors      []ActorTag `json:"actors,omitempty"`   // videos only
	Comic       *ComicMeta `json:"comic,omitempty"`    // comics only
	Video       *VideoMeta `json:"video,omitempty"`    // videos only, once extracted
	Progress    *Progress  `json:"progress,omitempty"` // once started
}

// HasActor reports whether any of m's actors is called name.
func (m Media) HasActor(name string) bool {
	for _, a := range m.Actors {
		if a.Name == name {
			return true
		}
	}
	return false
}

// Domain errors. The API maps each to a status in one place (api.writeError).
var (
	ErrNotFound    = errors.New("media: not found")
	ErrNotVideo    = errors.New("media: actors apply to videos only")
	ErrFolderActor = errors.New("media: folder actors change by renaming the folder")
)
