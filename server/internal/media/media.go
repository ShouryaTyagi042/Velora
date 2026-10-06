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
	ActorManual     ActorSource = "manual" // added in the app (phase 7)
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

// Media is one item in the library: a video file or a comic folder.
type Media struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Kind      Kind       `json:"kind"`
	RelPath   string     `json:"-"` // path under the library root (file for a video, folder for a comic); never sent to clients
	MimeType  string     `json:"mimeType,omitempty"`
	SizeBytes int64      `json:"sizeBytes"`
	ModTime   time.Time  `json:"modTime"`
	AddedAt   time.Time  `json:"addedAt"`
	Actors    []ActorTag `json:"actors,omitempty"` // videos only
	Comic     *ComicMeta `json:"comic,omitempty"`  // comics only
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

// ErrNotFound is returned when no media has the requested id.
var ErrNotFound = errors.New("media: not found")
