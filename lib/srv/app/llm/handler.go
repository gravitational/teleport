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
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/srv/app/llm/anthropic"
	"github.com/gravitational/teleport/lib/srv/app/llm/bedrock"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	llmlimiter "github.com/gravitational/teleport/lib/srv/app/llm/limiter"
	"github.com/gravitational/teleport/lib/srv/app/llm/openai"
	llmrequest "github.com/gravitational/teleport/lib/srv/app/llm/request"
)

// Handler proxies LLM API requests for authorized app sessions.
type Handler struct {
	cfg          HandlerConfig
	closeContext context.Context
	metrics      *llmMetrics

	anthropicProviderURL *url.URL
	anthropicAPIKey      string
	openAIProviderURL    *url.URL
	openAIAPIKey         string
}

// HandlerConfig configures dependencies for the LLM proxy handler.
type HandlerConfig struct {
	// Log is the logger used by the handler.
	Log *slog.Logger
	// MetricsRegistry configures where metrics should be registered.
	// When nil, metrics are created but not registered.
	MetricsRegistry *metrics.Registry
	// AWSConfigProvider is the AWS config provider used by the handler.
	AWSConfigProvider awsconfig.Provider
	// Transport is the transport used to issue requests to the upstream
	// LLM provider.
	Transport *http.Transport
	// Limiter is the tokens limiter applied to the requests.
	Limiter Limiter
}

// CheckAndSetDefaults validates required dependencies and sets defaults.
func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, teleport.ComponentLLM)
	}
	if c.MetricsRegistry == nil {
		c.MetricsRegistry = metrics.NoopRegistry()
	}
	if c.AWSConfigProvider == nil {
		var err error
		c.AWSConfigProvider, err = awsconfig.NewCache()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.Transport == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Transport = tr
	}
	if c.Limiter == nil {
		// Attempt to initialize the in-memory limiter which acts on the entire
		// server instance.
		inputRaw := os.Getenv(inputTokensQuotaEnvVar)
		outputRaw := os.Getenv(outputTokensQuotaEnvVar)
		if inputRaw != "" || outputRaw != "" {
			if inputRaw == "" || outputRaw == "" {
				return trace.BadParameter("both %q and %q environment variables must be set to enable the in-memory limiter", inputTokensQuotaEnvVar, outputTokensQuotaEnvVar)
			}

			input, err := strconv.ParseUint(inputRaw, 10, 64)
			if err != nil {
				return trace.Wrap(err, "failed to parse input limit set on %q environment variable", inputTokensQuotaEnvVar)
			}
			output, err := strconv.ParseUint(outputRaw, 10, 64)
			if err != nil {
				return trace.Wrap(err, "failed to parse output limit set on %q environment variable", outputTokensQuotaEnvVar)
			}
			c.Limiter = llmlimiter.NewInMemory(uint(input), uint(output))
			c.Log.InfoContext(context.Background(), "using in-memory limiter", "input", input, "output", output)
		} else {
			// No settings, default to no limiter.
			c.Limiter = &noopLimiter{}
		}
	}
	return nil
}

// NewHandler creates a configured LLM proxy handler.
func NewHandler(ctx context.Context, cfg HandlerConfig) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := newMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		closeContext: ctx,
		cfg:          cfg,
		metrics:      m,
		// Not much validation can be applied here since some providers might
		// not require an actual API key.
		anthropicAPIKey: os.Getenv(anthropicAPIKeyEnvVarName),
		openAIAPIKey:    os.Getenv(openAIAPIKeyEnvVarName),
	}

	// It ok to leave this value as `nil`, the value receivers must implement a
	// default value for it.
	if rawAnthropicURL := os.Getenv(anthropicAddressEnvVarName); rawAnthropicURL != "" {
		var err error
		h.anthropicProviderURL, err = url.Parse(rawAnthropicURL)
		if err != nil {
			// Hard failure.
			return nil, trace.Wrap(err)
		}
		h.cfg.Log.InfoContext(ctx, "using anthropic provider address from environment", "host", h.anthropicProviderURL.Host)
	}
	if rawOpenAIURL := os.Getenv(openAIAddressEnvVarName); rawOpenAIURL != "" {
		var err error
		h.openAIProviderURL, err = url.Parse(rawOpenAIURL)
		if err != nil {
			// Hard failure.
			return nil, trace.Wrap(err)
		}
		h.cfg.Log.InfoContext(ctx, "using openai provider address from environment", "host", h.openAIProviderURL.Host)
	}

	return h, nil
}

