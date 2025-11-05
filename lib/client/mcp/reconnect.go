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
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// ProxyStdioConnConfig is the config for ProxyStdioConn.
type ProxyStdioConnConfig struct {
	// ClientStdio is the client stdin and stdout.
	ClientStdio io.ReadWriteCloser
	// MakeReconnectUserMessage generates a user-friendly message based on the
	// error.
	MakeReconnectUserMessage func(error) string
	// DialServer makes a new connection to the remote MCP server.
	DialServer func(context.Context) (net.Conn, error)
	// GetApp returns the MCP application.
	GetApp func(context.Context) (types.Application, error)
	// AutoReconnect attempts to re-establish new MCP sessions with the remote
	// server when encounter connection issues.
	AutoReconnect bool
	// HTTPHeaders defines extra custom headers for HTTP transports.
	HTTPHeaders map[string]string

	// Logger is the slog logger.
	Logger *slog.Logger

	// clientResponseWriter replies to ClientStdio.
	clientResponseWriter mcputils.MessageWriter
	// onServerConnClosed is a callback when remote server connection is dead.
	onServerConnClosed func()
}

// CheckAndSetDefaults validates the config and sets default values.
func (cfg *ProxyStdioConnConfig) CheckAndSetDefaults() error {
	if cfg.ClientStdio == nil {
		return trace.BadParameter("missing ClientStdio")
	}
	if cfg.GetApp == nil {
		return trace.BadParameter("missing GetApp")
	}
	if cfg.DialServer == nil {
		return trace.BadParameter("missing DialServer")
	}
	if cfg.MakeReconnectUserMessage == nil {
		cfg.MakeReconnectUserMessage = func(err error) string {
			return err.Error()
		}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentMCP, "autoreconnect"),
		)
	}
	if cfg.clientResponseWriter == nil {
		cfg.clientResponseWriter = mcputils.NewSyncStdioMessageWriter(cfg.ClientStdio)
	}
	return nil
}

// ProxyStdioConn serves a stdio client and handles transport conversion to
// the remote MCP servers. When AutoConnect is set, it also reconnects to the
// remote server with new MCP sessions upon connection issues.
func ProxyStdioConn(ctx context.Context, cfg ProxyStdioConnConfig) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	serverConn, err := newServerConnWithAutoReconnect(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	clientRequestReader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport:    mcputils.NewStdioReader(cfg.ClientStdio),
		Logger:       cfg.Logger.With("client", "stdin"),
		OnParseError: mcputils.ReplyParseError(cfg.clientResponseWriter),
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			// By spec, we should not reply to notifications. Try our best to
			// send a notification with the error message. In practice, only the
			// initialize notification is sent from client after receiving the
			// initialize response so it's unlikely to hit here.
			if writeError := serverConn.WriteMessage(ctx, notification); writeError != nil {
				if serverConn.shouldExitOnWriteError() {
					return trace.Wrap(writeError)
				}
				cfg.Logger.WarnContext(ctx, "failed to write notification to server. Notification is dropped.", "error", writeError)
				userMessage := cfg.MakeReconnectUserMessage(writeError)
				errNotification := mcp.Notification{
					Method: "notifications/tsherr",
					Params: mcp.NotificationParams{
						AdditionalFields: map[string]any{
							"error": fmt.Sprintf("Notification %q was dropped. %s", notification.Method, userMessage),
						},
					},
				}
				return trace.Wrap(cfg.clientResponseWriter.WriteMessage(ctx, errNotification))
			}
			return nil
		},
		OnRequest: func(ctx context.Context, request *mcputils.JSONRPCRequest) error {
			if writeError := serverConn.WriteMessage(ctx, request); writeError != nil {
				if serverConn.shouldExitOnWriteError() {
					return trace.Wrap(writeError)
				}
				cfg.Logger.WarnContext(ctx, "failed to write request to server", "error", writeError)
				userMessage := cfg.MakeReconnectUserMessage(writeError)
				errResp := mcp.NewJSONRPCError(request.ID, mcp.INTERNAL_ERROR, userMessage, writeError)
				return trace.Wrap(cfg.clientResponseWriter.WriteMessage(ctx, errResp))
			}
			return nil
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	clientRequestReader.Run(ctx)
	return nil

}

