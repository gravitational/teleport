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
	"github.com/gravitational/teleport/api/types"
)

// LLMRequest describes an inbound LLM API request being proxied to a provider.
type LLMRequest struct {
	// Format identifies the LLM format used on the request.
	Format types.LLMFormat
	// Provider identifies the LLM provider handling the request.
	Provider types.LLMProvider
	// Model is the resolved model name sent to the provider.
	Model string
	// RequestedModel is the original model name as specified by the client.
	RequestedModel string
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
