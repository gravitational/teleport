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
	"context"
	"crypto"
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
	"github.com/gravitational/teleport/lib/auth/keystore/internal"
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
	backendForNewKeys internal.Backend

	// usableBackends is a list of all backends the manager can get signers or
	// decrypters from, in preference order. [backendForNewKeys] is expected to be
	// the first element.
	usableBackends []internal.Backend

	currentSuiteGetter cryptosuites.GetSuiteFunc
	logger             *slog.Logger
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

	AWSKMSClient internal.KMSClient
	AWSMRKClient internal.MRKClient
	AWSSTSClient internal.STSClient

	GCPKMSClient *kms.KeyManagementClient

	RSAKeyPairSource internal.RSAKeyPairSource

	Clock clockwork.Clock
	// GCPKMS uses a special fake clock that seemed more testable at the time.
	FakeTime faketime.Clock
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

	softwareBackend := internal.NewSoftwareKeyStore(opts.RSAKeyPairSource)
	var backendForNewKeys internal.Backend = softwareBackend
	usableBackends := []internal.Backend{softwareBackend}

	switch {
	case cfg.PKCS11 != (servicecfg.PKCS11Config{}):
		pkcs11Backend, err := internal.NewPKCS11KeyStore(internal.PKCS11KeyStoreConfig{
			PKCS11Config: cfg.PKCS11,
			HostUUID:     opts.HostUUID,
			Logger:       opts.Logger,
			OAEPHash:     opts.OAEPHash,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = pkcs11Backend
		usableBackends = []internal.Backend{pkcs11Backend, softwareBackend}
	case cfg.GCPKMS != (servicecfg.GCPKMSConfig{}):
		gcpBackend, err := internal.NewGCPKMSKeyStore(ctx, internal.GCPKMSKeyStoreConfig{
			GCPKMSConfig: cfg.GCPKMS,
			KMSClient:    opts.GCPKMSClient,
			Clock:        opts.FakeTime,
			HostUUID:     opts.HostUUID,
			Logger:       opts.Logger,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = gcpBackend
		usableBackends = []internal.Backend{gcpBackend, softwareBackend}
	case cfg.AWSKMS != nil:
		awsBackend, err := internal.NewAWSKMSKeystore(ctx, internal.AWSKMSKeystoreConfig{
			AWSKMSConfig: cfg.AWSKMS,
			FIPS:         opts.FIPS,
			Logger:       opts.Logger,
			ClusterName:  opts.ClusterName.GetClusterName(),
			Clock:        opts.Clock,
			KMSClient:    opts.AWSKMSClient,
			MRKClient:    opts.AWSMRKClient,
			STSClient:    opts.AWSSTSClient,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		backendForNewKeys = awsBackend
		usableBackends = []internal.Backend{awsBackend, softwareBackend}
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
			canUse, err := backend.CanUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
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
			signer, err := backend.GetSigner(ctx, keyPair.PrivateKey, pub)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			signer = &cryptoCountSigner{Signer: signer, keyType: keyTypeSSH, store: backend.Name()}
			sshSigner, err := internal.SSHSignerFromCryptoSigner(signer)
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
			canUse, err := backend.CanUseKey(ctx, keyPair.Key, keyPair.KeyType)
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
			signer, err := backend.GetSigner(ctx, keyPair.Key, pub)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			signer = &cryptoCountSigner{Signer: signer, keyType: keyTypeTLS, store: backend.Name()}
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
			canUse, err := backend.CanUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
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
			signer, err := backend.GetSigner(ctx, keyPair.PrivateKey, pub)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &cryptoCountSigner{Signer: signer, keyType: keyTypeJWT, store: backend.Name()}, nil
		}
	}
	return nil, trace.NotFound("no usable JWT key pairs found")
}

// GetDecrypter returns the [crypto.Decrypter] associated with a given EncryptionKeyPair if accessible.
func (m *Manager) GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error) {
	for _, backend := range m.usableBackends {
		canUse, err := backend.CanUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
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

		decrypter, err := backend.GetDecrypter(ctx, keyPair.PrivateKey, pub, crypto.Hash(keyPair.Hash))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &cryptoCountDecrypter{Decrypter: decrypter, keyType: keyTypeEncryption, store: backend.Name()}, nil
	}

	return nil, trace.NotFound("no compatible backend found for keypair")
}

// NewSSHKeyPair generates a new SSH keypair in the keystore backend and returns it.
func (m *Manager) NewSSHKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.SSHKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeSSH, m.backendForNewKeys.Name(), alg.String()).Inc()
	key, err := m.newSSHKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeSSH, m.backendForNewKeys.Name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newSSHKeyPair(ctx context.Context, alg cryptosuites.Algorithm) (*types.SSHKeyPair, error) {
	sshKey, signer, err := m.backendForNewKeys.GenerateSigner(ctx, alg)
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
		PrivateKeyType: internal.KeyType(sshKey),
	}, nil
}

// NewTLSKeyPair creates a new TLS keypair in the keystore backend and returns it.
func (m *Manager) NewTLSKeyPair(ctx context.Context, clusterName string, purpose cryptosuites.KeyPurpose) (*types.TLSKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeTLS, m.backendForNewKeys.Name(), alg.String()).Inc()
	key, err := m.newTLSKeyPair(ctx, clusterName, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeTLS, m.backendForNewKeys.Name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newTLSKeyPair(ctx context.Context, clusterName string, alg cryptosuites.Algorithm) (*types.TLSKeyPair, error) {
	tlsKey, signer, err := m.backendForNewKeys.GenerateSigner(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		&cryptoCountSigner{Signer: signer, keyType: keyTypeTLS, store: m.backendForNewKeys.Name()},
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
		KeyType: internal.KeyType(tlsKey),
	}, nil
}

// New JWTKeyPair create a new JWT keypair in the keystore backend and returns
// it.
func (m *Manager) NewJWTKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.JWTKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	createCounter.WithLabelValues(keyTypeJWT, m.backendForNewKeys.Name(), alg.String()).Inc()
	key, err := m.newJWTKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeJWT, m.backendForNewKeys.Name(), alg.String()).Inc()
		if alg == cryptosuites.RSA2048 {
			return nil, trace.Wrap(err)
		}
		// Try to fall back to RSA if using the legacy suite. The HSM/KMS
		// credentials may not have permission to create ECDSA keys, especially
		// if set up before ECDSA support was added.
		origErr := trace.Wrap(err, "generating %s key in %s", alg.String(), m.backendForNewKeys.Name())
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
	jwtKey, signer, err := m.backendForNewKeys.GenerateSigner(ctx, alg)
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
		PrivateKeyType: internal.KeyType(jwtKey),
	}, nil
}

