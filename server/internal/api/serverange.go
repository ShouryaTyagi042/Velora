package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// serveRange is a hand-written stand-in for http.ServeContent, written to learn what
// ServeContent does. It has the same signature so streamVideo can use either one.
// The route uses ServeContent; this one runs only in tests.
//
// It handles: no Range (200), one byte range in the three forms (a-b, a-, -n),
// clamping, 416, and HEAD. It does not handle: multiple ranges (serves the first),
// If-Modified-Since / If-None-Match (never 304), If-Range (a stale range is still
// served as 206), or Content-Type sniffing (the caller must set it).
func serveRange(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, content io.ReadSeeker) {
	// The size comes from seeking to the end; Seek returns the new offset.
	size, err := content.Seek(0, io.SeekEnd)
	if err != nil {
		http.Error(w, "seek failed", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Accept-Ranges", "bytes") // on every response, so players know they can seek
	h.Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))

	start, length, status := int64(0), size, http.StatusOK
	if hdr := r.Header.Get("Range"); hdr != "" {
		s, e, err := parseRange(hdr, size)
		switch {
		case errors.Is(err, errUnsatisfiable):
			h.Set("Content-Range", fmt.Sprintf("bytes */%d", size)) // tells the client the real size
			http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		case err != nil:
			// RFC 9110: a Range the server can't parse is ignored, and the whole file is sent.
		default:
			start, length, status = s, e-s+1, http.StatusPartialContent
			h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, size))
		}
	}

	if _, err := content.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "seek failed", http.StatusInternalServerError)
		return
	}
	h.Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return // same headers as GET, no body
	}
	// CopyN reads only the requested bytes, in small chunks. An error here almost always
	// means the player closed the connection (seek, pause, app closed): not a server fault,
	// and the status is already sent, so there is nothing to report.
	io.CopyN(w, content, length)
}

var (
	errMalformed     = errors.New("range: malformed")
	errUnsatisfiable = errors.New("range: not satisfiable")
)

// parseRange parses a Range header against a file of size bytes and returns the first
// range as inclusive offsets [start, end], with end clamped to the last byte.
//
//	bytes=0-99    → 0, 99
//	bytes=900-    → 900, size-1        (open-ended: from 900 to the end)
//	bytes=-100    → size-100, size-1   (suffix: the last 100 bytes)
//
// errMalformed means "ignore the header"; errUnsatisfiable means "answer 416".
func parseRange(header string, size int64) (start, end int64, err error) {
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return 0, 0, errMalformed // another unit, e.g. items=0-10
	}
	// Players send one range. For bytes=0-9,20-29 only the first is served.
	first, _, _ := strings.Cut(spec, ",")
	first = strings.TrimSpace(first)

	from, to, ok := strings.Cut(first, "-")
	if !ok {
		return 0, 0, errMalformed // no dash, e.g. bytes=abc
	}

	if from == "" {
		// Suffix form: -n is the last n bytes, all of the file if n > size.
		n, err := parseOffset(to)
		if err != nil {
			return 0, 0, err
		}
		if n == 0 {
			return 0, 0, errUnsatisfiable // asked for zero bytes
		}
		return max(size-n, 0), size - 1, nil
	}

	start, err = parseOffset(from)
	if err != nil {
		return 0, 0, err
	}
	if start >= size {
		return 0, 0, errUnsatisfiable // starts past the end; also covers an empty file
	}
	if to == "" {
		return start, size - 1, nil // open-ended
	}
	end, err = parseOffset(to)
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, errMalformed // bytes=500-100
	}
	return start, min(end, size-1), nil // asking past the end is fine: clamp it
}

// parseOffset accepts only ASCII digits. strconv.ParseInt alone would also accept
// "+5" and "-5", which the Range grammar doesn't allow. It still catches overflow.
func parseOffset(s string) (int64, error) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, errMalformed
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errMalformed // more digits than fit in an int64
	}
	return n, nil
}
