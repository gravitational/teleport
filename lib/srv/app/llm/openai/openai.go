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
	"bytes"
	"cmp"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/app/llm/bedrock"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/srv/app/llm/models"
	llmrequest "github.com/gravitational/teleport/lib/srv/app/llm/request"
	"github.com/gravitational/teleport/lib/utils"
)

// NewRequest creates a new provider request based on the downstream request,
// and inference endpoint configuration.
func NewRequest(cfg *llmrequest.Config) (*http.Request, llmrequest.RequestInfo, error) {
	var (
		info            = &RequestInfo{}
		providerPath    string
		providerMethod  string
		providerHeaders = http.Header{}
		req             apiRequest
	)

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, info, trace.Wrap(err)
	}

	// Default URL address.
	//
	// https://developers.openai.com/api/reference/overview
	cfg.ProviderURL = cmp.Or(cfg.ProviderURL, &url.URL{
		Scheme: "https",
		Host:   "api.openai.com",
		Path:   "/v1",
	})

	// All endpoints use JSON content-type.
	providerHeaders.Set("Content-Type", "application/json")
	llm := cfg.App.GetLLM()

	// Both `openai` and `bedrock` providers support responses and chat
	// completions endpoints.
	//
	// https://docs.aws.amazon.com/bedrock/latest/userguide/bedrock-mantle.html
	switch strings.TrimPrefix(cfg.DownstreamRequest.URL.Path, "/v1") {
	// https://developers.openai.com/api/reference/resources/responses/methods/create
	case "/responses":
		if cfg.DownstreamRequest.Method != http.MethodPost {
			// We're ok with returning 404 back to clients instead 405 status.
			return nil, info, trace.NotFound("responses API supports only POST requests")
		}

		// Teleport and Bedrock don't support WebSocket mode.
		//
		// https://developers.openai.com/api/docs/guides/websocket-mode
		// https://developers.openai.com/api/docs/guides/amazon-bedrock#responses-api-feature-availability
		if websocket.IsWebSocketUpgrade(cfg.DownstreamRequest) {
			return nil, info, trace.NotFound("websocket mode is not supported")
		}

		info.endpointType = endpointTypeResponses
		providerPath = "/responses"
		providerMethod = http.MethodPost
		req = &responsesAPIRequest{}
	case "/chat/completions":
		if cfg.DownstreamRequest.Method != http.MethodPost {
			// We're ok with returning 404 back to clients instead 405 status.
			return nil, info, trace.NotFound("chat completions API supports only POST requests")
		}

		info.endpointType = endpointTypeChatCompletions
		providerPath = "/chat/completions"
		providerMethod = http.MethodPost
		req = &chatCompletionsAPIRequest{}
	default:
		return nil, info, trace.NotFound("unsupported endpoint")
	}

	switch llm.Provider {
	case types.LLMProviderOpenAI:
		// OpenAI API requires only `Authorization` header carrying the API key.
		//
		// https://developers.openai.com/api/reference/overview#authentication
		providerHeaders.Set("Authorization", "Bearer "+cfg.GetAPIKeyFunc())
	case types.LLMProviderAWSBedrock:
		cfg.ProviderURL = bedrock.BuildOpenAIURL(cfg.Logger, cfg.App, info.endpointType == endpointTypeResponses)
	default:
		return nil, info, trace.NotImplemented("provider %q is not supported", llm.Provider)
	}

	body, err := utils.ReadAtMost(cfg.DownstreamRequest.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return nil, info, trace.Wrap(err)
	}
	defer cfg.DownstreamRequest.Body.Close()

	if err := utils.FastUnmarshal(body, &req); err != nil {
		return nil, info, trace.BadParameter("unable to parse request body")
	}

	// Clients can send `null` JSON values, and since we're decoding into an
	// interface (which can hold a `nil` reference), decoding `null` into the
	// interface causes it to be `nil`. This condition guards against this case
	// avoiding a nil pointer exception.
	if req == nil {
		return nil, info, trace.BadParameter("invalid request body")
	}

	if err := req.Validate(); err != nil {
		return nil, info, trace.Wrap(err)
	}

	info.requestedModel = req.GetModel()
	info.stream = req.GetStream()

	providerModel, found := models.ConvertName(llm.Models, llm.FallbackModel, req.GetModel())
	if !found {
		return nil, info, trace.BadParameter("requested model %q is not supported", req.GetModel())
	}
	info.providerModel = providerModel
	req.SetModel(providerModel)
	req.EnableReportUsage()
	req.DisableDataRetention()

	providerBody, err := utils.FastMarshal(req)
	if err != nil {
		return nil, info, trace.ConnectionProblem(err, "failed to generate provider request")
	}
	info.requestSize = len(providerBody)

	// Since we're doing a complete rewrite of the downstream request, it is
	// easier to use a "fresh" request, and copy what is used from downstream
	// request.
	providerReq, err := http.NewRequestWithContext(
		cfg.DownstreamRequest.Context(), // Keep original context so cancelation propagates.
		providerMethod,
		cfg.ProviderURL.JoinPath(providerPath).String(),
		bytes.NewBuffer(providerBody),
	)
	if err != nil {
		return nil, info, trace.Wrap(err)
	}
	providerReq.Header = providerHeaders

	if llm.Provider == types.LLMProviderAWSBedrock {
		if err := cfg.SignBedrockRequest(providerReq.Context(), cfg.App, providerReq, providerBody); err != nil {
			// The signing failure cause can carry AWS credentials/config
			// details, so it is only logged, and clients receive a generic
			// internal error message.
			cfg.Logger.ErrorContext(providerReq.Context(), "failed to sign provider request", "error", err)
			return nil, info, llmerrors.ErrConfig
		}
	}

	return providerReq, info, nil
}
