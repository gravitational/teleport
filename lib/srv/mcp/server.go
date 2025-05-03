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
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ServerConfig is the config for the MCP forward server.
type ServerConfig struct {
	Emitter apievents.Emitter
	Log     *slog.Logger
}

// CheckAndSetDefaults checks values and sets defaults
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, "mcp")
	}
	return nil
}

// Server handles forwarding client connections to MCP servers.
type Server struct {
	cfg ServerConfig
}

// NewServer creates a new Server.
func NewServer(c ServerConfig) (*Server, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Server{
		cfg: c,
	}, nil
}

// HandleAuthorizedAppConnection handles an authorized client connection.
func (s *Server) HandleAuthorizedAppConnection(ctx context.Context, clientConn net.Conn, authCtx *authz.Context, app types.Application) error {
	switch types.GetMCPServerTransportType(app.GetURI()) {
	case types.MCPTransportStdio:
		return trace.Wrap(s.handleStdio(ctx, clientConn, authCtx, app))
	default:
		return trace.BadParameter("unsupported MCP server transport type: %v", types.GetMCPServerTransportType(app.GetURI()))
	}
}

func (s *Server) handleStdio(ctx context.Context, clientConn net.Conn, authCtx *authz.Context, app types.Application) error {
	mcpSpec := app.GetMCP()
	if mcpSpec == nil {
		return trace.BadParameter("missing MCP spec")
	}

	identity := authCtx.Identity.GetIdentity()
	log := s.cfg.Log.With("session", identity.RouteToApp.SessionID, "app", app.GetName(), "user", identity.Username)

	log.DebugContext(ctx, "Running mcp",
		"cmd", mcpSpec.Command,
		"args", mcpSpec.Args,
	)

	cmd := exec.CommandContext(ctx, mcpSpec.Command, mcpSpec.Args...)
	if err := s.setRunAsLocalUser(ctx, cmd, mcpSpec.RunAsLocalUser); err != nil {
		return trace.Wrap(err)
	}

	// Response may come from the server or from internal access check.
	responseWriter := utils.NewSyncWriter(clientConn)

	// Parse incoming request, then forward or reject.
	// TODO(greedy52) convert this to a proper MCP server.
	in, out := io.Pipe()
	requestReader := &requestReader{
		clientConn:     clientConn,
		authCtx:        authCtx,
		app:            app,
		responseWriter: responseWriter,
		out:            out,
		log:            log.With("stdio", "stdin"),
	}
	go requestReader.process(ctx)

	cmd.Stdin = in
	cmd.Stdout = io.MultiWriter(responseWriter, newTraceLogWriter(ctx, log.With("stdio", "stdout")))
	cmd.Stderr = newTraceLogWriter(ctx, log.With("stdio", "stderr"))
	return trace.Wrap(cmd.Run())
}

type requestReader struct {
	clientConn     net.Conn
	authCtx        *authz.Context
	app            types.Application
	responseWriter io.Writer
	log            *slog.Logger
	out            *io.PipeWriter
}

func (r *requestReader) process(ctx context.Context) {
	defer r.clientConn.Close()

	lineReader := bufio.NewReader(r.clientConn)
	for {
		if ctx.Err() != nil {
			return
		}
		line, err := lineReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				r.out.CloseWithError(err)
			}
			if !utils.IsOKNetworkError(err) {
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
	var msg baseMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		r.log.DebugContext(ctx, "Failed to parse request", "error", err, "line", line)
		return true
	}

	switch {
	case msg.ID != nil && msg.Method == mcp.MethodToolsCall:
		if authErr := r.checkToolAccess(ctx, &msg); authErr != nil {
			r.audit(ctx, &msg, authErr)
			r.replyToolResultWithError(ctx, &msg, authErr)
			return false
		}
	}

	if shouldEmitMCPEvent(msg.Method) {
		r.audit(ctx, &msg, nil)
	}
	return true
}

func (r *requestReader) checkToolAccess(ctx context.Context, msg *baseMessage) error {
	toolName, ok := msg.getName()
	if !ok {
		return trace.BadParameter("missing tool name")
	}

	return trace.Wrap(r.authCtx.Checker.CheckAccess(
		r.app,
		services.AccessState{
			MFAVerified:    true,
			DeviceVerified: true,
		},
		&services.MCPToolMatcher{
			Name: toolName,
		},
	))
}

func (r *requestReader) replyToolResultWithError(ctx context.Context, msg *baseMessage, authErr error) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      msg.ID,
		Result: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Access denied to this MCP tool: %v. RBAC is enforced by your Teleport roles.", authErr),
			}},
			IsError: false,
		},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		r.log.ErrorContext(ctx, "Failed to marshal JSON RPC response", "error", err)
	}

	if _, err := fmt.Fprintf(r.responseWriter, "%s\n", respBytes); err != nil {
		r.log.ErrorContext(ctx, "Failed to send JSON RPC response", "error", err)
	}
}

func (r *requestReader) audit(ctx context.Context, msg *baseMessage, err error) {
	r.log.DebugContext(ctx, "Received request", "method", msg.Method)
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

func (s *Server) setRunAsLocalUser(ctx context.Context, cmd *exec.Cmd, localUserName string) error {
	localUser, err := user.Lookup(localUserName)
	if err != nil {
		return trace.Wrap(err, "finding local user")
	}
	cred, err := host.GetLocalUserCredential(localUser)
	if err != nil {
		return trace.Wrap(err, "getting local user credential")
	}

	if os.Getuid() == int(cred.Uid) || os.Getgid() == int(cred.Gid) {
		s.cfg.Log.DebugContext(ctx, "Launching process with ambient credentials")
		return nil
	}

	s.cfg.Log.DebugContext(ctx, "Launching process as local user", "user", localUserName, "uid", cred.Uid, "gid", cred.Gid)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = cred
	return nil
}

// HandleUnauthorizedAppConnection handles an unauthorized client connection.
func (s *Server) HandleUnauthorizedAppConnection(ctx context.Context, clientConn net.Conn, err error) error {
	// TODO(greedy52) serve the error within MCP protocol. Currently the
	// connection is killed without extra information to the client.
	return trace.Wrap(err)
}
