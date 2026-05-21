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

package anthropic

import (
	"encoding/json"
	"maps"

	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"

	"github.com/gravitational/teleport/lib/utils"
)

// RequestInfo contains the request information.
type RequestInfo struct {
	requestedModel string
	providerModel  string
	stream         bool
	requestSize    int
}

func (r *RequestInfo) RequestedModel() string { return r.requestedModel }
func (r *RequestInfo) ProviderModel() string  { return r.providerModel }
func (r *RequestInfo) IsStream() bool         { return r.stream }
func (r *RequestInfo) RequestSize() int       { return r.requestSize }

// messagesAPIResult contains part of the fields from messages API response body.
//
// https://platform.claude.com/docs/en/api/messages/create#message
type messagesAPIResult struct {
	Usage struct {
		OutputTokens int `json:"output_tokens"`
		InputTokens  int `json:"input_tokens"`
	} `json:"usage"`
}

// sseMessageStartEvent contains part of the fields from messages API SSE
// "message_start" event.
//
// https://platform.claude.com/docs/en/api/messages/create#raw_message_start_event
type sseMessageStartEvent struct {
	Message struct {
		Usage struct {
			OutputTokens int `json:"output_tokens"`
			InputTokens  int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// errorMessage contains the error returned from Anthropic's API.
type errorMessage struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

// errorEnvelope wraps the error message from Anthropic's API.
type errorEnvelope struct {
	Type  string       `json:"type,omitempty"`
	Error errorMessage `json:"error"`
}

// List of Anthropic error types.
//
// https://platform.claude.com/docs/en/api/errors
const (
	errorTypeInvalidRequestError = "invalid_request_error"
	errorTypeAuthenticationError = "authentication_error"
	errorTypePermissionError     = "permission_error"
	errorTypeNotFoundError       = "not_found_error"
	errorTypeRateLimitError      = "rate_limit_error"
	errorTypeTimeoutError        = "timeout_error"
	errorTypeOverloadedError     = "overloaded_error"
	errorTypeAPIError            = "api_error"
	errorTypeBillingError        = "billing_error"
)

// messagesAPIRequest contains part of the fields from messages API request
// body.
//
// https://platform.claude.com/docs/en/api/messages/create
type messagesAPIRequest struct {
	Model     string `json:"-"`
	Stream    bool   `json:"-"`
	MaxTokens int    `json:"-"`

	Raw map[string]json.RawMessage `json:"-"`
}

func (m *messagesAPIRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := caseSensitiveJSONConfig.Unmarshal(data, &raw); err != nil {
		return err
	}

	if err := unmarshalField(raw, "model", &m.Model); err != nil {
		return trace.Wrap(err)
	}
	if err := unmarshalField(raw, "stream", &m.Stream); err != nil {
		return trace.Wrap(err)
	}
	if err := unmarshalField(raw, "max_tokens", &m.MaxTokens); err != nil {
		return trace.Wrap(err)
	}

	m.Raw = raw
	return nil
}

func (m messagesAPIRequest) MarshalJSON() ([]byte, error) {
	final := make(map[string]json.RawMessage, len(m.Raw)+3) // Current len + taken fields.
	maps.Copy(final, m.Raw)
	if err := marshalField(final, "model", m.Model); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := marshalField(final, "stream", m.Stream); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := marshalField(final, "max_tokens", m.MaxTokens); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := utils.FastMarshal(final)
	return res, trace.Wrap(err)
}

func unmarshalField(raw map[string]json.RawMessage, fieldName string, field any) error {
	if v, ok := raw[fieldName]; ok {
		if err := caseSensitiveJSONConfig.Unmarshal(v, field); err != nil {
			return trace.Wrap(err)
		}
		delete(raw, fieldName)
	}
	return nil
}

func marshalField(raw map[string]json.RawMessage, fieldName string, value any) error {
	field, err := json.Marshal(value)
	if err != nil {
		return trace.Wrap(err)
	}
	raw[fieldName] = field
	return nil
}

// caseSensitiveJSONConfig is used to decode JSON messages. The config is
// based on jsoniter.ConfigCompatibleWithStandardLibrary with the addition of
// CaseSensitive enabled.
// TODO(gabrielcorado): Migrate to encoding/json/v2 once it's out of experimentation.
var caseSensitiveJSONConfig = jsoniter.Config{
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
	CaseSensitive:          true,
}.Froze()
