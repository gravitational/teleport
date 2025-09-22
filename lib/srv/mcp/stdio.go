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

	hostutils "github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// handleAuthErrStdio starts a stdio message reader and replies with the auth
// error regardless what message the reader receives (though the first message
// is most likely client's initialize request). The reader handler quits right
// after the auth error is delivered to client, by always returning an error to
// the handler callbacks.
func (s *Server) handleAuthErrStdio(ctx context.Context, clientConn net.Conn, authErr error) error {
	logger := s.cfg.Log.With("client_ip", clientConn.RemoteAddr())
	errMsg := mcp.NewJSONRPCError(mcp.NewRequestId(nil), mcp.INTERNAL_ERROR, authErr.Error(), nil)
	writer := mcputils.NewStdioMessageWriter(clientConn)
	reader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport: mcputils.NewStdioReader(clientConn),
		Logger:    logger,
		OnRequest: func(ctx context.Context, req *mcputils.JSONRPCRequest) error {
			// Use request ID when available. Return auth error after writing
			// back to client to stop the reader.
			errMsg.ID = req.ID
			return trace.NewAggregate(writer.WriteMessage(ctx, errMsg), authErr)
		},
		OnParseError: func(ctx context.Context, _ mcp.RequestId, _ error) error {
			return trace.NewAggregate(writer.WriteMessage(ctx, errMsg), authErr)
		},
		OnNotification: func(ctx context.Context, _ *mcputils.JSONRPCNotification) error {
			return trace.NewAggregate(writer.WriteMessage(ctx, errMsg), authErr)
		},
	})
	if err != nil {
		return trace.NewAggregate(authErr, err)
	}
	reader.Run(ctx)

	// Returns the original auth error for caller to log the error and close the
	// connection just to be safe.
	return trace.Wrap(authErr)
}

// handleStdio handles a stdio connection.
// makeMCPServer defaults to makeExecServerRunner to launch a command but can be
// mocked for testing.
func (s *Server) handleStdio(ctx context.Context, sessionCtx *SessionCtx, makeServerRunner makeStdioServerRunnerFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	session, err := s.makeSessionHandler(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	session.logger.InfoContext(ctx, "Started handling stdio session")
	defer session.logger.InfoContext(ctx, "Completed handling stdio session")

	serverRunner, err := makeServerRunner(ctx, session)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverRunner.close()

	readFromServer, err := serverRunner.getStdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	writeToServer, err := serverRunner.getStdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	clientResponseWriter := mcputils.NewSyncStdioMessageWriter(sessionCtx.ClientConn)
	serverRequestWriter := mcputils.NewSyncStdioMessageWriter(writeToServer)

	clientRequestReader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport: mcputils.NewStdioReader(sessionCtx.ClientConn),
		Logger:    session.logger.With("stdio", "stdin"),
		// make sure launched process is getting shut down when client is closed.
		OnClose:        serverRunner.close,
		OnParseError:   mcputils.ReplyParseError(clientResponseWriter),
		OnRequest:      session.onClientRequest(clientResponseWriter, serverRequestWriter),
		OnNotification: session.onClientNotification(serverRequestWriter),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	stdoutLogger := session.logger.With("stdio", "stdout")
	serverResponseReader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport:      mcputils.NewStdioReader(readFromServer),
		Logger:         stdoutLogger,
		OnClose:        serverRunner.close,
		OnParseError:   mcputils.LogAndIgnoreParseError(stdoutLogger),
		OnNotification: session.onServerNotification(clientResponseWriter),
		OnResponse:     session.onServerResponse(clientResponseWriter),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) capture client info then emit start event with client
	// information.
	session.emitStartEvent(s.cfg.ParentContext)
	defer session.emitEndEvent(s.cfg.ParentContext, nil)

	go clientRequestReader.Run(ctx)
	go serverResponseReader.Run(ctx)
	return trace.Wrap(serverRunner.run(ctx))
}