// ServeHTTP handles an authorized client connection.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.serveHTTP(w, r); err != nil {
		h.cfg.Log.ErrorContext(r.Context(), "failed to handle LLM request", "error", err)
		trace.WriteError(w, err)
	}
}

// HandleError handles an error and forwards LLM compatible results to the
// client.
func (h *Handler) HandleError(r *http.Request, w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	sessionCtx, sessionCtxErr := common.GetSessionContext(r)
	if sessionCtxErr != nil {
		trace.WriteError(w, err)
		return
	}

	llm := sessionCtx.App.GetLLM()
	log := h.cfg.Log.With(
		"app", sessionCtx.App.GetName(),
		"format", llm.Format,
		"provider", llm.Provider,
	)

	var formattedErr error
	switch {
	case trace.IsNotImplemented(err):
		formattedErr = llmerrors.NewProviderError(llmerrors.ErrUnsupported, err.Error())
	default:
		// Other errors are most likely related to Teleport processing the
		// request which is not valuable for callers, so log the error and
		// return a generic one for the downstream.
		log.ErrorContext(h.closeContext, "failed to process llm request due to error", "err", err)
		formattedErr = llmerrors.ErrInternal
	}

	defer func() {
		if auditErr := sessionCtx.Audit.OnLLMRequest(
			h.closeContext,
			sessionCtx,
			r,
			common.LLMRequest{Format: llm.Format, Provider: llm.Provider},
			common.LLMResponse{Error: formattedErr},
		); auditErr != nil {
			log.ErrorContext(h.closeContext, "failed to emit audit event for failed request", "error", auditErr)
		}
	}()

	switch llm.Format {
	case types.LLMFormatAnthropic:
		if werr := anthropic.WriteError(w, formattedErr); werr != nil {
			log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
		}
	case types.LLMFormatOpenAI:
		if werr := openai.WriteError(w, formattedErr); werr != nil {
			log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
		}
	default:
		trace.WriteError(w, formattedErr)
	}
}

// serveHTTP routes requests to the configured LLM provider handler.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	sessionCtx, err := common.GetSessionContext(r)
	if err != nil {
		return trace.Wrap(err)
	}

	llm := sessionCtx.App.GetLLM()
	start := time.Now()
	defer func() {
		h.metrics.requestDuration.WithLabelValues(
			teleport.ComponentLLM,
			llm.Format,
			llm.Provider,
		).Observe(time.Since(start).Seconds())
	}()

	switch llm.Format {
	case types.LLMFormatAnthropic:
		h.handleRequest(
			sessionCtx, w, r,
			func(app types.Application, r *http.Request) (*llmrequest.Request, error) {
				return anthropic.NewRequest(&llmrequest.Config{
					App:               app,
					DownstreamRequest: r,
					ProviderURL:       h.anthropicProviderURL,
					GetAPIKeyFunc: func() string {
						return h.anthropicAPIKey
					},
					SignBedrockRequest: h.signBedrockRequest,
					Reserve:            h.cfg.Limiter.Reserve,
				})
			},
			func(l *slog.Logger, _ llmrequest.RequestInfo, w http.ResponseWriter) (UpstreamRecorder, error) {
				return anthropic.NewResponseRecorder(l, w)
			},
			anthropic.WriteError,
		)
	case types.LLMFormatOpenAI:
		h.handleRequest(
			sessionCtx, w, r,
			func(app types.Application, r *http.Request) (*llmrequest.Request, error) {
				return openai.NewRequest(&llmrequest.Config{
					App:               app,
					DownstreamRequest: r,
					ProviderURL:       h.openAIProviderURL,
					GetAPIKeyFunc: func() string {
						return h.openAIAPIKey
					},
					SignBedrockRequest: h.signBedrockRequest,
					Reserve:            h.cfg.Limiter.Reserve,
				})
			},
			func(l *slog.Logger, info llmrequest.RequestInfo, w http.ResponseWriter) (UpstreamRecorder, error) {
				return openai.NewResponseRecorder(l, info, w)
			},
			openai.WriteError,
		)
	default:
		return trace.NotImplemented("llm format %q not supported", llm.Format)
	}

	return nil
}

