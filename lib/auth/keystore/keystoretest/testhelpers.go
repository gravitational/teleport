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

package keystoretest

import (
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/keystore/internal"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
)

func HSMTestConfig(t *testing.T) servicecfg.KeystoreConfig {
	if cfg, ok := YubiHSMTestConfig(t); ok {
		t.Log("Running test with YubiHSM")
		return cfg
	}
	if cfg, ok := CloudHSMTestConfig(t); ok {
		t.Log("Running test with AWS CloudHSM")
		return cfg
	}
	if cfg, ok := AWSKMSTestConfig(t); ok {
		t.Log("Running test with AWS KMS")
		return cfg
	}
	if cfg, ok := GCPKMSTestConfig(t); ok {
		t.Log("Running test with GCP KMS")
		return cfg
	}
	if cfg, ok := SoftHSMTestConfig(t); ok {
		t.Log("Running test with SoftHSM")
		return cfg
	}
	t.Skip("No HSM available for test")
	return servicecfg.KeystoreConfig{}
}

func YubiHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	yubiHSMPath := os.Getenv("TELEPORT_TEST_YUBIHSM_PKCS11_PATH")
	yubiHSMPin := os.Getenv("TELEPORT_TEST_YUBIHSM_PIN")
	if yubiHSMPath == "" || yubiHSMPin == "" {
		return servicecfg.KeystoreConfig{}, false
	}

	slotNumber := 0
	return servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       yubiHSMPath,
			SlotNumber: &slotNumber,
			PIN:        yubiHSMPin,
		},
	}, true
}

func CloudHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	cloudHSMPin := os.Getenv("TELEPORT_TEST_CLOUDHSM_PIN")
	if cloudHSMPin == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
			TokenLabel: "cavium",
			PIN:        cloudHSMPin,
		},
	}, true
}

func AWSKMSTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	awsKMSAccount := os.Getenv("TELEPORT_TEST_AWS_KMS_ACCOUNT")
	awsKMSRegion := os.Getenv("TELEPORT_TEST_AWS_KMS_REGION")
	if awsKMSAccount == "" || awsKMSRegion == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		AWSKMS: &servicecfg.AWSKMSConfig{
			AWSAccount: awsKMSAccount,
			AWSRegion:  awsKMSRegion,
		},
	}, true
}

func GCPKMSTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	gcpKeyring := os.Getenv("TELEPORT_TEST_GCP_KMS_KEYRING")
	if gcpKeyring == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		GCPKMS: servicecfg.GCPKMSConfig{
			KeyRing:         gcpKeyring,
			ProtectionLevel: "SOFTWARE",
		},
	}, true
}

var (
	cachedSoftHSMConfig      *servicecfg.KeystoreConfig
	cachedSoftHSMConfigMutex sync.Mutex
)

// softHSMTestConfig is for use in tests only and creates a test SOFTHSM2 token.
// This should be used for all tests which need to use SoftHSM because the
// library can only be initialized once and SOFTHSM2_PATH and SOFTHSM2_CONF
// cannot be changed. New tokens added after the library has been initialized
// will not be found by the library.
//
// A new token will be used for each `go test` invocation, but it's difficult
// to create a separate token for each test because because new tokens
// added after the library has been initialized will not be found by the
// library. It's also difficult to clean up the token because tests for all
// packages are run in parallel there is not a good time to safely
// delete the token or the entire token directory. Each test should clean up
// all keys that it creates because SoftHSM2 gets really slow when there are
// many keys for a given token.
func SoftHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	path := os.Getenv("SOFTHSM2_PATH")
	if path == "" {
		return servicecfg.KeystoreConfig{}, false
	}

	cachedSoftHSMConfigMutex.Lock()
	defer cachedSoftHSMConfigMutex.Unlock()

	if cachedSoftHSMConfig != nil {
		return *cachedSoftHSMConfig, true
	}

	if os.Getenv("SOFTHSM2_CONF") == "" {
		// create tokendir
		tokenDir, err := os.MkdirTemp("", "tokens")
		require.NoError(t, err)

		// create config file
		configFile, err := os.CreateTemp("", "softhsm2.conf")
		require.NoError(t, err)

		// write config file
		_, err = fmt.Fprintf(configFile, "directories.tokendir = %s\nobjectstore.backend = file\nlog.level = DEBUG\n", tokenDir)
		require.NoError(t, err)
		require.NoError(t, configFile.Close())

		// set env
		os.Setenv("SOFTHSM2_CONF", configFile.Name())
	}

	// create test token (max length is 32 chars)
	tokenLabel := strings.ReplaceAll(uuid.NewString(), "-", "")
	cmd := exec.Command("softhsm2-util", "--init-token", "--free", "--label", tokenLabel, "--so-pin", "password", "--pin", "password")
	t.Logf("Running command: %q", cmd)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			require.NoError(t, exitErr, "error creating test softhsm token: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err, "error attempting to run softhsm2-util")
	}

	cachedSoftHSMConfig = &servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       path,
			TokenLabel: tokenLabel,
			PIN:        "password",
		},
	}
	return *cachedSoftHSMConfig, true
}