// stdioServerRunner is an interface that represents a stdio-based MCP server to
// be launched. Can be mocked for testing.
type stdioServerRunner interface {
	// getStdoutPipe returns an io.ReadCloser for reading responses from
	// server's stdout.
	getStdoutPipe() (io.ReadCloser, error)
	// getStdinPipe returns an io.Writer for writing messages to server's
	// stdin.
	getStdinPipe() (io.WriteCloser, error)
	// run starts the MCP server and blocks until it is shut down.
	run(context.Context) error
	// close shuts down the MCP server.
	close()
}

type makeStdioServerRunnerFunc func(context.Context, *sessionHandler) (stdioServerRunner, error)

// execServer is the real implementation for stdioServerRunner that launches an
// exec.Command.
type execServer struct {
	cmd     *exec.Cmd
	session *sessionHandler
}

func (s *execServer) getStdoutPipe() (io.ReadCloser, error) {
	return s.cmd.StdoutPipe()
}

func (s *execServer) getStdinPipe() (io.WriteCloser, error) {
	return s.cmd.StdinPipe()
}

func (s *execServer) run(context.Context) error {
	if err := s.cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	err := s.cmd.Wait()
	s.session.logger.DebugContext(s.session.parentCtx, "Command exited",
		"error", err,
		"exit", s.cmd.ProcessState.ExitCode(),
	)
	if err != nil && !isOKExitError(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *execServer) close() {
	if err := s.cmd.Cancel(); err != nil && !isOKExitError(err) {
		s.session.logger.WarnContext(s.session.parentCtx, "Failed to cancel command", "error", err)
	}
}

func makeExecServerRunner(ctx context.Context, session *sessionHandler) (stdioServerRunner, error) {
	mcpSpec := session.App.GetMCP()
	if mcpSpec == nil {
		return nil, trace.BadParameter("missing MCP spec")
	}

	logger := session.logger
	logger.DebugContext(ctx, "Preparing command to execute",
		"cmd", mcpSpec.Command,
		"args", mcpSpec.Args,
	)

	cmdCtx, cmdCancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, mcpSpec.Command, mcpSpec.Args...)
	cmd.Stderr = mcputils.NewStderrLogWriter(ctx, logger, slog.LevelDebug)
	// WaitDelay forces a SIGKILL if the process fails to exit 10 seconds after
	// cmd.Cancel is called. See the WaitDelay doc for details.
	cmd.WaitDelay = 10 * time.Second
	// We put all shutdown procedures in cmd.Cancel because we are too lazy to
	// make a separate function. Since cmd.Cancel can be called outside here by
	// the server handler, we make sure 'cmdCancel' is called to cancel the
	// command in that case.
	cmd.Cancel = sync.OnceValue(func() error {
		cmdCancel()

		if cmd.Process != nil {
			// Use SIGINT for graceful shutdown since stdio servers are
			// "interactive".
			logger.DebugContext(ctx, "Sending SIGINT to command")
			return trace.Wrap(cmd.Process.Signal(syscall.SIGINT))
		}
		return nil
	})

	// Set host user.
	hostUser, err := user.Lookup(mcpSpec.RunAsHostUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := hostutils.MaybeSetCommandCredentialAsUser(ctx, cmd, hostUser, logger); err != nil {
		return nil, trace.Wrap(err)
	}

	mcpServer := &execServer{
		cmd:     cmd,
		session: session,
	}
	return mcpServer, nil
}

func isExitErrorSignal(exitErr error, signal syscall.Signal) bool {
	var execExitError *exec.ExitError
	if !errors.As(exitErr, &execExitError) {
		return false
	}
	waitStatus, ok := execExitError.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	return waitStatus.Signaled() && waitStatus.Signal() == signal
}

func isOKExitError(exitError error) bool {
	return errors.Is(exitError, context.Canceled) ||
		errors.Is(exitError, os.ErrProcessDone) ||
		// it's fine if the command is gracefully stopped by us.
		isExitErrorSignal(exitError, syscall.SIGINT)
}
