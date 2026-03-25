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

package vnet

import (
	"context"
	"crypto/rand"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/utils/sshutils"
)

// sshAgent implements [agent.ExtendedAgent]. The sole purpose is to forward
// the user's Teleport SSH key to the proxy in case the cluster is in proxy
// recording mode. In this case there will be an SSH connection between VNet
// and the root cluster proxy terminated with the SSH key in the
// [ssh.ClientConfig], and then the key forwarded via this agent will be used
// to terminate the final SSH connection to the target node.
type sshAgent struct {
	mu     sync.Mutex
	signer ssh.Signer
}

func newSSHAgent() *sshAgent {
	return &sshAgent{}
}

// setSessionKey must be called at most once, before the agent will be used.
// It's not possible to initialize sshAgent with the SSH signer because the
// agent must be passed to [proxy.Client.DialHost] before the session SSH
// signer has been created.
func (a *sshAgent) setSessionKey(signer ssh.Signer) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.signer != nil {
		return trace.Errorf("sshAgent.setSessionKey must be called at most once (this is a bug)")
	}
	a.signer = signer
	return nil
}

// List implements [agent.ExtendedAgent.List], it returns a single key if it
// has been set by setSessionKey.
func (a *sshAgent) List() ([]*agent.Key, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.signer == nil {
		return nil, nil
	}
	pub := a.signer.PublicKey()
	return []*agent.Key{{
		Format: pub.Type(),
		Blob:   pub.Marshal(),
	}}, nil
}

// List implements [agent.ExtendedAgent.Signers], it returns a single key if it
// has been set by setSessionKey.
func (a *sshAgent) Signers() ([]ssh.Signer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.signer == nil {
		return nil, nil
	}
	return []ssh.Signer{a.signer}, nil
}

// SignWithFlags implements [agent.ExtendedAgent.Sign], it returns an SSH
// signature with a.signer if it has been set and matches the requested key.
func (a *sshAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

// SignWithFlags implements [agent.ExtendedAgent.SignWithFlags], it returns an
// SSH signature with a.signer if it has been set and matches the requested
// key.
func (a *sshAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.signer == nil {
		return nil, trace.Errorf("VNet SSH agent has no signer")
	}
	if !sshutils.KeysEqual(a.signer.PublicKey(), key) {
		return nil, trace.BadParameter("requested key does not equal VNet SSH agent key")
	}
	var algo string
	switch flags {
	case 0:
	case agent.SignatureFlagRsaSha256:
		algo = ssh.KeyAlgoRSASHA256
	case agent.SignatureFlagRsaSha512:
		algo = ssh.KeyAlgoRSASHA512
	default:
		return nil, trace.Errorf("unsupported signature flag %v", flags)
	}
	log.DebugContext(context.Background(), "VNet SSH agent signature requested",
		"key_type", a.signer.PublicKey().Type(), "algo", algo)
	if algo == "" {
		sig, err := a.signer.Sign(rand.Reader, data)
		return sig, trace.Wrap(err)
	}
	algorithmSigner, ok := a.signer.(ssh.AlgorithmSigner)
	if !ok {
		return nil, trace.Errorf("VNet SSH agent signer does not implement ssh.AlgorithmSigner")
	}
	sig, err := algorithmSigner.SignWithAlgorithm(rand.Reader, data, algo)
	return sig, trace.Wrap(err, "signing with VNet SSH agent signer")
}

// Add implements [agent.ExtendedAgent.Add]. It is irrelevant for this
// implementation and always returns an error, it is not called.
func (a *sshAgent) Add(key agent.AddedKey) error {
	return trace.NotImplemented("sshAgent.Add is not implemented")
}

// Remove implements [agent.ExtendedAgent.Remove]. It is irrelevant for this
// implementation and always returns an error, it is not called.
func (a *sshAgent) Remove(key ssh.PublicKey) error {
	return trace.NotImplemented("sshAgent.Remove is not implemented")
}

// RemoveAll implements [agent.ExtendedAgent.RemoveAll]. It is irrelevant for this
// implementation and always returns an error, it is not called.
func (a *sshAgent) RemoveAll() error {
	return trace.NotImplemented("sshAgent.RemoveAll is not implemented")
}

// Lock implements [agent.ExtendedAgent.Lock]. It is irrelevant for this
// implementation and always returns an error, it is not called.
func (a *sshAgent) Lock(passphrase []byte) error {
	return trace.NotImplemented("sshAgent.Lock is not implemented")
}

// Unlock implements [agent.ExtendedAgent.Unlock]. It is irrelevant for this
// implementation and always returns an error, it is not called.
func (a *sshAgent) Unlock(passphrase []byte) error {
	return trace.NotImplemented("sshAgent.Unlock is not implemented")
}

// Extension implements [agent.ExtendedAgent.Extension]. It is irrelevant for
// this implementation and always returns an error, it is not called.
func (a *sshAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, trace.NotImplemented("sshAgent.Extension is not implemented")
}
