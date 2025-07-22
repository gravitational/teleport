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
	"log/slog"
	"net"
	"os"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
)

// Client is a client implementation of [agent.ExtendedAgent] that opens
// a new connection for every agent request. This strategy ensures that the system
// agent remains reachable by the client during long requests or agent forwarding
// sessions. Specifically, this is necessary for Windows' named pipe ssh agent
// implementation, as the named pipe connection can be disrupted after signature
// requests. This issue may be resolved directly by the crypto/ssh/agent library
// once https://github.com/golang/go/issues/61383 is addressed.
type Client struct {
	path string
}

// NewClient creates a new SSH Agent client.
func NewClient() *Client {
	socketPath := os.Getenv(teleport.SSHAuthSock)
	return &Client{
		path: socketPath,
	}
}

func (a *Client) connect() (net.Conn, error) {
	logger := slog.With(teleport.ComponentKey, teleport.ComponentKeyAgent)
	logger.DebugContext(context.Background(), "Connecting to the system agent", "socket_path", a.path)
	return Dial(a.path)
}

// List implements [agent.ExtendedAgent.List].
func (a *Client) List() ([]*agent.Key, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	keys, err := agent.NewClient(conn).List()
	return keys, trace.Wrap(err)
}

// List implements [agent.ExtendedAgent.Signers].
func (a *Client) Signers() ([]ssh.Signer, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signers, err := agent.NewClient(conn).Signers()
	return signers, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.Sign].
func (a *Client) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signature, err := agent.NewClient(conn).Sign(key, data)
	return signature, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.SignWithFlags].
// key.
func (a *Client) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	signature, err := agent.NewClient(conn).SignWithFlags(key, data, flags)
	return signature, trace.Wrap(err)
}

// Add implements [agent.ExtendedAgent.Add].
func (a *Client) Add(key agent.AddedKey) error {
	conn, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Add(key)
	return trace.Wrap(err)
}

// Remove implements [agent.ExtendedAgent.Remove].
func (a *Client) Remove(key ssh.PublicKey) error {
	conn, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Remove(key)
	return trace.Wrap(err)
}

// RemoveAll implements [agent.ExtendedAgent.RemoveAll].
func (a *Client) RemoveAll() error {
	conn, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).RemoveAll()
	return trace.Wrap(err)
}

// Lock implements [agent.ExtendedAgent.Lock].
func (a *Client) Lock(passphrase []byte) error {
	conn, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Lock(passphrase)
	return trace.Wrap(err)
}

// Unlock implements [agent.ExtendedAgent.Unlock].
func (a *Client) Unlock(passphrase []byte) error {
	conn, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	err = agent.NewClient(conn).Unlock(passphrase)
	return trace.Wrap(err)
}

// Extension implements [agent.ExtendedAgent.Extension].
func (a *Client) Extension(extensionType string, contents []byte) ([]byte, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	response, err := agent.NewClient(conn).Extension(extensionType, contents)
	return response, trace.Wrap(err)
}
