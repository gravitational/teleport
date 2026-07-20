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
	"encoding/json"
	"maps"
	"strings"

	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"

	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/utils"
)

// endpointType is the OpenAI API endpoint format.
type endpointType string

const (
	endpointTypeResponses       endpointType = "responses"
	endpointTypeChatCompletions endpointType = "chat_completions"
)

// RequestInfo contains the request information.
type RequestInfo struct {
	requestedModel string
	providerModel  string
	stream         bool
	requestSize    int
	endpointType   endpointType
}

func (r *RequestInfo) RequestedModel() string { return r.requestedModel }
func (r *RequestInfo) ProviderModel() string  { return r.providerModel }
func (r *RequestInfo) IsStream() bool         { return r.stream }
func (r *RequestInfo) RequestSize() int       { return r.requestSize }

type apiRequest interface {
	GetModel() string
	SetModel(string)
	GetStream() bool
	EnableReportUsage()
	DisableDataRetention()
	Validate() error
}

// responsesAPIRequest contains part of the fields from responses API request
// body.
//
// https://developers.openai.com/api/reference/resources/responses/methods/create
type responsesAPIRequest struct {
	Model      string `json:"model"`
	Stream     bool   `json:"stream"`
	Background bool   `json:"background"`
	Store      bool   `json:"store"`

	raw map[string]json.RawMessage `json:"-"`
}

func (r *responsesAPIRequest) DisableDataRetention() {
	r.Store = false
	r.Background = false
}

// Nothing to do, usage is always reported on responses API.
func (r *responsesAPIRequest) EnableReportUsage()    {}
func (r *responsesAPIRequest) GetModel() string      { return r.Model }
func (r *responsesAPIRequest) GetStream() bool       { return r.Stream }
func (r *responsesAPIRequest) SetModel(model string) { r.Model = model }
func (r *responsesAPIRequest) Validate() error {
	if r.Background {
		return llmerrors.NewProviderError(llmerrors.ErrUnsupported, "background responses not supported")
	}
	if r.Store {
		return llmerrors.NewProviderError(llmerrors.ErrUnsupported, "storing responses not supported")
	}

	return nil
}

var (
	_ (apiRequest) = (*responsesAPIRequest)(nil)
	_ (apiRequest) = (*chatCompletionsAPIRequest)(nil)
)

func (r *responsesAPIRequest) UnmarshalJSON(data []byte) error {
	type Alias responsesAPIRequest
	aux := &struct{ *Alias }{Alias: (*Alias)(r)}
	if err := caseSensitiveJSONConfig.Unmarshal(data, aux); err != nil {
		return trace.Wrap(err)
	}

	var raw map[string]json.RawMessage
	if err := utils.FastUnmarshal(data, &raw); err != nil {
		return trace.Wrap(err)
	}

	for key := range raw {
		switch strings.ToLower(key) {
		case "model", "stream", "store", "background":
			delete(raw, key)
		default:
		}
	}

	r.raw = raw
	return nil
}

