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
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client is a client implementation of [agent.ExtendedAgent] that opens
// a new connection for every agent request. This strategy ensures that the system
// agent remains reachable by the client during long requests or agent forwarding
// sessions. Specifically, this is necessary for Windows' named pipe ssh agent
// implementation, as the named pipe connection can be disrupted after signature
// requests. This issue may be resolved directly by the crypto/ssh/agent library
// once https://github.com/golang/go/issues/61383 is addressed.
type Client struct {
	connect connectFn
}

// connectFn is a function to connect to an SSH agent. The returned connection is
// any [io.ReadWriteCloser], such as a [net.Conn] or [ssh.Channel].
type connectFn func() (io.ReadWriteCloser, error)

// NewClient creates a new SSH Agent client. For each agent request, the
// client will use the provided connect function to open a new connection
// to the ssh agent.
func NewClient(connect connectFn) *Client {
	return &Client{
		connect: connect,
	}
}

// List implements [agent.ExtendedAgent.List].
func (c *Client) List() ([]*agent.Key, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	keys, err := agent.NewClient(conn).List()
	return keys, trace.Wrap(err)
}

// List implements [agent.ExtendedAgent.Signers].
func (c *Client) Signers() ([]ssh.Signer, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signers, err := agent.NewClient(conn).Signers()
	return signers, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.Sign].
func (c *Client) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signature, err := agent.NewClient(conn).Sign(key, data)
	return signature, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.SignWithFlags].
// key.
func (c *Client) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signature, err := agent.NewClient(conn).SignWithFlags(key, data, flags)
	return signature, trace.Wrap(err)
}

// Add implements [agent.ExtendedAgent.Add].
func (c *Client) Add(key agent.AddedKey) error {
	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Add(key)
	return trace.Wrap(err)
}

// Remove implements [agent.ExtendedAgent.Remove].
func (c *Client) Remove(key ssh.PublicKey) error {
	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Remove(key)
	return trace.Wrap(err)
}

// RemoveAll implements [agent.ExtendedAgent.RemoveAll].
func (c *Client) RemoveAll() error {
	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).RemoveAll()
	return trace.Wrap(err)
}

// Lock implements [agent.ExtendedAgent.Lock].
func (c *Client) Lock(passphrase []byte) error {
	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Lock(passphrase)
	return trace.Wrap(err)
}

// Unlock implements [agent.ExtendedAgent.Unlock].
func (c *Client) Unlock(passphrase []byte) error {
	conn, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Unlock(passphrase)
	return trace.Wrap(err)
}

// Extension implements [agent.ExtendedAgent.Extension].
func (c *Client) Extension(extensionType string, contents []byte) ([]byte, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	response, err := agent.NewClient(conn).Extension(extensionType, contents)
	return response, trace.Wrap(err)
}
