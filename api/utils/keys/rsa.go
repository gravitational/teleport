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
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// RSAPrivateKey is an rsa.PrivateKey with additional methods.
type RSAPrivateKey struct {
	*rsa.PrivateKey
	privateKeyDER []byte
	sshPub        ssh.PublicKey
}

// NewRSAPrivateKey creates a new RSAPrivateKey from a rsa.PrivateKey.
func NewRSAPrivateKey(priv *rsa.PrivateKey) (*RSAPrivateKey, error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RSAPrivateKey{
		PrivateKey:    priv,
		privateKeyDER: keyDER,
		sshPub:        sshPub,
	}, nil
}

// PrivateKeyPEM returns the PEM encoded RSA private key.
func (r *RSAPrivateKey) PrivateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    pkcs8PrivateKeyType,
		Headers: nil,
		Bytes:   r.privateKeyDER,
	})
}

// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
func (r *RSAPrivateKey) SSHPublicKey() ssh.PublicKey {
	return r.sshPub
}

// TLSCertificate parses the given TLS certificate paired with the private key
// to rerturn a tls.Certificate, ready to be used in a TLS handshake.
func (r *RSAPrivateKey) TLSCertificate(certRaw []byte) (tls.Certificate, error) {
	cert, err := tls.X509KeyPair(certRaw, r.PrivateKeyPEM())
	return cert, trace.Wrap(err)
}
