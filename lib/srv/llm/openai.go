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

	opts := openai.DefaultClientOptions()
	log.DebugContext(r.Context(), "no inference model config found, defaulting to using environment config")

	if h.cfg.openAIBaseURL != "" {
		opts = append(opts, openaiopt.WithBaseURL(h.cfg.openAIBaseURL), openaiopt.WithAPIKey("test-key"))
	}

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
