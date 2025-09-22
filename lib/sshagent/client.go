// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sshagent

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// Client extends the [agent.ExtendedAgent] interface with an [io.Closer].
type Client interface {
	agent.ExtendedAgent
	io.Closer
}

// ClientGetter is a function used to get a new agent client.
type ClientGetter = func() (Client, error)

type client struct {
	agent.ExtendedAgent
	conn io.Closer
}

// NewClient creates a new SSH Agent client with an open connection using
// the provided connection function. The resulting connection can be any
// [io.ReadWriteCloser], such as a [net.Conn] or [ssh.Channel].
func NewClient(connect func() (io.ReadWriteCloser, error)) (Client, error) {
	conn, err := connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client{
		ExtendedAgent: agent.NewClient(conn),
		conn:          conn,
	}, nil
}

// NewSystemAgentClient creates a new SSH Agent client with an open connection
// to the system agent, advertised by SSH_AUTH_SOCK or other system parameters.
func NewSystemAgentClient() (Client, error) {
	return NewClient(DialSystemAgent)
}

// NewStaticClient creates a new SSH Agent client for the given static agent.
func NewStaticClient(agentClient agent.ExtendedAgent) Client {
	return &client{
		ExtendedAgent: agentClient,
	}
}

// NewStaticClientGetter returns a [ClientGetter] for a static agent client.
func NewStaticClientGetter(agentClient agent.ExtendedAgent) ClientGetter {
	return func() (Client, error) {
		return &client{
			ExtendedAgent: agentClient,
		}, nil
	}
}

// Close the agent client and prevent further requests.
func (c *client) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	return trace.Wrap(err)
}

const channelType = "auth-agent@openssh.com"

// ServeChannelRequests routes agent channel requests to a new agent
// connection retrieved from the provided getter.
//
// This method differs from [agent.ForwardToAgent] in that each agent
// forwarding channel is handled with a new connection to the forward
// agent, rather than sharing a single long-lived connection.
//
// Specifically, this is necessary for Windows' named pipe ssh agent
// implementation, as the named pipe connection can be disrupted after
// signature requests. This issue may be resolved directly by the
// [agent] library once https://github.com/golang/go/issues/61383
// is addressed.
func ServeChannelRequests(ctx context.Context, client *ssh.Client, getForwardAgent ClientGetter) error {
	channels := client.HandleChannelOpen(channelType)
	if channels == nil {
		return errors.New("agent forwarding channel already open")
	}

	go func() {
		for ch := range channels {
			channel, reqs, err := ch.Accept()
			if err != nil {
				continue
			}

			go ssh.DiscardRequests(reqs)

			forwardAgent, err := getForwardAgent()
			if err != nil {
				_ = channel.Close()
				slog.ErrorContext(ctx, "failed to connect to forwarded agent", "err", err)
				continue
			}

			go func() {
				defer channel.Close()
				defer forwardAgent.Close()
				if err := agent.ServeAgent(forwardAgent, channel); err != nil && !errors.Is(err, io.EOF) {
					slog.ErrorContext(ctx, "unexpected error serving forwarded agent", "err", err)
				}
			}()
		}
	}()
	return nil
}

// RequestAgentForwarding sets up agent forwarding for the session.
// ForwardToAgent or ForwardToRemote should be called to route
// the authentication requests.
func RequestAgentForwarding(ctx context.Context, session *tracessh.Session) error {
	ok, err := session.SendRequest(ctx, "auth-agent-req@openssh.com", true, nil)
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("agent forwarding request denied")
	}
	return nil
}
