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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"maps"
	"slices"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/keystore/internal/faketime"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	keystoreSubsystem = "keystore"

	labelKeyType      = "key_type"
	keyTypeTLS        = "tls"
	keyTypeSSH        = "ssh"
	keyTypeJWT        = "jwt"
	keyTypeEncryption = "enc"

	labelStoreType = "store_type"
	storePKCS11    = "pkcs11"
	storeGCP       = "gcp_kms"
	storeAWS       = "aws_kms"
	storeSoftware  = "software"

	labelCryptoAlgorithm = "key_algorithm"
)

// keyUsage marks a given key to be used either with signing or decryption
type keyUsage string

const (
	keyUsageNone    keyUsage = ""
	keyUsageSign    keyUsage = "sign"
	keyUsageDecrypt keyUsage = "decrypt"
)

var (
	signCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "sign_requests_total",
		Help:      "Total number of sign requests",
	}, []string{labelKeyType, labelStoreType})
	signErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "sign_requests_error",
		Help:      "Total number of sign request errors",
	}, []string{labelKeyType, labelStoreType})
	decryptCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "decrypt_requests_total",
		Help:      "Total number of decrypt requests",
	}, []string{labelKeyType, labelStoreType})
	decryptErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "decrypt_requests_error",
		Help:      "Total number of decrypt request errors",
	}, []string{labelKeyType, labelStoreType})
	createCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "key_create_requests_total",
		Help:      "Total number of key create requests",
	}, []string{labelKeyType, labelStoreType, labelCryptoAlgorithm})
	createErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: keystoreSubsystem,
		Name:      "key_create_requests_error",
		Help:      "Total number of key create request errors",
	}, []string{labelKeyType, labelStoreType, labelCryptoAlgorithm})
)

// Manager provides an interface to interact with teleport CA private keys,
// which may be software keys or held in an HSM or other key manager.
type Manager struct {
	// backendForNewKeys is the preferred backend the Manager is configured to
	// use, all new keys will be generated in this backend.
	backendForNewKeys backend

	// usableBackends is a list of all backends the manager can get signers or
	// decrypters from, in preference order. [backendForNewKeys] is expected to be
	// the first element.
	usableBackends []backend

	currentSuiteGetter cryptosuites.GetSuiteFunc
	logger             *slog.Logger
}

// backend is an interface that holds private keys and provides signing and decryption
// operations.
type backend interface {
	// generateSigner creates a new key pair and returns its identifier and a crypto.Signer. The returned
	// identifier can be passed to getSigner later to get an equivalent crypto.Signer.
	generateSigner(context.Context, cryptosuites.Algorithm) (keyID []byte, signer crypto.Signer, err error)

	// generateDecrypter creates a new key pair and returns its identifier and a crypto.Decrypter. The returned
	// identifier can be passed to getDecrypter later to get an equivalent crypto.Decrypter.
	generateDecrypter(context.Context, cryptosuites.Algorithm) (keyID []byte, decrypter crypto.Decrypter, hash crypto.Hash, err error)

	// getSigner returns a crypto.Signer for the given key identifier, if it is found.
	// The public key is passed as well so that it does not need to be fetched
	// from the underlying backend, and it is always stored in the CA anyway.
	getSigner(ctx context.Context, keyID []byte, pub crypto.PublicKey) (crypto.Signer, error)

	// getDecrypter returns a crypto.Decrypter for the given key identifier, if it is found.
	// The public key is passed as well so that it does not need to be fetched
	// from the underlying backend.
	getDecrypter(ctx context.Context, keyID []byte, pub crypto.PublicKey, hash crypto.Hash) (crypto.Decrypter, error)

	// canUseKey returns true if this backend is able to sign or decrypt with the
	// given key.
	canUseKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error)

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

	// name returns the name of the backend.
	name() string
}

