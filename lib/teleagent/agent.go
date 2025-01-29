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

package teleagent

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

// Agent extends the agent.ExtendedAgent interface.
// APIs which accept this interface promise to
// call `Close()` when they are done using the
// supplied agent.
type Agent interface {
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
func NopCloser(std agent.ExtendedAgent) Agent {
	return nopCloser{std}
}

// Getter is a function used to get an agent instance.
type Getter func() (Agent, error)

// AgentServer is implementation of SSH agent server
type AgentServer struct {
	getAgent Getter
	listener net.Listener
	Path     string
	Dir      string
}

// NewServer returns new instance of agent server
func NewServer(getter Getter) *AgentServer {
	return &AgentServer{getAgent: getter}
}

func (a *AgentServer) SetListener(l net.Listener) {
	a.listener = l
	a.Path = l.Addr().String()
	a.Dir = filepath.Dir(a.Path)
}

// ListenUnixSocket starts listening on a new unix socket.
func (a *AgentServer) ListenUnixSocket(sockDir, sockName string, _ *user.User) error {
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
func (a *AgentServer) Serve() error {
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

// ListenAndServe is similar http.ListenAndServe
func (a *AgentServer) ListenAndServe(addr utils.NetAddr) error {
	l, err := net.Listen(addr.AddrNetwork, addr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	a.listener = l
	return a.Serve()
}

// Close closes listener and stops serving agent
func (a *AgentServer) Close() error {
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
