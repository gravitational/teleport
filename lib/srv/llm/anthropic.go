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
	"fmt"
	"io"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/gravitational/trace"
	"github.com/tidwall/gjson"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// handleAnthropic proxies an Anthropic-compatible request and forwards the response.
func (h *Handler) handleAnthropic(sessionCtx *common.SessionContext, w http.ResponseWriter, r *http.Request) error {
	app := sessionCtx.App
	log := h.cfg.Log.With(
		"format", app.GetLLMFormat().DisplayName(),
		"provider", app.GetLLMProvider().DisplayName(),
		"method", r.Method,
		"path", r.URL.Path,
		"params", r.URL.RawQuery,
	)

	var (
		opts []option.RequestOption
		raw  *http.Response
		buf  bytes.Buffer
	)

	// TODO(gabrielcorado): validate what should be the limit here
	if r.ContentLength > 0 {
		buf.Grow(int(r.ContentLength))
	}

	if _, err := buf.ReadFrom(r.Body); err != nil {
		log.ErrorContext(r.Context(), "failed to read request body", "error", err)
		return trace.Wrap(err)
	}

	switch app.GetLLMProvider() {
	case types.LLM_PROVIDER_ANTHROPIC:
		// TODO(gabrielcorado): should we do model mapping here as well?
		log.DebugContext(r.Context(), "defaulting to using environment config")
		opts = anthropic.DefaultClientOptions()
	case types.LLM_PROVIDER_AWS_BEDROCK:
		log.DebugContext(r.Context(), "defaulting to using environment config")
		awscfg, err := h.cfg.AWSConfigProvider.GetConfig(
			r.Context(),
			"", /* get region from env > profile > fallback func */
			awsconfig.WithAmbientCredentials(),
			// TODO(gabrielcorado): add support for AWS integration.
		)
		if err != nil {
			log.ErrorContext(r.Context(), "unable to retrieve aws config", "error", err)
			return trace.Wrap(err)
		}

		reqModel := gjson.GetBytes(buf.Bytes(), "model").String()
		resolvedModel := convertModelName(app.GetLLM().ModelMappings, app.GetLLM().DefaultModel, reqModel)
		log.DebugContext(r.Context(), "switching model", "from", reqModel, "to", resolvedModel)
		opts = []option.RequestOption{
			bedrock.WithConfig(awscfg),
			option.WithJSONSet("model", resolvedModel),
		}
	default:
		log.ErrorContext(r.Context(), "unsupported provider")
		return trace.BadParameter("unsupported provider")
	}

	// Used in tests to direct requests to a mock API.
	if h.cfg.anthropicBaseURL != "" {
		opts = append(opts, option.WithBaseURL(h.cfg.anthropicBaseURL))
	}

	clt := anthropic.NewClient(opts...)
	// TODO(gabrielcorado): forward HTTP headers
	// TODO(gabrielcorado): better URL params handling
	if err := clt.Execute(r.Context(), r.Method, r.URL.Path+"?"+r.URL.RawQuery, &buf, &raw); err != nil {
		var reqErr *anthropic.Error
		if !errors.As(err, &reqErr) {
			// In case we cannot grab the actual status code, we set it to
			// always be internal server error (as mostly likely the API
			// returned bad error JSON).
			if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, http.StatusInternalServerError, nil); err != nil {
				log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
			}

			log.ErrorContext(r.Context(), "request failed", "error", err)
			return trace.Wrap(err)
		}

		if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, uint32(reqErr.StatusCode), nil); err != nil {
			log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
		}

		// Return a better error message in case of auth failure.
		if reqErr.StatusCode == http.StatusUnauthorized {
			log.ErrorContext(r.Context(), "authentication failure, ensure your Anthropic credentials are configured correctly")
			// here we're just overwriting the message
			errObj := anthropic.AuthenticationError{
				Message: "Anthropic provider is not configured for the requested app. Contact your Teleport adminstrator about this issue.",
			}
			if err := json.NewEncoder(w).Encode(errObj); err != nil {
				log.WarnContext(r.Context(), "failed to write proper auth error", "error", err)
				return trace.Wrap(err)
			}

			return nil
		}

		log.DebugContext(r.Context(), "request failed", "status_code", reqErr.StatusCode)
		w.WriteHeader(reqErr.StatusCode)
		if _, err := io.Copy(w, reqErr.Response.Body); err != nil {
			log.WarnContext(r.Context(), "failed to forward API error", "error", err)
		}

		return nil
	}

	if err := sessionCtx.Audit.OnRequest(h.closeContext, sessionCtx, r, uint32(raw.StatusCode), nil); err != nil {
		log.WarnContext(r.Context(), "failed to emit audit event", "error", err)
	}

	w.WriteHeader(raw.StatusCode)
	defer raw.Body.Close()

	streaming := gjson.GetBytes(buf.Bytes(), "stream").Bool()
	switch {
	case streaming:
		if err := streamBedrockEvents(w, raw); err != nil {
			log.WarnContext(r.Context(), "failed to forward response", "error", err)
			return trace.Wrap(err)
		}
	default:
		if _, err := io.Copy(w, raw.Body); err != nil {
			log.WarnContext(r.Context(), "failed to forward response", "error", err)
			return trace.Wrap(err)
		}
	}

	log.DebugContext(r.Context(), "request handled with success", "streaming", streaming)
	return nil
}

// streamBedrockEvents rewrites Bedrock SSE payloads into standard text events.
//
// Bedrock uses an AWS-specific streaming event shape, so this normalization is
// required to keep compatibility with Anthropic-style SSE clients.
func streamBedrockEvents(w http.ResponseWriter, bedrockResp *http.Response) error {
	rc := http.NewResponseController(w)
	dec := ssestream.NewDecoder(bedrockResp)
	defer dec.Close()

	for dec.Next() {
		evt := dec.Event()
		fmt.Fprintf(w, "event: %s\n", evt.Type)
		for data := range bytes.SplitSeq(evt.Data, []byte("\n")) {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		}
		if err := rc.Flush(); err != nil {
			return trace.Wrap(err)
		}
	}

	err := dec.Err()
	if err != io.EOF {
		return trace.Wrap(err)
	}

	return nil
}
