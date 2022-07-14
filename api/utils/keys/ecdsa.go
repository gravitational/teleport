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
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ECDSAPrivateKey is an ecdsa.PrivateKey with additional methods.
type ECDSAPrivateKey struct {
	*ecdsa.PrivateKey
	privateKeyDER []byte
	sshPub        ssh.PublicKey
}

// newECDSAPrivateKey creates a new ECDSAPrivateKey from a ecdsa.PrivateKey.
func newECDSAPrivateKey(priv *ecdsa.PrivateKey) (*ECDSAPrivateKey, error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ECDSAPrivateKey{
		PrivateKey:    priv,
		privateKeyDER: keyDER,
		sshPub:        sshPub,
	}, nil
}

// PrivateKeyPEM returns the PEM encoded ECDSA private key.
func (r *ECDSAPrivateKey) PrivateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    pkcs8PrivateKeyType,
		Headers: nil,
		Bytes:   r.privateKeyDER,
	})
}

// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
func (r *ECDSAPrivateKey) SSHPublicKey() ssh.PublicKey {
	return r.sshPub
}

// TLSCertificate parses the given TLS certificate paired with the private key
// to rerturn a tls.Certificate, ready to be used in a TLS handshake.
func (r *ECDSAPrivateKey) TLSCertificate(certRaw []byte) (tls.Certificate, error) {
	cert, err := tls.X509KeyPair(certRaw, r.PrivateKeyPEM())
	return cert, trace.Wrap(err)
}
