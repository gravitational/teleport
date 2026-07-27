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

package openai

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib/sse"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	llmrecorder "github.com/gravitational/teleport/lib/srv/app/llm/recorder"
	llmrequest "github.com/gravitational/teleport/lib/srv/app/llm/request"
	"github.com/gravitational/teleport/lib/utils"
)

// NewResponseRecorder creates a new OpenAI response recorder.
func NewResponseRecorder(log *slog.Logger, info llmrequest.RequestInfo, w http.ResponseWriter) (*llmrecorder.ResponseRecorder, error) {
	oaiInfo, ok := info.(*RequestInfo)
	if !ok {
		return nil, trace.BadParameter("expected openai.RequestInfo but got %T", info)
	}

	return llmrecorder.NewResponseRecorder(log, w, Endpoint{
		endpointType: oaiInfo.endpointType,
	})
}

// Endpoint implements [llmrecorder.Endpoint] for the OpenAI API.
type Endpoint struct {
	endpointType endpointType
}

// MarshalError implements [recorder.Endpoint].
func (e Endpoint) MarshalError(err error) []byte {
	return marshalError(newErrorEnvelope(err))
}

// ParseError implements [recorder.Endpoint].
func (e Endpoint) ParseError(statusCode int, body []byte) (providerError *llmerrors.ProviderError, err error) {
	return parseProviderError(statusCode, body)
}

// ParseUsage implements [recorder.Endpoint].
func (e Endpoint) ParseUsage(body []byte) (inputTokens int, outputTokens int, err error) {
	switch e.endpointType {
	case endpointTypeResponses:
		var result responsesAPIUsage
		if err := utils.FastUnmarshal(body, &result); err != nil {
			return 0, 0, trace.Wrap(err)
		}
		return result.Usage.InputTokens, result.Usage.OutputTokens, nil
	case endpointTypeChatCompletions:
		var result chatCompletionsResponse
		if err := utils.FastUnmarshal(body, &result); err != nil {
			return 0, 0, trace.Wrap(err)
		}
		if result.Usage == nil {
			return 0, 0, llmerrors.NewProviderError(llmerrors.ErrBadResponse, "missing usage information")
		}
		return result.Usage.PromptTokens, result.Usage.CompletionTokens, nil
	default:
		return 0, 0, trace.BadParameter("endpoint type not supported")
	}
}

// ProcessSSE implements [recorder.Endpoint].
func (e Endpoint) ProcessSSE(ctx context.Context, log *slog.Logger, reader io.ReadCloser, writer io.Writer) (inputTokens int, outputTokens int, err error) {
	switch e.endpointType {
	case endpointTypeResponses:
		return processResponsesSSEEvents(ctx, log, reader, writer)
	case endpointTypeChatCompletions:
		return processChatCompletionsSSEEvents(ctx, log, reader, writer)
	default:
		return 0, 0, trace.BadParameter("endpoint type not supported")
	}
}

// processResponsesSSEEvents processes responses API streaming (SSE) events.
//
// Usage information is read only from the terminal `response.completed` and
// `response.incomplete` events.
//
// This means that if the request is canceled before any of those "final events"
// arrives, we cannot track the usage of the request. This is a known limitation
// and will be addressed in the future when we introduce token budgeting.
//
// For error events, OpenAI specifies `response.failed` and `error` events.
// There isn't much information about what we can expect from both, so we treat
// them the same way for now.
//
// Other events don't contain relevant information for the recorder, so we
// forward them as is.
func processResponsesSSEEvents(ctx context.Context, log *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
	defer r.Close()

	var (
		done              bool
		inputTokensCount  int
		outputTokensCount int
	)
	for event, err := range sse.ReadEvents(r) {
		if err != nil {
			return inputTokensCount, outputTokensCount, trace.Wrap(err)
		}

		switch event.Event {
		case responsesFailedEventName:
			var streamEventPayload responsesFailedSSEEvent
			if err := utils.FastUnmarshal(event.Data, &streamEventPayload); err != nil {
				log.ErrorContext(ctx, "failed to parse error event", "error", err)
				if _, err := sse.WriteEvent(w, sse.Event{
					Event: responsesFailedEventName,
					Data:  marshalResponsesFailedError(newResponsesFailedError(0 /* seqNumber */, llmerrors.ErrBadResponse)),
				}); err != nil {
					// Preserve the provider response for recorder Err().
					// Downstream delivery failure is a secondary transport
					// condition here.
					log.ErrorContext(ctx, "failed to write error event", "error", err)
				}
				return inputTokensCount, outputTokensCount, llmerrors.ErrBadResponse
			}

			apiErr := llmerrors.NewProviderError(llmerrors.ErrUnknown, streamEventPayload.Response.Error.Message)
			if _, err := sse.WriteEvent(w, sse.Event{
				Event: responsesFailedEventName,
				Data:  marshalResponsesFailedError(newResponsesFailedError(streamEventPayload.SequenceNumber, apiErr)),
			}); err != nil {
				// Preserve the provider error for recorder Err(). A failed
				// downstream write here often means the client canceled or
				// timed out after the provider had already rejected the
				// request.
				log.ErrorContext(ctx, "failed to write error event", "error", err)
			}
			return inputTokensCount, outputTokensCount, apiErr
		case responsesErrorEventName:
			var streamEventPayload responsesErrorSSEEvent
			if err := utils.FastUnmarshal(event.Data, &streamEventPayload); err != nil {
				log.ErrorContext(ctx, "failed to parse error event", "error", err)
				if _, err := sse.WriteEvent(w, sse.Event{
					Event: responsesErrorEventName,
					// We use error message directly since we don't need specific
					// error type set (it must always default to the event name).
					Data: marshalResponsesErrorEvent(&responsesErrorSSEEvent{Type: responsesErrorEventName, Message: llmerrors.ErrBadResponse.Error()}),
				}); err != nil {
					// Preserve the provider response for recorder Err().
					// Downstream delivery failure is a secondary transport
					// condition here.
					log.ErrorContext(ctx, "failed to write error event", "error", err)
				}
				return inputTokensCount, outputTokensCount, llmerrors.ErrBadResponse
			}

			apiErr := llmerrors.NewProviderError(llmerrors.ErrUnknown, streamEventPayload.Message)
			if _, err := sse.WriteEvent(w, sse.Event{
				Event: responsesErrorEventName,
				Data: marshalResponsesErrorEvent(&responsesErrorSSEEvent{
					Type:           responsesErrorEventName,
					Message:        apiErr.Error(),
					SequenceNumber: streamEventPayload.SequenceNumber,
				}),
			}); err != nil {
				// Preserve the provider error for recorder Err(). A failed
				// downstream write here often means the client canceled or
				// timed out after the provider had already rejected the
				// request.
				log.ErrorContext(ctx, "failed to write error event", "error", err)
			}
			return inputTokensCount, outputTokensCount, apiErr
		case responsesCompletedEventName, responsesIncompleteEventName:
			var streamEventPayload responsesSSEEventWithUsage
			if err := utils.FastUnmarshal(event.Data, &streamEventPayload); err != nil {
				log.ErrorContext(ctx, "failed to parse response.completed event", "error", err)
				return 0, 0, llmerrors.ErrBadResponse
			}
			inputTokensCount = streamEventPayload.Response.Usage.InputTokens
			outputTokensCount = streamEventPayload.Response.Usage.OutputTokens
			done = true
		}

		if _, err := sse.WriteEvent(w, event); err != nil {
			return inputTokensCount, outputTokensCount, trace.Wrap(err)
		}
	}

	// This would mean the stream did not work correctly (due to missing final
	// event).
	if !done {
		return 0, 0, llmerrors.NewProviderError(llmerrors.ErrBadResponse, "provider did not send final event")
	}

	return inputTokensCount, outputTokensCount, nil
}

