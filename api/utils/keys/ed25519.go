/*
Copyright 2022 Gravitational, Inc.

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

package keys

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ED25519PrivateKey is an ed25519.PrivateKey with additional methods.
type ED25519PrivateKey struct {
	ed25519.PrivateKey
	privateKeyDER []byte
	sshPub        ssh.PublicKey
}

// newED25519 creates a new ED25519 from a ed25519.PrivateKey.
func newED25519(priv ed25519.PrivateKey) (*ED25519PrivateKey, error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ED25519PrivateKey{
		PrivateKey:    priv,
		privateKeyDER: keyDER,
		sshPub:        sshPub,
	}, nil
}

// Equal returns whether the given key is the same as this ED25519PrivateKey.
func (r *ED25519PrivateKey) Equal(other PrivateKey) bool {
	if o, ok := other.(*ED25519PrivateKey); ok {
		return r.PrivateKey.Equal(o.PrivateKey)
	}
	return false
}

// PrivateKeyPEM returns the PEM encoded ed25519 private key.
func (r *ED25519PrivateKey) PrivateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    pkcs8PrivateKeyType,
		Headers: nil,
		Bytes:   r.privateKeyDER,
	})
}

// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
func (r *ED25519PrivateKey) SSHPublicKey() ssh.PublicKey {
	return r.sshPub
}

// TLSCertificate parses the given TLS certificate paired with the private key
// to rerturn a tls.Certificate, ready to be used in a TLS handshake.
func (r *ED25519PrivateKey) TLSCertificate(certRaw []byte) (tls.Certificate, error) {
	cert, err := tls.X509KeyPair(certRaw, r.PrivateKeyPEM())
	return cert, trace.Wrap(err)
}
