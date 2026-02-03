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
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// Handler proxies LLM API requests for authorized app sessions.
type Handler struct {
	cfg          HandlerConfig
	closeContext context.Context
}

// HandlerConfig configures dependencies for the LLM proxy handler.
type HandlerConfig struct {
	// Log is the logger used by the handler.
	Log *slog.Logger
	// AWSConfigProvider is the AWS config provider used by the handler.
	AWSConfigProvider awsconfig.Provider
	// anthropicBaseURL overrides the Anthropic SDK base URL. Test use only.
	anthropicBaseURL string
	// openAIBaseURL overrides the OpenAI SDK base URL. Test use only.
	openAIBaseURL string
}

// CheckAndSetDefaults validates required dependencies and sets defaults.
func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, teleport.ComponentLLM)
	}
	if c.AWSConfigProvider == nil {
		var err error
		c.AWSConfigProvider, err = awsconfig.NewCache()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewHandler creates a configured LLM proxy handler.
func NewHandler(ctx context.Context, cfg HandlerConfig) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		closeContext: ctx,
		cfg:          cfg,
	}, nil
}

// ServeHTTP handles an authorized client connection.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.serveHTTP(w, r); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// serveHTTP routes requests to the configured LLM provider handler.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	sessionCtx, err := common.GetSessionContext(r)
	if err != nil {
		return trace.Wrap(err)
	}

	format := sessionCtx.App.GetLLMFormat()
	switch format {
	case types.LLM_FORMAT_OPENAI:
		return trace.Wrap(h.handleOpenAI(sessionCtx, w, r))
	case types.LLM_FORMAT_ANTHROPIC:
		return trace.Wrap(h.handleAnthropic(sessionCtx, w, r))
	default:
		return trace.NotImplemented("llm format %q not supported", format.DisplayName())
	}
}

// convertModelName takes a set model name and do the conversion based on the
// mapping rules.
func convertModelName(mappings []*types.LLM_ModelMap, defaultModel string, reqModel string) string {
	for _, m := range mappings {
		if strings.EqualFold(strings.TrimSpace(reqModel), m.From) {
			return m.To
		}
	}

	return defaultModel
}
