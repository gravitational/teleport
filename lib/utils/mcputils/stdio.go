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

// Write implements io.Writer.
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
type HandleParseErrorFunc func(context.Context, error) error

// ReplyParseError returns a HandleParseErrorFunc that forwards the error to
// provided writer.
func ReplyParseError(w *StdioMessageWriter) HandleParseErrorFunc {
	return func(ctx context.Context, parseError error) error {
		rpcErr := mcp.NewJSONRPCError(nil, mcp.PARSE_ERROR, parseError.Error(), nil)
		return trace.Wrap(w.WriteMessage(ctx, rpcErr))
	}
}

// LogAndIgnoreParseError returns a HandleParseErrorFunc that logs the parse
// error.
func LogAndIgnoreParseError(log *slog.Logger) HandleParseErrorFunc {
	return func(ctx context.Context, parseError error) error {
		log.DebugContext(ctx, "Ignore parse error", "error", parseError)
		return nil
	}
}

// StdioMessageReaderConfig is the config for StdioMessageReader.
type StdioMessageReaderConfig struct {
	// SourceReadCloser is the input to the read the message from.
	// SourceReadCloser will be closed when reader finishes.
	SourceReadCloser io.ReadCloser
	// ReadRequest indicates this reader reads request. Mutually exclusive with
	// ReadResponse.
	ReadRequest bool
	// ReadResponse indicates this reader reads response. Mutually exclusive
	// with ReadRequest.
	ReadResponse bool
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
	if !c.ReadRequest && !c.ReadResponse {
		return trace.BadParameter("one of ReadRequest or ReadResponse must be true")
	} else if c.ReadRequest && c.ReadResponse {
		return trace.BadParameter("only one of ReadRequest or ReadResponse can be true")
	}
	if c.OnParseError == nil {
		return trace.BadParameter("missing parameter OnParseError")
	}
	if c.OnNotification == nil {
		return trace.BadParameter("missing parameter OnNotification")
	}
	if c.OnRequest == nil && c.ReadRequest {
		return trace.BadParameter("missing parameter OnRequest")
	}
	if c.OnResponse == nil && c.ReadResponse {
		return trace.BadParameter("missing parameter OnResponse")
	}
	if c.ParentContext == nil {
		return trace.BadParameter("missing parameter CloseContext")
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

// Start starts reading requests from provided reader. Start blocks until an
// error happens from the provided reader or any of the handler.
func (r *StdioMessageReader) Start(ctx context.Context) {
	if r.cfg.ReadRequest {
		r.cfg.Logger.InfoContext(ctx, "Start processing stdio request")
	} else {
		r.cfg.Logger.InfoContext(ctx, "Start processing stdio response")
	}

	finished := make(chan struct{})
	go func() {
		r.startProcess(ctx)
		close(finished)
	}()

	select {
	case <-finished:
	case <-ctx.Done():
	}

	if r.cfg.ReadRequest {
		r.cfg.Logger.InfoContext(r.cfg.ParentContext, "Finished processing stdio response")
	} else {
		r.cfg.Logger.InfoContext(r.cfg.ParentContext, "Finished processing stdio response")
	}
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

	if r.cfg.ReadRequest {
		r.cfg.Logger.Log(ctx, logutils.TraceLevel, "Trace request", "request", line)
	} else {
		r.cfg.Logger.Log(ctx, logutils.TraceLevel, "Trace response", "response", line)
	}

	var base baseJSONRPCMessage
	if parseError := json.Unmarshal([]byte(line), &base); parseError != nil {
		if err := r.cfg.OnParseError(ctx, parseError); err != nil {
			return trace.Wrap(err, "handling parse error")
		}
	}

	switch {
	case base.isNotification():
		return trace.Wrap(r.cfg.OnNotification(ctx, base.makeNotification()), "handling notification")
	case r.cfg.ReadRequest:
		return trace.Wrap(r.cfg.OnRequest(ctx, base.makeRequest()), "handling request")
	case r.cfg.ReadResponse:
		return trace.Wrap(r.cfg.OnResponse(ctx, base.makeResponse()), "handling response")
	default:
		// Not possible to hit.
		return nil
	}
}
