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

package request

import (
	"net/http"
	"net/url"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// Config is config used to create a new provide request.
type Config struct {
	// LLM inference endpoint configuration.
	LLM *types.LLM
	// DownstreamRequest is the received downstream request.
	DownstreamRequest *http.Request
	// ProviderURL is the provider URL address.
	ProviderURL *url.URL
	// GetAPIKeyFunc is the function used to retrieve Anthropic API keys.
	GetAPIKeyFunc func() string
}

func (c *Config) CheckAndSetDefaults() error {
	if c.LLM == nil {
		return trace.BadParameter("llm information is required")
	}
	if c.DownstreamRequest == nil {
		return trace.BadParameter("downstream request is required")
	}
	if c.GetAPIKeyFunc == nil {
		return trace.BadParameter("get api key function is required")
	}

	return nil
}
