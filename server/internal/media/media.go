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

// Media is one item in the library: a video or a comic.
type Media struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Kind      Kind      `json:"kind"`
	Path      string    `json:"-"` // filesystem path; never sent to clients
	SizeBytes int64     `json:"sizeBytes"`
	AddedAt   time.Time `json:"addedAt"`
}

// ErrNotFound is returned when no media has the requested id.
var ErrNotFound = errors.New("media: not found")
