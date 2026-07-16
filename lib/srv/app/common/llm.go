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

package common

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// LLMRequest describes an inbound LLM API request being proxied to a provider.
type LLMRequest struct {
	// Provider identifies the LLM provider handling the request.
	Provider types.LLMProvider
	// Model is the resolved model name sent to the provider.
	Model string
	// RequestedModel is the original model name as specified by the client.
	RequestedModel string
	// Streaming indicates whether the response is streamed back to the client.
	Streaming bool
	// MaxTokens is the maximum number of tokens the response may contain.
	MaxTokens int64
	// Timeout is the maximum duration allowed for the request.
	Timeout time.Duration
}

// LLMResponse describes the outcome of a proxied LLM API request.
type LLMResponse struct {
	// Error is the error encountered while handling the request, if any.
	Error error
	// InputTokenCount is the number of tokens consumed by the request input.
	InputTokenCount int
	// OutputTokenCount is the number of tokens produced in the response.
	OutputTokenCount int
}
