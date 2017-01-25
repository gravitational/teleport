/*
Copyright 2015 Gravitational, Inc.

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
	GetKeys(username string) ([]Key, error)
	AddKey(host string, username string, key *Key) error
	GetKey(host string, username string) (*Key, error)
	DeleteKey(host string, username string) error

	// interface to known_hosts file:
	AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)
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

// CertValidBefore returns the time of the cert expiration
func (k *Key) CertValidBefore() (t time.Time, err error) {
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return t, trace.Wrap(err)
	}
	cert, ok := pcert.(*ssh.Certificate)
	if !ok {
		return t, trace.Errorf("not supported certificate type")
	}
	return time.Unix(int64(cert.ValidBefore), 0), nil
}
