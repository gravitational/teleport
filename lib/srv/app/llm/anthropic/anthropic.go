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
	"bytes"
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"

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
func NewRequest(cfg *llmrequest.Config) (*http.Request, *RequestInfo, error) {
	var (
		info            = &RequestInfo{}
		providerPath    string
		providerMethod  string
		providerHeaders = http.Header{}
	)

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, info, trace.Wrap(err)
	}

	llm := cfg.App.GetLLM()
	switch llm.Provider {
	case types.LLMProviderAnthropic:
		// Default URL address.
		//
		// https://platform.claude.com/docs/en/api/overview
		cfg.ProviderURL = cmp.Or(cfg.ProviderURL, &url.URL{
			Scheme: "https",
			Host:   "api.anthropic.com",
			Path:   "/v1",
		})

		switch strings.TrimPrefix(cfg.DownstreamRequest.URL.Path, "/v1") {
		case "/messages":
			if cfg.DownstreamRequest.Method != http.MethodPost {
				// We're ok with returning 404 back to clients instead 405 status.
				return nil, info, trace.NotFound("messages API supports only POST requests")
			}
			// Messages API endpoint supported.
			//
			// https://platform.claude.com/docs/en/api/messages/create
			providerPath = "/messages"
			providerMethod = http.MethodPost

			// Anthropic API supports the following headers:
			//   - "x-api-key": Holds the Anthropic API key. Mutually exclusive with "Authorization".
			//   - "Authorization": Carries short lived token generated using
			//     "Workload identity federation". We currently don't have support
			//     for this flow. Mutually exclusive with "x-api-key".
			//   - "anthropic-version": Indicates the Anthropic API version.
			//   - "content-type": Request content type.
			//
			// https://platform.claude.com/docs/en/api/overview#authentication
			providerHeaders.Set("x-api-key", cfg.GetAPIKeyFunc())
			providerHeaders.Set("anthropic-version", "2023-06-01") // Currently, the only version supported.
			providerHeaders.Set("content-type", "application/json")
		default:
			return nil, info, trace.NotFound("unsupported endpoint")
		}
	case types.LLMProviderAWSBedrock:
		var err error
		cfg.ProviderURL, err = bedrock.BuildURL(cfg.Logger, cfg.App)
		if err != nil {
			return nil, info, trace.Wrap(err)
		}

		switch strings.TrimPrefix(cfg.DownstreamRequest.URL.Path, "/v1") {
		case "/messages":
			if cfg.DownstreamRequest.Method != http.MethodPost {
				// We're ok with returning 404 back to clients instead 405 status.
				return nil, info, trace.NotFound("messages API supports only POST requests")
			}
			// Messages API endpoint supported.
			//
			// https://docs.aws.amazon.com/bedrock/latest/userguide/inference-messages-api.html
			providerPath = "/messages"
			providerMethod = http.MethodPost

			// Bedrock mantle require a specific API version and we're using
			// signed requests.
			//
			// https://docs.aws.amazon.com/bedrock/latest/userguide/inference-messages-api.html#inference-messages-api-prereq
			providerHeaders.Set("anthropic-version", "2023-06-01")
			providerHeaders.Set("content-type", "application/json")
		default:
			return nil, info, trace.NotFound("unsupported endpoint")
		}
	default:
		return nil, info, trace.NotImplemented("provider %q is not supported", llm.Provider)
	}

	body, err := utils.ReadAtMost(cfg.DownstreamRequest.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return nil, info, trace.Wrap(err)
	}
	defer cfg.DownstreamRequest.Body.Close()

	var req messagesAPIRequest
	if err := utils.FastUnmarshal(body, &req); err != nil {
		return nil, info, trace.BadParameter("unable to parse request body")
	}
	info.requestedModel = req.Model
	info.stream = req.Stream

	providerModel, found := models.ConvertName(llm.Models, llm.FallbackModel, req.Model)
	if !found {
		return nil, info, trace.BadParameter("requested model %q is not supported", req.Model)
	}
	info.providerModel = providerModel
	req.Model = providerModel

	if req.MaxTokens > maxNonStreamingTokens && !info.IsStream() {
		return nil, info, trace.BadParameter("max_tokens must be %d or less for non-streaming requests", maxNonStreamingTokens)
	}

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

