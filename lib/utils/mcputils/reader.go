/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package mcputils

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// HandleParseErrorFunc handles parse errors.
type HandleParseErrorFunc func(context.Context, mcp.RequestId, error) error

// HandleRequestFunc handles a request.
type HandleRequestFunc func(context.Context, *JSONRPCRequest) error

// HandleResponseFunc handles a response.
type HandleResponseFunc func(context.Context, *JSONRPCResponse) error

// HandleNotificationFunc handles a notification.
type HandleNotificationFunc func(context.Context, *JSONRPCNotification) error

// ReplyParseError returns a HandleParseErrorFunc that forwards the error to
// provided writer.
func ReplyParseError(w MessageWriter) HandleParseErrorFunc {
	return func(ctx context.Context, id mcp.RequestId, parseError error) error {
		rpcError := mcp.NewJSONRPCError(id, mcp.PARSE_ERROR, parseError.Error(), nil)
		return trace.Wrap(w.WriteMessage(ctx, rpcError))
	}
}

// LogAndIgnoreParseError returns a HandleParseErrorFunc that logs the parse
// error.
func LogAndIgnoreParseError(log *slog.Logger) HandleParseErrorFunc {
	return func(ctx context.Context, id mcp.RequestId, parseError error) error {
		log.DebugContext(ctx, "Ignore parse error", "error", parseError, "id", id)
		return nil
	}
}

// TransportReader defines an interface for reading next raw/unmarshalled
// message from the MCP transport.
type TransportReader interface {
	// Type is the transport type for logging purpose.
	Type() string
	// ReadMessage reads the next raw message.
	ReadMessage(context.Context) (string, error)
	// Close closes the transport.
	Close() error
}

// MessageReaderConfig is the config for MessageReader.
type MessageReaderConfig struct {
	// Transport is the input to the read the raw/unmarshalled messages from.
	// Transport will be closed when reader finishes.
	Transport TransportReader
	// Logger is the slog.Logger.
	Logger *slog.Logger

	// OnClose is an optional callback when reader finishes.
	OnClose func()
	// OnParseError specifies the handler for handling parse error. Any error
	// returned by the handler stops this message reader.
	OnParseError HandleParseErrorFunc
	// OnRequest specifies the handler for handling request. Any error by the
	// handler stops this message reader.
	OnRequest HandleRequestFunc
	// OnResponse specifies the handler for handling response. Any error by the
	// handler stops this message reader.
	OnResponse HandleResponseFunc
	// OnNotification specifies the handler for handling notification. Any error
	// returned by the handler stops this message reader.
	OnNotification HandleNotificationFunc
}

// CheckAndSetDefaults checks values and sets defaults.
func (c *MessageReaderConfig) CheckAndSetDefaults() error {
	if c.Transport == nil {
		return trace.BadParameter("missing parameter Transport")
	}
	if c.OnParseError == nil {
		return trace.BadParameter("missing parameter OnParseError")
	}
	if c.OnNotification == nil {
		return trace.BadParameter("missing parameter OnNotification")
	}
	if c.OnRequest == nil && c.OnResponse == nil {
		return trace.BadParameter("one of OnRequest or OnResponse must be set")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "mcp")
	}
	return nil
}

// MessageReader reads requests from provided reader.
type MessageReader struct {
	cfg MessageReaderConfig
}

// NewMessageReader creates a new MessageReader. Must call "Start" to
// start the processing.
func NewMessageReader(cfg MessageReaderConfig) (*MessageReader, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &MessageReader{
		cfg: cfg,
	}, nil
}

// Run starts reading requests from provided reader. Run blocks until an
// error happens from the provided reader or any of the handler.
func (r *MessageReader) Run(ctx context.Context) {
	r.cfg.Logger.InfoContext(ctx, "Start processing messages", "transport", r.cfg.Transport.Type())

	finished := make(chan struct{})
	go func() {
		r.startProcess(ctx)
		close(finished)
	}()

	select {
	case <-finished:
	case <-ctx.Done():
	}

	r.cfg.Logger.InfoContext(ctx, "Finished processing messages", "transport", r.cfg.Transport.Type())
	if err := r.cfg.Transport.Close(); err != nil && !IsOKCloseError(err) {
		r.cfg.Logger.ErrorContext(ctx, "Failed to close reader", "error", err)
	}
	if r.cfg.OnClose != nil {
		r.cfg.OnClose()
	}
}

func (r *MessageReader) startProcess(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		if err := r.processNextLine(ctx); err != nil {
			if !IsOKCloseError(err) {
				r.cfg.Logger.ErrorContext(ctx, "Failed to process line", "error", err)
			}
			return
		}
	}
}

func (r *MessageReader) processNextLine(ctx context.Context) error {
	rawMessage, err := r.cfg.Transport.ReadMessage(ctx)
	switch {
	case isReaderParseError(err):
		if err := r.cfg.OnParseError(ctx, mcp.NewRequestId(nil), err); err != nil {
			return trace.Wrap(err, "handling reader parse error")
		}
	case err != nil:
		return trace.Wrap(err, "reading next message")
	}

	r.cfg.Logger.Log(ctx, logutils.TraceLevel, "Trace read", "raw", rawMessage)

	var base BaseJSONRPCMessage
	if parseError := json.Unmarshal([]byte(rawMessage), &base); parseError != nil {
		if err := r.cfg.OnParseError(ctx, mcp.NewRequestId(nil), parseError); err != nil {
			return trace.Wrap(err, "handling JSON unmarshal error")
		}
	}

	switch {
	case base.IsNotification():
		return trace.Wrap(r.cfg.OnNotification(ctx, base.MakeNotification()), "handling notification")
	case base.IsRequest():
		if r.cfg.OnRequest != nil {
			return trace.Wrap(r.cfg.OnRequest(ctx, base.MakeRequest()), "handling request")
		}
		// Should not happen. Log something just in case.
		r.cfg.Logger.DebugContext(ctx, "Skipping request", "id", base.ID)
		return nil
	case base.IsResponse():
		if r.cfg.OnResponse != nil {
			return trace.Wrap(r.cfg.OnResponse(ctx, base.MakeResponse()), "handling response")
		}
		// Should not happen. Log something just in case.
		r.cfg.Logger.DebugContext(ctx, "Skipping response", "id", base.ID)
		return nil
	default:
		return trace.Wrap(
			r.cfg.OnParseError(ctx, base.ID, trace.BadParameter("unknown message type")),
			"handling unknown message type error",
		)
	}
}

// ReadOneResponse reads one message from the reader and marshals it to a
// response.
func ReadOneResponse(ctx context.Context, reader TransportReader) (*JSONRPCResponse, error) {
	rawMessage, err := reader.ReadMessage(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return unmarshalResponse(rawMessage)
}

// NewForwardMessageReader creates a MessageReader that simply forwards every
// message read from the provided reader to the provided writer.
func NewForwardMessageReader(logger *slog.Logger, reader TransportReader, writer MessageWriter) (*MessageReader, error) {
	return NewMessageReader(MessageReaderConfig{
		Logger:    logger,
		Transport: reader,
		OnNotification: func(ctx context.Context, notification *JSONRPCNotification) error {
			return trace.Wrap(writer.WriteMessage(ctx, notification))
		},
		OnRequest: func(ctx context.Context, request *JSONRPCRequest) error {
			return trace.Wrap(writer.WriteMessage(ctx, request))
		},
		OnResponse: func(ctx context.Context, response *JSONRPCResponse) error {
			return trace.Wrap(writer.WriteMessage(ctx, response))
		},
		OnParseError: LogAndIgnoreParseError(logger),
	})
}
