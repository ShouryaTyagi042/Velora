package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ShouryaTyagi042/Velora/server/internal/media"
)

// HandlerE is a handler that returns an error instead of writing one.
// Its ServeHTTP turns the error into a response, so every endpoint fails the same way.
type HandlerE func(w http.ResponseWriter, r *http.Request) error

func (h HandlerE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		writeError(w, r, err)
	}
}

// errorEnvelope is the body of every error response:
// {"error": {"code": "not_found", "message": "media m9 not found"}}
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondJSON writes v as JSON with the given status.
// Content-Type must be set before WriteHeader: that call sends the headers.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeError maps err to a status and the error envelope.
// Unknown errors are logged in full and sent to the client as a bare "internal error".
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *Error
	switch {
	case errors.As(err, &apiErr):
		// already client-safe
	case errors.Is(err, media.ErrNotFound):
		apiErr = notFoundError("not found")
	default:
		log.Printf("internal error: request_id=%s %s %s: %v",
			requestIDFrom(r.Context()), r.Method, r.URL.Path, err)
		apiErr = &Error{Status: 500, Code: "internal", Message: "internal error"}
	}
	respondJSON(w, apiErr.Status, errorEnvelope{Error: errorBody{Code: apiErr.Code, Message: apiErr.Message}})
}
