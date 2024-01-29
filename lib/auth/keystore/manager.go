/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package keystore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Manager provides an interface to interact with teleport CA private keys,
// which may be software keys or held in an HSM or other key manager.
type Manager struct {
	// backendForNewKeys is the preferred backend the Manager is configured to
	// use, all new keys will be generated in this backend.
	backendForNewKeys backend

	// usableSigningBackends is a list of all backends the manager can get
	// signers from, in preference order. [backendForNewKeys] is expected to be
	// the first element.
	usableSigningBackends []backend
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
	// generateRSA creates a new RSA key pair and returns its identifier and a
	// crypto.Signer. The returned identifier can be passed to getSigner later
	// to get the same crypto.Signer.
	generateRSA(context.Context, ...RSAKeyOption) (keyID []byte, signer crypto.Signer, err error)

	// getSigner returns a crypto.Signer for the given key identifier, if it is found.
	// The public key is passed as well so that it does not need to be fetched
	// from the underlying backend, and it is always stored in the CA anyway.
	getSigner(ctx context.Context, keyID []byte, pub crypto.PublicKey) (crypto.Signer, error)

	// canSignWithKey returns true if this backend is able to sign with the
	// given key.
	canSignWithKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error)

	// deleteKey deletes the given key from the backend.
	deleteKey(ctx context.Context, keyID []byte) error

	// deleteUnusedKeys deletes all keys from the backend if they are:
	// 1. Not included in the argument activeKeys which is meant to contain all
	//    active keys currently referenced in the backend CA.
	// 2. Created in the backend by this Teleport cluster.
	// 3. Each backend may apply extra restrictions to which keys may be deleted.
	deleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error

	// keyTypeDescription returns a human-readable description of the types of
	// keys this backend uses.
	keyTypeDescription() string
}

// Config holds configuration parameters for the keystore. A software keystore
// will be the default if no other is configured. Only one inner config other
// than Software should be set. It is okay to always set the Software config even
// when a different keystore is desired because it will only be used if all
// others are empty.
type Config struct {
	// Software holds configuration parameters specific to software keystores.
	Software SoftwareConfig
	// PKCS11 holds configuration parameters specific to PKCS#11 keystores.
	PKCS11 PKCS11Config
	// GCPKMS holds configuration parameters specific to GCP KMS keystores.
	GCPKMS GCPKMSConfig
	// AWSKMS holds configuration parameter specific to AWS KMS keystores.
	AWSKMS AWSKMSConfig
	// Logger is a logger to be used by the keystore.
	Logger logrus.FieldLogger
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = logrus.StandardLogger()
	}

	if err := cfg.Software.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// We check for mutual exclusion when parsing the file config.
	if (cfg.PKCS11 != PKCS11Config{}) {
		return trace.Wrap(cfg.PKCS11.CheckAndSetDefaults())
	}
	if (cfg.GCPKMS != GCPKMSConfig{}) {
		return trace.Wrap(cfg.GCPKMS.CheckAndSetDefaults())
	}
	if (cfg.AWSKMS != AWSKMSConfig{}) {
		return trace.Wrap(cfg.AWSKMS.CheckAndSetDefaults())
	}
	return nil
}

// NewManager returns a new keystore Manager
func NewManager(ctx context.Context, cfg Config) (*Manager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	softwareBackend := newSoftwareKeyStore(&cfg.Software, cfg.Logger)

	if (cfg.PKCS11 != PKCS11Config{}) {
		pkcs11Backend, err := newPKCS11KeyStore(&cfg.PKCS11, cfg.Logger)
		return &Manager{
			backendForNewKeys:     pkcs11Backend,
			usableSigningBackends: []backend{pkcs11Backend, softwareBackend},
		}, trace.Wrap(err)
	}
	if (cfg.GCPKMS != GCPKMSConfig{}) {
		gcpBackend, err := newGCPKMSKeyStore(ctx, &cfg.GCPKMS, cfg.Logger)
		return &Manager{
			backendForNewKeys:     gcpBackend,
			usableSigningBackends: []backend{gcpBackend, softwareBackend},
		}, trace.Wrap(err)
	}
	if (cfg.AWSKMS != AWSKMSConfig{}) {
		awsBackend, err := newAWSKMSKeystore(ctx, &cfg.AWSKMS, cfg.Logger)
		return &Manager{
			backendForNewKeys:     awsBackend,
			usableSigningBackends: []backend{awsBackend, softwareBackend},
		}, trace.Wrap(err)
	}
	return &Manager{
		backendForNewKeys:     softwareBackend,
		usableSigningBackends: []backend{softwareBackend},
	}, nil
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
	for _, backend := range m.usableSigningBackends {
		for _, keyPair := range keySet.SSH {
			canSign, err := backend.canSignWithKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !canSign {
				continue
			}
			pub, err := publicKeyFromSSHAuthorizedKey(keyPair.PublicKey)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse SSH public key")
			}
			signer, err := backend.getSigner(ctx, keyPair.PrivateKey, pub)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			sshSigner, err := ssh.NewSignerFromSigner(signer)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// SHA-512 to match NewSSHKeyPair.
			return toRSASHA512Signer(sshSigner), trace.Wrap(err)
		}
	}
	return nil, trace.NotFound("no usable SSH key pairs found")
}

