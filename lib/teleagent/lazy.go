package teleagent

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// lazyAgent is an agent.Agent instance which is lazily initialized
// via a call to an AgentGetter.  Helpful for dealing with APIs
// which require, but may not actually use, an agent.
type lazyAgent struct {
	ctx  context.Context
	get  AgentGetter
	once sync.Once
	ref  agent.Agent
	err  error
}

// Lazy wraps an AgentGetter in an agent.Agent interface which lazily
// initializes the agent on first method invocation.
func Lazy(ctx context.Context, get AgentGetter) agent.Agent {
	return &lazyAgent{
		ctx: ctx,
		get: get,
	}
}

// agent gets the lazily initialized agent
func (l *lazyAgent) agent() (agent.Agent, error) {
	l.once.Do(func() {
		l.ref, l.err = l.get(l.ctx)
	})
	return l.ref, l.err
}

// List returns the identities known to the agent.
func (l *lazyAgent) List() ([]*agent.Key, error) {
	a, err := l.agent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.List()
}

// Sign has the agent sign the data using a protocol 2 key as defined
// in [PROTOCOL.agent] section 2.6.2.
func (l *lazyAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	a, err := l.agent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.Sign(key, data)
}

// Add adds a private key to the agent.
func (l *lazyAgent) Add(key agent.AddedKey) error {
	a, err := l.agent()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Add(key)
}

// Remove removes all identities with the given public key.
func (l *lazyAgent) Remove(key ssh.PublicKey) error {
	a, err := l.agent()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Remove(key)
}

// RemoveAll removes all identities.
func (l *lazyAgent) RemoveAll() error {
	a, err := l.agent()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.RemoveAll()
}

// Lock locks the agent. Sign and Remove will fail, and List will empty an empty list.
func (l *lazyAgent) Lock(passphrase []byte) error {
	a, err := l.agent()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Lock(passphrase)
}

// Unlock undoes the effect of Lock
func (l *lazyAgent) Unlock(passphrase []byte) error {
	a, err := l.agent()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Unlock(passphrase)
}

// Signers returns signers for all the known keys.
func (l *lazyAgent) Signers() ([]ssh.Signer, error) {
	a, err := l.agent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.Signers()
}
