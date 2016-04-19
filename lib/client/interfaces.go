package client

import (
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Key describes a stored client key
type Key struct {
	Priv []byte `json:"Priv,omitempty"`
	Pub  []byte `json:"Pub,omitempty"`
	Cert []byte `json:"Cert,omitempty"`

	// Deadline AKA TTL is the time when this key is safe to be discarded
	// for garbage collection purposes
	Deadline time.Time `json:"Deadline,omitempty"`
}

// LocalKeyStore interface allows for different storage back-ends for TSH to load/save its keys
type LocalKeyStore interface {
	// client key management
	GetKeys() ([]Key, error)
	AddKey(host string, key *Key) error
	GetKey(host string) (*Key, error)

	// trusted hosts key management:
	AddKnownHost(hostname string, publicKeys []ssh.PublicKey) error
	GetKnownHosts() ([]ssh.PublicKey, error)
}

// AsAgentKey converts our Key structure to ssh.Agent.Key
func (k *Key) AsAgentKey() (*agent.AddedKey, error) {
	// parse the returned&signed key:
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pk, err := ssh.ParseRawPrivateKey(k.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}, nil
}
