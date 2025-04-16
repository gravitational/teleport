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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	clusterName = "test-cluster"
)

var (
	testRSA2048PrivateKeyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqiD2rRJ5kq7hP55eOCM9DtdkWPMI8PBKgxaAiQ9J9YF3aNur
98b8kACcTQ8ixSkHsLccVqRdt/Cnb7jtBSrwxJ9BN09fZEiyCvy7lwxNGBMQEaov
9UU722nvuWKb+EkHzcVV9ie9i8wM88xpzzYO8eda8FZjHxaaoe2lkrHiiOFQRubJ
qHVhW+SNFQOV6OsVETTZlg5rmWhA5rKiB6G0QeLHysSMJbbLMOXr1Vbu7Rqohmq0
AF6EdMgix3OJz3qL9YDKQPAzhj7ViPzT07Pv/9vh5fjXaE5iThPT4n33uY1N2fJA
nzscZvVmpxxuSOqxwWBqkRzIJez1vv3F+5xDLwIDAQABAoIBAFQ6KaYZ5XKHfiD/
COqGF66HWLjo6d5POLSZqV0x4o3XYQTa7NKpA1VP2BIWkkJGQ/ZrUW5bxcJRNLQN
O9s5HSZbKfB2LWX6z5q88Sqg/nISzfvQ5BlsA2xnkDWZ6loL3f8z2ZEar67MgQUa
iK/7tX5x6gXe3wf/KuNMQpLT2rGk/HKxm6FE/oH9/IWgd7NBUOKCkhS+cdiTYCGD
9m2UYgug6nISpNRALsE93E0lCKzhUQ4kC/dVzrzhhhvYz3c7Nun/GpJsTqMI4HRv
BXAU8W/lIUtoMHatKT+NqJ0yRmD28v25ZuIJLNnsyGLd4B/KvqtpJ8vz/+m/jKzH
JmYqVqECgYEA0AjyniECeIZFR0bU7pdC69y/0xL0FFZJZma9/ZRT1AqY5xjeaO3i
zzLCRvOxekMxfb+j084yJXvpu4ZAEyDsVydsx1KbeWb5u1RWfrjM3tUaZ3ZQNjeA
U7406l4+kM/za6sUFEGhfW1Wmf4Egf7CYj5Gd5210uebEQAiGjfKkfcCgYEA0Vqk
OcWF0NdKe3n41UXQVf13yEPiQP0MIf4FlzLiMhU2Ox9nbqvZ1LBq5QkF1360fmY5
yQ0vx2Yw5MpCaam4r1//DRDFm/i9JTW2DOcP5NWOApUTywhU/ikuxhVmxtBfxBHE
LcI6pknRRwWcIug4Mo3xkve6PwhzdFNlsJ1jiokCgYBuGq4+Hv5tx7LW/JgqBwi2
SMmF71wbf2etuOcJVP3hFhLDDRh5tJ38R8MnRkdCjFmfUlRk/5bu29xjEbTL6vrr
TcR24jPDV0sJaKO2whw8O9GTvLzLVSioKd1bxbGbd1RAQfWImwvblIjnS9ga7Tj4
QjmNiXz4OPiLUOS7t5eRFQKBgB8d9tzzY/FnnpV9yqOAjffKBdzJYj7AneYLiK8x
i/dfucDN6STE/Eqlsi26yph+J7vF2/7rK9fac5f+DCMCbAX9Ib7CaGzHau217wo5
6d3cdBAkMl3yLhfc7SvaEH2qiSFudpdKkEcZH7cLuWpi07+H44kxswgdbHO01Z+L
tTjpAoGBALKz4TpotvhZZ1iFAv3FeOWXCZz4jrLc+2GsViSgaHrCFmuV4tc/WB4z
fPTgihJAeKdWbBmRMjIDe8hkz/oxR6JE2Ap+4G+KZtwVON4b+ucCYTQS+1CQp2Xc
RPAMyjbzPhWQpfJnIxLcqGmvXxosABvs/b2CWaPqfCQhZIWpLeKW
-----END RSA PRIVATE KEY-----
`)
	testRSASSHPublicKey = []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCqIPatEnmSruE/nl44Iz0O12RY8wjw8EqDFoCJD0n1gXdo26v3xvyQAJxNDyLFKQewtxxWpF238KdvuO0FKvDEn0E3T19kSLIK/LuXDE0YExARqi/1RTvbae+5Ypv4SQfNxVX2J72LzAzzzGnPNg7x51rwVmMfFpqh7aWSseKI4VBG5smodWFb5I0VA5Xo6xURNNmWDmuZaEDmsqIHobRB4sfKxIwltssw5evVVu7tGqiGarQAXoR0yCLHc4nPeov1gMpA8DOGPtWI/NPTs+//2+Hl+NdoTmJOE9Piffe5jU3Z8kCfOxxm9WanHG5I6rHBYGqRHMgl7PW+/cX7nEMv")
	testRSAPublicKeyPEM = []byte(`-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAqiD2rRJ5kq7hP55eOCM9DtdkWPMI8PBKgxaAiQ9J9YF3aNur98b8
kACcTQ8ixSkHsLccVqRdt/Cnb7jtBSrwxJ9BN09fZEiyCvy7lwxNGBMQEaov9UU7
22nvuWKb+EkHzcVV9ie9i8wM88xpzzYO8eda8FZjHxaaoe2lkrHiiOFQRubJqHVh
W+SNFQOV6OsVETTZlg5rmWhA5rKiB6G0QeLHysSMJbbLMOXr1Vbu7Rqohmq0AF6E
dMgix3OJz3qL9YDKQPAzhj7ViPzT07Pv/9vh5fjXaE5iThPT4n33uY1N2fJAnzsc
ZvVmpxxuSOqxwWBqkRzIJez1vv3F+5xDLwIDAQAB
-----END RSA PUBLIC KEY-----`)
	testRSACert = []byte(`-----BEGIN CERTIFICATE-----
MIIDeTCCAmGgAwIBAgIRALmlBQhTQQiGIS/P0PwF97wwDQYJKoZIhvcNAQELBQAw
VjEQMA4GA1UEChMHc2VydmVyMTEQMA4GA1UEAxMHc2VydmVyMTEwMC4GA1UEBRMn
MjQ2NzY0MDEwMjczNTA2ODc3NjY1MDEyMTc3Mzg5MTkyODY5ODIwMB4XDTIxMDcx
NDE5MDY1MloXDTMxMDcxMjE5MDY1MlowVjEQMA4GA1UEChMHc2VydmVyMTEQMA4G
A1UEAxMHc2VydmVyMTEwMC4GA1UEBRMnMjQ2NzY0MDEwMjczNTA2ODc3NjY1MDEy
MTc3Mzg5MTkyODY5ODIwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
qiD2rRJ5kq7hP55eOCM9DtdkWPMI8PBKgxaAiQ9J9YF3aNur98b8kACcTQ8ixSkH
sLccVqRdt/Cnb7jtBSrwxJ9BN09fZEiyCvy7lwxNGBMQEaov9UU722nvuWKb+EkH
zcVV9ie9i8wM88xpzzYO8eda8FZjHxaaoe2lkrHiiOFQRubJqHVhW+SNFQOV6OsV
ETTZlg5rmWhA5rKiB6G0QeLHysSMJbbLMOXr1Vbu7Rqohmq0AF6EdMgix3OJz3qL
9YDKQPAzhj7ViPzT07Pv/9vh5fjXaE5iThPT4n33uY1N2fJAnzscZvVmpxxuSOqx
wWBqkRzIJez1vv3F+5xDLwIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0T
AQH/BAUwAwEB/zAdBgNVHQ4EFgQUAJprFmwUDaYguqQJxHLBC35BeQ0wDQYJKoZI
hvcNAQELBQADggEBAG3k42nHnJvsf3M4EPBqMFqLHJOcwt5jpfPrjLmTCAnjialq
v0qp/JAwC3SgrDFQMNNcWTi+H+E9FqYVavysZbBd0/cFlYH9SWe9CJi5CyfbsLGD
PX8hBRDZmmhTXMMHzyHqolVAFCf5nNQyQVeQGt3fBDh++WNjmkS+886lhag/a2hh
nskCVzvig1/6Exp06k2mMlphC25/bNB3SDeQj+dIJdej6btZs2goQZ/5Sx/iwB5c
xrmzqBs9YMU//QIN5ZFE+7opw5v6mbeGCCk3woH46VmVwO6mHCfLha4K/K92MMdg
JhuTMEqUaAOZBoQLn+txjl3nu9WwTThJzlY0L4w=
-----END CERTIFICATE-----
`)
	testPKCS11Key = []byte(`pkcs11:{"host_id": "server2", "key_id": "00000000-0000-0000-0000-000000000000"}`)

	testRawSSHKeyPair = &types.SSHKeyPair{
		PublicKey:      testRSASSHPublicKey,
		PrivateKey:     testRSA2048PrivateKeyPEM,
		PrivateKeyType: types.PrivateKeyType_RAW,
	}
	testRawTLSKeyPair = &types.TLSKeyPair{
		Cert:    testRSACert,
		Key:     testRSA2048PrivateKeyPEM,
		KeyType: types.PrivateKeyType_RAW,
	}
	testRawJWTKeyPair = &types.JWTKeyPair{
		PublicKey:      testRSAPublicKeyPEM,
		PrivateKey:     testRSA2048PrivateKeyPEM,
		PrivateKeyType: types.PrivateKeyType_RAW,
	}

	testPKCS11SSHKeyPair = &types.SSHKeyPair{
		PublicKey:      testRSASSHPublicKey,
		PrivateKey:     testPKCS11Key,
		PrivateKeyType: types.PrivateKeyType_PKCS11,
	}
	testPKCS11TLSKeyPair = &types.TLSKeyPair{
		Cert:    testRSACert,
		Key:     testPKCS11Key,
		KeyType: types.PrivateKeyType_PKCS11,
	}
	testPKCS11JWTKeyPair = &types.JWTKeyPair{
		PublicKey:      testRSAPublicKeyPEM,
		PrivateKey:     testPKCS11Key,
		PrivateKeyType: types.PrivateKeyType_PKCS11,
	}
)

func TestBackends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	message := []byte("Lorem ipsum dolor sit amet...")
	messageHash := sha256.Sum256(message)

	pack := newTestPack(ctx, t)

	for _, backendDesc := range pack.backends {
		t.Run(backendDesc.name, func(t *testing.T) {
			backend := backendDesc.backend

			for _, tc := range []struct {
				alg    cryptosuites.Algorithm
				verify func(pubkey any, hash, signature []byte) error
			}{
				{
					alg: cryptosuites.RSA2048,
					verify: func(pubkey any, hash, signature []byte) error {
						return rsa.VerifyPKCS1v15(pubkey.(*rsa.PublicKey), crypto.SHA256, messageHash[:], signature)
					},
				},
				{
					alg: cryptosuites.ECDSAP256,
					verify: func(pubkey any, hash, signature []byte) error {
						if !ecdsa.VerifyASN1(pubkey.(*ecdsa.PublicKey), messageHash[:], signature) {
							return errors.New("ECDSA signature is invalid")
						}
						return nil
					},
				},
			} {
				t.Run(tc.alg.String(), func(t *testing.T) {
					// create a key
					key, signer, err := backend.generateKey(ctx, tc.alg)
					require.NoError(t, err, trace.DebugReport(err))
					require.Equal(t, backendDesc.expectedKeyType, keyType(key))

					// delete the key when we're done with it
					t.Cleanup(func() { require.NoError(t, backend.deleteKey(ctx, key)) })

					// get a signer from the key
					signer, err = backend.getSigner(ctx, key, signer.Public())
					require.NoError(t, err)

					// try signing something
					signature, err := signer.Sign(rand.Reader, messageHash[:], crypto.SHA256)
					require.NoError(t, err, trace.DebugReport(err))
					require.NoError(t, tc.verify(signer.Public(), messageHash[:], signature))
				})
			}
		})
	}

	for _, backendDesc := range pack.backends {
		t.Run(backendDesc.name+"_deleteUnusedKeys", func(t *testing.T) {
			backend := backendDesc.backend

			// create some keys to test deleteUnusedKeys
			const numKeys = 3
			rawPrivateKeys := make([][]byte, numKeys)
			publicKeys := make([]crypto.PublicKey, numKeys)
			for i := 0; i < numKeys; i++ {
				var signer crypto.Signer
				var err error
				rawPrivateKeys[i], signer, err = backend.generateKey(ctx, cryptosuites.ECDSAP256)
				require.NoError(t, err)
				publicKeys[i] = signer.Public()
			}

			// AWS KMS keystore will not delete any keys created in the past 5
			// minutes.
			pack.clock.Advance(6 * time.Minute)

			// say that only the first key is in use, delete the rest
			usedKeys := [][]byte{rawPrivateKeys[0]}
			err := backend.deleteUnusedKeys(ctx, usedKeys)
			require.NoError(t, err, trace.DebugReport(err))

			// make sure the first key is still good
			signer, err := backend.getSigner(ctx, rawPrivateKeys[0], publicKeys[0])
			require.NoError(t, err)
			_, err = signer.Sign(rand.Reader, messageHash[:], crypto.SHA256)
			require.NoError(t, err)

			// make sure all other keys are deleted
			for i := 1; i < numKeys; i++ {
				signer, err := backend.getSigner(ctx, rawPrivateKeys[i], publicKeys[0])
				if err != nil {
					// For PKCS11 we expect to fail to get the signer, for cloud
					// KMS backends it won't fail until actually signing.
					continue
				}
				_, err = signer.Sign(rand.Reader, messageHash[:], crypto.SHA256)
				if backendDesc.deletionDoesNothing {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			}

			// Make sure key deletion is aborted when one of the active keys
			// cannot be found. This makes sure that we don't accidentally
			// delete current active keys in case the ListKeys operation fails.
			fakeActiveKey := backendDesc.unusedRawKey
			err = backend.deleteUnusedKeys(ctx, [][]byte{fakeActiveKey})
			if backendDesc.deletionDoesNothing {
				require.NoError(t, err)
			} else {
				require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
			}

			// delete the final key so we don't leak it
			err = backend.deleteKey(ctx, rawPrivateKeys[0])
			require.NoError(t, err)
		})
	}
}

func TestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sshSubjectKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshSubjectPubKey, err := ssh.NewPublicKey(sshSubjectKey)
	require.NoError(t, err)

	pack := newTestPack(ctx, t)

	for _, backendDesc := range pack.backends {
		t.Run(backendDesc.name, func(t *testing.T) {
			manager, err := NewManager(ctx, &backendDesc.config, backendDesc.opts)
			require.NoError(t, err)

			// Delete all keys to clean up the test.
			t.Cleanup(func() {
				require.NoError(t, manager.DeleteUnusedKeys(context.Background(), nil /*activeKeys*/))
			})

			sshKeyPair, err := manager.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
			require.NoError(t, err)
			require.Equal(t, backendDesc.expectedKeyType, sshKeyPair.PrivateKeyType)

			tlsKeyPair, err := manager.NewTLSKeyPair(ctx, clusterName, cryptosuites.UserCATLS)
			require.NoError(t, err)
			require.Equal(t, backendDesc.expectedKeyType, tlsKeyPair.KeyType)

			jwtKeyPair, err := manager.NewJWTKeyPair(ctx, cryptosuites.JWTCAJWT)
			require.NoError(t, err)
			require.Equal(t, backendDesc.expectedKeyType, jwtKeyPair.PrivateKeyType)

			// Test a CA with multiple active keypairs. Each element of ActiveKeys
			// includes a keypair generated above and a PKCS11 keypair with a
			// different hostID that this manager should not be able to use.
			ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: clusterName,
				ActiveKeys: types.CAKeySet{
					SSH: []*types.SSHKeyPair{
						testPKCS11SSHKeyPair,
						sshKeyPair,
					},
					TLS: []*types.TLSKeyPair{
						testPKCS11TLSKeyPair,
						tlsKeyPair,
					},
					JWT: []*types.JWTKeyPair{
						testPKCS11JWTKeyPair,
						jwtKeyPair,
					},
				},
			})
			require.NoError(t, err)

			// Test that the manager is able to select the correct key and get a
			// signer.
			sshSigner, err := manager.GetSSHSigner(ctx, ca)
			require.NoError(t, err, trace.DebugReport(err))
			require.Equal(t, sshKeyPair.PublicKey, ssh.MarshalAuthorizedKey(sshSigner.PublicKey()))

			tlsCert, tlsSigner, err := manager.GetTLSCertAndSigner(ctx, ca)
			require.NoError(t, err)
			require.Equal(t, tlsKeyPair.Cert, tlsCert)
			require.NotNil(t, tlsSigner)

			jwtSigner, err := manager.GetJWTSigner(ctx, ca)
			require.NoError(t, err, trace.DebugReport(err))
			pubkeyPem, err := keys.MarshalPublicKey(jwtSigner.Public())
			require.NoError(t, err)
			require.Equal(t, jwtKeyPair.PublicKey, pubkeyPem)

			// Try signing an SSH cert.
			sshCert := ssh.Certificate{
				Key:         sshSubjectPubKey,
				ValidBefore: uint64(time.Now().Add(time.Hour).Unix()),
			}
			require.NoError(t, sshCert.SignCert(rand.Reader, sshSigner))
			// Verify the signature.
			checker := ssh.CertChecker{
				IsUserAuthority: func(pub ssh.PublicKey) bool {
					return pub == sshSigner.PublicKey()
				},
			}
			require.NoError(t, checker.CheckCert("root", &sshCert))

			// Test what happens when the CA has only raw keys, which will be the
			// initial state when migrating from software to a HSM/KMS backend.
			ca, err = types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: clusterName,
				ActiveKeys: types.CAKeySet{
					SSH: []*types.SSHKeyPair{
						testRawSSHKeyPair,
					},
					TLS: []*types.TLSKeyPair{
						testRawTLSKeyPair,
					},
					JWT: []*types.JWTKeyPair{
						testRawJWTKeyPair,
					},
				},
			})
			require.NoError(t, err)

			// Manager should always be able to get a signer for software keys.
			usableKeysResult, err := manager.HasUsableActiveKeys(ctx, ca)
			require.NoError(t, err)
			require.True(t, usableKeysResult.CAHasUsableKeys)
			if backendDesc.expectedKeyType == types.PrivateKeyType_RAW {
				require.True(t, usableKeysResult.CAHasPreferredKeyType)
			} else {
				require.False(t, usableKeysResult.CAHasPreferredKeyType)
			}

			sshSigner, err = manager.GetSSHSigner(ctx, ca)
			require.NoError(t, err)
			require.NotNil(t, sshSigner)

			tlsCert, tlsSigner, err = manager.GetTLSCertAndSigner(ctx, ca)
			require.NoError(t, err)
			require.NotNil(t, tlsCert)
			require.NotNil(t, tlsSigner)

			jwtSigner, err = manager.GetJWTSigner(ctx, ca)
			require.NoError(t, err)
			require.NotNil(t, jwtSigner)

			// Test a CA with only unusable keypairs - PKCS11 keypairs with a
			// different hostID that this manager should not be able to use.
			ca, err = types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: clusterName,
				ActiveKeys: types.CAKeySet{
					SSH: []*types.SSHKeyPair{
						testPKCS11SSHKeyPair,
					},
					TLS: []*types.TLSKeyPair{
						testPKCS11TLSKeyPair,
					},
					JWT: []*types.JWTKeyPair{
						testPKCS11JWTKeyPair,
					},
				},
			})
			require.NoError(t, err)

			// The manager should not be able to select a key.
			usableKeysResult, err = manager.HasUsableActiveKeys(ctx, ca)
			require.NoError(t, err)
			require.False(t, usableKeysResult.CAHasUsableKeys)
			require.False(t, usableKeysResult.CAHasPreferredKeyType)

			_, err = manager.GetSSHSigner(ctx, ca)
			require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

			_, _, err = manager.GetTLSCertAndSigner(ctx, ca)
			require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)

			_, err = manager.GetJWTSigner(ctx, ca)
			require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
		})
	}
}

// TestAlgorithmSuites asserts that the keystore generates keys with the
// expected signature algorithm for all valid signature algorithm suites.
func TestAlgorithmSuites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	pack := newTestPack(ctx, t)
	for _, suite := range []types.SignatureAlgorithmSuite{
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
	} {
		t.Run(suite.String(), func(t *testing.T) {
			testAlgorithmSuite(t, ctx, pack, suite)
		})
	}
}

func testAlgorithmSuite(t *testing.T, ctx context.Context, pack *testPack, suite types.SignatureAlgorithmSuite) {
	for _, backendDesc := range pack.backends {
		t.Run(backendDesc.name, func(t *testing.T) {
			authPrefGetter := &fakeAuthPreferenceGetter{suite}
			backendDesc.opts.AuthPreferenceGetter = authPrefGetter
			currentSuiteGetter := cryptosuites.GetCurrentSuiteFromAuthPreference(authPrefGetter)
			manager, err := NewManager(ctx, &backendDesc.config, backendDesc.opts)
			require.NoError(t, err)

			// Delete all keys to clean up the test.
			t.Cleanup(func() {
				assert.NoError(t, manager.DeleteUnusedKeys(context.Background(), nil /*activeKeys*/))
			})

			sshKeyPair, err := manager.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
			require.NoError(t, err)
			sshPubKey, _, _, _, err := ssh.ParseAuthorizedKey(sshKeyPair.PublicKey)
			require.NoError(t, err)
			sshPub := sshPubKey.(ssh.CryptoPublicKey).CryptoPublicKey()
			expectedAlgorithm, err := cryptosuites.AlgorithmForKey(ctx, currentSuiteGetter, cryptosuites.UserCASSH)
			require.NoError(t, err)
			assertKeyAlgorithm(t, expectedAlgorithm, sshPub)

			tlsKeyPair, err := manager.NewTLSKeyPair(ctx, clusterName, cryptosuites.DatabaseClientCATLS)
			require.NoError(t, err)
			tlsCert, err := tlsca.ParseCertificatePEM(tlsKeyPair.Cert)
			require.NoError(t, err)
			expectedAlgorithm, err = cryptosuites.AlgorithmForKey(ctx, currentSuiteGetter, cryptosuites.DatabaseClientCATLS)
			require.NoError(t, err)
			assertKeyAlgorithm(t, expectedAlgorithm, tlsCert.PublicKey)

			jwtKeyPair, err := manager.NewJWTKeyPair(ctx, cryptosuites.JWTCAJWT)
			require.NoError(t, err)
			jwtPubKey, err := keys.ParsePublicKey(jwtKeyPair.PublicKey)
			require.NoError(t, err)
			expectedAlgorithm, err = cryptosuites.AlgorithmForKey(ctx, currentSuiteGetter, cryptosuites.JWTCAJWT)
			require.NoError(t, err)
			assertKeyAlgorithm(t, expectedAlgorithm, jwtPubKey)
		})
	}
}

func assertKeyAlgorithm(t *testing.T, expectedAlgorithm cryptosuites.Algorithm, pubKey crypto.PublicKey) {
	t.Helper()
	switch expectedAlgorithm {
	case cryptosuites.RSA2048:
		assert.IsType(t, &rsa.PublicKey{}, pubKey)
	case cryptosuites.ECDSAP256:
		assert.IsType(t, &ecdsa.PublicKey{}, pubKey)
	case cryptosuites.Ed25519:
		assert.IsType(t, ed25519.PublicKey{}, pubKey)
	default:
		t.Fatalf("test does not support algorithm %s", expectedAlgorithm.String())
	}
}

type testPack struct {
	backends []*backendDesc
	clock    *clockwork.FakeClock
}

type backendDesc struct {
	name                string
	config              servicecfg.KeystoreConfig
	opts                *Options
	backend             backend
	expectedKeyType     types.PrivateKeyType
	unusedRawKey        []byte
	deletionDoesNothing bool
}

func newTestPack(ctx context.Context, t *testing.T) *testPack {
	clock := clockwork.NewFakeClock()
	var backends []*backendDesc

	hostUUID := uuid.NewString()
	logger := utils.NewSlogLoggerForTests()

	unusedPKCS11Key, err := keyID{
		HostID: hostUUID,
		KeyID:  "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	}.marshal()
	require.NoError(t, err)

	_, gcpKMSDialer := newTestGCPKMSService(t)
	testGCPKMSClient := newTestGCPKMSClient(t, gcpKMSDialer)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: clusterName,
	})
	require.NoError(t, err)

	baseOpts := Options{
		ClusterName:          clusterName,
		HostUUID:             hostUUID,
		Logger:               logger,
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
		awsKMSClient:         newFakeAWSKMSService(t, clock, "123456789012", "us-west-2", 100),
		awsSTSClient: &fakeAWSSTSClient{
			account: "123456789012",
		},
		kmsClient:         testGCPKMSClient,
		clockworkOverride: clock,
	}

	softwareBackend := newSoftwareKeyStore(&softwareConfig{})
	backends = append(backends, &backendDesc{
		name:                "software",
		config:              servicecfg.KeystoreConfig{},
		opts:                &baseOpts,
		backend:             softwareBackend,
		unusedRawKey:        testRSA2048PrivateKeyPEM,
		deletionDoesNothing: true,
	})

	if config, ok := softHSMTestConfig(t); ok {
		backend, err := newPKCS11KeyStore(&config.PKCS11, &baseOpts)
		require.NoError(t, err)
		backends = append(backends, &backendDesc{
			name:            "softhsm",
			config:          config,
			opts:            &baseOpts,
			backend:         backend,
			expectedKeyType: types.PrivateKeyType_PKCS11,
			unusedRawKey:    unusedPKCS11Key,
		})
	}

	if config, ok := yubiHSMTestConfig(t); ok {
		backend, err := newPKCS11KeyStore(&config.PKCS11, &baseOpts)
		require.NoError(t, err)
		backends = append(backends, &backendDesc{
			name:            "yubihsm",
			config:          config,
			opts:            &baseOpts,
			backend:         backend,
			expectedKeyType: types.PrivateKeyType_PKCS11,
			unusedRawKey:    unusedPKCS11Key,
		})
	}

	if config, ok := cloudHSMTestConfig(t); ok {
		backend, err := newPKCS11KeyStore(&config.PKCS11, &baseOpts)
		require.NoError(t, err)
		backends = append(backends, &backendDesc{
			name:            "yubihsm",
			config:          config,
			opts:            &baseOpts,
			backend:         backend,
			expectedKeyType: types.PrivateKeyType_PKCS11,
			unusedRawKey:    unusedPKCS11Key,
		})
	}

	// Test with real GCP account if environment is enabled.
	if config, ok := gcpKMSTestConfig(t); ok {
		opts := baseOpts
		opts.kmsClient = nil

		backend, err := newGCPKMSKeyStore(ctx, &config.GCPKMS, &opts)
		require.NoError(t, err)
		backends = append(backends, &backendDesc{
			name:            "gcp_kms",
			config:          config,
			opts:            &opts,
			backend:         backend,
			expectedKeyType: types.PrivateKeyType_GCP_KMS,
			unusedRawKey: gcpKMSKeyID{
				keyVersionName: config.GCPKMS.KeyRing + "/cryptoKeys/FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF" + keyVersionSuffix,
			}.marshal(),
		})
	}
	// Always test with fake GCP client.
	fakeGCPKMSConfig := servicecfg.KeystoreConfig{
		GCPKMS: servicecfg.GCPKMSConfig{
			ProtectionLevel: "HSM",
			KeyRing:         "test-keyring",
		},
	}
	fakeGCPKMSBackend, err := newGCPKMSKeyStore(ctx, &fakeGCPKMSConfig.GCPKMS, &baseOpts)
	require.NoError(t, err)
	backends = append(backends, &backendDesc{
		name:            "fake_gcp_kms",
		config:          fakeGCPKMSConfig,
		opts:            &baseOpts,
		backend:         fakeGCPKMSBackend,
		expectedKeyType: types.PrivateKeyType_GCP_KMS,
		unusedRawKey: gcpKMSKeyID{
			keyVersionName: "test-keyring/cryptoKeys/FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF" + keyVersionSuffix,
		}.marshal(),
	})

	// Test AWS with and without multi-region keys
	for _, multiRegion := range []bool{false, true} {
		// Test with real AWS account if environment is enabled.
		if config, ok := awsKMSTestConfig(t); ok {
			config.AWSKMS.MultiRegion.Enabled = multiRegion
			opts := baseOpts
			// Unset the fake clients so this test can use the real AWS clients.
			opts.awsKMSClient = nil
			opts.awsSTSClient = nil

			backend, err := newAWSKMSKeystore(ctx, config.AWSKMS, &opts)
			require.NoError(t, err)
			name := "aws_kms"
			if multiRegion {
				name += "_multi_region"
			}
			backends = append(backends, &backendDesc{
				name:            name,
				config:          config,
				opts:            &opts,
				backend:         backend,
				expectedKeyType: types.PrivateKeyType_AWS_KMS,
				unusedRawKey: awsKMSKeyID{
					arn: arn.ARN{
						Partition: "aws",
						Service:   "kms",
						Region:    config.AWSKMS.AWSRegion,
						AccountID: config.AWSKMS.AWSAccount,
						Resource:  "unused",
					}.String(),
					account: config.AWSKMS.AWSAccount,
					region:  config.AWSKMS.AWSRegion,
				}.marshal(),
			})
		}

		// Always test with fake AWS client.
		fakeAWSKMSConfig := servicecfg.KeystoreConfig{
			AWSKMS: &servicecfg.AWSKMSConfig{
				AWSAccount: "123456789012",
				AWSRegion:  "us-west-2",
				MultiRegion: struct{ Enabled bool }{
					Enabled: multiRegion,
				},
			},
		}
		fakeAWSKMSBackend, err := newAWSKMSKeystore(ctx, fakeAWSKMSConfig.AWSKMS, &baseOpts)
		require.NoError(t, err)
		name := "fake_aws_kms"
		if multiRegion {
			name += "_multi_region"
		}
		backends = append(backends, &backendDesc{
			name:            name,
			config:          fakeAWSKMSConfig,
			opts:            &baseOpts,
			backend:         fakeAWSKMSBackend,
			expectedKeyType: types.PrivateKeyType_AWS_KMS,
			unusedRawKey: awsKMSKeyID{
				arn: arn.ARN{
					Partition: "aws",
					Service:   "kms",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Resource:  "unused",
				}.String(),
				account: "123456789012",
				region:  "us-west-2",
			}.marshal(),
		})
	}

	return &testPack{
		backends: backends,
		clock:    clock,
	}
}