func (h *Handler) signBedrockRequest(ctx context.Context, app types.Application, request *http.Request, requestBody []byte) error {
	return trace.Wrap(bedrock.SignRequest(ctx, bedrock.SignRequestOptions{
		App:         app,
		Credentials: h.cfg.AWSConfigProvider,
		Request:     request,
		RequestBody: requestBody,
	}))
}

// WriteErrorFunc is function used to write an error into the downstream request.
type WriteErrorFunc func(http.ResponseWriter, error) error

// NewUpstreamRequestFunc function used to create a new upstream request.
type NewUpstreamRequestFunc func(app types.Application, r *http.Request) (*llmrequest.Request, error)

// NewUpstreamRecoderFunc function used to initialize a upstream response
// recorder.
type NewUpstreamRecoderFunc func(*slog.Logger, llmrequest.RequestInfo, http.ResponseWriter) (UpstreamRecorder, error)

// UpstreamRecorder records upstream results.
type UpstreamRecorder interface {
	http.ResponseWriter

	// Written is the number of bytes written in the downstream connection.
	Written() int
	// Err is the error returned to the downstream connection.
	Err() error
	// InputTokensCount is the number of input tokens that were used on the
	// request.
	InputTokensCount() int
	// OutputTokensCount is the number of output tokens generated by the
	// request.
	OutputTokensCount() int
	// Close closes the recorder.
	Close() error
}

// handleRequest handles an LLM request.
func (h *Handler) handleRequest(
	sessionCtx *common.SessionContext,
	w http.ResponseWriter,
	r *http.Request,
	newRequestFunc NewUpstreamRequestFunc,
	newRecorderFunc NewUpstreamRecoderFunc,
	writeErrorFunc WriteErrorFunc,
) {
	llm := sessionCtx.App.GetLLM()
	log := h.cfg.Log.With(
		"app", sessionCtx.App.GetName(),
		"format", llm.Format,
		"provider", llm.Provider,
	)

	var (
		err        error
		info       llmrequest.RequestInfo
		rec        UpstreamRecorder
		settleFunc llmlimiter.SettleFunc = llmlimiter.EmptySettleFunc
	)

	defer func() {
		req := common.LLMRequest{Format: llm.Format, Provider: llm.Provider}
		if info != nil {
			req.Model = info.ProviderModel()
			req.RequestedModel = info.RequestedModel()
		}
		resp := common.LLMResponse{Error: err}
		if rec != nil {
			resp.InputTokenCount = rec.InputTokensCount()
			resp.OutputTokenCount = rec.OutputTokensCount()
		}
		if settleFunc != nil {
			settleFunc(h.closeContext, llmlimiter.Usage{
				InputTokens:  uint(resp.InputTokenCount),
				OutputTokens: uint(resp.OutputTokenCount),
			})
		}
		if err := sessionCtx.Audit.OnLLMRequest(h.closeContext, sessionCtx, r, req, resp); err != nil {
			log.ErrorContext(h.closeContext, "failed to emit audit event", "error", err)
		}
	}()

	providerReq, err := newRequestFunc(sessionCtx.App, r)
	// Even in cases of error the request could contain the info and settle
	// we need to still perform. So we do the assignment before dealing with
	// the error.
	if providerReq != nil {
		info = providerReq.Info
		settleFunc = providerReq.SettleFunc
	}
	if err != nil {
		log.ErrorContext(r.Context(), "failed to rewrite request", "error", err)
		if werr := writeErrorFunc(w, err); werr != nil {
			log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
		}
		return
	}

	rec, err = newRecorderFunc(log, info, w)
	if err != nil {
		log.ErrorContext(r.Context(), "failed to initialize recorder", "error", err)
		// This is considered an "internal" error. For downstream connections,
		// just return a generic error.
		if werr := writeErrorFunc(w, llmerrors.ErrUnknown); werr != nil {
			log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
		}
		return
	}

	h.metrics.requestSize.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
		llm.Provider,
	).Observe(float64(info.RequestSize()))

	fwd, err := reverseproxy.New(
		// TODO(gabrielcorado): revisit this flush interval to reduce time to first token (TTFT).
		reverseproxy.WithFlushInterval(50*time.Millisecond),
		reverseproxy.WithRoundTripper(h.cfg.Transport),
		reverseproxy.WithLogger(log),
		reverseproxy.WithErrorHandler(func(_ http.ResponseWriter, r *http.Request, fwdErr error) {
			// reverseproxy already logs this error, so no need to log it here.
			if werr := writeErrorFunc(w, fwdErr); werr != nil {
				log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
			}
			err = fwdErr
		}),
	)
	if err != nil {
		log.ErrorContext(h.closeContext, "failed to initialize provider forwarder", "error", err)
		if werr := writeErrorFunc(w, err); werr != nil {
			log.ErrorContext(h.closeContext, "failed to write error", "error", werr)
		}
		return
	}

	start := time.Now()
	fwd.ServeHTTP(rec, providerReq.HTTPRequest)
	if err := rec.Close(); err != nil {
		log.ErrorContext(h.closeContext, "failed to close llm recorder", "error", err)
	}

	// In case a forwarder error happens, we must keep that.
	if err == nil {
		err = rec.Err()
	}

	h.metrics.providerRequestDuration.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
		llm.Provider,
		strconv.FormatBool(info.IsStream()),
	).Observe(time.Since(start).Seconds())

	h.metrics.providerResponseSize.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
		llm.Provider,
		strconv.FormatBool(info.IsStream()),
	).Observe(float64(rec.Written()))

	h.metrics.inputTokens.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
	).Add(float64(rec.InputTokensCount()))

	h.metrics.outputTokens.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
	).Add(float64(rec.OutputTokensCount()))
}

