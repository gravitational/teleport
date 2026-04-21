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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/gravitational/trace"
	"github.com/tidwall/gjson"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// handleAnthropic proxies an Anthropic-compatible request and forwards the
// response.
func (h *Handler) handleAnthropic(sessionCtx *common.SessionContext, w http.ResponseWriter, r *http.Request) {
	llm := sessionCtx.App.GetLLM()
	log := h.cfg.Log.With(
		"format", llm.Format,
		"provider", llm.Provider,
		"method", r.Method,
		"path", r.URL.Path,
	)

	start := time.Now()
	defer func() {
		requestDurationHist.WithLabelValues(
			teleport.ComponentLLM,
			llm.Format,
			llm.Provider,
		).Observe(time.Since(start).Seconds())
	}()

	req, body, err := h.prepareAnthropicRequest(llm, r)
	if err != nil {
		apiErr := respondAnthropicError(w, err)
		auditErr := sessionCtx.Audit.OnLLMRequest(h.closeContext, sessionCtx, r, req, common.LLMResponse{Error: apiErr})
		if auditErr != nil {
			log.ErrorContext(r.Context(), "failed to emit audit event", "error", auditErr)
		}
		// Log `err` to get the root cause, not the user-friendly message.
		log.WarnContext(r.Context(), "anthropic request failed", "error", err)
		return
	}

	err = h.executeAnthropicRequest(r.Context(), llm, req, w, r, body)
	if err != nil {
		// Here we unwrap the error to get the root cause, not the user-friendly
		// message.
		log.WarnContext(r.Context(), "anthropic request failed", "error", errors.Unwrap(trace.Unwrap(err)))
	}

	auditErr := sessionCtx.Audit.OnLLMRequest(
		h.closeContext,
		sessionCtx,
		r,
		req,
		common.LLMResponse{Error: err},
	)
	if auditErr != nil {
		log.ErrorContext(r.Context(), "failed to emit audit event", "error", auditErr)
	}
}

func (h *Handler) prepareAnthropicRequest(llm *types.LLM, r *http.Request) (common.LLMRequest, *bytes.Buffer, error) {
	auditReq := common.LLMRequest{
		Provider: llm.Provider,
	}

	_, supportedEndpoint := supportedAnthropicEndpoints[strings.TrimPrefix(r.URL.Path, "/v1")]
	if !supportedEndpoint || r.Method != http.MethodPost {
		return auditReq, nil, trace.NotFound("endpoint not supported")
	}

	buf, err := h.readLimitedRequestBody(r, llmMaxRequestSize)
	if err != nil {
		return auditReq, nil, trace.Wrap(err)
	}
	requestSizeHist.WithLabelValues(teleport.ComponentLLM, llm.Format, llm.Provider).Observe(float64(buf.Len()))

	req, err := readAnthropicRequest(buf.Bytes())
	if err != nil {
		return auditReq, nil, trace.Wrap(err)
	}

	auditReq.Streaming = req.streaming
	auditReq.RequestedModel = req.model
	auditReq.MaxTokens = req.maxTokens

	resolvedModel, found := convertModelName(llm.Models, llm.FallbackModel, auditReq.RequestedModel)
	if !found {
		return auditReq, nil, trace.BadParameter("requested model is not supported")
	}
	auditReq.Model = resolvedModel

	// This function applies Anthropic logic on calculating the adequate request
	// timeout and also can indicate that the request should be using streaming
	// instead (due to the potential long response time).
	timeout := defaultStreamingRequestTimeout
	if !auditReq.Streaming {
		var calcErr error
		timeout, calcErr = anthropic.CalculateNonStreamingTimeout(int(auditReq.MaxTokens), resolvedModel, nil)
		if calcErr != nil {
			return auditReq, nil, trace.BadParameter("request needs to use streaming")
		}
	}
	auditReq.Timeout = timeout

	return auditReq, buf, nil
}

