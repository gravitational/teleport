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
	"cmp"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// Handler proxies LLM API requests for authorized app sessions.
type Handler struct {
	cfg          HandlerConfig
	closeContext context.Context

	// TODO(gabrielcorado): use a persistent format for limiting LLM token consumption.
	anthropicInputTokensCount  atomic.Int64
	anthropicOutputTokensCount atomic.Int64
	openAIInputTokensCount     atomic.Int64
	openAIOutputTokensCount    atomic.Int64
}

// HandlerConfig configures dependencies for the LLM proxy handler.
type HandlerConfig struct {
	// Log is the logger used by the handler.
	Log *slog.Logger
	// AWSConfigProvider is the AWS config provider used by the handler.
	AWSConfigProvider awsconfig.Provider
	// HTTPClient is the HTTP client used to issue requests to the upstream
	// LLM provider. Defaults to a Teleport-configured client.
	HTTPClient *http.Client
	// InputTokensQuotaPerFormat defines the service-level input tokens quota
	// per API format.
	//
	// TODO(gabrielcorado): revisit this.
	InputTokensQuotaPerFormat int64
	// OutputTokensQuotaPerFormat defines the service-level output tokens quota
	// per API format.
	//
	// TODO(gabrielcorado): revisit this.
	OutputTokensQuotaPerFormat int64
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
	if c.HTTPClient == nil {
		client, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		c.HTTPClient = client
	}

	// Defaults to 10M tokens.
	c.InputTokensQuotaPerFormat = cmp.Or(c.InputTokensQuotaPerFormat, 10_000_000)
	c.OutputTokensQuotaPerFormat = cmp.Or(c.OutputTokensQuotaPerFormat, 10_000_000)

	if raw := os.Getenv(inputTokensQuotaEnvVar); raw != "" {
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return trace.Wrap(err, "failed to parse %q environment setting", inputTokensQuotaEnvVar)
		}
		c.InputTokensQuotaPerFormat = val
	}

	if raw := os.Getenv(outputTokensQuotaEnvVar); raw != "" {
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return trace.Wrap(err, "failed to parse %q environment setting", outputTokensQuotaEnvVar)
		}
		c.OutputTokensQuotaPerFormat = val
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
		h.cfg.Log.ErrorContext(r.Context(), "failed to handle LLM request", "error", err)
		trace.WriteError(w, err)
	}
}

// serveHTTP routes requests to the configured LLM provider handler.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	sessionCtx, err := common.GetSessionContext(r)
	if err != nil {
		return trace.Wrap(err)
	}

	format := sessionCtx.App.GetLLM().Format
	switch format {
	case types.LLMFormatAnthropic:
		h.handleAnthropic(sessionCtx, w, r)
		return nil
	case types.LLMFormatOpenAI:
		h.handleOpenAI(sessionCtx, w, r)
		return nil
	default:
		return trace.NotImplemented("llm format %q not supported", format)
	}
}

// readLimitedRequestBody reads request body respecting the app service memory
// limits.
func (h *Handler) readLimitedRequestBody(r *http.Request, maxSize int64) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if r.ContentLength > maxSize {
		return nil, trace.BadParameter("request content is too large, consider splitting the request")
	}
	if _, err := buf.ReadFrom(io.LimitReader(r.Body, maxSize+1)); err != nil {
		return nil, trace.Wrap(err)
	}
	if int64(buf.Len()) > maxSize {
		return nil, trace.BadParameter("request content is too large, consider splitting the request")
	}
	return &buf, nil
}

// ensureV1PathPrefix is middleware that guarantees the outgoing
// request URL path always contains exactly one /v1 prefix. It strips all
// existing /v1 prefixes (which may come from both the incoming request path
// and the SDK base URL) and then prepends a single /v1. This handles all
// combinations of request paths and base URLs regardless of whether they
// include /v1.
func ensureV1PathPrefix(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	path := req.URL.Path
	for strings.HasPrefix(path, "/v1") {
		path = strings.TrimPrefix(path, "/v1")
	}
	req.URL.Path = "/v1" + path
	return next(req)
}

const (
	// llmMaxRequestSize represents the max size (in bytes) of the LLM requests.
	// For LLMs, we use a higher value than `MaxHTTPRequestSize` since those
	// requests are naturally large.
	llmMaxRequestSize = 32 * 1024 * 1024
	// defaultStreamingRequestTimeout establishes a default timing for a
	// streaming request. Note that those type of requests can take long time
	// to complete.
	defaultStreamingRequestTimeout = 1 * time.Hour
	// maxNonStreamingTokens represents the max tokens a non-streaming request
	// can request/return.
	maxNonStreamingTokens = 8192
)

const (
	// inputTokensQuotaEnvVar is the name of the environment variable used to
	// set the input tokens quota.
	inputTokensQuotaEnvVar = "TELEPORT_BEAMS_BETA_INPUT_TOKENS_QUOTA"
	// outputTokensQuotaEnvVar is the name of the environment variable used to
	// set the output tokens quota.
	outputTokensQuotaEnvVar = "TELEPORT_BEAMS_BETA_OUTPUT_TOKENS_QUOTA"
)
