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
	"slices"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
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

	llm := cfg.App.GetLLM()
	// TODO(gabrielcorado): add support for bedrock provider.
	switch llm.Provider {
	case types.LLMProviderOpenAI:
		switch strings.TrimPrefix(cfg.DownstreamRequest.URL.Path, "/v1") {
		// https://developers.openai.com/api/reference/resources/responses/methods/create
		case "/responses":
			if cfg.DownstreamRequest.Method != http.MethodPost {
				// We're ok with returning 404 back to clients instead 405 status.
				return nil, info, trace.NotFound("responses API supports only POST requests")
			}

			// Currently, websocket mode is not supported.
			//
			// https://developers.openai.com/api/docs/guides/websocket-mode
			if slices.Contains(cfg.DownstreamRequest.Header.Values(constants.WebAPIConnUpgradeHeader), constants.WebAPIConnUpgradeTypeWebSocket) {
				return nil, info, trace.NotFound("websocket mode is not supported")
			}

			info.endpointType = endpointTypeResponses
			providerPath = "/responses"
			providerMethod = http.MethodPost
			req = &responsesAPIRequest{}
		default:
			return nil, info, trace.NotFound("unsupported endpoint")
		}

		// OpenAI API requires only `Authorization` header carrying the API key.
		//
		// https://developers.openai.com/api/reference/overview#authentication
		providerHeaders.Set("Authorization", "Bearer "+cfg.GetAPIKeyFunc())
		providerHeaders.Set("content-type", "application/json")
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
	return providerReq, info, nil
}
