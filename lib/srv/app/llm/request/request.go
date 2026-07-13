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
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Config is config used to create a new provide request.
type Config struct {
	// Logger is the logger used to emit log entries.
	Logger *slog.Logger
	// App is the app being served.
	App types.Application
	// DownstreamRequest is the received downstream request.
	DownstreamRequest *http.Request
	// ProviderURL is the provider URL address.
	ProviderURL *url.URL
	// GetAPIKeyFunc is the function used to retrieve Anthropic API keys.
	GetAPIKeyFunc func() string
	// SignBedrockRequest signs the AWS Bedrock request.
	// Required for the AWS Bedrock provider.
	SignBedrockRequest func(ctx context.Context, app types.Application, request *http.Request, requestBody []byte) error
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.App == nil {
		return trace.BadParameter("app is required")
	}
	if c.App.GetLLM() == nil {
		return trace.BadParameter("app llm information is required")
	}
	if c.DownstreamRequest == nil {
		return trace.BadParameter("downstream request is required")
	}
	if c.GetAPIKeyFunc == nil {
		return trace.BadParameter("get api key function is required")
	}
	if c.App.GetLLM().Provider == types.LLMProviderAWSBedrock {
		if c.SignBedrockRequest == nil {
			return trace.BadParameter("sign aws bedrock request function is required for the bedrock provider")
		}
	}

	return nil
}
