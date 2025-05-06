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

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func isOKCloseError(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		utils.IsOKNetworkError(err)
}

type traceLogWriter struct {
	ctx context.Context
	log *slog.Logger
}

func newTraceLogWriter(ctx context.Context, log *slog.Logger) *traceLogWriter {
	return &traceLogWriter{
		log: log,
		ctx: ctx,
	}
}

func (l *traceLogWriter) Write(p []byte) (n int, err error) {
	l.log.Log(l.ctx, logutils.TraceLevel, string(p))
	return len(p), nil
}

type requestReader struct {
	*sessionCtx
	toClient     io.Writer
	closeCommand func()
	out          *io.PipeWriter
}

func (r *requestReader) process(ctx context.Context) {
	r.log.DebugContext(ctx, "Started request reader")
	defer r.log.DebugContext(ctx, "Finished request reader")
	defer r.clientConn.Close()
	defer r.closeCommand()

	lineReader := bufio.NewReader(r.clientConn)
	for {
		if ctx.Err() != nil {
			return
		}
		line, err := lineReader.ReadString('\n')
		if err != nil {
			r.out.CloseWithError(err)
			if !isOKCloseError(err) {
				r.log.ErrorContext(ctx, "Failed to read request from client", "error", err)
			}
			return
		}

		r.log.Log(ctx, logutils.TraceLevel, line)

		if r.shouldForwardLine(ctx, line) {
			if _, err := r.out.Write([]byte(line)); err != nil {
				r.log.ErrorContext(ctx, "Failed to write request to server", "error", err)
				return
			}
		}
	}
}

func (r *requestReader) shouldForwardLine(ctx context.Context, line string) bool {
	var msg baseRequest
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		r.log.DebugContext(ctx, "Failed to parse request", "error", err, "line", line)
		return true
	}

	// TODO(greedy52) refactor
	r.idTracker.track(&msg)

	switch {
	case msg.ID != nil && msg.Method == mcp.MethodToolsCall:
		if authErr := r.checkToolAccess(ctx, &msg); authErr != nil {
			emitRequestEvent(r.parentCtx, r.sessionCtx, &msg, authErr)
			r.replyToolResultWithError(ctx, &msg, authErr)
			return false
		}
	}

	if shouldEmitMCPEvent(msg.Method) {
		emitRequestEvent(r.parentCtx, r.sessionCtx, &msg, nil)
	}
	return true
}

func (r *requestReader) checkToolAccess(ctx context.Context, msg *baseRequest) error {
	toolName, ok := msg.getName()
	if !ok {
		return trace.BadParameter("missing tool name")
	}
	return trace.Wrap(r.checkAccessToTool(toolName))
}

func (r *requestReader) replyToolResultWithError(ctx context.Context, msg *baseRequest, authErr error) {
	resp := makeToolAccessDeniedResponse(msg, authErr)

	respBytes, err := json.Marshal(resp)
	if err != nil {
		r.log.ErrorContext(ctx, "Failed to marshal JSON RPC response", "error", err)
		return
	}

	if _, err := fmt.Fprintf(r.toClient, "%s\n", respBytes); err != nil {
		r.log.ErrorContext(ctx, "Failed to send JSON RPC response", "error", err)
	}
}

type idTracker struct {
	// TODO(greedy52) maybe use a linked list instead of a map?
	byIDs map[string]mcp.MCPMethod
	mu    sync.Mutex
}

func newIDTracker() *idTracker {
	return &idTracker{
		byIDs: make(map[string]mcp.MCPMethod),
	}
}

func (t *idTracker) track(msg *baseRequest) {
	if msg.ID == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byIDs[msg.getID()] = msg.Method
}

func (t *idTracker) remove(msg *baseResponse) mcp.MCPMethod {
	if msg.ID == nil {
		return "unknown"
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if method, ok := t.byIDs[msg.getID()]; ok {
		delete(t.byIDs, msg.getID())
		return method
	}
	return "unknown"
}

type responseWriter struct {
	*sessionCtx
	toClient   io.Writer
	toProcess  io.ReadCloser
	fromServer io.WriteCloser
}

func newResponseWriter(sessionCtx *sessionCtx) *responseWriter {
	toProcess, fromServer := io.Pipe()
	return &responseWriter{
		sessionCtx: sessionCtx,
		fromServer: fromServer,
		toProcess:  toProcess,
		toClient:   utils.NewSyncWriter(sessionCtx.clientConn),
	}
}

func (w *responseWriter) process(ctx context.Context) {
	w.log.DebugContext(ctx, "Started response writer")
	defer w.log.DebugContext(ctx, "Finished response writer")
	defer w.toProcess.Close()
	defer w.fromServer.Close()

	lineReader := bufio.NewReader(w.toProcess)
	for {
		if ctx.Err() != nil {
			return
		}
		readLine, err := lineReader.ReadString('\n')
		if err != nil {
			if !isOKCloseError(err) {
				w.log.ErrorContext(ctx, "Failed to read response from server", "error", err)
			}
			return
		}
		readLine = strings.TrimSuffix(readLine, "\n")
		writeLine := w.processLine(ctx, readLine)
		if _, err := fmt.Fprintf(w.toClient, "%s\n", writeLine); err != nil {
			if !isOKCloseError(err) {
				w.log.ErrorContext(ctx, "Failed to write response to client", "error", err)
			}
			return
		}
	}
}

func (w *responseWriter) processLine(ctx context.Context, line string) string {
	var msg baseResponse
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		w.log.DebugContext(ctx, "Failed to parse response", "error", err, "line", line)
		return line
	}

	method := w.idTracker.remove(&msg)
	switch method {
	case mcp.MethodToolsList:
		return w.processToolsList(ctx, &msg, line)
	default:
		return line
	}
}

func (w *responseWriter) processToolsList(ctx context.Context, msg *baseResponse, line string) string {
	// could be an error, unlikely though.
	if len(msg.Result) == 0 || len(msg.Error) > 0 {
		return line
	}

	var listResult mcp.ListToolsResult
	if err := json.Unmarshal([]byte(msg.Result), &listResult); err != nil {
		w.log.DebugContext(ctx, "Failed to parse response", "error", err, "line", line)
		return line
	}

	var allowed []mcp.Tool
	for _, tool := range listResult.Tools {
		if w.checkAccessToTool(tool.Name) == nil {
			allowed = append(allowed, tool)
		}
	}
	w.log.DebugContext(ctx, "Got tools/list result", "received", len(listResult.Tools), "allowed", len(allowed))
	listResult.Tools = allowed

	resp := mcp.JSONRPCResponse{
		JSONRPC: msg.JSONRPC,
		ID:      msg.ID,
		Result:  &listResult,
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		w.log.ErrorContext(ctx, "Failed to marshal JSON RPC response", "error", err)
	}
	return string(respBytes)
}
