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
	"bytes"
	"fmt"
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

	// ProxyHost (optionally) contains the hostname of the proxy server
	// which issued this key
	ProxyHost string
}

// AsAgentKeys converts client.Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
//
// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1
// (2016-08-01) have a bug in how they use certificates that have been loaded
// in an agent. Specifically when you add a certificate to an agent, you can't
// just embed the private key within the certificate, you have to add the
// certificate and private key to the agent separately. Teleport works around
// this behavior to ensure OpenSSH interoperability.
//
// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
// WARNING: callers expect the returned slice to be __exactly as it is__
func (k *Key) AsAgentKeys() ([]*agent.AddedKey, error) {
	// unmarshal certificate bytes into a ssh.PublicKey
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// unmarshal private key bytes into a *rsa.PrivateKey
	privateKey, err := ssh.ParseRawPrivateKey(k.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", publicKey.(*ssh.Certificate).KeyId)

	// return a certificate (with embedded private key) as well as a private key
	return []*agent.AddedKey{
		&agent.AddedKey{
			PrivateKey:       privateKey,
			Certificate:      publicKey.(*ssh.Certificate),
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
		&agent.AddedKey{
			PrivateKey:       privateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}, nil
}

// EqualsTo returns true if this key is the same as the other.
// Primarily used in tests
func (k *Key) EqualsTo(other *Key) bool {
	if k == other {
		return true
	}
	return bytes.Equal(k.Cert, other.Cert) &&
		bytes.Equal(k.Priv, other.Priv) &&
		bytes.Equal(k.Pub, other.Pub)
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

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func (k *Key) AsAuthMethod() (ssh.AuthMethod, error) {
	keys, err := k.AsAgentKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewSignerFromKey(keys[0].PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer, err = ssh.NewCertSigner(keys[0].Certificate, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAuthMethodForCert(signer), nil
}