// processChatCompletionsSSEEvents process chat completions API streaming (SSE)
// events.
//
// This streaming has only one type of event, and they have roughly the same
// shape as the non-stream chat completions responses, with only a subset of
// fields available in the events.
//
// Usage information will be available on the last JSON event of the stream, and
// similarly to responses API streaming, if the request is canceled before the
// event arrives we cannot determine the token usage. This is a known limitation
// and will be addressed in the future when we introduce token budgeting.
func processChatCompletionsSSEEvents(ctx context.Context, log *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
	defer r.Close()

	// Instead of trying to unmarshal every chunk to check if usage is
	// available, we'll do that only on the last JSON event which is the only
	// event that contains usage information.
	//
	// Chat completions always send a `[DONE]` event when streaming is done, and
	// when usage is enabled the chunk before that will contain the usage
	// information. We can use that as differentiator of "non-data" event,
	// but not a requirement. If the last event contains the usage information
	// we should be good to go.
	//
	// Note usage information needs to be enabled in the request using
	// `stream_options`. Note from OpenAI docs:
	//
	//	> If set, an additional chunk will be streamed before the data: [DONE] message.
	//	> The usage field on this chunk shows the token usage statistics for the
	//	> entire request, and the choices field will always be an empty array.
	//	> All other chunks will also include a usage field, but with a null value.
	//	> NOTE: If the stream is interrupted, you may not receive the final usage
	//	> chunk which contains the total token usage for the request.

	var lastEventPayload []byte
	for event, err := range sse.ReadEvents(r) {
		if err != nil {
			return 0, 0, trace.Wrap(err)
		}

		if !bytes.Equal(event.Data, []byte("[DONE]")) {
			lastEventPayload = event.Data
		}

		if _, err := sse.WriteEvent(w, event); err != nil {
			return 0, 0, trace.Wrap(err)
		}
	}

	if len(lastEventPayload) == 0 {
		log.ErrorContext(ctx, "empty chat completion response")
		return 0, 0, llmerrors.ErrBadResponse
	}

	var completionResp chatCompletionsResponse
	if err := utils.FastUnmarshal(lastEventPayload, &completionResp); err != nil {
		log.ErrorContext(ctx, "failed to parse chat completion event", "error", err)
		return 0, 0, llmerrors.ErrBadResponse
	}

	// This behavior is not documented, but the SDKs do handle it. So here we're
	// being defensive, otherwise an error event would be seen as success with
	// empty usage.
	if completionResp.Error != nil {
		return 0, 0, llmerrors.NewProviderError(llmerrors.ErrUnknown, completionResp.Error.Message)
	}

	if completionResp.Usage == nil {
		return 0, 0, llmerrors.NewProviderError(llmerrors.ErrBadResponse, "provider did not send final event")
	}

	return completionResp.Usage.PromptTokens, completionResp.Usage.CompletionTokens, nil
}

const (
	// responsesFailedEventName is the SSE event of a failed response.
	responsesFailedEventName = "response.failed"
	// responsesCompletedEventName is the SSE event of a completed response.
	responsesCompletedEventName = "response.completed"
	// responsesIncompleteEventName is the SSE event of a incomplete response.
	responsesIncompleteEventName = "response.incomplete"
	// responsesErrorEventName is the SSE event of an error.
	responsesErrorEventName = "error"
)
