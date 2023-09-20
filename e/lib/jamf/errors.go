package jamf

import "fmt"

// APIError is an error returned by the Jamf API.
type APIError struct {
	// StatusCode is the status of the HTTP response.
	// Jamf API errors carry an "httpStatus" field inside the error JSON, which
	// typically (always?) matches the responses' status code. That field is not
	// mapped here, instead we rely solely on the HTTP status code.
	StatusCode int
}

// Error returns a textual representation of the error.
func (e *APIError) Error() string {
	if e == nil {
		return "nil error"
	}
	return fmt.Sprintf("status=%v", e.StatusCode)
}
