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
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/net/context"
)

const channelType = "auth-agent@openssh.com"

// ServeChannelRequests routes agent channel requests to a new agent
// connection retrieved from the provided getter.
func ServeChannelRequests(ctx context.Context, client *ssh.Client, getter Getter) error {
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

			forwardAgent, err := getter()
			if err != nil {
				slog.ErrorContext(context.Background(), "failed to connect to agent for forwarding", "err", err)
				continue
			}

			go func() {
				defer channel.Close()
				if err := agent.ServeAgent(forwardAgent, channel); err != nil && !errors.Is(err, io.EOF) {
					slog.ErrorContext(context.Background(), "unexpected error serving forwarded agent", "err", err)
				}
			}()
		}
	}()
	return nil
}

// Client is a client implementation of [agent.ExtendedAgent] that handles reconnects
// to gracefully handle connection and ssh agent service disruptions.
//
// This is specifically needed for Window's named pipe ssh agent implementation as the named pipe
// connection can be disrupted after signature requests. This issue may be resolved directly by
// the crypto/ssh/agent library once https://github.com/golang/go/issues/61383 is addressed.
type Client struct {
	connect connectFn
	conn    io.ReadWriteCloser
	connMu  sync.Mutex
	closed  bool
}

// connectFn is a function to connect to an SSH agent. The returned connection is
// any [io.ReadWriteCloser], such as a [net.Conn] or [ssh.Channel].
type connectFn func() (io.ReadWriteCloser, error)

// NewClient creates a new SSH Agent client with an open connection.
func NewClient(connect connectFn) (*Client, error) {
	conn, err := connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		connect: connect,
		conn:    conn,
	}, nil
}

func (c *Client) withReconnectRetry(do func(a agent.ExtendedAgent) error) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.closed {
		return trace.Errorf("the ssh agent client is closed")
	}

	if c.conn != nil {
		err := do(agent.NewClient(c.conn))
		if isClosedConnectionError(err) {
			// unset conn and reconnect below.
			c.conn = nil
		} else {
			return err
		}
	}

	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err, "failed to connect to the ssh agent service")
	}
	c.conn = conn

	return do(agent.NewClient(conn))
}

// Close the agent client and prevent further requests.
func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return nil
	}

	c.closed = true
	return trace.Wrap(c.conn.Close())
}

// List implements [agent.ExtendedAgent.List].
func (c *Client) List() (keys []*agent.Key, err error) {
	err = c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		keys, err = a.List()
		return trace.Wrap(err)
	})
	return keys, trace.Wrap(err)
}

// List implements [agent.ExtendedAgent.Signers].
func (c *Client) Signers() (signers []ssh.Signer, err error) {
	err = c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		signers, err = a.Signers()
		return trace.Wrap(err)
	})
	return signers, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.Sign].
func (c *Client) Sign(key ssh.PublicKey, data []byte) (signature *ssh.Signature, err error) {
	err = c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		signature, err = a.Sign(key, data)
		return trace.Wrap(err)
	})
	return signature, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.SignWithFlags].
// key.
func (c *Client) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (signature *ssh.Signature, err error) {
	err = c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		signature, err = a.SignWithFlags(key, data, flags)
		return trace.Wrap(err)
	})
	return signature, trace.Wrap(err)
}

// Add implements [agent.ExtendedAgent.Add].
func (c *Client) Add(key agent.AddedKey) error {
	return c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		return trace.Wrap(a.Add(key))
	})
}

// Remove implements [agent.ExtendedAgent.Remove].
func (c *Client) Remove(key ssh.PublicKey) error {
	return c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		return trace.Wrap(a.Remove(key))
	})
}

// RemoveAll implements [agent.ExtendedAgent.RemoveAll].
func (c *Client) RemoveAll() error {
	return c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		return trace.Wrap(a.RemoveAll())
	})
}

// Lock implements [agent.ExtendedAgent.Lock].
func (c *Client) Lock(passphrase []byte) error {
	return c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		return trace.Wrap(a.Lock(passphrase))
	})
}

// Unlock implements [agent.ExtendedAgent.Unlock].
func (c *Client) Unlock(passphrase []byte) error {
	return c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		return trace.Wrap(a.Unlock(passphrase))
	})
}

// Extension implements [agent.ExtendedAgent.Extension].
func (c *Client) Extension(extensionType string, contents []byte) (response []byte, err error) {
	err = c.withReconnectRetry(func(a agent.ExtendedAgent) error {
		response, err = a.Extension(extensionType, contents)
		return trace.Wrap(err)
	})
	return response, trace.Wrap(err)
}
