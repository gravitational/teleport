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
	"crypto"
	"crypto/tls"
	"encoding/pem"
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	rsaPrivateKeyType        = "RSA PRIVATE KEY"
	pivYubikeyPrivateKeyType = "PIV YUBIKEY PRIVATE KEY"
)

// PrivateKey implements crypto.PrivateKey.
type PrivateKey interface {
	// Implement crypto.Signer and crypto.PrivateKey
	Public() crypto.PublicKey
	Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	Equal(x crypto.PrivateKey) bool

	// PrivateKeyDataPEM returns PEM encoded private key data. This may be data necessary
	// to retrieve the key, such as a Yubikey serial number and slot, or it can be a
	// full PEM encoded RSA private key.
	PrivateKeyDataPEM() []byte
	TLSCertificate(tlsCert []byte) (tls.Certificate, error)
	AsAgentKeys(*ssh.Certificate) []agent.AddedKey
}

// ParsePrivateKey returns a new KeyPair for the given private key data and public key PEM.
func ParsePrivateKey(privateKeyData []byte) (PrivateKey, error) {
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return nil, trace.BadParameter("expected PEM encoded private key")
	}

	switch block.Type {
	case rsaPrivateKeyType:
		return ParseRSAPrivateKey(block.Bytes)
	case pivYubikeyPrivateKeyType:
		return ParseYubikeyPrivateKey(block.Bytes)
	default:
		return nil, trace.BadParameter("unexpected private key PEM type %q", block.Type)
	}
}