// WriteError writes an error in Anthropic format.
func WriteError(w http.ResponseWriter, err error) error {
	w.WriteHeader(llmerrors.StatusCodeFromErr(err))
	_, werr := w.Write(marshalError(newErrorMessage(err)))
	return trace.Wrap(werr)
}

// marshalError marshals an error into Anthropic format.
func marshalError(apiErr *errorEnvelope) []byte {
	enc, err := utils.FastMarshal(apiErr)
	if err != nil {
		return []byte(
			`{"type": "error", "error": {"type": "api_error", "message": "` + llmerrors.ErrUnknown.Error() + `"}}`,
		)
	}
	return enc
}

// newErrorMessage creates a new error message.
func newErrorMessage(err error) *errorEnvelope {
	if err == nil {
		return nil
	}

	r := &errorEnvelope{
		Type: "error",
		Error: errorMessage{
			Type:    errorTypeAPIError,
			Message: err.Error(),
		},
	}
	switch {
	case errors.Is(err, llmerrors.ErrCanceled), errors.Is(err, llmerrors.ErrTimeout):
		r.Error.Type = errorTypeTimeoutError
	case errors.Is(err, llmerrors.ErrBadRequest), trace.IsBadParameter(err):
		r.Error.Type = errorTypeInvalidRequestError
	case errors.Is(err, llmerrors.ErrUnauthorized):
		r.Error.Type = errorTypeAuthenticationError
	case errors.Is(err, llmerrors.ErrRejected):
		r.Error.Type = errorTypeRateLimitError
	case errors.Is(err, llmerrors.ErrUnsupported), trace.IsNotFound(err):
		r.Error.Type = errorTypeNotFoundError
	}

	return r
}

// parseProviderError parses errors that come from Anthropic API.
func parseProviderError(body []byte) (*llmerrors.ProviderError, error) {
	var r errorEnvelope
	if err := utils.FastUnmarshal(body, &r); err != nil {
		return nil, trace.Wrap(err)
	}

	message := r.Error.Message
	switch r.Error.Type {
	case errorTypeTimeoutError:
		return llmerrors.NewProviderError(llmerrors.ErrTimeout, message), nil
	case errorTypeInvalidRequestError:
		return llmerrors.NewProviderError(llmerrors.ErrBadRequest, message), nil
	case errorTypeAuthenticationError, errorTypePermissionError:
		return llmerrors.NewProviderError(llmerrors.ErrUnauthorized, ""), nil
	case errorTypeRateLimitError, errorTypeOverloadedError, errorTypeBillingError:
		return llmerrors.NewProviderError(llmerrors.ErrRejected, message), nil
	case errorTypeNotFoundError:
		return llmerrors.NewProviderError(llmerrors.ErrUnsupported, message), nil
	default:
		return llmerrors.NewProviderError(llmerrors.ErrUnknown, ""), nil
	}
}

const (
	// maxNonStreamingTokens defines the max tokens that can be generated for
	// non-streaming requests.
	//
	// The rule for this definition by Anthropic is any request that might take
	// more than 10 minutes to complete, must use streaming.
	// To calculate how many tokens would 10 minutes take, we use the formula
	// available on Anthropic SDKs, for example, in the Golang SDK: https://github.com/anthropics/anthropic-sdk-go/blob/058d85cd7e656f5fe972591bcf841c99564581e9/client.go#L297
	// The result give us around 21k tokens. This value covers all non-legacy
	// models.
	maxNonStreamingTokens = 21_000
)