// Limiter defines the interface of tokens limiter.
type Limiter interface {
	Reserve(context.Context, llmlimiter.ReserveRequest) (llmlimiter.ReserveInfo, llmlimiter.SettleFunc, error)
}

// noopLimiter implements a noop tokens limiter.
type noopLimiter struct{}

// Reserve implements [Limiter].
func (n *noopLimiter) Reserve(_ context.Context, req llmlimiter.ReserveRequest) (llmlimiter.ReserveInfo, llmlimiter.SettleFunc, error) {
	return llmlimiter.ReserveInfo{
		// This should skip all APIs from limits.
		NotApplicable: true,
	}, llmlimiter.EmptySettleFunc, nil
}

const (
	// anthropicAddressEnvVarName is the Anthropic's default environment
	// variable used to set base API address.
	//
	// https://code.claude.com/docs/en/env-vars#variables
	anthropicAddressEnvVarName = "ANTHROPIC_BASE_URL"
	// anthropicAPIKeyEnvVarName is the Anthropic's default environment variable
	// used to set API keys.
	//
	// https://code.claude.com/docs/en/env-vars#variables
	anthropicAPIKeyEnvVarName = "ANTHROPIC_API_KEY"
	// openAIAddressEnvVarName is the OpenAI's environment variable used to set
	// base API address.
	openAIAddressEnvVarName = "OPENAI_BASE_URL"
	// openAIAPIKeyEnvVarName is the OpenAI's environment variable used to
	// set API keys.
	openAIAPIKeyEnvVarName = "OPENAI_API_KEY"

	// inputTokensQuotaEnvVar is the name of the environment variable used to
	// set the input tokens quota.
	inputTokensQuotaEnvVar = "TELEPORT_BEAMS_BETA_INPUT_TOKENS_QUOTA"
	// outputTokensQuotaEnvVar is the name of the environment variable used to
	// set the output tokens quota.
	outputTokensQuotaEnvVar = "TELEPORT_BEAMS_BETA_OUTPUT_TOKENS_QUOTA"
)
