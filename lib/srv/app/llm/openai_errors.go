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
	"net/http"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// convertOpenAIError converts an error from the OpenAI SDK (or a
// trace error from upstream code) into a sanitized *apiError suitable for
// client responses and audit logging.
func convertOpenAIError(err error) *apiError {
	if err == nil {
		return nil
	}
	if e := convertError(err); e != nil {
		return e
	}

	var sdkErr *openai.Error
	if !errors.As(err, &sdkErr) {
		return newUnknown(err, http.StatusBadGateway)
	}

	status := sdkErr.StatusCode
	if status == 0 {
		status = http.StatusBadGateway
	}

	return errorFromOpenAIBody(err, *sdkErr, status)
}

// errorFromOpenAIBody maps a parsed OpenAI API error body to an
// apiError using shared constructors.
func errorFromOpenAIBody(cause error, obj openai.Error, status int) *apiError {
	switch obj.Type {
	case "invalid_api_key", "insufficient_quota":
		return newUnauthorized(cause, status)
	case "invalid_request_error":
		return newBadRequest(cause, status, obj.Message)
	case "rate_limit_exceeded":
		return newRejected(cause, status)
	}

	// Sometimes the error type is not available, for those cases we can rely on
	// the status code to properly set the error type.
	return convertErrorByHTTPStatus(cause, status, obj.Message)
}

// openAIErrorBody serializes an apiError as an OpenAI-format JSON body.
func openAIErrorBody(e *apiError) []byte {
	body, err := json.Marshal(struct {
		Error shared.ErrorObject `json:"error"`
	}{
		Error: shared.ErrorObject{
			Type:    openAIErrorType(e),
			Message: e.UserMessage(),
		},
	})
	if err != nil {
		return []byte(`{"error": {"code": "server_error", "message": "The inference provider returned an unexpected error. Contact your Teleport administrator"}}`)
	}
	return body
}

func convertOpenAIErrorEvent(data []byte) *apiError {
	var envelope struct {
		Error openai.Error `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return newUnknown(fmt.Errorf("malformed openai sse error event: %w", err), http.StatusInternalServerError)
	}
	return errorFromOpenAIBody(&envelope.Error, envelope.Error, 0 /* statusCode */)
}

// respondOpenAIError converts err, writes an OpenAI-format HTTP error
// response, and returns the resulting *apiError for audit/logging.
func respondOpenAIError(w http.ResponseWriter, err error) *apiError {
	apiErr := convertOpenAIError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.status)
	w.Write(openAIErrorBody(apiErr))
	return apiErr
}

// openAIErrorType maps an apiError's kind to the matching OpenAI error
// type.
func openAIErrorType(e *apiError) string {
	// Grab the error type from original message since it contains more detailed
	// info.
	var sdkErr *openai.Error
	if errors.As(e.cause, &sdkErr) {
		return sdkErr.Type
	}

	switch e.kind {
	case errKindCanceled, errKindTimeout:
		return "timeout"
	case errKindBadRequest:
		return "invalid_request_error"
	case errKindUnauthorized:
		return "invalid_api_key"
	case errKindRejected:
		return "rate_limit_exceeded"
	case errKindUnsupportedEndpoint:
		return "server_error"
	}
	return "server_error"
}
