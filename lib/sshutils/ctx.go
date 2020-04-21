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
	"io"
	"net"
	"sync"

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

	// agent is a client to remote SSH agent.
	agent agent.Agent

	// agentCh is SSH channel using SSH agent protocol.
	agentChannel ssh.Channel
	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging.
	closers []io.Closer
}

// NewConnectionContext creates a new ConnectionContext instance.
func NewConnectionContext(nconn net.Conn, sconn *ssh.ServerConn) *ConnectionContext {
	return &ConnectionContext{
		NetConn:    nconn,
		ServerConn: sconn,
		env:        make(map[string]string),
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

// ExportEnv writes all env vars to supplied map (used to configure
// env of child contexts).
func (c *ConnectionContext) ExportEnv(m map[string]string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for key, val := range c.env {
		m[key] = val
	}
}

// GetAgent returns a agent.Agent which represents the capabilities of an SSH agent,
// or nil if no agent is available in this context.
func (c *ConnectionContext) GetAgent() agent.Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agent
}

// GetAgentChannel returns the channel over which communication with the agent occurs,
// or nil if no agent is available in this context.
func (c *ConnectionContext) GetAgentChannel() ssh.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentChannel
}

// SetAgent sets the agent and channel over which communication with the agent occurs.
func (c *ConnectionContext) SetAgent(a agent.Agent, channel ssh.Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.agentChannel != nil {
		c.agentChannel.Close()
	}
	c.agentChannel = channel
	c.agent = a
}

// AddCloser adds any closer in ctx that will be called
// when the underlying connection is closed.
func (c *ConnectionContext) AddCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	if c.agentChannel != nil {
		closers = append(closers, c.agentChannel)
		c.agentChannel = nil
	}
	return closers
}

// Close closes associated resources (e.g. agent channel).
func (c *ConnectionContext) Close() error {
	var errs []error

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
