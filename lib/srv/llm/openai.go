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

package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"
	openaiopt "github.com/openai/openai-go/v3/option"

	"github.com/gravitational/teleport/lib/srv/app/common"
)

// handleOpenAI proxies an OpenAI-compatible request and forwards the response.
//
// TODO(gabrielcorado): add support for OpenAI models in AWS Bedrock.
func (h *Handler) handleOpenAI(sessionCtx *common.SessionContext, w http.ResponseWriter, r *http.Request) error {
	app := sessionCtx.App
	log := h.cfg.Log.With("format", app.GetLLMFormat().DisplayName(), "provider", app.GetLLMProvider().DisplayName(), "method", r.Method, "path", r.URL.Path, "params", r.URL.RawQuery)
	log.DebugContext(r.Context(), "handling request")

	_, pattern := h.openAIMux.Handler(r)
	if len(pattern) == 0 {
		log.WarnContext(r.Context(), "requested endpoint not supported")
		errObj := openai.ErrorObject{
			Message: "The requested endpoint is not supported by Teleport LLM proxy.",
		}
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(errObj); err != nil {
			log.WarnContext(r.Context(), "failed to write not found error", "error", err)
			return trace.Wrap(err)
		}

		return nil
	}

	opts := openai.DefaultClientOptions()
	log.DebugContext(r.Context(), "no inference model config found, defaulting to using environment config")

	if h.cfg.openAIBaseURL != "" {
		opts = append(opts, openaiopt.WithBaseURL(h.cfg.openAIBaseURL), openaiopt.WithAPIKey("test-key"))
	}

	// Ensure the outgoing request URL always contains exactly one /v1 path
	// prefix. This protects against misconfigured base URLs (e.g. via
	// OPENAI_BASE_URL env var) that omit the /v1 segment.
	opts = append(opts, openaiopt.WithMiddleware(ensureV1PathPrefix))

	var (
		clt = openai.NewClient(opts...)
		raw *http.Response
		buf bytes.Buffer
	)

	// TODO(gabrielcorado): validate what should be the limit here
	if r.ContentLength > 0 {
		buf.Grow(int(r.ContentLength))
	}

	if _, err := buf.ReadFrom(r.Body); err != nil {
		log.ErrorContext(r.Context(), "failed to read request body", "error", err)
		return trace.Wrap(err)
	}

	// TODO(gabrielcorado): forward HTTP headers
	// TODO(gabrielcorado): better URL params handling
	if err := clt.Execute(r.Context(), r.Method, r.URL.Path, &buf, &raw); err != nil {
		var openaiErr *openai.Error
		if !errors.As(err, &openaiErr) {
			// In case we cannot grab the actual status code, we set it to
			// always be internal server error (as mostly likely the API
			// returned bad error JSON).
			if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, http.StatusInternalServerError, nil); err != nil {
				log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
			}

			log.ErrorContext(r.Context(), "request failed", "error", err)
			return trace.Wrap(err)
		}

		if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, uint32(openaiErr.StatusCode), nil); err != nil {
			log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
		}

		// Return a better error message in case of auth failure.
		if openaiErr.StatusCode == http.StatusUnauthorized {
			log.ErrorContext(r.Context(), "authentication failure, ensure your OpenAI credentials are configured correctly")
			// here we're just overwriting the message
			errObj := openai.ErrorObject{
				Code:    openaiErr.Code,
				Message: "OpenAI provider is not configured for the requested app. Contact your Teleport adminstrator about this issue.",
				Param:   openaiErr.Param,
				Type:    openaiErr.Type,
			}
			if err := json.NewEncoder(w).Encode(errObj); err != nil {
				log.WarnContext(r.Context(), "failed to write proper auth error", "error", err)
				return trace.Wrap(err)
			}

			return nil
		}

		log.DebugContext(r.Context(), "request failed", "status_code", openaiErr.StatusCode)
		w.WriteHeader(openaiErr.StatusCode)
		if _, err := io.Copy(w, openaiErr.Response.Body); err != nil {
			log.WarnContext(r.Context(), "failed to forward API error", "error", err)
		}

		return nil
	}

	if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, uint32(raw.StatusCode), nil); err != nil {
		log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
	}

	w.WriteHeader(raw.StatusCode)
	defer raw.Body.Close()
	if _, err := io.Copy(w, raw.Body); err != nil {
		log.WarnContext(r.Context(), "failed to forward response", "error", err)
		return trace.Wrap(err)
	}

	log.DebugContext(r.Context(), "request handled with success")
	return nil
}

// ensureV1PathPrefix is an OpenAI SDK middleware that guarantees the outgoing
// request URL path always contains exactly one /v1 prefix. It strips all
// existing /v1 prefixes (which may come from both the incoming request path
// and the SDK base URL) and then prepends a single /v1. This handles all
// combinations of request paths and base URLs regardless of whether they
// include /v1.
func ensureV1PathPrefix(req *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
	path := req.URL.Path
	for strings.HasPrefix(path, "/v1") {
		path = strings.TrimPrefix(path, "/v1")
	}
	req.URL.Path = "/v1" + path
	return next(req)
}

// newOpenAIMux initializes a http serve mux that has the list of supported
// OpenAI endpoints. This mux purpose is only to match requested endpoints and
// not serve any request, so it uses empty handlers.
//
// Each endpoint is registered under both "/" and "/v1/" prefixes so that
// Handler(r) returns a non-empty pattern only for supported routes.
func newOpenAIMux() *http.ServeMux {
	mux := http.NewServeMux()

	// registerEndpoint registers a handler for the given method and path under
	// both "/" and "/v1/" prefixes.
	registerEndpoint := func(method, path string) {
		mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {})
		mux.HandleFunc(method+" /v1"+path, func(w http.ResponseWriter, r *http.Request) {})
	}

	// Responses API: https://developers.openai.com/api/reference/responses/overview
	//
	// https://developers.openai.com/api/reference/resources/responses/methods/create
	registerEndpoint("POST", "/responses")
	// https://developers.openai.com/api/reference/resources/responses/methods/retrieve
	registerEndpoint("GET", "/responses/{response_id}")
	// https://developers.openai.com/api/reference/resources/responses/methods/delete
	registerEndpoint("DELETE", "/responses/{response_id}")
	// https://developers.openai.com/api/reference/resources/responses/subresources/input_items/methods/list
	registerEndpoint("GET", "/responses/{response_id}/input_items")
	// https://developers.openai.com/api/reference/resources/responses/methods/cancel
	registerEndpoint("POST", "/responses/{response_id}/cancel")
	// https://developers.openai.com/api/reference/resources/responses/methods/compact
	registerEndpoint("POST", "/responses/compact")
	// https://developers.openai.com/api/reference/resources/responses/subresources/input_tokens/methods/count
	registerEndpoint("POST", "/responses/input_tokens")

	// Legacy chat completions: https://developers.openai.com/api/reference/chat-completions/overview
	//
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/list
	registerEndpoint("GET", "/chat/completions")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
	registerEndpoint("POST", "/chat/completions")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/retrieve
	registerEndpoint("GET", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/update
	registerEndpoint("POST", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/delete
	registerEndpoint("DELETE", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/subresources/messages/methods/list
	registerEndpoint("GET", "/chat/completions/{completion_id}/messages")

	return mux
}
