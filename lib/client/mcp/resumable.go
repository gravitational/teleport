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
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type AutoReconnectStdioConnConfig struct {
	ParentCtx            context.Context
	Logger               *slog.Logger
	ClientResponseWriter mcputils.MessageWriter
	DialServer           func(ctx context.Context) (net.Conn, error)
}

func (cfg *AutoReconnectStdioConnConfig) CheckAndSetDefaults() error {
	if cfg.ParentCtx == nil {
		return trace.BadParameter("missing parent context")
	}
	if cfg.ClientResponseWriter == nil {
		return trace.BadParameter("missing client response writer")
	}
	if cfg.DialServer == nil {
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentMCP, "autoreconnect"),
		)
	}
	return nil
}

type AutoReconnectStdioConn struct {
	AutoReconnectStdioConnConfig

	mu                  sync.Mutex
	serverRequestWriter mcputils.MessageWriter
	seenFirstConn       bool
	initRequest         *mcputils.JSONRPCRequest
	initNotification    *mcputils.JSONRPCNotification
}

func NewAutoReconnectStdioConn(c AutoReconnectStdioConnConfig) (*AutoReconnectStdioConn, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AutoReconnectStdioConn{
		AutoReconnectStdioConnConfig: c,
	}, nil
}

func (r *AutoReconnectStdioConn) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	writer, err := r.getServerRequestWriterLocked(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(writer.WriteMessage(ctx, msg))
}

func (r *AutoReconnectStdioConn) getServerRequestWriterLocked(ctx context.Context) (mcputils.MessageWriter, error) {
	if r.serverRequestWriter != nil {
		return r.serverRequestWriter, nil
	}

	r.Logger.InfoContext(ctx, "Connecting to server")
	serverConn, err := r.DialServer(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if r.seenFirstConn {
		if err := r.replayInitializeLocked(ctx, serverConn); err != nil {
			return nil, trace.Wrap(err)
		}
		r.serverRequestWriter = mcputils.NewStdioMessageWriter(serverConn)
	} else {
		r.seenFirstConn = true
		r.serverRequestWriter = mcputils.NewMultiMessageWriter(
			mcputils.MessageWriterFunc(r.cacheMessageLocked),
			mcputils.NewStdioMessageWriter(serverConn),
		)
	}

	// This should never fail as long the correct config is passed in.
	serverResponseReader, err := mcputils.NewStdioMessageReader(mcputils.StdioMessageReaderConfig{
		SourceReadCloser: serverConn,
		ParentContext:    r.ParentCtx,
		OnClose: func() {
			r.Logger.InfoContext(ctx, "Lost server connection, resetting...")
			r.mu.Lock()
			r.serverRequestWriter = nil
			r.mu.Unlock()
		},
		Logger:       r.Logger.With("server", "stdout"),
		OnParseError: mcputils.LogAndIgnoreParseError(r.Logger),
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			return trace.Wrap(r.ClientResponseWriter.WriteMessage(ctx, notification))
		},
		OnResponse: func(ctx context.Context, response *mcputils.JSONRPCResponse) error {
			return trace.Wrap(r.ClientResponseWriter.WriteMessage(ctx, response))
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go serverResponseReader.Run(r.ParentCtx)
	r.Logger.InfoContext(ctx, "Started a new MCP server connection")
	return r.serverRequestWriter, nil
}

func (r *AutoReconnectStdioConn) initializedLocked() bool {
	return r.initRequest != nil && r.initNotification != nil
}

func (r *AutoReconnectStdioConn) replayInitializeLocked(ctx context.Context, serverConn net.Conn) error {
	if !r.initializedLocked() {
		return trace.BadParameter("client has not initialized yet")
	}

	r.Logger.InfoContext(ctx, "Resending initialize request")
	serverWriter := mcputils.NewStdioMessageWriter(serverConn)
	if err := serverWriter.WriteMessage(ctx, r.initRequest); err != nil {
		return trace.Wrap(err)
	}

	msg, err := mcputils.ReadOneMessageFromStdioReader(serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) should we double-check whether server name, capabilities,
	// etc. has changed?
	if !isSuccessMCPResponseWithID(msg, r.initRequest.ID) {
		return trace.BadParameter("invalid response from server")
	}

	r.Logger.InfoContext(ctx, "Resending initialized notification")
	if err := serverWriter.WriteMessage(ctx, r.initNotification); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cacheMessageLocked caches client init request and notification.
func (r *AutoReconnectStdioConn) cacheMessageLocked(ctx context.Context, msg mcp.JSONRPCMessage) error {
	if r.initializedLocked() {
		return nil
	}

	switch m := msg.(type) {
	case *mcputils.JSONRPCRequest:
		if m.Method == mcp.MethodInitialize && r.initRequest == nil {
			r.initRequest = m
			r.Logger.DebugContext(ctx, "Initialize request cached", "msg", m)
		}
	case *mcputils.JSONRPCNotification:
		if m.Method == "notifications/initialized" && r.initNotification == nil {
			r.initNotification = m
			r.Logger.DebugContext(ctx, "Initialize notification cached", "msg", m)
		}
	}
	return nil
}

func isSuccessMCPResponseWithID(msg any, wantID mcp.RequestId) bool {
	resp, ok := msg.(*mcputils.JSONRPCResponse)
	if !ok {
		return false
	}
	if resp.Error != nil {
		return false
	}
	return resp.ID.String() == wantID.String()
}

func DebugErrorType(ctx context.Context, err error, logger *slog.Logger) {
	debugErrorType(ctx, err, logger, 1)
}

func debugErrorType(ctx context.Context, err error, logger *slog.Logger, level int) {
	if level >= 10 {
		logger.DebugContext(ctx, "err too many levels")
		return
	}

	logger.DebugContext(ctx, "debug error type", "err", err, "level", level, "type", logutils.TypeAttr(err))
	if unwrappable, ok := err.(interface{ Unwrap() error }); ok {
		debugErrorType(ctx, unwrappable.Unwrap(), logger, level+1)
	}
}

func IsLikelyTemporaryNetworkError(err error) bool {
	if trace.IsConnectionProblem(err) ||
		isTemporarySyscallNetError(err) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return IsLikelyTemporaryNetworkError(opErr.Err)
	}
	return false
}

func isTemporarySyscallNetError(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ECONNREFUSED)
}
