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

package keystore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509/pkix"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Manager provides an interface to interact with teleport CA private keys,
// which may be software keys or held in an HSM or other key manager.
type Manager struct {
	backend
}

// RSAKeyOptions configure options for RSA key generation.
type RSAKeyOptions struct {
	DigestAlgorithm crypto.Hash
}

// RSAKeyOption is a functional option for RSA key generation.
type RSAKeyOption func(*RSAKeyOptions)

func WithDigestAlgorithm(alg crypto.Hash) RSAKeyOption {
	return func(opts *RSAKeyOptions) {
		opts.DigestAlgorithm = alg
	}
}

// backend is an interface that holds private keys and provides signing
// operations.
type backend interface {
	// DeleteUnusedKeys deletes all keys from the KeyStore if they are:
	// 1. Labeled by this KeyStore when they were created
	// 2. Not included in the argument activeKeys
	DeleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error

	// generateRSA creates a new RSA private key and returns its identifier and
	// a crypto.Signer. The returned identifier can be passed to getSigner
	// later to get the same crypto.Signer.
	generateRSA(context.Context, ...RSAKeyOption) (keyID []byte, signer crypto.Signer, err error)

	// getSigner returns a crypto.Signer for the given key identifier, if it is found.
	getSigner(ctx context.Context, keyID []byte) (crypto.Signer, error)

	// deleteKey deletes the given key from the KeyStore.
	deleteKey(ctx context.Context, keyID []byte) error

	// canSignWithKey returns true if this KeyStore is able to sign with the
	// given key.
	canSignWithKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error)
}

// Config holds configuration parameters for the keystore. A software keystore
// will be the default if no other is configured. Only one inner config other
// than Softare should be set. It is okay to always set the Software config even
// when a different keystore is desired because it will only be used if all
// others are empty.
type Config struct {
	// Software holds configuration parameters specific to software keystores.
	Software SoftwareConfig
	// PKCS11 holds configuration parameters specific to PKCS#11 keystores.
	PKCS11 PKCS11Config
	// GCPKMS holds configuration parameters specific to GCP KMS keystores.
	GCPKMS GCPKMSConfig
	// Logger is a logger to be used by the keystore.
	Logger logrus.FieldLogger
}

func (cfg *Config) CheckAndSetDefaults() error {
	// We check for mutual exclusion when parsing the file config.
	if (cfg.PKCS11 != PKCS11Config{}) {
		return trace.Wrap(cfg.PKCS11.CheckAndSetDefaults())
	}
	if (cfg.GCPKMS != GCPKMSConfig{}) {
		return trace.Wrap(cfg.GCPKMS.CheckAndSetDefaults())
	}
	return trace.Wrap(cfg.Software.CheckAndSetDefaults())
}

// NewManager returns a new keystore Manager
func NewManager(ctx context.Context, cfg Config) (*Manager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	if (cfg.PKCS11 != PKCS11Config{}) {
		backend, err := newPKCS11KeyStore(&cfg.PKCS11, logger)
		return &Manager{backend: backend}, trace.Wrap(err)
	}
	if (cfg.GCPKMS != GCPKMSConfig{}) {
		backend, err := newGCPKMSKeyStore(ctx, &cfg.GCPKMS, logger)
		return &Manager{backend: backend}, trace.Wrap(err)
	}
	return &Manager{backend: newSoftwareKeyStore(&cfg.Software, logger)}, nil
}

// GetSSHSigner selects a usable SSH keypair from the given CA ActiveKeys and
// returns an [ssh.Signer].
func (m *Manager) GetSSHSigner(ctx context.Context, ca types.CertAuthority) (ssh.Signer, error) {
	signer, err := m.getSSHSigner(ctx, ca.GetActiveKeys())
	return signer, trace.Wrap(err)
}

// GetSSHSigner selects a usable SSH keypair from the given CA
// AdditionalTrustedKeys and returns an [ssh.Signer].
func (m *Manager) GetAdditionalTrustedSSHSigner(ctx context.Context, ca types.CertAuthority) (ssh.Signer, error) {
	signer, err := m.getSSHSigner(ctx, ca.GetAdditionalTrustedKeys())
	return signer, trace.Wrap(err)
}

func (m *Manager) getSSHSigner(ctx context.Context, keySet types.CAKeySet) (ssh.Signer, error) {
	for _, keyPair := range keySet.SSH {
		canSign, err := m.backend.canSignWithKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !canSign {
			continue
		}
		signer, err := m.backend.getSigner(ctx, keyPair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sshSigner, err := ssh.NewSignerFromSigner(signer)
		return sshSigner, trace.Wrap(err)
	}
	return nil, trace.NotFound("no usable SSH key pairs found")
}

// GetTLSCertAndSigner selects a usable TLS keypair from the given CA
// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
func (m *Manager) GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	cert, signer, err := m.getTLSCertAndSigner(ctx, ca.GetActiveKeys())
	return cert, signer, trace.Wrap(err)
}