func (h *Handler) executeAnthropicRequest(ctx context.Context, llm *types.LLM, req common.LLMRequest, w http.ResponseWriter, r *http.Request, buf *bytes.Buffer) error {
	start := time.Now()
	defer func() {
		providerRequestDurationHist.WithLabelValues(
			teleport.ComponentLLM,
			llm.Format,
			req.Provider,
			strconv.FormatBool(req.Streaming),
		).Observe(time.Since(start).Seconds())
	}()

	opts := []option.RequestOption{
		option.WithHTTPClient(h.cfg.HTTPClient),
		option.WithJSONSet(modelRequestKey, req.Model),
		option.WithMaxRetries(0), // Disable retries.
		option.WithRequestTimeout(req.Timeout),
		// Ensure the outgoing request URL always contains exactly one /v1 path
		// prefix. This protects against misconfigured base URLs (e.g. via
		// ANTHROPIC_BASE_URL env var) that omit the /v1 segment, and covers the
		// required v1 prefix for Bedrock endpoints.
		option.WithMiddleware(ensureV1PathPrefix),
	}

	switch llm.Provider {
	case types.LLMProviderAnthropic:
		// Use environment variables to configure Anthropic address and
		// secrets.
		opts = append(opts, anthropic.DefaultClientOptions()...)
	case types.LLMProviderAWSBedrock:
		awscfg, awsErr := h.cfg.AWSConfigProvider.GetConfig(
			ctx,
			"", /* get region from env > profile > fallback func */
			awsconfig.WithAmbientCredentials(),
			// TODO(gabrielcorado): add support for AWS integration.
		)
		if awsErr != nil {
			return trace.Wrap(respondAnthropicError(w, awsErr))
		}

		opts = append(opts, bedrock.WithConfig(awscfg))
	}

	var raw *http.Response
	// Avoid calling NewClient since it does automatically read credentials from
	// environment. In addition, since we're not directly using services and
	// doing "raw Execute", nothing other than options is required.
	clt := &anthropic.Client{Options: opts}
	if err := clt.Execute(ctx, r.Method, r.URL.Path, buf, &raw); err != nil {
		return trace.Wrap(respondAnthropicError(w, err))
	}

	defer raw.Body.Close()

	var (
		written int64
		err     error
	)
	switch {
	case req.Streaming:
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(raw.StatusCode)

		written, err = streamAnthropicSSEEvents(w, raw)
	default:
		ct := raw.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(raw.StatusCode)

		// We cannot assure what was already written into the request, for this
		// reason, don't forward errors.
		written, err = io.Copy(w, raw.Body)
	}

	providerResponseSizeHist.WithLabelValues(
		teleport.ComponentLLM,
		llm.Format,
		req.Provider,
		strconv.FormatBool(req.Streaming),
	).Observe(float64(written))

	// No need to forward errors, this should be handled by the
	// streaming/non-streaming writers.
	return trace.Wrap(err)
}

// streamAnthropicSSEEvents copies streaming events from upstream to `w` as
// standard SSE frames. The Anthropic's SDK `ssestream` decoder transparently
// handles both, native SSE (Anthropic API) and AWS eventstream (Bedrock) by
// switching on the response Content-Type.
//
// Any error returned on the events stream causes the connection to be
// interrupted.
func streamAnthropicSSEEvents(w http.ResponseWriter, resp *http.Response) (int64, error) {
	rc := http.NewResponseController(w)
	dec := ssestream.NewDecoder(resp)
	defer dec.Close()

	writeAndFlush := func(evt ssestream.Event) (int64, error) {
		n, err := writeAnthropicSSEEvent(w, evt)
		if err != nil {
			return n, trace.Wrap(err)
		}
		return n, trace.Wrap(rc.Flush())
	}

	var written int64
	for dec.Next() {
		evt := dec.Event()
		if evt.Type == "error" {
			// Forward errors so clients can handle them mid-response.
			apiErr := convertAnthropicErrorEvent(evt.Data)
			n, _ := writeAndFlush(ssestream.Event{
				Type: "error",
				Data: anthropicErrorBody(apiErr),
			})
			written += n
			return written, apiErr
		}

		eventWritten, err := writeAnthropicSSEEvent(w, evt)
		written += eventWritten
		if err != nil {
			return written, trace.Wrap(err)
		}
		if err := rc.Flush(); err != nil {
			return written, trace.Wrap(err)
		}
	}

	if err := dec.Err(); err != nil && !errors.Is(err, io.EOF) {
		// Bedrock error events are returned on the decoder. To keep the same
		// behavior as standard SSE, convert such error to *apiError.
		return written, trace.Wrap(convertAnthropicError(err))
	}

	return written, nil
}