func publicKeyFromSSHAuthorizedKey(sshAuthorizedKey []byte) (crypto.PublicKey, error) {
	sshPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(sshAuthorizedKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SSH public key")
	}
	cryptoPublicKey, ok := sshPublicKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, trace.BadParameter("unsupported SSH public key type %q", sshPublicKey.Type())
	}
	return cryptoPublicKey.CryptoPublicKey(), nil
}

// toRSASHA512Signer forces an ssh.MultiAlgorithmSigner into using
// "rsa-sha2-sha512" (instead of its SHA256 default).
func toRSASHA512Signer(signer ssh.Signer) ssh.Signer {
	ss, ok := signer.(ssh.MultiAlgorithmSigner)
	if !ok {
		return signer
	}
	return rsaSHA512Signer{MultiAlgorithmSigner: ss}
}

type rsaSHA512Signer struct {
	ssh.MultiAlgorithmSigner
}

func (s rsaSHA512Signer) Algorithms() []string {
	return []string{ssh.KeyAlgoRSASHA512}
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
	for _, backend := range m.usableSigningBackends {
		for _, keyPair := range keySet.TLS {
			canSign, err := backend.canSignWithKey(ctx, keyPair.Key, keyPair.KeyType)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			if !canSign {
				continue
			}
			pub, err := publicKeyFromTLSCertPem(keyPair.Cert)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			signer, err := backend.getSigner(ctx, keyPair.Key, pub)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return keyPair.Cert, signer, nil
		}
	}
	return nil, nil, trace.NotFound("no usable TLS key pairs found")
}

func publicKeyFromTLSCertPem(certPem []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(certPem)
	if block == nil {
		return nil, trace.BadParameter("failed to parse PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse x509 certificate")
	}
	return cert.PublicKey, nil
}

