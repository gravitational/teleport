package jamf

import "fmt"

// APIError is an error returned by the Jamf API.
type APIError struct {
	// HTTPStatus is the status of the response, either from the response body
	// or from the actual HTTP code.
	HTTPStatus int `json:"httpStatus"`
	// RawBody is the raw JSON body of the error.
	RawBody string `json:"-"`
}

// Error returns a textual representation of the error.
func (e *APIError) Error() string {
	if e == nil {
		return "nil error"
	}
	return fmt.Sprintf("status=%v, body=%s", e.HTTPStatus, e.RawBody)
}
