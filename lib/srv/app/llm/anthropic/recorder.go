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

package anthropic

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib/sse"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	llmrecorder "github.com/gravitational/teleport/lib/srv/app/llm/recorder"
	"github.com/gravitational/teleport/lib/utils"
)

// NewResponseRecorder creates a new Anthropic response recorder.
func NewResponseRecorder(log *slog.Logger, w http.ResponseWriter) (*llmrecorder.ResponseRecorder, error) {
	return llmrecorder.NewResponseRecorder(log, w, Endpoint{})
}

// Endpoint implements [llmrecorder.Endpoint] for the Anthropic messages API.
type Endpoint struct{}

// ParseError implements [llmrecorder.Endpoint]. The Anthropic API presents
// errors in the body, so the status code is ignored.
func (Endpoint) ParseError(_ int, body []byte) (*llmerrors.ProviderError, error) {
	return parseProviderError(body)
}

// MarshalError implements [llmrecorder.Endpoint].
func (Endpoint) MarshalError(err error) []byte {
	return marshalError(newErrorMessage(err))
}

// ParseUsage implements [llmrecorder.Endpoint].
func (Endpoint) ParseUsage(body []byte) (int, int, error) {
	var result messagesAPIResult
	if err := utils.FastUnmarshal(body, &result); err != nil {
		return 0, 0, trace.Wrap(err)
	}
	return result.Usage.InputTokens, result.Usage.OutputTokens, nil
}

// ProcessSSE implements [llmrecorder.Endpoint].
func (Endpoint) ProcessSSE(ctx context.Context, log *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
	return processSSEEvents(ctx, log, r, w)
}

func processSSEEvents(ctx context.Context, log *slog.Logger, r io.ReadCloser, w io.Writer) (int, int, error) {
	defer r.Close()

	var (
		inputTokensCount  int
		outputTokensCount int
	)
	for event, err := range sse.ReadEvents(r) {
		if err != nil {
			return inputTokensCount, outputTokensCount, trace.Wrap(err)
		}

		switch event.Event {
		case "error":
			apiErr, parseErr := parseProviderError(event.Data)
			if parseErr == nil {
				if _, err := sse.WriteEvent(w, sse.Event{Event: "error", Data: marshalError(newErrorMessage(apiErr))}); err != nil {
					// Preserve the provider error for recorder Err(). A failed
					// downstream write here often means the client canceled or
					// timed out after the provider had already rejected the
					// request.
					log.ErrorContext(ctx, "failed to write error event", "error", err)
				}
				return inputTokensCount, outputTokensCount, apiErr
			}
			log.ErrorContext(ctx, "failed to parse error event", "error", parseErr)
			if _, err := sse.WriteEvent(w, sse.Event{Event: "error", Data: marshalError(newErrorMessage(llmerrors.ErrBadResponse))}); err != nil {
				// Preserve the provider response for recorder Err().
				// Downstream delivery failure is a secondary transport
				// condition here.
				log.ErrorContext(ctx, "failed to write error event", "error", err)
			}
			return inputTokensCount, outputTokensCount, llmerrors.ErrBadResponse
		case "message_start":
			// This event will include the input tokens information.
			//
			// https://platform.claude.com/docs/en/api/messages/create#raw_message_start_event
			var d sseMessageStartEvent
			if err := utils.FastUnmarshal(event.Data, &d); err != nil {
				log.ErrorContext(ctx, "failed to parse message_start event", "error", err)
				continue
			}
			inputTokensCount = d.Message.Usage.InputTokens
		case "message_delta":
			// Message delta progressively sends the accumulative output tokens
			// information.
			//
			// https://platform.claude.com/docs/en/api/messages/create#raw_message_delta_event
			var d messagesAPIResult
			if err := utils.FastUnmarshal(event.Data, &d); err != nil {
				log.ErrorContext(ctx, "failed to parse message_delta event", "error", err)
				continue
			}
			outputTokensCount = d.Usage.OutputTokens
		}

		if _, err := sse.WriteEvent(w, event); err != nil {
			return inputTokensCount, outputTokensCount, trace.Wrap(err)
		}
	}

	return inputTokensCount, outputTokensCount, nil
}