// Options holds keystore options.
type Options struct {
	// HostUUID is the ID of the Auth Service host.
	HostUUID string
	// ClusterName provides the name of the Teleport cluster.
	ClusterName types.ClusterName
	// Logger is a logger to be used by the keystore.
	Logger *slog.Logger
	// AuthPreferenceGetter provides the current cluster auth preference.
	AuthPreferenceGetter cryptosuites.AuthPreferenceGetter
	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool
	// OAEPHash function to use with keystores that support OAEP with a configurable hash.
	OAEPHash crypto.Hash

	awsKMSClient kmsClient
	mrkClient    mrkClient
	awsSTSClient stsClient
	kmsClient    *kms.KeyManagementClient

	clockworkOverride clockwork.Clock
	// GCPKMS uses a special fake clock that seemed more testable at the time.
	faketimeOverride faketime.Clock
}

// CheckAndSetDefaults checks that the options are valid and sets defaults.
func (opts *Options) CheckAndSetDefaults() error {
	if opts.ClusterName == nil {
		return trace.BadParameter("ClusterName is required")
	}
	if opts.AuthPreferenceGetter == nil {
		return trace.BadParameter("AuthPreferenceGetter is required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.With(teleport.ComponentKey, "Keystore")
	}
	return nil
}

// NewManager returns a new keystore Manager
func NewManager(ctx context.Context, cfg *servicecfg.KeystoreConfig, opts *Options) (*Manager, error) {
	if err := metrics.RegisterPrometheusCollectors(
		signCounter,
		signErrorCounter,
		createCounter,
		createErrorCounter,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	softwareBackend := newSoftwareKeyStore(&softwareConfig{})
	var backendForNewKeys backend = softwareBackend
	usableBackends := []backend{softwareBackend}

	switch {
	case cfg.PKCS11 != (servicecfg.PKCS11Config{}):
		pkcs11Backend, err := newPKCS11KeyStore(&cfg.PKCS11, opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = pkcs11Backend
		usableBackends = []backend{pkcs11Backend, softwareBackend}
	case cfg.GCPKMS != (servicecfg.GCPKMSConfig{}):
		gcpBackend, err := newGCPKMSKeyStore(ctx, &cfg.GCPKMS, opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = gcpBackend
		usableBackends = []backend{gcpBackend, softwareBackend}
	case cfg.AWSKMS != nil:
		awsBackend, err := newAWSKMSKeystore(ctx, cfg.AWSKMS, opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = awsBackend
		usableBackends = []backend{awsBackend, softwareBackend}
	}

	return &Manager{
		backendForNewKeys:  backendForNewKeys,
		usableBackends:     usableBackends,
		currentSuiteGetter: cryptosuites.GetCurrentSuiteFromAuthPreference(opts.AuthPreferenceGetter),
		logger:             opts.Logger,
	}, nil
}

type cryptoCountSigner struct {
	crypto.Signer
	keyType string
	store   string
}

func (s *cryptoCountSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	signCounter.WithLabelValues(s.keyType, s.store).Inc()
	sig, err := s.Signer.Sign(rand, digest, opts)
	if err != nil {
		signErrorCounter.WithLabelValues(s.keyType, s.store).Inc()
		return nil, trace.Wrap(err)
	}
	return sig, nil
}

type cryptoCountDecrypter struct {
	crypto.Decrypter
	keyType string
	store   string
}

func (d *cryptoCountDecrypter) Decrypt(rand io.Reader, ciphertext []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	decryptCounter.WithLabelValues(d.keyType, d.store).Inc()
	plaintext, err := d.Decrypter.Decrypt(rand, ciphertext, opts)
	if err != nil {
		decryptErrorCounter.WithLabelValues(d.keyType, d.store).Inc()
		return nil, trace.Wrap(err)
	}

	return plaintext, nil
}

// GetSSHSigner selects a usable SSH keypair from the given CA ActiveKeys and
// returns an [ssh.Signer].
func (m *Manager) GetSSHSigner(ctx context.Context, ca types.CertAuthority) (ssh.Signer, error) {
	signer, err := m.GetSSHSignerFromKeySet(ctx, ca.GetActiveKeys())
	return signer, trace.Wrap(err)
}

// GetSSHSigner selects a usable SSH keypair from the given CA
// AdditionalTrustedKeys and returns an [ssh.Signer].
func (m *Manager) GetAdditionalTrustedSSHSigner(ctx context.Context, ca types.CertAuthority) (ssh.Signer, error) {
	signer, err := m.GetSSHSignerFromKeySet(ctx, ca.GetAdditionalTrustedKeys())
	return signer, trace.Wrap(err)
}

// GetSSHSignerFromKeySet selects a usable SSH keypair from the provided key
// set.
func (m *Manager) GetSSHSignerFromKeySet(ctx context.Context, keySet types.CAKeySet) (ssh.Signer, error) {
	for _, backend := range m.usableBackends {
		for _, keyPair := range keySet.SSH {
			canUse, err := backend.canUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !canUse {
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
			signer = &cryptoCountSigner{Signer: signer, keyType: keyTypeSSH, store: backend.name()}
			sshSigner, err := sshSignerFromCryptoSigner(signer)
			return sshSigner, trace.Wrap(err)
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

func sshSignerFromCryptoSigner(cryptoSigner crypto.Signer) (ssh.Signer, error) {
	sshSigner, err := ssh.NewSignerFromSigner(cryptoSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// [ssh.NewSignerFromSigner] currently always returns an [ssh.AlgorithmSigner].
	algorithmSigner, ok := sshSigner.(ssh.AlgorithmSigner)
	if !ok {
		return nil, trace.BadParameter("SSH CA: unsupported key type: %s", sshSigner.PublicKey().Type())
	}
	// Note: we don't actually create keys with all the algorithms supported
	// below, but customers have been known to import their own existing keys.
	switch pub := cryptoSigner.Public().(type) {
	case *rsa.PublicKey:
		// The current default hash used in ssh.(*Certificate).SignCert for an
		// RSA signer created via ssh.NewSignerFromSigner is always SHA256,
		// irrespective of the key size.
		// This was a change in golang.org/x/crypto 0.14.0, prior to that the
		// default was always SHA512.
		//
		// Due to the historical SHA512 default that existed at a time when
		// hash algorithm selection was much more difficult, there are many
		// existing GCP KMS keys that were created as 4096-bit keys using a
		// SHA512 hash. GCP KMS is very particular about RSA hash algorithms:
		// - 2048-bit or 3072-bit keys *must* use SHA256
		// - 4096-bit keys *must* use SHA256 or SHA512
		// - the hash length must be set *when the key is created* and can't be
		//   changed.
		//
		// The chosen signature algorithms below are necessary to support
		// existing GCP KMS keys, but they are also reasonable defaults for keys
		// outside of GCP KMS.
		//
		// [rsa.PublicKey.Size()] returns 256 for a 2048-bit key; more generally
		// it always returns the bit length divided by 8.
		keySize := pub.Size()
		switch {
		case keySize < 256:
			return nil, trace.BadParameter("SSH CA: RSA key size (%d) is too small", keySize)
		case keySize < 512:
			// This case matches 2048 and 3072 bit GCP KMS keys which *must* use SHA256.
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA256})
		default:
			// This case matches existing 4096 bit GCP KMS keys which *must* use SHA512
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA512})
		}
	case *ecdsa.PublicKey:
		// These are all the current defaults, but let's set them explicitly so
		// golang.org/x/crypto/ssh can't change them in an update and break some
		// HSM or KMS that wouldn't support the new default.
		switch pub.Curve {
		case elliptic.P256():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA256})
		case elliptic.P384():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA384})
		case elliptic.P521():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA521})
		default:
			return nil, trace.BadParameter("SSH CA: ECDSA curve: %s", pub.Curve.Params().Name)
		}
	case ed25519.PublicKey:
		// This is the current default, but let's set it explicitly so
		// golang.org/x/crypto/ssh can't change it in an update and break some
		// HSM or KMS that wouldn't support the new default.
		return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoED25519})
	default:
		return nil, trace.BadParameter("SSH CA: unsupported key type: %s", sshSigner.PublicKey().Type())
	}
}

