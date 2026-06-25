package huly

import (
	"errors"
	"fmt"
)

var (
	// ErrUnauthorized is returned on HTTP 401 — token expired or invalid.
	ErrUnauthorized = errors.New("huly: unauthorized")
	// ErrNotFound is returned on HTTP 404 for direct-id lookups.
	ErrNotFound = errors.New("huly: not found")
)

// APIError wraps a non-2xx response that has no more specific sentinel.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("huly: api error %d: %s", e.Status, e.Body)
}
