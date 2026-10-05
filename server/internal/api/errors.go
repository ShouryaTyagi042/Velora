package api

import "fmt"

// Error is an error whose code and message are safe to send to the client.
// Anything that isn't an *Error (or media.ErrNotFound) becomes a 500 with no details.
type Error struct {
	Status  int    // HTTP status code
	Code    string // stable, machine-readable; clients switch on it
	Message string // human-readable; free to change
}

func (e *Error) Error() string { return e.Message }

func validationError(format string, args ...any) *Error {
	return &Error{Status: 400, Code: "validation", Message: fmt.Sprintf(format, args...)}
}

func notFoundError(format string, args ...any) *Error {
	return &Error{Status: 404, Code: "not_found", Message: fmt.Sprintf(format, args...)}
}

func notImplementedError(format string, args ...any) *Error {
	return &Error{Status: 501, Code: "not_implemented", Message: fmt.Sprintf(format, args...)}
}