// writeAnthropicSSEEvent writes an SSE event using standard format (accepted by
// Anthropic clients).
func writeAnthropicSSEEvent(w io.Writer, evt ssestream.Event) (int64, error) {
	var written int64
	if evt.Type != "" {
		n, err := fmt.Fprintf(w, "event: %s\n", evt.Type)
		written += int64(n)
		if err != nil {
			return written, trace.Wrap(err)
		}
	}
	// Per SSE spec, events can have no data. This check is required, otherwise
	// SplitSeq yields on empty data, causing an empty "data: \n".
	//
	// Ref: https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
	if len(evt.Data) > 0 {
		data := bytes.TrimSuffix(evt.Data, []byte("\n"))
		for line := range bytes.SplitSeq(data, []byte("\n")) {
			n, err := fmt.Fprintf(w, "data: %s\n", line)
			written += int64(n)
			if err != nil {
				return written, trace.Wrap(err)
			}
		}
	}
	n, err := fmt.Fprint(w, "\n")
	written += int64(n)
	if err != nil {
		return written, trace.Wrap(err)
	}
	return written, nil
}

// anthropicRequest is the part of the Anthropic request body that will
// be used by app service.
type anthropicRequest struct {
	model     string
	streaming bool
	maxTokens int64
}

// readAnthropicRequest reads the Anthropic request info from buf, validates
// it and returns.
//
// This function also avoids duplicate fields to be pass through the
// verification.
//
// A requestor could include {"model": "allowed-model", "model": "denied-model"}.
// Readers (either through Unmarshal or gjson) would stick with the first
// occurance, leaving others unchecked/unchanged. There is no guarantee that
// the object that arrives into the provider contains a single key, and if it
// does have multiple which it will use or if it will reject the request.
//
// Instead of trusting the provider will reject such requests, we'll ensure, for
// the used fields, that there are no duplicates. The remaining of the request
// is not checked, and it will be up to the provider to handle them.
func readAnthropicRequest(buf []byte) (*anthropicRequest, error) {
	var (
		err             error
		res             = &anthropicRequest{}
		fieldsOccurency = make(map[string]int)
	)

	gjson.ParseBytes(buf).ForEach(func(k, v gjson.Result) bool {
		switch k.String() {
		case modelRequestKey:
			if fieldsOccurency[modelRequestKey] == 1 {
				err = trace.BadParameter("invalid request body. contains duplicated %q key", modelRequestKey)
				return false
			}
			fieldsOccurency[modelRequestKey]++
			res.model = v.String()
		case maxTokensRequestKey:
			if fieldsOccurency[maxTokensRequestKey] == 1 {
				err = trace.BadParameter("invalid request body. contains duplicated %q key", maxTokensRequestKey)
				return false
			}
			fieldsOccurency[maxTokensRequestKey]++
			res.maxTokens = v.Int()
		case streamingRequestKey:
			if fieldsOccurency[streamingRequestKey] == 1 {
				err = trace.BadParameter("invalid request body. contains duplicated %q key", streamingRequestKey)
				return false
			}
			fieldsOccurency[streamingRequestKey]++

			// Streaming key is a boolean. To avoid mistakenly assuming the
			// request is non-streamble due to malformed streaming information,
			// we validate it is the correct type.
			if v.Exists() && !v.IsBool() {
				err = trace.BadParameter("requested %q field must be a boolean (true/false)", streamingRequestKey)
				return false
			}
			res.streaming = v.Bool()
		}

		return true
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

// supportedAnthropicEndpoints is a list of supported Anthropic endpoints.
var supportedAnthropicEndpoints = map[string]struct{}{
	// Completions endpoint.
	//
	// https://platform.claude.com/docs/en/api/completions/create
	"/complete": {},
	// Messages endpoint.
	//
	// https://platform.claude.com/docs/en/api/messages/create
	"/messages": {},
}

// List of API request/response JSON keys. Some Anthopic API keys are not
// defined as constants on the SDK, for those we define them as plain strings.
//
// Ref: https://platform.claude.com/docs/en/api/messages/create
var (
	modelRequestKey     = string(constant.ValueOf[constant.Model]())
	streamingRequestKey = "stream"
	maxTokensRequestKey = "max_tokens"
)
