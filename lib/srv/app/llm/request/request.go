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
	llmlimiter "github.com/gravitational/teleport/lib/srv/app/llm/limiter"
)

// ReserveFunc reserves tokens out of the application limits for the request
// being built.
type ReserveFunc func(context.Context, llmlimiter.ReserveRequest) (llmlimiter.ReserveInfo, llmlimiter.SettleFunc, error)

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
	// Reserve is the function used to reserve tokens for the request.
	Reserve ReserveFunc
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
	if c.Reserve == nil {
		return trace.BadParameter("reserve function is required")
	}
	if c.App.GetLLM().Provider == types.LLMProviderAWSBedrock {
		if c.SignBedrockRequest == nil {
			return trace.BadParameter("sign aws bedrock request function is required for the bedrock provider")
		}
	}

	return nil
}

// Request is the provider request built out of the downstream request. It is
// partially filled when building it fails, so its fields must be checked before
// being used.
type Request struct {
	// HTTPRequest is the request to be sent to the provider.
	HTTPRequest *http.Request
	// Info contains the request information.
	Info RequestInfo
	// SettleFunc settles the tokens reserved for the request. It is only
	// present after the reservation is made.
	SettleFunc llmlimiter.SettleFunc
}

// RequestInfo interface that contains the request information.
type RequestInfo interface {
	// RequestedModel returns the requested model name.
	RequestedModel() string
	// ProviderModel returns the model name sent to the provider.
	ProviderModel() string
	// IsStream indicates if the request uses streaming.
	IsStream() bool
	// RequestSize contains the total request size in bytes.
	RequestSize() int
}
