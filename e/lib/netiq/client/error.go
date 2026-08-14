package client

import (
	"fmt"
	"net/http"
)

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	// StatusCode is the HTTP status code of the error response.
	StatusCode int
	// Payload is the error payload of the error response.
	Payload *ErrorPayload
	// Error is the error returned by the API.
	// Error is only set if the error payload could not be decoded.
	Err error
}

// ErrorPayload represents an error returned by the API.
type ErrorPayload struct {
	Code struct {
		Subcode struct {
			Value string `json:"Value"`
		} `json:"Subcode"`
		Value string `json:"Value"`
	} `json:"Code"`
	Reason struct {
		Text string `json:"Text"`
	} `json:"Reason"`
}

func (e *ErrorResponse) Error() string {
	switch {
	case e.Payload != nil:
		return fmt.Sprintf("error %s: %s", http.StatusText(e.StatusCode), e.Payload.Reason.Text)
	case e.Err != nil:
		return fmt.Sprintf("error %s: %v", http.StatusText(e.StatusCode), e.Err)
	default:
		return fmt.Sprintf("error %s", http.StatusText(e.StatusCode))
	}
}
