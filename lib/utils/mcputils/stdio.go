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
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// StderrTraceLogWriter implements io.Writer and logs the content at TRACE
// level. Used for tracing stderr.
type StderrTraceLogWriter struct {
	ctx context.Context
	log *slog.Logger
}

// NewStderrTraceLogWriter returns a new StderrTraceLogWriter.
func NewStderrTraceLogWriter(ctx context.Context, log *slog.Logger) *StderrTraceLogWriter {
	return &StderrTraceLogWriter{
		ctx: ctx,
		log: cmp.Or(log, slog.Default()),
	}
}

// Write implements io.Writer and logs the given input p at trace level.
// Note that the input p may contain arbitrary-length data, which can span
// multiple lines or include partial lines.
func (l *StderrTraceLogWriter) Write(p []byte) (int, error) {
	l.log.Log(l.ctx, logutils.TraceLevel, "Trace stderr", "data", p)
	return len(p), nil
}

// StdioMessageWriter writes a JSONRPC message in stdio transport.
type StdioMessageWriter struct {
	w io.Writer
}

// NewStdioMessageWriter returns a MessageWriter using stdio transport.
func NewStdioMessageWriter(w io.Writer) *StdioMessageWriter {
	return &StdioMessageWriter{
		w: w,
	}
}

// WriteMessage writes a JSONRPC message in stdio transport.
func (w *StdioMessageWriter) WriteMessage(_ context.Context, resp mcp.JSONRPCMessage) error {
	bytes, err := json.Marshal(resp)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintf(w.w, "%s\n", string(bytes))
	return trace.Wrap(err)
}

// HandleParseErrorFunc handles parse errors.
type HandleParseErrorFunc func(context.Context, *mcp.JSONRPCError) error

// ReplyParseError returns a HandleParseErrorFunc that forwards the error to
// provided writer.
func ReplyParseError(w *StdioMessageWriter) HandleParseErrorFunc {
	return func(ctx context.Context, parseError *mcp.JSONRPCError) error {
		return trace.Wrap(w.WriteMessage(ctx, parseError))
	}
}

// LogAndIgnoreParseError returns a HandleParseErrorFunc that logs the parse
// error.
func LogAndIgnoreParseError(log *slog.Logger) HandleParseErrorFunc {
	return func(ctx context.Context, parseError *mcp.JSONRPCError) error {
		log.DebugContext(ctx, "Ignore parse error", "error", parseError)
		return nil
	}
}

// StdioMessageReaderConfig is the config for StdioMessageReader.
type StdioMessageReaderConfig struct {
	// SourceReadCloser is the input to the read the message from.
	// SourceReadCloser will be closed when reader finishes.
	SourceReadCloser io.ReadCloser
	// Logger is the slog.Logger.
	Logger *slog.Logger
	// ParentContext is the parent's context. Used for logging during tear down.
	ParentContext context.Context

	// OnClose is an optional callback when reader finishes.
	OnClose func()
	// OnParseError specifies the handler for handling parse error. Any error
	// returned by the handler stops this message reader.
	OnParseError HandleParseErrorFunc
	// OnRequest specifies the handler for handling request. Any error by the
	// handler stops this message reader.
	OnRequest func(context.Context, *JSONRPCRequest) error
	// OnResponse specifies the handler for handling response. Any error by the
	// handler stops this message reader.
	OnResponse func(context.Context, *JSONRPCResponse) error
	// OnNotification specifies the handler for handling notification. Any error
	// returned by the handler stops this message reader.
	OnNotification func(context.Context, *JSONRPCNotification) error
}

// CheckAndSetDefaults checks values and sets defaults.
func (c *StdioMessageReaderConfig) CheckAndSetDefaults() error {
	if c.SourceReadCloser == nil {
		return trace.BadParameter("missing parameter SourceReadCloser")
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
	if c.ParentContext == nil {
		return trace.BadParameter("missing parameter ParentContext")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "mcp")
	}
	return nil
}

// StdioMessageReader reads requests from provided reader.
type StdioMessageReader struct {
	cfg StdioMessageReaderConfig
}

// NewStdioMessageReader creates a new StdioMessageReader. Must call "Start" to
// start the processing.
func NewStdioMessageReader(cfg StdioMessageReaderConfig) (*StdioMessageReader, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &StdioMessageReader{
		cfg: cfg,
	}, nil
}

// Run starts reading requests from provided reader. Run blocks until an
// error happens from the provided reader or any of the handler.
func (r *StdioMessageReader) Run(ctx context.Context) {
	r.cfg.Logger.InfoContext(ctx, "Start processing stdio messages")

	finished := make(chan struct{})
	go func() {
		r.startProcess(ctx)
		close(finished)
	}()

	select {
	case <-finished:
	case <-ctx.Done():
	}

	r.cfg.Logger.InfoContext(r.cfg.ParentContext, "Finished processing stdio messages")
	if err := r.cfg.SourceReadCloser.Close(); err != nil && !IsOKCloseError(err) {
		r.cfg.Logger.ErrorContext(r.cfg.ParentContext, "Failed to close reader", "error", err)
	}
	if r.cfg.OnClose != nil {
		r.cfg.OnClose()
	}
}

func (r *StdioMessageReader) startProcess(ctx context.Context) {
	lineReader := bufio.NewReader(r.cfg.SourceReadCloser)
	for {
		if ctx.Err() != nil {
			return
		}

		if err := r.processNextLine(ctx, lineReader); err != nil {
			if !IsOKCloseError(err) {
				r.cfg.Logger.ErrorContext(ctx, "Failed to process line", "error", err)
			}
			return
		}
	}
}

func (r *StdioMessageReader) processNextLine(ctx context.Context, lineReader *bufio.Reader) error {
	line, err := lineReader.ReadString('\n')
	if err != nil {
		return trace.Wrap(err, "reading line")
	}

	r.cfg.Logger.Log(ctx, logutils.TraceLevel, "Trace stdio", "line", line)

	var base baseJSONRPCMessage
	if parseError := json.Unmarshal([]byte(line), &base); parseError != nil {
		rpcError := mcp.NewJSONRPCError(mcp.NewRequestId(nil), mcp.PARSE_ERROR, parseError.Error(), nil)
		if err := r.cfg.OnParseError(ctx, &rpcError); err != nil {
			return trace.Wrap(err, "handling JSON unmarshal error")
		}
	}

	switch {
	case base.isNotification():
		return trace.Wrap(r.cfg.OnNotification(ctx, base.makeNotification()), "handling notification")
	case base.isRequest():
		if r.cfg.OnRequest != nil {
			return trace.Wrap(r.cfg.OnRequest(ctx, base.makeRequest()), "handling request")
		}
		// Should not happen. Log something just in case.
		r.cfg.Logger.DebugContext(ctx, "Skipping request", "id", base.ID)
		return nil
	case base.isResponse():
		if r.cfg.OnResponse != nil {
			return trace.Wrap(r.cfg.OnResponse(ctx, base.makeResponse()), "handling response")
		}
		// Should not happen. Log something just in case.
		r.cfg.Logger.DebugContext(ctx, "Skipping response", "id", base.ID)
		return nil
	default:
		rpcError := mcp.NewJSONRPCError(base.ID, mcp.PARSE_ERROR, "unknown message type", line)
		return trace.Wrap(
			r.cfg.OnParseError(ctx, &rpcError),
			"handling unknown message type error",
		)
	}
}
