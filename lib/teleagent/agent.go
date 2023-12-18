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
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
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
	// testPermissions is a test provided function used to test
	// the permissions of the agent server during potentially
	// vulnerable moments in permission changes.
	testPermissions func()
}

// NewServer returns new instance of agent server
func NewServer(getter Getter) *AgentServer {
	return &AgentServer{getAgent: getter}
}

// ListenUnixSocket starts listening on a new unix socket.
func (a *AgentServer) ListenUnixSocket(sockDir, sockName string, user *user.User) error {
	// Create a temp directory to hold the agent socket.
	sockDir, err := os.MkdirTemp(os.TempDir(), sockDir+"-")
	if err != nil {
		return trace.Wrap(err)
	}
	a.Dir = sockDir

	sockPath := filepath.Join(sockDir, sockName)
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		a.Close()
		return trace.Wrap(err)
	}

	a.listener = l
	a.Path = sockPath

	if err := a.updatePermissions(user); err != nil {
		a.Close()
		return trace.Wrap(err)
	}

	return nil
}

// Update the agent server permissions to give the user sole ownership
// of the socket path and prevent other users from accessing or seeing it.
func (a *AgentServer) updatePermissions(user *user.User) error {
	// Tests may provide a testPermissions function to test potentially
	// vulnerable moments during permission updating.
	testPermissions := func() {
		if a.testPermissions != nil {
			a.testPermissions()
		}
	}

	testPermissions()

	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		return trace.Wrap(err)
	}

	testPermissions()

	if err := os.Chmod(a.Path, teleport.FileMaskOwnerOnly); err != nil {
		return trace.ConvertSystemError(err)
	}

	testPermissions()

	if err := os.Lchown(a.Path, uid, gid); err != nil {
		return trace.ConvertSystemError(err)
	}

	testPermissions()

	// To prevent a privilege escalation attack, this must occur
	// after the socket permissions are updated.
	if err := os.Lchown(a.Dir, uid, gid); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// SetTestPermissions can be used by tests to test agent socket permissions.
func (a *AgentServer) SetTestPermissions(testPermissions func()) {
	a.testPermissions = testPermissions
}

// Serve starts serving on the listener, assumes that Listen was called before
func (a *AgentServer) Serve() error {
	if a.listener == nil {
		return trace.BadParameter("Serve needs a Listen call first")
	}
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := a.listener.Accept()
		if err != nil {
			neterr, ok := err.(net.Error)
			if !ok {
				return trace.Wrap(err, "unknown error")
			}
			if utils.IsUseOfClosedNetworkError(neterr) {
				return nil
			}
			if !neterr.Timeout() {
				log.WithError(err).Error("Got non-timeout error.")
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
			log.WithError(err).Errorf("Got timeout error (will sleep %v).", tempDelay)
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0

		// get an agent instance for serving this conn
		instance, err := a.getAgent()
		if err != nil {
			log.WithError(err).Error("Failed to get agent.")
			return trace.Wrap(err)
		}

		// serve agent protocol against conn in a
		// separate goroutine.
		go func() {
			defer instance.Close()
			if err := agent.ServeAgent(instance, conn); err != nil {
				if err != io.EOF {
					log.Error(err)
				}
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
		log.Debugf("AgentServer(%v) is closing", a.listener.Addr())
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
