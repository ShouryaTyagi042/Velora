package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// Each middleware takes a handler and returns a new one that runs code before and after it.

// Recover turns a panic in any inner handler into a 500 instead of killing the connection.
// It runs outermost, before RequestID has put the id into the context, so it reads the id
// from the response header that RequestID sets.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // net/http's own signal to abort the response; let it through
			}
			log.Printf("panic: request_id=%s %s %s: %v\n%s",
				w.Header().Get("X-Request-ID"), r.Method, r.URL.Path, rec, debug.Stack())
			// If the handler already wrote, the headers are gone and this write is ignored.
			respondJSON(w, http.StatusInternalServerError,
				errorEnvelope{Error: errorBody{Code: "internal", Message: "internal error"}})
		}()
		next.ServeHTTP(w, r)
	})
}

type ctxKey int

const requestIDKey ctxKey = 0

// RequestID gives every request a random id, returns it in the X-Request-ID header,
// and stores it in the request context for logging.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 8)
	rand.Read(b) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b)
}

// Logging logs one line per request: method, path, status, bytes, duration, request id.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %dB %s request_id=%s",
			r.Method, r.URL.Path, rec.status, rec.bytes, time.Since(start), requestIDFrom(r.Context()))
	})
}

// statusRecorder wraps a ResponseWriter to remember the status and body size,
// which the ResponseWriter interface itself never exposes.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK // a Write without WriteHeader means 200
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real writer (phase 4 needs Flush).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