func (r responsesAPIRequest) MarshalJSON() ([]byte, error) {
	final := make(map[string]json.RawMessage, len(r.raw)+2) // Current len + taken fields.
	maps.Copy(final, r.raw)
	if err := marshalField(final, "model", r.Model); err != nil {
		return nil, trace.Wrap(err)
	}
	if r.Stream {
		if err := marshalField(final, "stream", r.Stream); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := marshalField(final, "background", r.Background); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := marshalField(final, "store", r.Store); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := utils.FastMarshal(final)
	return res, trace.Wrap(err)
}

// chatCompletionsAPIRequestStreamOptions contains `stream_options` fields from
// chat completions API.
type chatCompletionsAPIRequestStreamOptions struct {
	// `include_obfuscation` defaults to `true` when not set in OpenAI spec,
	// meaning that we must avoid emitting the value when it is not set to
	// comply with the default behavior.
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
	IncludeUsage       bool  `json:"include_usage"`
}

// chatCompletionsAPIRequest contains part of the fields from chat completions
// API request body.
//
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
type chatCompletionsAPIRequest struct {
	Model         string                                 `json:"model"`
	Stream        bool                                   `json:"stream"`
	StreamOptions chatCompletionsAPIRequestStreamOptions `json:"stream_options"`
	Store         bool                                   `json:"store"`

	raw map[string]json.RawMessage `json:"-"`
}

func (r *chatCompletionsAPIRequest) DisableDataRetention() {
	r.Store = false
}

func (r *chatCompletionsAPIRequest) EnableReportUsage() {
	r.StreamOptions.IncludeUsage = true
}
func (r *chatCompletionsAPIRequest) GetModel() string      { return r.Model }
func (r *chatCompletionsAPIRequest) GetStream() bool       { return r.Stream }
func (r *chatCompletionsAPIRequest) SetModel(model string) { r.Model = model }
func (r *chatCompletionsAPIRequest) Validate() error {
	if r.Store {
		return llmerrors.NewProviderError(llmerrors.ErrUnsupported, "storing chat completions not supported")
	}

	return nil
}

func (r *chatCompletionsAPIRequest) UnmarshalJSON(data []byte) error {
	type Alias chatCompletionsAPIRequest
	aux := &struct{ *Alias }{Alias: (*Alias)(r)}
	if err := caseSensitiveJSONConfig.Unmarshal(data, aux); err != nil {
		return trace.Wrap(err)
	}

	var raw map[string]json.RawMessage
	if err := utils.FastUnmarshal(data, &raw); err != nil {
		return trace.Wrap(err)
	}

	for key := range raw {
		switch strings.ToLower(key) {
		case "model", "stream", "stream_options", "store":
			delete(raw, key)
		default:
		}
	}

	r.raw = raw
	return nil
}

func (r chatCompletionsAPIRequest) MarshalJSON() ([]byte, error) {
	final := make(map[string]json.RawMessage, len(r.raw)+4) // Current len + taken fields.
	maps.Copy(final, r.raw)
	if err := marshalField(final, "model", r.Model); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := marshalField(final, "stream", r.Stream); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := marshalField(final, "store", r.Store); err != nil {
		return nil, trace.Wrap(err)
	}
	// We must include `stream_options` only for streaming requests, as some
	// providers might reject it as bad request.
	if r.Stream {
		if err := marshalField(final, "stream_options", r.StreamOptions); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	res, err := utils.FastMarshal(final)
	return res, trace.Wrap(err)
}

func marshalField(raw map[string]json.RawMessage, fieldName string, value any) error {
	field, err := json.Marshal(value)
	if err != nil {
		return trace.Wrap(err)
	}
	raw[fieldName] = field
	return nil
}

// responsesAPIUsage contains part of the fields from responses API response
// body.
//
// https://developers.openai.com/api/reference/resources/responses/methods/create#(resource)%20responses%20%3E%20(model)%20response%20%3E%20(schema)
type responsesAPIUsage struct {
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// responsesSSEEventWithUsage contains part of responses SSE event that contains
// `usage` information.
type responsesSSEEventWithUsage struct {
	Response responsesAPIUsage `json:"response"`
}

// responsesFailedSSEEvent contains part of the fields from responses API
// `response.failed` SSE event.
//
// https://developers.openai.com/api/reference/resources/responses/streaming-events#response.failed
type responsesFailedSSEEvent struct {
	Type           string        `json:"type"`
	Response       errorEnvelope `json:"response"`
	SequenceNumber int           `json:"sequence_number"`
}

// responsesErrorSSEEvent contains part of the fields from responses API `error`
// SSE event.
//
// https://developers.openai.com/api/reference/resources/responses/streaming-events#error
type responsesErrorSSEEvent struct {
	Type           string `json:"type"`
	Message        string `json:"message"`
	SequenceNumber int    `json:"sequence_number"`
}

// chatCompletionsResponse contains part of the fields from chat completions API
// response body.
//
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)
type chatCompletionsResponse struct {
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *errorMessage `json:"error"`
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
