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

package sshutils

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils/networking"
	"github.com/gravitational/teleport/lib/teleagent"
)

// ConnectionContext manages connection-level state.
type ConnectionContext struct {
	// sessionID is the Teleport session ID that all child ServerContexts will inherit.
	sessionID rsession.ID

	// NetConn is the base connection object.
	NetConn net.Conn

	// ServerConn is authenticated ssh connection.
	ServerConn *ssh.ServerConn

	// mu protects the rest of the state
	mu sync.RWMutex

	// env holds environment variables which should be
	// set for all channels.
	env map[string]string

	// forwardAgent indicates that agent forwarding has
	// been requested for this connection.
	forwardAgent bool

	// sessions is the number of currently active session channels; only tracked
	// when handling node-side connections for users with MaxSessions applied.
	sessions int64

	// networkingProcess is a lazily initialized connection to the subprocess that
	// handles connection-level networking requests. e.g. port/agent forwarding.
	networkingProcess *networking.Process

	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging.
	closers []io.Closer

	// closed indicates that closers have been run.
	closed bool

	// cancel cancels the context.Context scope associated with this ConnectionContext.
	cancel context.CancelFunc

	// clientLastActive records the last time there was activity from the client.
	clientLastActive time.Time

	// UserCreatedByTeleport is true when the system user was created by Teleport user auto-provision.
	UserCreatedByTeleport bool

	clock clockwork.Clock
}

type ConnectionContextOption func(c *ConnectionContext)

// SetConnectionContextClock sets the connection context's internal clock.
func SetConnectionContextClock(clock clockwork.Clock) ConnectionContextOption {
	return func(c *ConnectionContext) {
		c.clock = clock
	}
}

// NewConnectionContext creates a new ConnectionContext and a child context.Context
// instance which will be canceled when the ConnectionContext is closed.
func NewConnectionContext(ctx context.Context, nconn net.Conn, sconn *ssh.ServerConn, opts ...ConnectionContextOption) (context.Context, *ConnectionContext) {
	ctx, cancel := context.WithCancel(ctx)
	ccx := &ConnectionContext{
		sessionID:  rsession.NewID(),
		NetConn:    nconn,
		ServerConn: sconn,
		env:        make(map[string]string),
		cancel:     cancel,
		clock:      clockwork.NewRealClock(),
	}

	for _, opt := range opts {
		opt(ccx)
	}

	return ctx, ccx
}

// agentChannel implements the extended teleteleagent.Agent interface,
// allowing the underlying ssh.Channel to be closed when the agent
// is no longer needed.
type agentChannel struct {
	agent.ExtendedAgent
	ch ssh.Channel
}

// Close closes the agent channel.
func (a *agentChannel) Close() error {
	// For graceful teardown, close the write part of the channel first. This
	// will send "EOF" packet (type 96) to the other side which will drain and
	// close the channel.
	//
	// The regular close after that will send "close" packet (type 97) which
	// won't attempt to send us any more data since the channel is already
	// closed.
	//
	// This mimics vanilla OpenSSH behavior. Without close_write first, the
	// agent client may be getting warnings like the following in stdout:
	//
	// channel 1: chan_shutdown_read: shutdown() failed for fd 8 [i0 o1]: Not a socket
	return trace.NewAggregate(
		a.ch.CloseWrite(),
		a.ch.Close())
}

// GetSessionID returns the Teleport session ID that all child ServerContexts will inherit.
func (c *ConnectionContext) GetSessionID() rsession.ID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sessionID
}

// SetSessionID sets the Teleport session ID that all child ServerContexts will inherit.
func (c *ConnectionContext) SetSessionID(sessionID rsession.ID) {
	c.mu.Lock()
	c.sessionID = sessionID
	c.mu.Unlock()
}

// StartAgentChannel sets up a new agent forwarding channel against this connection.  The channel
// is automatically closed when either ConnectionContext, or the supplied context.Context
// gets canceled.
func (c *ConnectionContext) StartAgentChannel() (teleagent.Agent, error) {
	// refuse to start an agent if forwardAgent has not yet been set.
	if !c.GetForwardAgent() {
		return nil, trace.AccessDenied("agent forwarding has not been requested")
	}
	// open a agent channel to client
	ch, reqC, err := c.ServerConn.OpenChannel(AuthAgentRequest, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go ssh.DiscardRequests(reqC)
	return &agentChannel{
		ExtendedAgent: agent.NewClient(ch),
		ch:            ch,
	}, nil
}

// VisitEnv grants visitor-style access to env variables.
func (c *ConnectionContext) VisitEnv(visit func(key, val string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, val := range c.env {
		visit(key, val)
	}
}

// SetEnv sets a environment variable within this context.
func (c *ConnectionContext) SetEnv(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.env[key] = val
}

// GetEnv returns a environment variable within this context.
func (c *ConnectionContext) GetEnv(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.env[key]
	return val, ok
}

// SetForwardAgent configures this context to support agent forwarding.
// Must not be set until agent forwarding is explicitly requested.
func (c *ConnectionContext) SetForwardAgent(forwardAgent bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forwardAgent = forwardAgent
}

// GetForwardAgent loads the forwardAgent flag with lock.
func (c *ConnectionContext) GetForwardAgent() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.forwardAgent
}

// TryIncrSessions tries to increment the active session count; if ok the
// returned decr function *must* be called when the associated session is closed.
func (c *ConnectionContext) IncrSessions(max int64) (decr func(), ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessions >= max {
		return func() {}, false
	}
	c.sessions++
	var decrOnce sync.Once
	return func() {
		decrOnce.Do(c.decrSessions)
	}, true
}

func (c *ConnectionContext) decrSessions() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions--
	if c.sessions < 0 {
		panic("underflow")
	}
}

// GetClientLastActive returns time when client was last active.
func (c *ConnectionContext) GetClientLastActive() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientLastActive
}

// UpdateClientActivity sets last recorded client activity associated with this context.
func (c *ConnectionContext) UpdateClientActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientLastActive = c.clock.Now().UTC()
}

// AddCloser adds any closer in ctx that will be called
// when the underlying connection is closed.
func (c *ConnectionContext) AddCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// if context was already closed, run the closer immediately
	// in the background.
	if c.closed {
		go closer.Close()
		return
	}
	c.closers = append(c.closers, closer)
}

// SetNetworkingProcess attempts to registers a networking process. If a
// different process was concurrently registered, ok is false and the previously
// registered process is returned.
func (c *ConnectionContext) SetNetworkingProcess(proc *networking.Process) (*networking.Process, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.networkingProcess != nil {
		return c.networkingProcess, false
	}
	c.networkingProcess = proc
	return proc, true
}

// GetNetworkingProcess gets the registered networking process if one exists.
func (c *ConnectionContext) GetNetworkingProcess() (*networking.Process, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.networkingProcess, c.networkingProcess != nil
}

// takeClosers returns all resources that should be closed and sets the properties to null
// we do this to avoid calling Close() under lock to avoid potential deadlocks
func (c *ConnectionContext) takeClosers() []io.Closer {
	// this is done to avoid any operation holding the lock for too long
	c.mu.Lock()
	defer c.mu.Unlock()

	closers := c.closers
	c.closers = nil
	c.closed = true

	return closers
}

// Close closes associated resources (e.g. agent channel).
func (c *ConnectionContext) Close() error {
	var errs []error

	c.cancel()

	closers := c.takeClosers()

	for _, cl := range closers {
		if cl == nil {
			continue
		}

		err := cl.Close()
		if err == nil {
			continue
		}

		errs = append(errs, err)
	}

	return trace.NewAggregate(errs...)
}