type serverConnWithAutoReconnect struct {
	ProxyStdioConnConfig
	closeCtx       context.Context
	closeCtxCancel context.CancelFunc

	mu                  sync.Mutex
	serverMessageWriter mcputils.MessageWriter
	serverMessageReader *mcputils.MessageReader
	firstConnectionDone bool
	initRequest         *mcputils.JSONRPCRequest
	initResponse        *mcp.InitializeResult
	initNotification    *mcputils.JSONRPCNotification
}

func newServerConnWithAutoReconnect(parentCtx context.Context, cfg ProxyStdioConnConfig) (*serverConnWithAutoReconnect, error) {
	closeCtx, closeCtxCancel := context.WithCancel(parentCtx)
	return &serverConnWithAutoReconnect{
		ProxyStdioConnConfig: cfg,
		closeCtx:             closeCtx,
		closeCtxCancel:       closeCtxCancel,
	}, nil
}

func (r *serverConnWithAutoReconnect) Close() error {
	r.Logger.InfoContext(r.closeCtx, "Closing server connection")

	r.mu.Lock()
	r.closeCtxCancel()
	if r.serverMessageReader == nil {
		r.mu.Unlock()
		return nil
	}

	// Wait without lock.
	wait := r.serverMessageReader.Done()
	r.mu.Unlock()
	<-wait
	return nil
}

func (r *serverConnWithAutoReconnect) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	writer, err := r.getServerRequestWriterLocked(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(writer.WriteMessage(ctx, msg))
}

func (r *serverConnWithAutoReconnect) makeServerTransport(ctx context.Context) (mcputils.TransportReader, mcputils.MessageWriter, error) {
	r.Logger.InfoContext(ctx, "Making new transport to server")
	app, err := r.GetApp(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch types.GetMCPServerTransportType(app.GetURI()) {
	case types.MCPTransportHTTP:
		transport, err := defaults.Transport()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return r.DialServer(ctx)
		}
		httpReaderWriter, err := mcputils.NewHTTPReaderWriter(
			r.closeCtx,
			"http://localhost", // does not matter with the custom transport.
			mcpclienttransport.WithHTTPBasicClient(&http.Client{
				Transport: transport,
			}),
			mcpclienttransport.WithContinuousListening(),
			mcpclienttransport.WithHTTPHeaders(r.HTTPHeaders),
		)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return httpReaderWriter, httpReaderWriter, nil

	default:
		serverConn, err := r.DialServer(ctx)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return mcputils.NewStdioReader(serverConn),
			mcputils.NewStdioMessageWriter(serverConn),
			nil
	}
}

func (r *serverConnWithAutoReconnect) canRetryLocked() bool {
	// We are closed, no retry.
	if r.closeCtx.Err() != nil {
		return false
	}

	// When auto-reconnect is on, always retry without exiting.
	// When auto-reconnect is off, see if we have made the first connection yet.
	// If not, we could retry until the first connection is established.
	return r.AutoReconnect || !r.firstConnectionDone
}

func (r *serverConnWithAutoReconnect) shouldExitOnWriteError() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// just exit if we cannot retry.
	return !r.canRetryLocked()
}

func (r *serverConnWithAutoReconnect) getServerRequestWriterLocked(ctx context.Context) (mcputils.MessageWriter, error) {
	if r.serverMessageWriter != nil {
		return r.serverMessageWriter, nil
	}

	if !r.canRetryLocked() {
		// We shouldn't hit here as the proxy should have been ended.
		// Double-check just in case.
		return nil, trace.Errorf("mcp session finished")
	}

	serverTransportReader, serverWriter, err := r.makeServerTransport(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if r.firstConnectionDone {
		// Replay initialize sequence. Any error here is likely permanent.
		if err := r.replayInitializeLocked(ctx, serverTransportReader, serverWriter); err != nil {
			serverTransportReader.Close()
			return nil, trace.Wrap(err)
		}
		r.serverMessageWriter = serverWriter
	} else {
		r.serverMessageWriter = mcputils.NewMultiMessageWriter(
			mcputils.MessageWriterFunc(func(ctx context.Context, msg mcp.JSONRPCMessage) error {
				r.cacheMessageLocked(ctx, msg)
				return nil
			}),
			serverWriter,
		)
		r.firstConnectionDone = true
	}

	// This should never fail as long the correct config is passed in.
	r.serverMessageReader, err = mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport: serverTransportReader,
		// OnClose is called when server connection is dead or if any handler
		// fails. Teleport Proxy automatically closes the connection when tsh
		// session is expired.
		OnClose: func() {
			r.mu.Lock()
			if r.canRetryLocked() {
				r.Logger.InfoContext(ctx, "Lost server session, resetting...")
			} else {
				r.Logger.InfoContext(ctx, "Lost server session, closing...")
				r.ClientStdio.Close()
			}
			r.serverMessageWriter = nil
			if r.onServerConnClosed != nil {
				r.onServerConnClosed()
			}
			r.mu.Unlock()
		},
		Logger:       r.Logger.With("server", "stdout"),
		OnParseError: mcputils.LogAndIgnoreParseError(r.Logger),
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			return trace.Wrap(r.clientResponseWriter.WriteMessage(ctx, notification))
		},
		OnResponse: func(ctx context.Context, response *mcputils.JSONRPCResponse) error {
			r.cacheMessageLocked(ctx, response)
			return trace.Wrap(r.clientResponseWriter.WriteMessage(ctx, response))
		},
	})
	if err != nil {
		serverTransportReader.Close()
		return nil, trace.Wrap(err)
	}

	go r.serverMessageReader.Run(r.closeCtx)

	r.Logger.InfoContext(ctx, "Started a new MCP server connection")
	return r.serverMessageWriter, nil
}

