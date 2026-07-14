// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package openai

import (
	"errors"
	"net/http"

	"github.com/gravitational/trace"

	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/utils"
)

// WriteError writes an error in OpenAI format.
func WriteError(w http.ResponseWriter, err error) error {
	w.WriteHeader(llmerrors.StatusCodeFromErr(err))
	_, werr := w.Write(marshalError(newErrorEnvelope(err)))
	return trace.Wrap(werr)
}

// marshalError marshals an error into OpenAI format.
func marshalError(apiErr *errorEnvelope) []byte {
	enc, err := utils.FastMarshal(apiErr)
	if err != nil {
		return []byte(
			`{"error": {"type": "server_error", "message": "` + llmerrors.ErrUnknown.Error() + `"}}`,
		)
	}
	return enc
}

// marshalResponsesErrorEvent marshals an OpenAI error message.
func marshalResponsesErrorEvent(evt *responsesErrorSSEEvent) []byte {
	enc, err := utils.FastMarshal(evt)
	if err != nil {
		return []byte(
			// Ignore SequenceNumber to avoid having to deal with number
			// conversions in this failure scenario.
			`{"type": "error", "message": "` + llmerrors.ErrUnknown.Error() + `"}`,
		)
	}
	return enc
}

// marshalResponsesFailedError marshals an OpenAI `response.failed` error
// message.
func marshalResponsesFailedError(evt *responsesFailedSSEEvent) []byte {
	enc, err := utils.FastMarshal(evt)
	if err != nil {
		return []byte(
			// Ignore SequenceNumber to avoid having to deal with number
			// conversions in this failure scenario.
			`{"type": "` + responsesFailedEventName + `", "response": {"type": "server_error", "message": "` + llmerrors.ErrUnknown.Error() + `"}}`,
		)
	}
	return enc
}

// newErrorEnvelope creates a new error envelope.
func newErrorEnvelope(err error) *errorEnvelope {
	if err == nil {
		return nil
	}

	return &errorEnvelope{
		Type:  "error",
		Error: newErrorMessage(err),
	}
}

// newResponsesFailedError creates a new error in the `response.failed` event
// format.
func newResponsesFailedError(seqNumber int, err error) *responsesFailedSSEEvent {
	return &responsesFailedSSEEvent{
		Type:           responsesFailedEventName,
		SequenceNumber: seqNumber,
		Response: errorEnvelope{
			Error: newErrorMessage(err),
		},
	}
}

// newErrorMessage creates a new error message.
func newErrorMessage(err error) errorMessage {
	msg := errorMessage{
		Type:    errorTypeServerError,
		Message: err.Error(),
	}
	switch {
	case errors.Is(err, llmerrors.ErrRejected):
		msg.Type = errorTypeRateLimitExceeded
	case errors.Is(err, llmerrors.ErrBadRequest),
		errors.Is(err, llmerrors.ErrUnauthorized),
		errors.Is(err, llmerrors.ErrUnsupported),
		trace.IsBadParameter(err),
		trace.IsNotFound(err):
		msg.Type = errorTypeInvalidRequest
	}

	return msg
}

// parseProviderError parses errors that come from OpenAI API.
func parseProviderError(statusCode int, body []byte) (*llmerrors.ProviderError, error) {
	var r errorEnvelope
	if err := utils.FastUnmarshal(body, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	// Prefer setting the LLM error on status code since it provides more
	// granularity.
	//
	// https://developers.openai.com/api/docs/guides/error-codes#api-errors
	switch statusCode {
	case http.StatusBadRequest:
		return llmerrors.NewProviderError(llmerrors.ErrBadRequest, r.Error.Message), nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return llmerrors.NewProviderError(llmerrors.ErrUnauthorized, ""), nil
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return llmerrors.NewProviderError(llmerrors.ErrRejected, r.Error.Message), nil
	}

	switch r.Error.Type {
	case errorTypeInvalidRequest:
		return llmerrors.NewProviderError(llmerrors.ErrBadRequest, r.Error.Message), nil
	case errorTypeRateLimitExceeded:
		return llmerrors.NewProviderError(llmerrors.ErrRejected, r.Error.Message), nil
	default:
		return llmerrors.NewProviderError(llmerrors.ErrUnknown, r.Error.Message), nil
	}
}

type errorMessage struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

// errorEnvelope wraps the error message from OpenAI's API.
type errorEnvelope struct {
	Type  string       `json:"type,omitempty"`
	Error errorMessage `json:"error"`
}

const (
	errorTypeServerError       = "server_error"
	errorTypeRateLimitExceeded = "rate_limit_exceeded"
	errorTypeInvalidRequest    = "invalid_request_error"
)
