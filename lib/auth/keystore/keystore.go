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
	"github.com/gravitational/teleport/lib/modules"

	"github.com/gravitational/trace"
)

var pkcs11Prefix = []byte("pkcs11:")

// KeyStore is an interface for creating and using cryptographic keys.
type KeyStore interface {
	// GenerateRSA creates a new RSA private key and returns its identifier and
	// a crypto.Signer. The returned identifier can be passed to GetSigner
	// later to get the same crypto.Signer.
	GenerateRSA() (keyID []byte, signer crypto.Signer, err error)

	// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
	GetSigner(keyID []byte) (crypto.Signer, error)

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

	// HasLocalActiveKeys returns true if the given CA has any additional
	// trusted keys that are usable with this KeyStore.
	HasLocalAdditionalKeys(ca types.CertAuthority) bool

	// DeleteKey deletes the given key from the KeyStore.
	DeleteKey(keyID []byte) error

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
}

// Config is used to pass KeyStore configuration to NewKeyStore.
type Config struct {
	// RSAKeyPairSource is a function which returns raw keypairs to be used if
	// an hsm is not configured
	RSAKeyPairSource RSAKeyPairSource

	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber points to the PKCS11 slot to use, or nil if slot is unused.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string
	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Path == "" && cfg.RSAKeyPairSource == nil {
		return trace.BadParameter("must provide one of Path or RSAKeyPairSource in keystore.Config")
	}
	if cfg.Path != "" {
		// HSM is configured, check the rest of the required params
		if cfg.SlotNumber == nil && cfg.TokenLabel == "" {
			return trace.BadParameter("must provide one of SlotNumber or TokenLabel in keystore.Config")
		}
		if cfg.HostUUID == "" {
			return trace.BadParameter("must provide HostUUID in keystore.Config")
		}
	}
	return nil
}

// NewKeyStore returns a new KeyStore
func NewKeyStore(cfg Config) (KeyStore, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Path == "" {
		return NewRawKeyStore(&RawConfig{cfg.RSAKeyPairSource}), nil
	}
	if !modules.GetModules().Features().HSM {
		return nil, trace.AccessDenied("HSM support is only available with an enterprise license")
	}
	return NewHSMKeyStore(&HSMConfig{
		Path:       cfg.Path,
		SlotNumber: cfg.SlotNumber,
		TokenLabel: cfg.TokenLabel,
		Pin:        cfg.Pin,
		HostUUID:   cfg.HostUUID,
	})
}

// KeyType returns the type of the given private key.
func KeyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	return types.PrivateKeyType_RAW
}