type testKeystoreOptions struct {
	rsaKeyPairSource internal.RSAKeyPairSource
}

type TestKeystoreOption func(*testKeystoreOptions)

func WithRSAKeyPairSource(rsaKeyPairSource internal.RSAKeyPairSource) TestKeystoreOption {
	return func(opts *testKeystoreOptions) {
		opts.rsaKeyPairSource = rsaKeyPairSource
	}
}

// NewTestKeystore returns a new TestKeystore that is valid for tests and
// not specifically testing the keystore functionality.
func NewTestKeystore(opts ...TestKeystoreOption) *TestKeystore {
	var options testKeystoreOptions
	for _, opt := range opts {
		opt(&options)
	}
	softwareBackend := internal.NewSoftwareKeyStore(options.rsaKeyPairSource)
	return &TestKeystore{
		backend: softwareBackend,
	}
}

type TestKeystore struct {
	backend internal.Backend
}

func (t *TestKeystore) NewSSHKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.SSHKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx,
		func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
		},
		purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshKey, signer, err := t.backend.GenerateSigner(ctx, alg)
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

func (t *TestKeystore) NewJWTKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.JWTKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx,
		func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
		},
		purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	jwtKey, signer, err := t.backend.GenerateSigner(ctx, alg)
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

func (t *TestKeystore) NewTLSKeyPair(ctx context.Context, clusterName string, purpose cryptosuites.KeyPurpose) (*types.TLSKeyPair, error) {
	alg, err := cryptosuites.AlgorithmForKey(ctx,
		func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
		},
		purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsKey, signer, err := t.backend.GenerateSigner(ctx, alg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
		signer,
		pkix.Name{
			CommonName:   clusterName,
			Organization: []string{clusterName},
		},
		nil,
		defaults.CATTL,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.TLSKeyPair{
		Cert:    tlsCert,
		Key:     tlsKey,
		KeyType: internal.KeyType(tlsKey),
	}, nil
}

func (t *TestKeystore) GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	for _, keyPair := range ca.GetActiveKeys().TLS {
		canUse, err := t.backend.CanUseKey(ctx, keyPair.Key, keyPair.KeyType)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if !canUse {
			continue
		}

		block, _ := pem.Decode(keyPair.Cert)
		if block == nil {
			return nil, nil, trace.BadParameter("failed to parse PEM block")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, trace.Wrap(err, "failed to parse x509 certificate")
		}

		signer, err := t.backend.GetSigner(ctx, keyPair.Key, cert.PublicKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return keyPair.Cert, signer, nil
	}

	return nil, nil, trace.NotFound("no usable TLS key pairs found")
}

func (t *TestKeystore) GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error) {
	for _, keyPair := range ca.GetActiveKeys().JWT {
		canUse, err := t.backend.CanUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
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
		signer, err := t.backend.GetSigner(ctx, keyPair.PrivateKey, pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return signer, nil
	}

	return nil, trace.NotFound("no usable JWT key pairs found")
}

func (t *TestKeystore) GetSSHSignerFromKeySet(ctx context.Context, keySet types.CAKeySet) (ssh.Signer, error) {
	for _, keyPair := range keySet.SSH {
		canUse, err := t.backend.CanUseKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !canUse {
			continue
		}

		sshPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(keyPair.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse SSH public key")
		}
		cryptoPublicKey, ok := sshPublicKey.(ssh.CryptoPublicKey)
		if !ok {
			return nil, trace.BadParameter("unsupported SSH public key type %q", sshPublicKey.Type())
		}

		signer, err := t.backend.GetSigner(ctx, keyPair.PrivateKey, cryptoPublicKey.CryptoPublicKey())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sshSigner, err := internal.SSHSignerFromCryptoSigner(signer)
		return sshSigner, trace.Wrap(err)
	}

	return nil, trace.NotFound("no usable SSH key pairs found")
}