// GetAdditionalTrustedTLSCertAndSigner selects a usable TLS keypair from the given CA
// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
func (m *Manager) GetAdditionalTrustedTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	cert, signer, err := m.getTLSCertAndSigner(ctx, ca.GetAdditionalTrustedKeys())
	return cert, signer, trace.Wrap(err)
}

func (m *Manager) getTLSCertAndSigner(ctx context.Context, keySet types.CAKeySet) ([]byte, crypto.Signer, error) {
	for _, keyPair := range keySet.TLS {
		canSign, err := m.backend.canSignWithKey(ctx, keyPair.Key, keyPair.KeyType)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if !canSign {
			continue
		}
		signer, err := m.backend.getSigner(ctx, keyPair.Key)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return keyPair.Cert, signer, nil
	}
	return nil, nil, trace.NotFound("no usable TLS key pairs found")
}

// GetJWTSigner selects a usable JWT keypair from the given keySet and returns
// a [crypto.Signer].
func (m *Manager) GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error) {
	for _, keyPair := range ca.GetActiveKeys().JWT {
		canSign, err := m.backend.canSignWithKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !canSign {
			continue
		}
		signer, err := m.backend.getSigner(ctx, keyPair.PrivateKey)
		return signer, trace.Wrap(err)
	}
	return nil, trace.NotFound("no usable JWT key pairs found")
}

// NewSSHKeyPair generates a new SSH keypair in the keystore backend and returns it.
func (m *Manager) NewSSHKeyPair(ctx context.Context) (*types.SSHKeyPair, error) {
	// The default hash length for SSH signers is 512 bits.
	sshKey, cryptoSigner, err := m.backend.generateRSA(ctx, WithDigestAlgorithm(crypto.SHA512))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err := ssh.NewSignerFromSigner(cryptoSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())
	return &types.SSHKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     sshKey,
		PrivateKeyType: keyType(sshKey),
	}, nil
}

// NewTLSKeyPair creates a new TLS keypair in the keystore backend and returns it.
func (m *Manager) NewTLSKeyPair(ctx context.Context, clusterName string) (*types.TLSKeyPair, error) {
	tlsKey, signer, err := m.backend.generateRSA(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		signer,
		pkix.Name{
			CommonName:   clusterName,
			Organization: []string{clusterName},
		}, nil, defaults.CATTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.TLSKeyPair{
		Cert:    tlsCert,
		Key:     tlsKey,
		KeyType: keyType(tlsKey),
	}, nil
}

// New JWTKeyPair create a new JWT keypair in the keystore backend and returns
// it.
func (m *Manager) NewJWTKeyPair(ctx context.Context) (*types.JWTKeyPair, error) {
	jwtKey, signer, err := m.backend.generateRSA(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, err := utils.MarshalPublicKey(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.JWTKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     jwtKey,
		PrivateKeyType: keyType(jwtKey),
	}, nil
}

// HasUsableActiveKeys returns true if the given CA has any usable active keys.
func (m *Manager) HasUsableActiveKeys(ctx context.Context, ca types.CertAuthority) (bool, error) {
	usable, err := m.hasUsableKeys(ctx, ca.GetActiveKeys())
	return usable, trace.Wrap(err)
}

// HasUsableActiveKeys returns true if the given CA has any usable additional
// trusted keys.
func (m *Manager) HasUsableAdditionalKeys(ctx context.Context, ca types.CertAuthority) (bool, error) {
	usable, err := m.hasUsableKeys(ctx, ca.GetAdditionalTrustedKeys())
	return usable, trace.Wrap(err)
}

func (m *Manager) hasUsableKeys(ctx context.Context, keySet types.CAKeySet) (bool, error) {
	for _, sshKeyPair := range keySet.SSH {
		usable, err := m.backend.canSignWithKey(ctx, sshKeyPair.PrivateKey, sshKeyPair.PrivateKeyType)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if usable {
			return true, nil
		}
	}
	for _, tlsKeyPair := range keySet.TLS {
		usable, err := m.backend.canSignWithKey(ctx, tlsKeyPair.Key, tlsKeyPair.KeyType)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if usable {
			return true, nil
		}
	}
	for _, jwtKeyPair := range keySet.JWT {
		usable, err := m.backend.canSignWithKey(ctx, jwtKeyPair.PrivateKey, jwtKeyPair.PrivateKeyType)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if usable {
			return true, nil
		}
	}
	return false, nil
}

// keyType returns the type of the given private key.
func keyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	if bytes.HasPrefix(key, []byte(gcpkmsPrefix)) {
		return types.PrivateKeyType_GCP_KMS
	}
	return types.PrivateKeyType_RAW
}