// NewEncryptionKeyPair creates and returns a new encryption keypair.
func (m *Manager) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx, m.currentSuiteGetter, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createCounter.WithLabelValues(keyTypeEncryption, m.backendForNewKeys.Name(), alg.String()).Inc()
	key, err := m.newEncryptionKeyPair(ctx, alg)
	if err != nil {
		createErrorCounter.WithLabelValues(keyTypeEncryption, m.backendForNewKeys.Name(), alg.String()).Inc()
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (m *Manager) newEncryptionKeyPair(ctx context.Context, alg cryptosuites.Algorithm) (*types.EncryptionKeyPair, error) {
	encKey, decrypter, hash, err := m.backendForNewKeys.GenerateDecrypter(ctx, alg)
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
		PrivateKeyType: internal.KeyType(encKey),
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
		PreferredKeyType: m.backendForNewKeys.KeyTypeDescription(),
	}
	var allRawKeys [][]byte
	for i, backend := range m.usableBackends {
		preferredBackend := i == 0
		for _, sshKeyPair := range keySet.SSH {
			usable, err := backend.CanUseKey(ctx, sshKeyPair.PrivateKey, sshKeyPair.PrivateKeyType)
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
			usable, err := backend.CanUseKey(ctx, tlsKeyPair.Key, tlsKeyPair.KeyType)
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
			usable, err := backend.CanUseKey(ctx, jwtKeyPair.PrivateKey, jwtKeyPair.PrivateKeyType)
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
		desc, err := internal.KeyDescription(rawKey)
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
	return trace.Wrap(m.backendForNewKeys.DeleteUnusedKeys(ctx, activeKeys))
}

// ApplyMultiRegionConfig configures the given keyID with the current multi-region
// parameters and returns the updated keyID. This is currently only implemented
// for AWS KMS.
func (m *Manager) ApplyMultiRegionConfig(ctx context.Context, keyID []byte) ([]byte, error) {
	backend, ok := m.backendForNewKeys.(*internal.AWSKMSKeystore)
	if !ok {
		return keyID, nil
	}
	keyID, err := backend.ApplyMultiRegionConfig(ctx, keyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyID, nil
}

// UsingHSMOrKMS returns true if the keystore is configured to use an HSM or KMS
// when generating new keys.
func (m *Manager) UsingHSMOrKMS() bool {
	_, usingSoftware := m.backendForNewKeys.(*internal.SoftwareKeyStore)
	return !usingSoftware
}
