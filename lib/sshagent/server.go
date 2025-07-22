/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package sshagent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/utils"
)

// AgentCloser extends the [agent.ExtendedAgent] interface with the
// `Close()` method. APIs which accept this interface promise to
// call `Close()` when they are done using the supplied agent.
type AgentCloser interface {
	agent.ExtendedAgent
	io.Closer
}

// nopCloser wraps an agent.Agent in the extended
// Agent interface by adding a NOP closer.
type nopCloser struct {
	agent.ExtendedAgent
}

func (n nopCloser) Close() error { return nil }

// NopCloser wraps an agent.Agent with a NOP closer, allowing it
// to be passed to APIs which expect the extended agent interface.
func NopCloser(std agent.ExtendedAgent) AgentCloser {
	return nopCloser{std}
}

// Getter is a function used to get an agent instance.
type Getter func() (AgentCloser, error)

// Server is implementation of SSH agent server
type Server struct {
	agent    agent.ExtendedAgent
	listener net.Listener
}

// NewServer returns a new ssh agent server.
func NewServer(agent agent.ExtendedAgent, listener net.Listener) (*Server, error) {
	return &Server{
		agent:    agent,
		listener: listener,
	}, nil
}

// Serve starts serving on the listener, assumes that Listen was called before
func (s *Server) Serve() error {
	ctx := context.Background()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			var neterr net.Error
			if !errors.As(err, &neterr) {
				return trace.Wrap(err, "unknown error")
			}
			if utils.IsUseOfClosedNetworkError(neterr) {
				return nil
			}
			if !neterr.Timeout() {
				slog.ErrorContext(ctx, "Got non-timeout error", "error", err)
				return trace.Wrap(err)
			}
			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 1 * time.Second; tempDelay > max {
				tempDelay = max
			}
			slog.ErrorContext(ctx, "Got timeout error - backing off", "delay_time", tempDelay, "error", err)
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		// serve agent protocol against conn in a
		// separate goroutine.
		go func() {
			if err := agent.ServeAgent(s.agent, conn); err != nil && !errors.Is(err, io.EOF) {
				slog.ErrorContext(ctx, "Serving agent terminated unexpectedly", "error", err)
			}
		}()
	}
}

// Close closes listener and stops serving agent
func (s *Server) Close() error {
	slog.DebugContext(context.Background(), "AgentServer is closing", "listen_addr", s.Addr())
	return s.listener.Close()
}

// Addr returns the ssh agent server listener's network address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// UnixListener is an ssh agent server unix listener. It is a thin wrapper
// around [net.UnixListener] with additional cleanup logic to ensure the
// temporary ssh agent dir and socket are removed.
type UnixListener struct {
	*net.UnixListener
}

// NewUnixListener creates a new teleport ssh agent listener.
func NewUnixListener() (*UnixListener, error) {
	// Create a temp directory to hold the agent socket.
	sockDir, err := os.MkdirTemp("", "teleport-")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sockPath := filepath.Join(sockDir, "agent.sock")
	sockAddr, err := net.ResolveUnixAddr("unix", sockPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l, err := net.ListenUnix("unix", sockAddr)
	if err != nil {
		os.RemoveAll(sockDir)
		return nil, trace.Wrap(err)
	}

	return &UnixListener{l}, nil
}

// Dir returns the unix socket path.
func (l *UnixListener) Path() string {
	return l.UnixListener.Addr().String()
}

// Dir returns the unix socket's temporary directory.
func (l *UnixListener) Dir() string {
	return filepath.Dir(l.UnixListener.Addr().String())
}

// Close the unix socket and fully remove the unix socket file and directory.
func (l *UnixListener) Close() error {
	var errors []error
	if err := l.UnixListener.Close(); err != nil {
		errors = append(errors, trace.ConvertSystemError(err))
	}
	// Ensure the listener file and directory is fully removed.
	if err := os.Remove(l.Path()); err != nil {
		errors = append(errors, trace.ConvertSystemError(err))
	}
	if err := os.RemoveAll(l.Dir()); err != nil {
		errors = append(errors, trace.ConvertSystemError(err))
	}
	return trace.NewAggregate(errors...)
}
