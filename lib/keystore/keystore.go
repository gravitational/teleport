/*
Copyright 2021 Gravitational, Inc.

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

package keystore

import (
	"bytes"
	"crypto"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

var pkcs11Prefix = []byte("pkcs11:")

// Config is used to pass HSM client configuration parameters.
type Config struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber is the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string

	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string

	// RSAKeyPairSource is a function type which returns new RSA keypairs. For
	// use when there is no real HSM.
	RSAKeyPairSource RSAKeyPairSource
}

func (config *Config) Validate() error {
	if (config.Path == "") == (config.RSAKeyPairSource == nil) {
		return trace.BadParameter("exactly one of Path or RSAKeyPairSource must be provided")
	}
	return nil
}

// KeyStore is an interface for creating and using cryptographic keys.
type KeyStore interface {
	// GenerateRSA creates a new RSA private key and returns its identifier and
	// a crypto.Signer. The returned identifier can be passed to GetSigner
	// later to get the same crypto.Signer.
	GenerateRSA() (keyID []byte, signer crypto.Signer, err error)

	// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
	GetSigner(keyID []byte) (crypto.Signer, error)

	// GetTLSCertAndSigner selects the local TLS keypair and returns the raw TLS cert and crypto.Signer.
	GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error)

	// GetSSHSigner selects the local SSH keypair and returns an ssh.Signer.
	GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error)

	// GetSSHSigner selects the local JWT keypair and returns a *jwt.Key.
	GetJWTSigner(ca types.CertAuthority, clock clockwork.Clock) (*jwt.Key, error)

	// DeleteKey deletes the given key from the KeyStore
	DeleteKey(keyID []byte) error
}

// KeyType returns the type of the given private key.
func KeyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	return types.PrivateKeyType_RAW
}
