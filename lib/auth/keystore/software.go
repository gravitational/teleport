// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keystore

import (
	"crypto"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type softwareKeyStore struct {
	rsaKeyPairSource RSAKeyPairSource
}

// RSAKeyPairSource is a function type which returns new RSA keypairs.
type RSAKeyPairSource func() (priv []byte, pub []byte, err error)

type SoftwareConfig struct {
	RSAKeyPairSource RSAKeyPairSource
}

func (cfg *SoftwareConfig) CheckAndSetDefaults() error {
	if cfg.RSAKeyPairSource == nil {
		return trace.BadParameter("must provide RSAKeyPairSource")
	}
	return nil
}

func NewSoftwareKeyStore(config *SoftwareConfig) KeyStore {
	return &softwareKeyStore{
		rsaKeyPairSource: config.RSAKeyPairSource,
	}
}

// generateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier for softwareKeyStore is a pem-encoded
// private key, and can be passed to getSigner later to get the same
// crypto.Signer.
func (s *softwareKeyStore) generateRSA() ([]byte, crypto.Signer, error) {
	priv, _, err := s.rsaKeyPairSource()
	if err != nil {
		return nil, nil, err
	}
	signer, err := s.getSigner(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, signer, trace.Wrap(err)
}

// GetSigner returns a crypto.Signer for the given pem-encoded private key.
func (s *softwareKeyStore) getSigner(rawKey []byte) (crypto.Signer, error) {
	signer, err := utils.ParsePrivateKeyPEM(rawKey)
	return signer, trace.Wrap(err)
}

// GetTLSCertAndSigner selects the first software TLS keypair and returns the raw
// TLS cert and a crypto.Signer.
func (s *softwareKeyStore) GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().TLS
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKeyPEM(keyPair.Key)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return keyPair.Cert, signer, nil
		}
	}
	return nil, nil, trace.NotFound("no matching TLS key pairs found in CA for %q", ca.GetClusterName())
}

// GetAdditionalTrustedTLSCertAndSigner selects the local TLS keypair from the
// CA AdditionalTrustedKeys and returns the PEM-encoded TLS cert and a
// crypto.Signer.
func (s *softwareKeyStore) GetAdditionalTrustedTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPairs := ca.GetAdditionalTrustedKeys().TLS
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKeyPEM(keyPair.Key)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return keyPair.Cert, signer, nil
		}
	}
	return nil, nil, trace.NotFound("no matching TLS key pairs found in CA for %q", ca.GetClusterName())
}

// GetSSHSigner selects the first software SSH keypair and returns an ssh.Signer
func (s *softwareKeyStore) GetSSHSigner(ca types.CertAuthority) (ssh.Signer, error) {
	keyPairs := ca.GetActiveKeys().SSH
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := ssh.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no software SSH key pairs found in CA for %q", ca.GetClusterName())
}

// GetAdditionalTrustedSSHSigner selects the local SSH keypair from the CA
// AdditionalTrustedKeys and returns an ssh.Signer.
func (s *softwareKeyStore) GetAdditionalTrustedSSHSigner(ca types.CertAuthority) (ssh.Signer, error) {
	keyPairs := ca.GetAdditionalTrustedKeys().SSH
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := ssh.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no software SSH key pairs found in CA for %q", ca.GetClusterName())
}

// GetJWTSigner returns the active JWT signer used to sign tokens.
func (s *softwareKeyStore) GetJWTSigner(ca types.CertAuthority) (crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().JWT
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no software JWT key pairs found in CA for %q", ca.GetClusterName())
}

// NewSSHKeyPair creates and returns a new SSHKeyPair.
func (s *softwareKeyStore) NewSSHKeyPair() (*types.SSHKeyPair, error) {
	return newSSHKeyPair(s)
}

// NewTLSKeyPair creates and returns a new TLSKeyPair.
func (s *softwareKeyStore) NewTLSKeyPair(clusterName string) (*types.TLSKeyPair, error) {
	return newTLSKeyPair(s, clusterName)
}

// NewJWTKeyPair creates and returns a new JWTKeyPair.
func (s *softwareKeyStore) NewJWTKeyPair() (*types.JWTKeyPair, error) {
	return newJWTKeyPair(s)
}

// deleteKey deletes the given key from the KeyStore. This is a no-op for
// softwareKeyStore.
func (s *softwareKeyStore) deleteKey(rawKey []byte) error {
	return nil
}

func (s *softwareKeyStore) keySetHasLocalKeys(keySet types.CAKeySet) bool {
	for _, sshKeyPair := range keySet.SSH {
		if sshKeyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			return true
		}
	}
	for _, tlsKeyPair := range keySet.TLS {
		if tlsKeyPair.KeyType == types.PrivateKeyType_RAW {
			return true
		}
	}
	for _, jwtKeyPair := range keySet.JWT {
		if jwtKeyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			return true
		}
	}
	return false
}

// HasLocalActiveKeys returns true if the given CA has any active keys that
// are usable with this KeyStore.
func (s *softwareKeyStore) HasLocalActiveKeys(ca types.CertAuthority) bool {
	return s.keySetHasLocalKeys(ca.GetActiveKeys())
}

// HasLocalAdditionalKeys returns true if the given CA has any additional
// trusted keys that are usable with this KeyStore.
func (s *softwareKeyStore) HasLocalAdditionalKeys(ca types.CertAuthority) bool {
	return s.keySetHasLocalKeys(ca.GetAdditionalTrustedKeys())
}

// DeleteUnusedKeys deletes all keys from the KeyStore if they are:
// 1. Labeled by this KeyStore when they were created
// 2. Not included in the argument usedKeys
// This is a no-op for rawKeyStore.
func (s *softwareKeyStore) DeleteUnusedKeys(usedKeys [][]byte) error {
	return nil
}
