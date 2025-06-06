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
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/lib/utils"
	hostutils "github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

func (s *Server) handleAuthErrStdio(ctx context.Context, clientConn net.Conn, authErr error) error {
	logger := s.cfg.Log.With("client_ip", clientConn.RemoteAddr())
	errMsg := mcp.NewJSONRPCError(mcp.NewRequestId(nil), mcp.INTERNAL_ERROR, authErr.Error(), nil)
	writer := mcputils.NewStdioMessageWriter(clientConn)
	reader, err := mcputils.NewStdioMessageReader(mcputils.StdioMessageReaderConfig{
		SourceReadCloser: clientConn,
		Logger:           logger,
		ParentContext:    s.cfg.ParentContext,
		OnParseError: func(ctx context.Context, _ *mcp.JSONRPCError) error {
			return trace.Wrap(writer.WriteMessage(ctx, errMsg))
		},
		OnRequest: func(ctx context.Context, req *mcputils.JSONRPCRequest) error {
			errMsg.ID = req.ID
			return trace.Wrap(writer.WriteMessage(ctx, errMsg))
		},
		OnNotification: func(ctx context.Context, _ *mcputils.JSONRPCNotification) error {
			return trace.Wrap(writer.WriteMessage(ctx, errMsg))
		},
	})
	if err != nil {
		return trace.NewAggregate(authErr, err)
	}
	reader.Run(ctx)

	// Returns the original auth error for caller to log the error and close the
	// connection.
	return trace.Wrap(authErr)
}

func (s *Server) handleStdio(ctx context.Context, sessionCtx SessionCtx) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mcpSpec := sessionCtx.App.GetMCP()
	if mcpSpec == nil {
		return trace.BadParameter("missing MCP spec")
	}

	session, err := s.newSessionHandler(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	logger := session.logger
	logger.DebugContext(ctx, "Running mcp",
		"cmd", mcpSpec.Command,
		"args", mcpSpec.Args,
	)

	serverStdio, writeToServer := io.Pipe()
	readFromServer, serverStdout := io.Pipe()

	cmd := exec.CommandContext(ctx, mcpSpec.Command, mcpSpec.Args...)
	cmd.Stdout = serverStdout
	cmd.Stdin = serverStdio
	// WaitDelay forces a SIGKILL if process hasn't exited 10 seconds after
	// cmd.Cancel is called.
	cmd.WaitDelay = 10 * time.Second
	cmd.Cancel = sync.OnceValue(func() error {
		logger.DebugContext(ctx, "Sending SIGINT to command")
		return trace.Wrap(cmd.Process.Signal(syscall.SIGINT))
	})

	if err := setRunAsHostUser(ctx, cmd, mcpSpec.RunAsHostUser, logger); err != nil {
		return trace.Wrap(err)
	}

	clientResponseWriter := mcputils.NewStdioMessageWriter(utils.NewSyncWriter(session.ClientConn))
	serverRequestWriter := mcputils.NewStdioMessageWriter(utils.NewSyncWriter(writeToServer))

	clientRequestReader, err := mcputils.NewStdioMessageReader(mcputils.StdioMessageReaderConfig{
		SourceReadCloser: session.ClientConn,
		Logger:           logger.With("stdio", "stdin"),
		ParentContext:    s.cfg.ParentContext,
		OnClose: func() {
			// Close all pipes and trigger a shutdown.
			if closePipeError := trace.NewAggregate(
				serverStdio.Close(), writeToServer.Close(),
				readFromServer.Close(), serverStdout.Close(),
			); err != nil {
				logger.DebugContext(ctx, "Failed to close pipes", "error", closePipeError)
			}
			if cancelErr := cmd.Cancel(); cancelErr != nil {
				logger.DebugContext(ctx, "Failed to cancel command", "error", cancelErr)
			}
		},
		OnParseError: mcputils.ReplyParseError(clientResponseWriter),
		OnRequest: func(ctx context.Context, req *mcputils.JSONRPCRequest) error {
			msg, replyDirection := session.processClientRequest(ctx, req)
			if replyDirection == replyToClient {
				return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msg))
			}
			return trace.Wrap(serverRequestWriter.WriteMessage(ctx, msg))
		},
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			session.processClientNotification(ctx, notification)
			return trace.Wrap(serverRequestWriter.WriteMessage(ctx, notification))
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	stdoutLogger := logger.With("stdio", "stdout")
	serverResponseReader, err := mcputils.NewStdioMessageReader(mcputils.StdioMessageReaderConfig{
		SourceReadCloser: readFromServer,
		Logger:           stdoutLogger,
		ParentContext:    s.cfg.ParentContext,
		OnParseError:     mcputils.LogAndIgnoreParseError(stdoutLogger),
		OnNotification: func(ctx context.Context, notification *mcputils.JSONRPCNotification) error {
			return trace.Wrap(clientResponseWriter.WriteMessage(ctx, notification))
		},
		OnResponse: func(ctx context.Context, response *mcputils.JSONRPCResponse) error {
			msgToClient := session.processServerResponse(ctx, response)
			return trace.Wrap(clientResponseWriter.WriteMessage(ctx, msgToClient))
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go clientRequestReader.Run(ctx)
	go serverResponseReader.Run(ctx)

	session.emitStartEvent(s.cfg.ParentContext, &sessionCtx)
	defer session.emitEndEvent(s.cfg.ParentContext, &sessionCtx)

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	waitErr := cmd.Wait()
	logger.DebugContext(ctx, "Command exited", "error", waitErr, "exit", cmd.ProcessState.ExitCode())
	return nil
}

func setRunAsHostUser(ctx context.Context, cmd *exec.Cmd, localUserName string, logger *slog.Logger) error {
	localUser, err := user.Lookup(localUserName)
	if err != nil {
		return trace.Wrap(err, "finding local user")
	}
	cred, err := hostutils.GetHostUserCredential(localUser)
	if err != nil {
		return trace.Wrap(err, "getting local user credential")
	}

	if os.Getuid() == int(cred.Uid) || os.Getgid() == int(cred.Gid) {
		logger.DebugContext(ctx, "Launching process with ambient credentials")
		return nil
	}

	logger.DebugContext(ctx, "Launching process as local user", "user", localUserName, "uid", cred.Uid, "gid", cred.Gid)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = cred
	return nil
}
