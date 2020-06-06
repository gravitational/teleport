/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sshutils

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/gravitational/teleport/lib/teleagent"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/trace"
)

// ConnectionContext manages connection-level state.
type ConnectionContext struct {
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

	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging.
	closers []io.Closer

	// closed indicates that closers have been run.
	closed bool

	// cancel cancels the context.Context scope associated with this ConnectionContext.
	cancel context.CancelFunc
}

// NewConnectionContext creates a new ConnectionContext and a child context.Context
// instance which will be canceled when the ConnectionContext is closed.
func NewConnectionContext(ctx context.Context, nconn net.Conn, sconn *ssh.ServerConn) (context.Context, *ConnectionContext) {
	ctx, cancel := context.WithCancel(ctx)
	return ctx, &ConnectionContext{
		NetConn:    nconn,
		ServerConn: sconn,
		env:        make(map[string]string),
		cancel:     cancel,
	}
}

// agentChannel implements the extended teleteleagent.Agent interface,
// allowing the underlying ssh.Channel to be closed when the agent
// is no longer needed.
type agentChannel struct {
	agent.Agent
	ch ssh.Channel
}

func (a *agentChannel) Close() error {
	return a.ch.Close()
}

// StartAgentChannel sets up a new agent forwarding channel against this connection.  The channel
// is automatically closed when either ConnectionContext, or the supplied context.Context
// gets canceled.
func (c *ConnectionContext) StartAgentChannel() (teleagent.Agent, error) {
	// refuse to start an agent if forwardAgent has not yet been set.
	if !c.GetForwardAgent() {
		return nil, trace.AccessDenied("agent forwarding not requested or not authorized")
	}
	// open a agent channel to client
	ch, _, err := c.ServerConn.OpenChannel(AuthAgentRequest, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &agentChannel{
		Agent: agent.NewClient(ch),
		ch:    ch,
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
