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

// NewSyncStdioMessageWriter returns a MessageWriter using stdio transport, the
// "sync" version.
func NewSyncStdioMessageWriter(w io.Writer) MessageWriter {
	return NewSyncMessageWriter(NewStdioMessageWriter(w))
}

// StdioReader implements TransportReader for stdio transport
type StdioReader struct {
	io.Closer
	br *bufio.Reader
}

// NewStdioReader creates a new StdioReader. Input reader can be either stdin or
// stdout.
func NewStdioReader(readCloser io.ReadCloser) *StdioReader {
	return &StdioReader{
		Closer: readCloser,
		br:     bufio.NewReader(readCloser),
	}
}

// ReadMessage reads the next line.
func (r *StdioReader) ReadMessage(context.Context) (string, error) {
	line, err := r.br.ReadString('\n')
	if err != nil {
		return "", trace.Wrap(err)
	}
	return line, nil
}

// Type returns "stdio".
func (r *StdioReader) Type() string {
	return "stdio"
}