func (r *serverConnWithAutoReconnect) initializedLocked() bool {
	return r.initRequest != nil && r.initResponse != nil && r.initNotification != nil
}

func (r *serverConnWithAutoReconnect) replayInitializeLocked(ctx context.Context, serverReader mcputils.TransportReader, serverWriter mcputils.MessageWriter) error {
	if !r.initializedLocked() {
		return trace.Errorf("client has not initialized yet")
	}

	r.Logger.DebugContext(ctx, "Replaying initialize request")
	if err := serverWriter.WriteMessage(ctx, r.initRequest); err != nil {
		return trace.Wrap(err)
	}

	r.Logger.DebugContext(ctx, "Reading and comparing initialize response")
	msg, err := mcputils.ReadOneResponse(ctx, serverReader)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := r.checkReplyResponseLocked(msg); err != nil {
		return trace.Wrap(err)
	}

	r.Logger.DebugContext(ctx, "Replaying initialized notification")
	if err := serverWriter.WriteMessage(ctx, r.initNotification); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *serverConnWithAutoReconnect) checkReplyResponseLocked(msg mcp.JSONRPCMessage) error {
	resp, ok := msg.(*mcputils.JSONRPCResponse)
	if !ok {
		return trace.Errorf("expected initialize response, got %T", resp)
	}
	if resp.Error != nil {
		return trace.Errorf("expected initialize result but got error")
	}
	if resp.ID.String() != r.initRequest.ID.String() {
		return trace.CompareFailed("expected initialize response with ID %s, got %s", r.initRequest.ID, resp.ID.String())
	}

	newResult, err := resp.GetInitializeResult()
	if err != nil {
		return trace.Wrap(err)
	}
	if newResult.ServerInfo != r.initResponse.ServerInfo {
		return trace.Wrap(&serverInfoChangedError{
			expectedInfo: r.initResponse.ServerInfo,
			currentInfo:  newResult.ServerInfo,
		})
	}
	return nil
}

// cacheMessageLocked caches client init request and notification.
func (r *serverConnWithAutoReconnect) cacheMessageLocked(ctx context.Context, msg mcp.JSONRPCMessage) {
	if r.initializedLocked() {
		return
	}

	switch m := msg.(type) {
	case *mcputils.JSONRPCRequest:
		if r.initRequest == nil && m.Method == mcp.MethodInitialize {
			r.initRequest = m
			r.Logger.DebugContext(ctx, "Cached initialize", "request", m)
		}
	case *mcputils.JSONRPCNotification:
		if r.initNotification == nil && m.Method == mcputils.MethodNotificationInitialized {
			r.initNotification = m
			r.Logger.DebugContext(ctx, "Cached notification", "notification", m)
		}
	case *mcputils.JSONRPCResponse:
		if r.initResponse == nil && r.initRequest != nil && r.initRequest.ID.String() == m.ID.String() {
			initResponse, err := m.GetInitializeResult()
			if err != nil {
				r.Logger.DebugContext(ctx, "Error parsing init response", "error", err)
			} else {
				r.initResponse = initResponse
				r.Logger.DebugContext(ctx, "Cached response", "response", m)
			}
		}
	}
}
