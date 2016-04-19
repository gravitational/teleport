package client

import (
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Key describes a complete (signed) client key
type Key struct {
	Priv []byte `json:"Priv,omitempty"`
	Pub  []byte `json:"Pub,omitempty"`
	Cert []byte `json:"Cert,omitempty"`
}

// LocalKeyStore interface allows for different storage back-ends for TSH to load/save its keys
type LocalKeyStore interface {
	// client key management
	GetKeys() ([]Key, error)
	AddKey(host string, key *Key) error
	GetKey(host string) (*Key, error)

	// trusted CAs management:
	AddKnownCA(domainName string, publicKeys []ssh.PublicKey) error
	GetKnownCAs() ([]ssh.PublicKey, error)
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

// CertValidBefore returns UTC time of the cert expiration
func (k *Key) CertValidBefore() (time.Time, error) {
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return time.Now().UTC(), trace.Wrap(err)
	}
	cert := pcert.(*ssh.Certificate)
	return time.Unix(0, int64(cert.ValidBefore)).UTC(), nil
}
