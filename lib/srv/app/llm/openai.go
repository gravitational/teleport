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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/tidwall/gjson"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// handleOpenAI proxies an OpenAI-compatible request and forwards the
// response.
func (h *Handler) handleOpenAI(sessionCtx *common.SessionContext, w http.ResponseWriter, r *http.Request) {
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

	req, body, err := h.prepareOpenAIRequest(llm, r)
	if err != nil {
		apiErr := respondOpenAIError(w, err)
		auditErr := sessionCtx.Audit.OnLLMRequest(h.closeContext, sessionCtx, r, req, common.LLMResponse{Error: apiErr})
		if auditErr != nil {
			log.ErrorContext(r.Context(), "failed to emit audit event", "error", auditErr)
		}
		// Log `err` to get the root cause, not the user-friendly message.
		log.WarnContext(r.Context(), "openai request failed", "error", err)
		return
	}

	if err := h.canServeRequest(r.Context(), req); err != nil {
		apiErr := respondOpenAIError(w, err)
		auditErr := sessionCtx.Audit.OnLLMRequest(h.closeContext, sessionCtx, r, req, common.LLMResponse{Error: apiErr})
		if auditErr != nil {
			log.ErrorContext(r.Context(), "failed to emit audit event", "error", auditErr)
		}
		log.WarnContext(r.Context(), "openai request failed", "error", err)
		log.ErrorContext(r.Context(), "request rejected due to limits", "error", err)
		return
	}

	err = h.executeOpenAIRequest(r.Context(), log, llm, req, w, r, body)
	if err != nil {
		// Here we unwrap the error to get the root cause, not the user-friendly
		// message.
		log.WarnContext(r.Context(), "openai request failed", "error", errors.Unwrap(trace.Unwrap(err)))
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

func (h *Handler) prepareOpenAIRequest(llm *types.LLM, r *http.Request) (common.LLMRequest, *bytes.Buffer, error) {
	auditReq := common.LLMRequest{
		Provider: llm.Provider,
		Format:   llm.Format,
	}

	endpointType := classifyOpenAIEndpoint(r)
	if endpointType == common.LLMEndpointTypeUnsupported {
		return auditReq, nil, trace.NotFound("endpoint not supported")
	}
	auditReq.EndpointType = endpointType

	buf, err := h.readLimitedRequestBody(r, llmMaxRequestSize)
	if err != nil {
		return auditReq, nil, trace.Wrap(err)
	}
	requestSizeHist.WithLabelValues(teleport.ComponentLLM, llm.Format, llm.Provider).Observe(float64(buf.Len()))

	req, err := readOpenAIRequest(buf.Bytes())
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

	timeout := defaultStreamingRequestTimeout
	if !auditReq.Streaming && auditReq.MaxTokens > maxNonStreamingTokens {
		return auditReq, nil, trace.BadParameter("request needs to use streaming")
	}
	auditReq.Timeout = timeout

	return auditReq, buf, nil
}

func (h *Handler) executeOpenAIRequest(ctx context.Context, log *slog.Logger, llm *types.LLM, req common.LLMRequest, w http.ResponseWriter, r *http.Request, buf *bytes.Buffer) error {
	start := time.Now()
	defer func() {
		providerRequestDurationHist.WithLabelValues(
			teleport.ComponentLLM,
			llm.Format,
			req.Provider,
			strconv.FormatBool(req.Streaming),
		).Observe(time.Since(start).Seconds())
	}()

	opts := append(openai.DefaultClientOptions(), []option.RequestOption{
		option.WithHTTPClient(h.cfg.HTTPClient),
		option.WithJSONSet(modelRequestKey, req.Model),
		option.WithMaxRetries(0), // Disable retries.
		option.WithRequestTimeout(req.Timeout),
		// Ensure the outgoing request URL always contains exactly one /v1 path
		// prefix. This protects against misconfigured base URLs (e.g. via
		// OPENAI_BASE_URL env var) that omit the /v1 segment, and covers the
		// required v1 prefix for Bedrock endpoints.
		option.WithMiddleware(ensureV1PathPrefix),
	}...)

	// Always require usage to be reported on chat completions streaming
	if req.Streaming && req.EndpointType == common.LLMEndpointTypeOpenAIChatCompletions {
		opts = append(opts, option.WithJSONSet("stream_options.include_usage", true))
	}

	var raw *http.Response
	// Avoid calling NewClient since it does automatically read credentials from
	// environment. In addition, since we're not directly using services and
	// doing "raw Execute", nothing other than options is required.
	clt := &openai.Client{Options: opts}
	if err := clt.Execute(ctx, r.Method, r.URL.Path, buf, &raw); err != nil {
		return trace.Wrap(respondOpenAIError(w, err))
	}

	defer raw.Body.Close()

	ct := raw.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(raw.StatusCode)

	var (
		written int64
		err     error
	)
	switch {
	case req.Streaming:
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(raw.StatusCode)

		switch req.EndpointType {
		case common.LLMEndpointTypeOpenAIResponses:
			written, err = streamOpenAIResponsesSSEEvents(r.Context(), log, w, raw, h)
		case common.LLMEndpointTypeOpenAIChatCompletions:
			written, err = streamOpenAIChatCompletionsSSEEvents(r.Context(), log, w, raw, h)
		default:
			// This should be unreachable, if this error happens there is a
			// mismatch of supported OpenAI endpoint types.
			return trace.NotImplemented("endpoint not implemented")
		}
	default:
		ct := raw.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(raw.StatusCode)

		var respRecorder bytes.Buffer
		writers := io.MultiWriter(w, &respRecorder)

		// We cannot assure what was already written into the request, for this
		// reason, don't forward errors.
		written, err = io.Copy(writers, raw.Body)

		var usage usageReport
		switch req.EndpointType {
		case common.LLMEndpointTypeOpenAIResponses:
			// https://developers.openai.com/api/reference/resources/responses/methods/create#(resource)%20responses%20%3E%20(model)%20response%20%3E%20(schema)%20%3E%20(property)%20usage
			usage = usageReport{
				InputTokens:  gjson.GetBytes(respRecorder.Bytes(), "usage.input_tokens").Int(),
				OutputTokens: gjson.GetBytes(respRecorder.Bytes(), "usage.output_tokens").Int(),
			}
		case common.LLMEndpointTypeOpenAIChatCompletions:
			// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create#(resource)%20chat.completions%20%3E%20(model)%20chat_completion%20%3E%20(schema)%20%3E%20(property)%20usage
			usage = usageReport{
				InputTokens:  gjson.GetBytes(respRecorder.Bytes(), "usage.prompt_tokens").Int(),
				OutputTokens: gjson.GetBytes(respRecorder.Bytes(), "usage.completion_tokens").Int(),
			}
		default:
			// At this stage we already served the request, so nothing much
			// to do.
			log.ErrorContext(ctx, "unsupported endpoint type. This should be unreachable, if this error happens there is a mismatch of supported OpenAI endpoint types.", "error", err)
		}

		if err := h.Report(ctx, types.LLMFormatOpenAI, usage); err != nil {
			log.WarnContext(ctx, "failed to report usage", "error", err)
		}
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

// openAIRequest is the part of the OpenAI request body that will
// be used by app service.
type openAIRequest struct {
	model     string
	maxTokens int64
	streaming bool
}

// readOpenAIRequest reads the OpenAI request info from buf, validates
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
func readOpenAIRequest(buf []byte) (*openAIRequest, error) {
	var (
		err             error
		res             = &openAIRequest{}
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

// streamOpenAIResponsesSSEEvents copies streaming events from upstream to `w`
// as standard SSE frames with OpenAI responses format.
//
// Any error returned on the events stream causes the connection to be
// interrupted.
func streamOpenAIResponsesSSEEvents(ctx context.Context, log *slog.Logger, w http.ResponseWriter, resp *http.Response, reporter usageReporter) (int64, error) {
	rc := http.NewResponseController(w)
	dec := ssestream.NewDecoder(resp)
	defer dec.Close()

	writeAndFlush := func(evt ssestream.Event) (int64, error) {
		n, err := writeOpenAISSEEvent(w, evt)
		if err != nil {
			return n, trace.Wrap(err)
		}
		return n, trace.Wrap(rc.Flush())
	}

	var written int64
	for dec.Next() {
		evt := dec.Event()
		switch evt.Type {
		case "response.failed":
			// Forward errors so clients can handle them mid-response.
			apiErr := convertOpenAIErrorEvent(evt.Data)
			n, _ := writeAndFlush(ssestream.Event{
				Type: evt.Type,
				Data: openAIErrorBody(apiErr),
			})
			written += n
			return written, apiErr
		case "response.completed":
			// https://developers.openai.com/api/reference/resources/responses/streaming-events#response.completed
			usage := usageReport{
				InputTokens:  gjson.GetBytes(evt.Data, "response.usage.input_tokens").Int(),
				OutputTokens: gjson.GetBytes(evt.Data, "response.usage.output_tokens").Int(),
			}
			if err := reporter.Report(ctx, types.LLMFormatOpenAI, usage); err != nil {
				log.WarnContext(ctx, "failed to report usage", "error", err)
			}
		}

		eventWritten, err := writeOpenAISSEEvent(w, evt)
		written += eventWritten
		if err != nil {
			return written, trace.Wrap(err)
		}
		if err := rc.Flush(); err != nil {
			return written, trace.Wrap(err)
		}
	}

	if err := dec.Err(); err != nil && !errors.Is(err, io.EOF) {
		return written, trace.Wrap(convertOpenAIError(err))
	}

	return written, nil
}

// streamOpenAIChatCompletionsSSEEvents copies streaming events from upstream to
// `w` as standard SSE frames with OpenAI chat completions format.
//
// Any error returned on the events stream causes the connection to be
// interrupted.
func streamOpenAIChatCompletionsSSEEvents(ctx context.Context, log *slog.Logger, w http.ResponseWriter, resp *http.Response, reporter usageReporter) (int64, error) {
	rc := http.NewResponseController(w)
	dec := ssestream.NewDecoder(resp)
	defer dec.Close()

	var written int64
	for dec.Next() {
		evt := dec.Event()
		// When present, it contains a null value **except for the last
		// chunk** which contains the token usage statistics for the entire
		// request.
		//
		// NOTE: If the stream is interrupted or cancelled, you may not
		// receive the final usage chunk which contains the total token
		// usage for the request.
		//
		// https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events
		if gjson.GetBytes(evt.Data, "usage").Exists() {
			usage := usageReport{
				InputTokens:  gjson.GetBytes(evt.Data, "usage.prompt_tokens").Int(),
				OutputTokens: gjson.GetBytes(evt.Data, "usage.completion_tokens").Int(),
			}
			if err := reporter.Report(ctx, types.LLMFormatOpenAI, usage); err != nil {
				log.WarnContext(ctx, "failed to report usage", "error", err)
			}
		}

		eventWritten, err := writeOpenAISSEEvent(w, evt)
		written += eventWritten
		if err != nil {
			return written, trace.Wrap(err)
		}
		if err := rc.Flush(); err != nil {
			return written, trace.Wrap(err)
		}
	}

	if err := dec.Err(); err != nil && !errors.Is(err, io.EOF) {
		return written, trace.Wrap(convertOpenAIError(err))
	}

	return written, nil
}

// writeOpenAISSEEvent writes an SSE event using standard format (accepted by
// OpenAI clients).
func writeOpenAISSEEvent(w io.Writer, evt ssestream.Event) (int64, error) {
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

func classifyOpenAIEndpoint(r *http.Request) common.LLMEndpointType {
	path := strings.TrimPrefix(r.URL.Path, "/v1")

	switch {
	case r.Method == http.MethodPost && path == "/responses":
		return common.LLMEndpointTypeOpenAIResponses
	case r.Method == http.MethodPost && path == "/chat/completions":
		return common.LLMEndpointTypeOpenAIChatCompletions
	default:
		return common.LLMEndpointTypeUnsupported
	}
}
