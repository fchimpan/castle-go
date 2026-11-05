package castle

import (
	"fmt"
	"net/http"
)

// APIError reports a non-success HTTP status without exposing response data.
type APIError struct{ StatusCode int }

func (e *APIError) Error() string {
	return fmt.Sprintf("castle: API returned HTTP %d (%s)", e.StatusCode, http.StatusText(e.StatusCode))
}

type stageError struct {
	stage string
	cause error
}

func (e *stageError) Error() string { return "castle: " + e.stage }
func (e *stageError) Unwrap() error { return e.cause }
