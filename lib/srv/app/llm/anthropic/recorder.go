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
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib/sse"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/utils"
)

// ResponseRecorder is the Anthropic's response modifier and recorder.
type ResponseRecorder struct {
	http.ResponseWriter
	flusher http.Flusher

	log          *slog.Logger
	status       int
	written      int
	containsErr  bool
	err          error
	responseType func() string
	closed       atomic.Bool

	// usage-related fields
	inputTokensCount  int
	outputTokensCount int

	// used for non-streaming requests.
	writer io.Writer
	buf    *bytes.Buffer

	// used for streaming requests.
	startSSEOnce sync.Once
	streamDoneCh chan struct{}
	sseWriter    io.WriteCloser

	// This values is guarded as atomic values since `Flush` calls may happen
	// from different goroutines.
	skipFlush atomic.Bool
}

// NewResponseRecorder creates a new response recorder.
func NewResponseRecorder(log *slog.Logger, w http.ResponseWriter) (*ResponseRecorder, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, trace.BadParameter("response writer must be flusher")
	}
	r := &ResponseRecorder{
		ResponseWriter: w,
		// Defaults to 200, this matches the [http.ResponseWriter] expectation when
		// [WriteHeader] is not called.
		status:  http.StatusOK,
		flusher: f,
		log:     log,
		buf:     new(bytes.Buffer),
	}
	r.writer = io.MultiWriter(w, r.buf)
	r.responseType = sync.OnceValue(func() string {
		mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
		if err != nil {
			// No need to log the error here, the caller of this function will
			// do the proper handling of this case (including logs).
			return ""
		}
		return mediaType
	})
	return r, nil
}

// WriteHeader implements [http.ResponseWriter].
func (r *ResponseRecorder) WriteHeader(status int) {
	// In case of errors or response type not JSON, we delete the original
	// Content-Length because we might rewrite the result contents, changing the
	// total length.
	contentType := r.responseType()
	if status != http.StatusOK || contentType != "application/json" {
		r.Header().Del("Content-Length")
	}

	// When the result is an unsupported content-type we set as error
	// here since it might not contain a body (resulting in no [Write] calls).
	//
	// In addition, we "force" an error status code. This is done to keep the
	// API contract where errors return only when status code is not 200.
	if contentType != "application/json" && contentType != "text/event-stream" {
		status = http.StatusInternalServerError
		r.log.ErrorContext(context.Background(), "unsupported response content-type", "content_type", contentType)
		r.err = llmerrors.NewProviderError(llmerrors.ErrBadResponse, "unsupported %q response type", contentType)
	}
	if status != http.StatusOK {
		r.containsErr = true
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Writer implements [http.ResponseWriter].
func (r *ResponseRecorder) Write(data []byte) (int, error) {
	if r.status != http.StatusOK {
		// Necessary because [WriteHeader] might not have been called before
		// [Write].
		r.containsErr = true
		return r.writeError(data)
	}

	contentType := r.responseType()
	switch contentType {
	case "application/json":
		n, err := r.writer.Write(data)
		r.written += n
		return n, trace.Wrap(err)
	case "text/event-stream":
		return r.writeStream(data)
	default:
		// This handling exists because callers are not guaranteed to call
		// [WriteHeader]. So we must mimic the behavior here in case it wasn't
		// called before.
		//
		// Note that at this point, we cannot change the status code returned,
		// but we'll still return an error object here.
		if !r.containsErr {
			r.log.ErrorContext(context.Background(), "unsupported response content-type", "content_type", contentType)
			r.err = llmerrors.NewProviderError(llmerrors.ErrBadResponse, "unsupported %q response type", contentType)
			r.containsErr = true
		}
		return len(data), nil
	}
}

// Flush implements [http.Flusher].
func (r *ResponseRecorder) Flush() {
	if r.skipFlush.Load() {
		return
	}

	if r.flusher != nil {
		r.flusher.Flush()
	}
}

// Written is the number of bytes written in the downstream connection.
//
// Must be called after [Close].
func (r *ResponseRecorder) Written() int {
	return r.written
}

// Err is the error encountered while processing/forwarding the response.
//
// Must be called after [Close].
func (r *ResponseRecorder) Err() error {
	return r.err
}

// InputTokensCount is the number of input tokens that were used on the request.
//
// Must be called after [Close].
func (r *ResponseRecorder) InputTokensCount() int {
	return r.inputTokensCount
}

// OutputTokensCount is the number of output tokens generated by the request.
//
// Must be called after [Close].
func (r *ResponseRecorder) OutputTokensCount() int {
	return r.outputTokensCount
}

// Close implements [io.Closer]. Errors returned are only related to the
// recorder teardown. For processing errors, callers must retrieve errors using
// [Err].
//
// Must be called once.
func (r *ResponseRecorder) Close() error {
	if r.closed.Swap(true) {
		return nil
	}

	// In case the response contains an error, this is the place where we write
	// it back to the upstream.
	if r.containsErr {
		// Error might already be defined, so we can skip parsing it.
		if r.err == nil {
			var err error
			r.err, err = parseProviderError(r.buf.Bytes())
			// Failure here could indicate that the message is incomplete or
			// that it is malformed.
			if err != nil {
				r.log.WarnContext(context.Background(), "unable to parse the provider error message", "error", err)
				r.err = llmerrors.NewProviderError(llmerrors.ErrBadResponse, "invalid JSON response")
			}
		}

		encodedErr := marshalError(newErrorMessage(r.err))
		r.written += len(encodedErr)
		if _, err := r.ResponseWriter.Write(encodedErr); err != nil {
			r.log.WarnContext(context.Background(), "unable to write error message to downstream", "error", err)
		}
	}

	if r.sseWriter != nil {
		err := r.sseWriter.Close()
		<-r.streamDoneCh
		return trace.Wrap(err)
	}

	// When request contains error, it won't contain usage information, so we
	// can skip it.
	if r.containsErr {
		return nil
	}

	// Non-streaming requests we must try to extract result metrics at the end.
	var result messagesAPIResult
	err := utils.FastUnmarshal(r.buf.Bytes(), &result)
	if err != nil {
		r.log.WarnContext(context.Background(), "unable to parse messages result", "error", err)
		r.err = llmerrors.NewProviderError(llmerrors.ErrBadResponse, "invalid JSON response")
		return nil
	}

	r.inputTokensCount = result.Usage.InputTokens
	r.outputTokensCount = result.Usage.OutputTokens
	return nil
}

func (r *ResponseRecorder) writeStream(data []byte) (int, error) {
	// first write should initialize the pipe and start the read events
	// goroutine.
	r.startSSEOnce.Do(func() {
		pr, pw := io.Pipe()
		r.sseWriter = pw
		r.streamDoneCh = make(chan struct{})
		r.skipFlush.Store(true)
		go func() {
			defer close(r.streamDoneCh)
			w := &writeFlusher{writer: r.ResponseWriter, flushFunc: r.flusher.Flush}
			r.inputTokensCount, r.outputTokensCount, r.err = processSSEEvents(context.Background(), r.log, pr, w)
			r.written = w.written
		}()
	})

	return r.sseWriter.Write(data)
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

func (r *ResponseRecorder) writeError(data []byte) (int, error) {
	// Collect the error, but do not immediately forward it as we might need
	// to perform modifications to it.
	n, err := r.buf.Write(data)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// We might not write the error data as is, but we should return the
	// original written value for writter that double check the total amount of
	// data written.
	return n, nil
}

type writeFlusher struct {
	written   int
	writer    io.Writer
	flushFunc func()
}

func (w *writeFlusher) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	w.written += n
	w.flushFunc()
	return n, nil
}
