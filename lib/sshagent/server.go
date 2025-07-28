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
	"os/user"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Server is an SSH agent server implementation.
type Server struct {
	getAgent ClientGetter
	listener net.Listener
	Path     string
	Dir      string
}

// NewServer returns a new [Server].
func NewServer(agentClient ClientGetter) *Server {
	return &Server{getAgent: agentClient}
}

func (a *Server) SetListener(l net.Listener) {
	a.listener = l
	a.Path = l.Addr().String()
	a.Dir = filepath.Dir(a.Path)
}

// ListenUnixSocket starts listening on a new unix socket.
func (a *Server) ListenUnixSocket(sockDir, sockName string, _ *user.User) error {
	// Create a temp directory to hold the agent socket.
	sockDir, err := os.MkdirTemp(os.TempDir(), sockDir+"-")
	if err != nil {
		return trace.Wrap(err)
	}

	sockPath := filepath.Join(sockDir, sockName)
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		a.Close()
		return trace.Wrap(err)
	}

	a.SetListener(l)
	return nil
}

// Serve starts serving on the listener, assumes that Listen was called before
func (a *Server) Serve() error {
	if a.listener == nil {
		return trace.BadParameter("Serve needs a Listen call first")
	}

	ctx := context.Background()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := a.listener.Accept()
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

		// get an agent instance for serving this conn
		instance, err := a.getAgent()
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get agent", "error", err)
			return trace.Wrap(err)
		}

		// serve agent protocol against conn in a
		// separate goroutine.
		go func() {
			defer instance.Close()
			if err := agent.ServeAgent(instance, conn); err != nil && !errors.Is(err, io.EOF) {
				slog.ErrorContext(ctx, "Serving agent terminated unexpectedly", "error", err)
			}
		}()
	}
}

// Close closes listener and stops serving agent
func (a *Server) Close() error {
	var errors []error
	if a.listener != nil {
		slog.DebugContext(context.Background(), "AgentServer is closing",
			"listen_addr", logutils.StringerAttr(a.listener.Addr()),
		)
		if err := a.listener.Close(); err != nil {
			errors = append(errors, trace.ConvertSystemError(err))
		}
	}
	if a.Path != "" {
		if err := os.Remove(a.Path); err != nil {
			errors = append(errors, trace.ConvertSystemError(err))
		}
	}
	if a.Dir != "" {
		if err := os.RemoveAll(a.Dir); err != nil {
			errors = append(errors, trace.ConvertSystemError(err))
		}
	}
	return trace.NewAggregate(errors...)
}
