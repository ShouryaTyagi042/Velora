package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// Cache-Control values, one per kind of response.
const (
	// cacheRevalidate is for lists that change sometimes (the library, Continue): the phone may
	// keep a copy but must ask before using it, and the ETag makes that question cheap.
	// private: the response was fetched with a token, so no shared cache may keep it.
	cacheRevalidate = "private, no-cache"
	// cacheForever is for a URL whose content never changes: it carries a version (?v=) that
	// changes whenever the content does. The phone never asks again.
	cacheForever = "private, max-age=31536000, immutable"
)

// respondCachedJSON writes v as a 200 with an ETag, or a bodyless 304 Not Modified when the
// client's If-None-Match already names this exact body.
//
// The ETag is a hash of the body, so it is correct by construction: same bytes, same tag. What a
// 304 saves is the transfer, not the work: the query and the encoding still run, because they
// are what produce the hash. That is the right trade here. The query takes about a millisecond;
// the bytes cross Wi-Fi to a phone's radio. (measured in phase 12: ADR 009)
func respondCachedJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Printf("encode json: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body = append(body, '\n') // as json.Encoder writes it, so a body is the same either way
	sum := sha256.Sum256(body)
	etag := `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`

	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", cacheRevalidate)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		log.Printf("write json: %v", err)
	}
}

// etagMatches implements If-None-Match's comparison (RFC 9110 §13.1.2): a list of tags, any of
// which may match, or "*" for any. It is the weak comparison, so W/"x" matches "x".
func etagMatches(header, etag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimPrefix(strings.TrimSpace(t), "W/")
		if t == "*" || t == etag {
			return true
		}
	}
	return false
}
