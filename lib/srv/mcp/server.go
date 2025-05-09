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
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/utils/host"
)

// ServerConfig is the config for the MCP forward server.
type ServerConfig struct {
	ParentCtx context.Context
	Emitter   apievents.Emitter
	Log       *slog.Logger
	ServerID  string
}

// CheckAndSetDefaults checks values and sets defaults
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.ParentCtx == nil {
		return trace.BadParameter("missing ParentCtx")
	}
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.ServerID == "" {
		return trace.BadParameter("missing ServerID")
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

func (s *Server) makeSessionCtx(ctx context.Context, clientConn net.Conn, authCtx *authz.Context, app types.Application) *sessionCtx {
	identity := authCtx.Identity.GetIdentity()
	allowedTools := authCtx.Checker.EnumerateMCPTools(app)
	sessionCtx := &sessionCtx{
		parentCtx:  s.cfg.ParentCtx,
		clientConn: clientConn,
		authCtx:    authCtx,
		identity:   identity,
		app:        app,
		serverID:   s.cfg.ServerID,
		log: s.cfg.Log.With(
			"session", identity.RouteToApp.SessionID,
			"app", app.GetName(),
			"user", identity.Username,
		),
		emitter:      s.cfg.Emitter,
		allowedTools: allowedTools,
		idTracker:    newIDTracker(),
	}
	return sessionCtx
}

// HandleAuthorizedAppConnection handles an authorized client connection.
func (s *Server) HandleAuthorizedAppConnection(ctx context.Context, clientConn net.Conn, authCtx *authz.Context, app types.Application) error {
	switch types.GetMCPServerTransportType(app.GetURI()) {
	case types.MCPTransportStdio:
		return trace.Wrap(s.handleStdio(ctx, s.makeSessionCtx(ctx, clientConn, authCtx, app)))
	default:
		return trace.BadParameter("unsupported MCP server transport type: %v", types.GetMCPServerTransportType(app.GetURI()))
	}
}

func (s *Server) handleStdio(ctx context.Context, sessionCtx *sessionCtx) error {
	mcpSpec := sessionCtx.app.GetMCP()
	if mcpSpec == nil {
		return trace.BadParameter("missing MCP spec")
	}

	sessionCtx.log.DebugContext(ctx, "Running mcp",
		"cmd", mcpSpec.Command,
		"args", mcpSpec.Args,
	)

	processDone := make(chan struct{}, 1)
	defer close(processDone)
	cmd := exec.CommandContext(ctx, mcpSpec.Command, mcpSpec.Args...)
	cmd.Cancel = sync.OnceValue(func() error {
		// TODO(greedy52) how to do this properly?
		if path.Base(mcpSpec.Command) == "docker" {
			cmd.Process.Signal(syscall.SIGINT)
		} else {
			cmd.Process.Signal(syscall.SIGTERM)
		}
		select {
		case <-processDone:
			sessionCtx.log.DebugContext(s.cfg.ParentCtx, "Process exited gracefully")
			return nil
		case <-time.After(10 * time.Second):
			sessionCtx.log.DebugContext(s.cfg.ParentCtx, "Process did not exit gracefully, killing with SIGKILL")
			return cmd.Process.Kill()
		}
	})
	if err := s.setRunAsLocalUser(ctx, cmd, mcpSpec.RunAsLocalUser); err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) merge responseWriter and requestWriter.
	// Response may come from the server or from internal access check.
	responseWriter := newResponseWriter(sessionCtx)
	go responseWriter.process(ctx)

	// Parse incoming request, then forward or reject.
	requestWriter, out := io.Pipe()
	requestReader := &requestReader{
		sessionCtx: sessionCtx,
		closeCommand: func() {
			responseWriter.toProcess.Close()
			if err := cmd.Cancel(); err != nil {
				sessionCtx.log.ErrorContext(ctx, "Failed to kill process", "error", err)
			}
		},
		toClient: responseWriter.toClient,
		out:      out,
	}
	go requestReader.process(ctx)

	// TODO(greedy52) refactor trace logger to avoid new logger when not
	// necessary.
	cmd.Stdin = requestWriter
	cmd.Stdout = responseWriter.fromServer
	cmd.Stderr = newTraceLogWriter(ctx, sessionCtx.log.With("stdio", "stderr"))

	emitStartEvent(s.cfg.ParentCtx, sessionCtx)
	defer emitEndEvent(s.cfg.ParentCtx, sessionCtx)
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	if err := cmd.Wait(); err != nil {
		processDone <- struct{}{}
		sessionCtx.log.DebugContext(ctx, "Failed to wait for process", "error", err)
	}
	return nil
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
