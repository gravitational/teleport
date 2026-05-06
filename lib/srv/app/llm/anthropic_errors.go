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

package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// convertAnthropicError converts an error from the Anthropic SDK (or a
// trace error from upstream code) into a sanitized *apiError suitable for
// client responses and audit logging.
func convertAnthropicError(err error) *apiError {
	if err == nil {
		return nil
	}
	if e := convertError(err); e != nil {
		return e
	}

	var sdkErr *anthropic.Error
	if !errors.As(err, &sdkErr) {
		return newUnknown(err, http.StatusBadGateway)
	}

	status := sdkErr.StatusCode
	if status == 0 {
		status = http.StatusBadGateway
	}

	body, ok := readAnthropicErrorBody(sdkErr)
	if !ok {
		return convertErrorByHTTPStatus(err, status, "")
	}
	obj, perr := parseAnthropicBody(body)
	if perr != nil {
		return convertErrorByHTTPStatus(err, status, "")
	}
	return errorFromAnthropicBody(err, obj, status)
}

// convertAnthropicErrorEvent converts an Anthropic SSE error event payload
// into an apiError. Since SSE events have no HTTP status, the status is
// not used.
func convertAnthropicErrorEvent(data []byte) *apiError {
	obj, err := parseAnthropicBody(data)
	if err != nil {
		return newUnknown(fmt.Errorf("malformed anthropic sse error event: %w", err), http.StatusInternalServerError)
	}
	return errorFromAnthropicBody(nil /* cause */, obj, 0 /* statusCode */)
}

// errorFromAnthropicBody maps a parsed Anthropic API error body to an
// apiError using shared constructors.
func errorFromAnthropicBody(cause error, obj shared.APIErrorObject, status int) *apiError {
	switch shared.ErrorType(obj.Type) {
	case shared.ErrorTypeAuthenticationError, shared.ErrorTypePermissionError:
		return newUnauthorized(cause, status)
	case shared.ErrorTypeInvalidRequestError:
		return newBadRequest(cause, status, obj.Message)
	case shared.ErrorTypeRateLimitError, shared.ErrorTypeBillingError:
		return newRejected(cause, status)
	}

	// Sometimes the error type is not available (when using Bedrock
	// provider), for those cases we can rely on the status code to properly set
	// the error type.
	return convertErrorByHTTPStatus(cause, status, "")
}

func readAnthropicErrorBody(e *anthropic.Error) ([]byte, bool) {
	if e.Response == nil || e.Response.Body == nil {
		return nil, false
	}
	defer e.Response.Body.Close()
	body, err := io.ReadAll(e.Response.Body)
	if err != nil {
		return nil, false
	}
	return body, true
}

// parseAnthropicBody parses an Anthropic-format API error response body.
func parseAnthropicBody(body []byte) (shared.APIErrorObject, error) {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Error) > 0 {
		var obj shared.APIErrorObject
		if err := obj.UnmarshalJSON(envelope.Error); err != nil {
			return shared.APIErrorObject{}, err
		}
		return obj, nil
	}

	var obj shared.APIErrorObject
	if err := obj.UnmarshalJSON(body); err != nil {
		return shared.APIErrorObject{}, err
	}
	return obj, nil
}

// anthropicErrorBody serializes an apiError as an Anthropic-format JSON body.
func anthropicErrorBody(e *apiError) []byte {
	body, err := json.Marshal(struct {
		Type  constant.Error        `json:"type"`
		Error shared.APIErrorObject `json:"error"`
	}{
		Type: constant.ValueOf[constant.Error](),
		Error: shared.APIErrorObject{
			Type:    constant.APIError(anthropicErrorType(e)),
			Message: e.UserMessage(),
		},
	})
	if err != nil {
		return []byte(`{"type": "error", "error": {"type": "api_error", "message": "The inference provider returned an unexpected error. Contact your Teleport administrator"}}`)
	}
	return body
}

// respondAnthropicError converts err, writes an Anthropic-format HTTP error
// response, and returns the resulting *apiError for audit/logging.
func respondAnthropicError(w http.ResponseWriter, err error) *apiError {
	apiErr := convertAnthropicError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.status)
	w.Write(anthropicErrorBody(apiErr))
	return apiErr
}

// anthropicErrorType maps an apiError's kind to the matching Anthropic error
// type.
func anthropicErrorType(e *apiError) shared.ErrorType {
	switch e.kind {
	case errKindCanceled, errKindTimeout:
		return shared.ErrorTypeTimeoutError
	case errKindBadRequest:
		return shared.ErrorTypeInvalidRequestError
	case errKindUnauthorized:
		return shared.ErrorTypeAuthenticationError
	case errKindRejected:
		return shared.ErrorTypeRateLimitError
	case errKindUnsupportedEndpoint:
		return shared.ErrorTypeNotFoundError
	}
	return shared.ErrorTypeAPIError
}
