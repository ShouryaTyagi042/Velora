package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxBodyBytes bounds every JSON request body. Velora's bodies are a name or a position.
const maxBodyBytes = 64 << 10

// decode reads r's body as one JSON value of type T. Every way the body can be wrong becomes a
// 400 validation error whose message is safe to show: too large, not JSON, the wrong type for a
// field, a field T doesn't have (a misspelling, caught instead of silently ignored), or a second
// value after the first.
func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return v, validationError("Content-Type must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()

	err := dec.Decode(&v)
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		tooBig    *http.MaxBytesError
	)
	switch {
	case err == nil:
	case errors.Is(err, io.EOF):
		return v, validationError("request body must not be empty")
	case errors.As(err, &tooBig):
		return v, validationError("request body must be at most %d bytes", tooBig.Limit)
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return v, validationError("request body is not valid JSON")
	case errors.As(err, &typeErr):
		return v, validationError("field %q has the wrong type", typeErr.Field)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json has no typed error for this one.
		return v, validationError("unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
	default:
		return v, fmt.Errorf("decode body: %w", err)
	}

	if dec.More() {
		return v, validationError("request body must be a single JSON value")
	}
	return v, nil
}
