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

	err = h.executeOpenAIRequest(r.Context(), llm, req, w, r, body)
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
	}

	_, pattern := h.openAIMux.Handler(r)
	if len(pattern) == 0 {
		return auditReq, nil, trace.NotFound("endpoint not supported")
	}

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
	if !auditReq.Streaming {
		// TODO
	}
	auditReq.Timeout = timeout

	return auditReq, buf, nil
}

func (h *Handler) executeOpenAIRequest(ctx context.Context, llm *types.LLM, req common.LLMRequest, w http.ResponseWriter, r *http.Request, buf *bytes.Buffer) error {
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

		written, err = streamOpenAISSEEvents(w, raw)
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

// streamOpenAISSEEvents copies streaming events from upstream to `w` as
// standard SSE frames.
//
// Any error returned on the events stream causes the connection to be
// interrupted.
func streamOpenAISSEEvents(w http.ResponseWriter, resp *http.Response) (int64, error) {
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
		if evt.Type == "response.failed" {
			// Forward errors so clients can handle them mid-response.
			apiErr := convertOpenAIErrorEvent(evt.Data)
			n, _ := writeAndFlush(ssestream.Event{
				Type: evt.Type,
				Data: openAIErrorBody(apiErr),
			})
			written += n
			return written, apiErr
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
	// TODO(gabrielcorado): check if this will be enabled.
	// registerEndpoint("GET", "/chat/completions")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
	registerEndpoint("POST", "/chat/completions")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/retrieve
	registerEndpoint("GET", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/update
	registerEndpoint("POST", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/delete
	registerEndpoint("DELETE", "/chat/completions/{completion_id}")
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/subresources/messages/methods/list
	// TODO(gabrielcorado): if this will be enabled, we must support query params.
	registerEndpoint("GET", "/chat/completions/{completion_id}/messages")

	return mux
}
