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

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

var pkcs11Prefix = []byte("pkcs11:")

// KeyStore is an interface for creating and using cryptographic keys.
type KeyStore interface {
	// GetTLSCertAndSigner selects the local TLS keypair from the CA ActiveKeys
	// and returns the PEM-encoded TLS cert and a crypto.Signer.
	GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error)

	// GetSSHSigner selects the local SSH keypair from the CA ActiveKeys and
	// returns an ssh.Signer.
	GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error)

	// GetJWTSigner selects the local JWT keypair from the CA ActiveKeys and
	// returns a crypto.Signer.
	GetJWTSigner(ca types.CertAuthority) (crypto.Signer, error)

	// NewSSHKeyPair creates and returns a new SSHKeyPair.
	NewSSHKeyPair() (*types.SSHKeyPair, error)

	// NewTLSKeyPair creates and returns a new TLSKeyPair.
	NewTLSKeyPair(clusterName string) (*types.TLSKeyPair, error)

	// NewJWTKeyPair creates and returns a new JWTKeyPair.
	NewJWTKeyPair() (*types.JWTKeyPair, error)

	// HasLocalActiveKeys returns true if the given CA has any active keys that
	// are usable with this KeyStore.
	HasLocalActiveKeys(ca types.CertAuthority) bool

	// HasLocalAdditionalKeys returns true if the given CA has any additional
	// trusted keys that are usable with this KeyStore.
	HasLocalAdditionalKeys(ca types.CertAuthority) bool

	// DeleteUnusedKeys deletes all keys from the KeyStore if they are:
	// 1. Labeled by this KeyStore when they were created
	// 2. Not included in the argument usedKeys
	DeleteUnusedKeys(usedKeys [][]byte) error

	// GetAdditionalTrustedSSHSigner selects the local SSH keypair from the CA
	// AdditionalTrustedKeys and returns an ssh.Signer.
	GetAdditionalTrustedSSHSigner(ca types.CertAuthority) (ssh.Signer, error)

	// GetAdditionalTrustedTLSCertAndSigner selects the local TLS keypair from
	// the CA AdditionalTrustedKeys and returns the PEM-encoded TLS cert and a
	// crypto.Signer.
	GetAdditionalTrustedTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error)

	// generateRSA creates a new RSA private key and returns its identifier and
	// a crypto.Signer. The returned identifier can be passed to getSigner
	// later to get the same crypto.Signer.
	generateRSA() (keyID []byte, signer crypto.Signer, err error)

	// getSigner returns a crypto.Signer for the given key identifier, if it is found.
	getSigner(keyID []byte) (crypto.Signer, error)

	// deleteKey deletes the given key from the KeyStore.
	deleteKey(keyID []byte) error
}

// Config is used to pass KeyStore configuration to NewKeyStore.
type Config struct {
	// Software holds configuration parameters specific to software KeyStores
	Software SoftwareConfig

	// PKCS11 hold configuration parameters specific to PKCS11 KeyStores.
	PKCS11 PKCS11Config
}

func (cfg *Config) CheckAndSetDefaults() error {
	if (cfg.PKCS11 != PKCS11Config{}) {
		return trace.Wrap(cfg.PKCS11.CheckAndSetDefaults())
	}
	return trace.Wrap(cfg.Software.CheckAndSetDefaults())
}

// NewKeyStore returns a new KeyStore
func NewKeyStore(cfg Config) (KeyStore, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if (cfg.PKCS11 != PKCS11Config{}) {
		if !modules.GetModules().Features().HSM {
			return nil, trace.AccessDenied("HSM support is only available with an enterprise license")
		}
		keyStore, err := NewPKCS11KeyStore(&cfg.PKCS11)
		return keyStore, trace.Wrap(err)
	}
	return NewSoftwareKeyStore(&cfg.Software), nil
}

// KeyType returns the type of the given private key.
func KeyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	return types.PrivateKeyType_RAW
}
