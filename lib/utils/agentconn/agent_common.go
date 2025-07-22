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

package agentconn

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
)

// SSHAgent is an implementation of [agent.ExtendedAgent].
type SSHAgent struct {
	path string
}

func NewSSHAgent(path string) *SSHAgent {
	return &SSHAgent{
		path: path,
	}
}

func (a *SSHAgent) connect() (agent.ExtendedAgent, func() error, error) {
	conn, err := Dial(a.path)
	if err != nil {
		return nil, nil, err
	}

	logger := slog.With(teleport.ComponentKey, teleport.ComponentKeyAgent)
	logger.InfoContext(context.Background(), "Connected to the system agent", "socket_path", a.path)
	return agent.NewClient(conn), conn.Close, nil
}

// List implements [agent.ExtendedAgent.List].
func (a *SSHAgent) List() ([]*agent.Key, error) {
	agent, release, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	keys, err := agent.List()
	return keys, trace.Wrap(err)
}

// List implements [agent.ExtendedAgent.Signers].
func (a *SSHAgent) Signers() ([]ssh.Signer, error) {
	agent, release, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	signers, err := agent.Signers()
	return signers, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.Sign].
func (a *SSHAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	agent, release, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	signature, err := agent.Sign(key, data)
	return signature, trace.Wrap(err)
}

// SignWithFlags implements [agent.ExtendedAgent.SignWithFlags].
// key.
func (a *SSHAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	agent, release, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	signature, err := agent.SignWithFlags(key, data, flags)
	return signature, trace.Wrap(err)
}

// Add implements [agent.ExtendedAgent.Add].
func (a *SSHAgent) Add(key agent.AddedKey) error {
	agent, release, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	err = agent.Add(key)
	return trace.Wrap(err)
}

// Remove implements [agent.ExtendedAgent.Remove].
func (a *SSHAgent) Remove(key ssh.PublicKey) error {
	agent, release, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	err = agent.Remove(key)
	return trace.Wrap(err)
}

// RemoveAll implements [agent.ExtendedAgent.RemoveAll].
func (a *SSHAgent) RemoveAll() error {
	agent, release, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	err = agent.RemoveAll()
	return trace.Wrap(err)
}

// Lock implements [agent.ExtendedAgent.Lock].
func (a *SSHAgent) Lock(passphrase []byte) error {
	agent, release, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	err = agent.Lock(passphrase)
	return trace.Wrap(err)
}

// Unlock implements [agent.ExtendedAgent.Unlock].
func (a *SSHAgent) Unlock(passphrase []byte) error {
	agent, release, err := a.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	err = agent.Unlock(passphrase)
	return trace.Wrap(err)
}

// Extension implements [agent.ExtendedAgent.Extension].
func (a *SSHAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	agent, release, err := a.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	response, err := agent.Extension(extensionType, contents)
	return response, trace.Wrap(err)
}