// GetTLSCertAndSigner selects a usable TLS keypair from the given CA
// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
func (m *Manager) GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	cert, signer, err := m.getTLSCertAndSigner(ctx, ca.GetActiveKeys())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return cert, signer, trace.Wrap(err)
}

// GetAdditionalTrustedTLSCertAndSigner selects a usable TLS keypair from the given CA
// and returns the PEM-encoded TLS certificate and a [crypto.Signer].
func (m *Manager) GetAdditionalTrustedTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	cert, signer, err := m.getTLSCertAndSigner(ctx, ca.GetAdditionalTrustedKeys())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return cert, signer, trace.Wrap(err)
}

func (m *Manager) getTLSCertAndSigner(ctx context.Context, keySet types.CAKeySet) ([]byte, crypto.Signer, error) {
	for _, backend := range m.usableBackends {
		for _, keyPair := range keySet.TLS {
			canUse, err := backend.canUseKey(ctx, keyPair.Key, keyPair.KeyType)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			if !canUse {
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
			signer = &cryptoCountSigner{Signer: signer, keyType: keyTypeTLS, store: backend.name()}
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
	for _, backend := range m.usableBackends {
		for _, keyPair := range ca.GetActiveKeys().JWT {
			canUse, err := backend.canUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !canUse {
				continue
			}
			pub, err := keys.ParsePublicKey(keyPair.PublicKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			signer, err := backend.getSigner(ctx, keyPair.PrivateKey, pub)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &cryptoCountSigner{Signer: signer, keyType: keyTypeJWT, store: backend.name()}, trace.Wrap(err)
		}
	}
	return nil, trace.NotFound("no usable JWT key pairs found")
}

// GetDecrypter returns the [crypto.Decrypter] associated with a given EncryptionKeyPair if accessible.
func (m *Manager) GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error) {
	for _, backend := range m.usableBackends {
		canUse, err := backend.canUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !canUse {
			continue
		}
		pub, err := keys.ParsePublicKey(keyPair.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		decrypter, err := backend.getDecrypter(ctx, keyPair.PrivateKey, pub, crypto.Hash(keyPair.Hash))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &cryptoCountDecrypter{Decrypter: decrypter, keyType: keyTypeEncryption, store: backend.name()}, nil
	}

	return nil, trace.NotFound("no compatible backend found for keypair")
}

// NewSSHKeyPair generates a new SSH keypair in the keystore backend and returns it.
func (m *Manager) NewSSHKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.SSHKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeSSH, m.backendForNewKeys.name(), alg.String()).Inc()
	key, err := m.newSSHKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeSSH, m.backendForNewKeys.name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newSSHKeyPair(ctx context.Context, alg cryptosuites.Algorithm) (*types.SSHKeyPair, error) {
	sshKey, signer, err := m.backendForNewKeys.generateSigner(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.SSHKeyPair{
		PublicKey:      ssh.MarshalAuthorizedKey(sshPub),
		PrivateKey:     sshKey,
		PrivateKeyType: keyType(sshKey),
	}, nil
}

// NewTLSKeyPair creates a new TLS keypair in the keystore backend and returns it.
func (m *Manager) NewTLSKeyPair(ctx context.Context, clusterName string, purpose cryptosuites.KeyPurpose) (*types.TLSKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeTLS, m.backendForNewKeys.name(), alg.String()).Inc()
	key, err := m.newTLSKeyPair(ctx, clusterName, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeTLS, m.backendForNewKeys.name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newTLSKeyPair(ctx context.Context, clusterName string, alg cryptosuites.Algorithm) (*types.TLSKeyPair, error) {
	tlsKey, signer, err := m.backendForNewKeys.generateSigner(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		&cryptoCountSigner{Signer: signer, keyType: keyTypeTLS, store: m.backendForNewKeys.name()},
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
func (m *Manager) NewJWTKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.JWTKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeJWT, m.backendForNewKeys.name(), alg.String()).Inc()
	key, err := m.newJWTKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeJWT, m.backendForNewKeys.name(), alg.String()).Inc()
		if alg == cryptosuites.RSA2048 {
			return nil, trace.Wrap(err)
		}
		// Try to fall back to RSA if using the legacy suite. The HSM/KMS
		// credentials may not have permission to create ECDSA keys, especially
		// if set up before ECDSA support was added.
		origErr := trace.Wrap(err, "generating %s key in %s", alg.String(), m.backendForNewKeys.name())
		m.logger.WarnContext(ctx, "Failed to generate key with default algorithm, falling back to RSA.", "error", origErr)
		currentSuite, suiteErr := m.currentSuiteGetter(ctx)
		if suiteErr != nil {
			return nil, trace.NewAggregate(origErr, trace.Wrap(suiteErr, "finding current algorithm suite"))
		}
		switch currentSuite {
		case types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY:
		default:
			// Not using the legacy suite, ECDSA key gen really should have
			// worked, return the original error.
			return nil, origErr
		}
		var rsaErr error
		if key, rsaErr = m.newJWTKeyPair(ctx, cryptosuites.RSA2048); rsaErr != nil {
			return nil, trace.NewAggregate(origErr, trace.Wrap(rsaErr, "attempting fallback to RSA key"))
		}
	}
	return key, nil
}

func (m *Manager) newJWTKeyPair(ctx context.Context, alg cryptosuites.Algorithm) (*types.JWTKeyPair, error) {
	jwtKey, signer, err := m.backendForNewKeys.generateSigner(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, err := keys.MarshalPublicKey(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.JWTKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     jwtKey,
		PrivateKeyType: keyType(jwtKey),
	}, nil
}

// NewEncryptionKeyPair creates and returns a new encryption keypair.
func (m *Manager) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createCounter.WithLabelValues(keyTypeEncryption, m.backendForNewKeys.name(), alg.String()).Inc()
	key, err := m.newEncryptionKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeEncryption, m.backendForNewKeys.name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newEncryptionKeyPair(ctx context.Context, alg cryptosuites.Algorithm) (*types.EncryptionKeyPair, error) {
	encKey, decrypter, hash, err := m.backendForNewKeys.generateDecrypter(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKey, err := keys.MarshalPublicKey(decrypter.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.EncryptionKeyPair{
		PublicKey:      publicKey,
		PrivateKey:     encKey,
		PrivateKeyType: keyType(encKey),
		Hash:           uint32(hash),
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
	for i, backend := range m.usableBackends {
		preferredBackend := i == 0
		for _, sshKeyPair := range keySet.SSH {
			usable, err := backend.canUseKey(ctx, sshKeyPair.PrivateKey, sshKeyPair.PrivateKeyType)
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
			usable, err := backend.canUseKey(ctx, tlsKeyPair.Key, tlsKeyPair.KeyType)
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
			usable, err := backend.canUseKey(ctx, jwtKeyPair.PrivateKey, jwtKeyPair.PrivateKeyType)
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
	result.CAKeyTypes = slices.Collect(maps.Keys(caKeyTypes))
	return result, nil
}

// DeleteUnusedKeys deletes any keys from the backend that were created by this
// cluster and are not present in [activeKeys].
func (m *Manager) DeleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	return trace.Wrap(m.backendForNewKeys.deleteUnusedKeys(ctx, activeKeys))
}

// ApplyMultiRegionConfig configures the given keyID with the current multi-region
// parameters and returns the updated keyID. This is currently only implemented
// for AWS KMS.
func (m *Manager) ApplyMultiRegionConfig(ctx context.Context, keyID []byte) ([]byte, error) {
	backend, ok := m.backendForNewKeys.(*awsKMSKeystore)
	if !ok {
		return keyID, nil
	}
	keyID, err := backend.applyMultiRegionConfig(ctx, keyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyID, nil
}

// UsingHSMOrKMS returns true if the keystore is configured to use an HSM or KMS
// when generating new keys.
func (m *Manager) UsingHSMOrKMS() bool {
	_, usingSoftware := m.backendForNewKeys.(*softwareKeyStore)
	return !usingSoftware
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