// GetJWTSigner selects a usable JWT keypair from the given keySet and returns
// a [crypto.Signer].
func (m *Manager) GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error) {
	for _, backend := range m.usableSigningBackends {
		for _, keyPair := range ca.GetActiveKeys().JWT {
			canSign, err := backend.canSignWithKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !canSign {
				continue
			}
			pub, err := utils.ParsePublicKey(keyPair.PublicKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			signer, err := backend.getSigner(ctx, keyPair.PrivateKey, pub)
			return signer, trace.Wrap(err)
		}
	}
	return nil, trace.NotFound("no usable JWT key pairs found")
}

// NewSSHKeyPair generates a new SSH keypair in the keystore backend and returns it.
func (m *Manager) NewSSHKeyPair(ctx context.Context) (*types.SSHKeyPair, error) {
	// The default hash length for SSH signers is 512 bits.
	sshKey, cryptoSigner, err := m.backendForNewKeys.generateRSA(ctx, WithDigestAlgorithm(crypto.SHA512))
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
	tlsKey, signer, err := m.backendForNewKeys.generateRSA(ctx)
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
	jwtKey, signer, err := m.backendForNewKeys.generateRSA(ctx)
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

// UsableKeysResult holds the result of a call to HasUsableActiveKeys or
// HasUsableAdditionalTrustedKeys.
type UsableKeysResult struct {
	// CAHasPreferredKeyType is true if the CA contains any key matching the key
	// type the keystore is currently configured to use when generating new
	// keys.
	CAHasPreferredKeyType bool
	// CAHasUsableKeys is true if the CA contains any key that the keystore as
	// currently configured can use for signatures.
	CAHasUsableKeys bool
	// PreferredKeyType is a description of the key type the keystore is
	// currently configured to use when generating new keys.
	PreferredKeyType string
	// CAKeyTypes is a list of descriptions of all the keys types currently
	// stored in the CA. It is only guaranteed to be valid if
	// CAHasPreferredKeyType is false.
	CAKeyTypes []string
}

// HasUsableActiveKeys returns true if the given CA has any usable active keys.
func (m *Manager) HasUsableActiveKeys(ctx context.Context, ca types.CertAuthority) (*UsableKeysResult, error) {
	return m.hasUsableKeys(ctx, ca.GetActiveKeys())
}

// HasUsableActiveKeys returns true if the given CA has any usable additional
// trusted keys.
func (m *Manager) HasUsableAdditionalKeys(ctx context.Context, ca types.CertAuthority) (*UsableKeysResult, error) {
	return m.hasUsableKeys(ctx, ca.GetAdditionalTrustedKeys())
}

func (m *Manager) hasUsableKeys(ctx context.Context, keySet types.CAKeySet) (*UsableKeysResult, error) {
	result := &UsableKeysResult{
		PreferredKeyType: m.backendForNewKeys.keyTypeDescription(),
	}
	var allRawKeys [][]byte
	for i, backend := range m.usableSigningBackends {
		preferredBackend := i == 0
		for _, sshKeyPair := range keySet.SSH {
			usable, err := backend.canSignWithKey(ctx, sshKeyPair.PrivateKey, sshKeyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if usable {
				result.CAHasUsableKeys = true
				if preferredBackend {
					result.CAHasPreferredKeyType = true
					return result, nil
				}
			}
			allRawKeys = append(allRawKeys, sshKeyPair.PrivateKey)
		}
		for _, tlsKeyPair := range keySet.TLS {
			usable, err := backend.canSignWithKey(ctx, tlsKeyPair.Key, tlsKeyPair.KeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if usable {
				result.CAHasUsableKeys = true
				if preferredBackend {
					result.CAHasPreferredKeyType = true
					return result, nil
				}
			}
			allRawKeys = append(allRawKeys, tlsKeyPair.Key)
		}
		for _, jwtKeyPair := range keySet.JWT {
			usable, err := backend.canSignWithKey(ctx, jwtKeyPair.PrivateKey, jwtKeyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if usable {
				result.CAHasUsableKeys = true
				if preferredBackend {
					result.CAHasPreferredKeyType = true
					return result, nil
				}
			}
			allRawKeys = append(allRawKeys, jwtKeyPair.PrivateKey)
		}
	}
	caKeyTypes := make(map[string]struct{})
	for _, rawKey := range allRawKeys {
		desc, err := keyDescription(rawKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caKeyTypes[desc] = struct{}{}
	}
	result.CAKeyTypes = maps.Keys(caKeyTypes)
	return result, nil
}

func (m *Manager) DeleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	return trace.Wrap(m.backendForNewKeys.deleteUnusedKeys(ctx, activeKeys))
}

// keyType returns the type of the given private key.
func keyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	if bytes.HasPrefix(key, []byte(gcpkmsPrefix)) {
		return types.PrivateKeyType_GCP_KMS
	}
	if bytes.HasPrefix(key, []byte(awskmsPrefix)) {
		return types.PrivateKeyType_AWS_KMS
	}
	return types.PrivateKeyType_RAW
}

func keyDescription(key []byte) (string, error) {
	switch keyType(key) {
	case types.PrivateKeyType_PKCS11:
		keyID, err := parsePKCS11KeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "PKCS#11 HSM keys created by " + keyID.HostID, nil
	case types.PrivateKeyType_GCP_KMS:
		keyID, err := parseGCPKMSKeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		keyring, err := keyID.keyring()
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "GCP KMS keys in keyring " + keyring, nil
	case types.PrivateKeyType_AWS_KMS:
		keyID, err := parseAWSKMSKeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "AWS KMS keys in account " + keyID.account + " and region " + keyID.region, nil
	default:
		return "raw software keys", nil
	}
}
