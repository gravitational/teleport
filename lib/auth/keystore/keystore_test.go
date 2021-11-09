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

package keystore_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509/pkix"
	"log"
	"os"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var (
	testRawPrivateKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
	testRawPublicKey = []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCqIPatEnmSruE/nl44Iz0O12RY8wjw8EqDFoCJD0n1gXdo26v3xvyQAJxNDyLFKQewtxxWpF238KdvuO0FKvDEn0E3T19kSLIK/LuXDE0YExARqi/1RTvbae+5Ypv4SQfNxVX2J72LzAzzzGnPNg7x51rwVmMfFpqh7aWSseKI4VBG5smodWFb5I0VA5Xo6xURNNmWDmuZaEDmsqIHobRB4sfKxIwltssw5evVVu7tGqiGarQAXoR0yCLHc4nPeov1gMpA8DOGPtWI/NPTs+//2+Hl+NdoTmJOE9Piffe5jU3Z8kCfOxxm9WanHG5I6rHBYGqRHMgl7PW+/cX7nEMv")
	testRawCert      = []byte(`-----BEGIN CERTIFICATE-----
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
		PublicKey:      testRawPublicKey,
		PrivateKey:     testRawPrivateKey,
		PrivateKeyType: types.PrivateKeyType_RAW,
	}
	testRawTLSKeyPair = &types.TLSKeyPair{
		Cert:    testRawCert,
		Key:     testRawPrivateKey,
		KeyType: types.PrivateKeyType_RAW,
	}
	testRawJWTKeyPair = &types.JWTKeyPair{
		PublicKey:      testRawPublicKey,
		PrivateKey:     testRawPrivateKey,
		PrivateKeyType: types.PrivateKeyType_RAW,
	}

	testPKCS11SSHKeyPair = &types.SSHKeyPair{
		PublicKey:      testRawPublicKey,
		PrivateKey:     testPKCS11Key,
		PrivateKeyType: types.PrivateKeyType_PKCS11,
	}
	testPKCS11TLSKeyPair = &types.TLSKeyPair{
		Cert:    testRawCert,
		Key:     testPKCS11Key,
		KeyType: types.PrivateKeyType_PKCS11,
	}
	testPKCS11JWTKeyPair = &types.JWTKeyPair{
		PublicKey:      testRawPublicKey,
		PrivateKey:     testPKCS11Key,
		PrivateKeyType: types.PrivateKeyType_PKCS11,
	}
)

func TestKeyStore(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(keystore.TestModules{})

	skipSoftHSM := os.Getenv("SOFTHSM2_PATH") == ""
	var softHSMConfig keystore.Config
	if !skipSoftHSM {
		softHSMConfig = keystore.SetupSoftHSMTest(t)
		softHSMConfig.HostUUID = "server1"
	}

	yubiSlotNumber := 0
	testcases := []struct {
		desc       string
		config     keystore.Config
		isRaw      bool
		shouldSkip func() bool
	}{
		{
			desc: "raw keystore",
			config: keystore.Config{
				RSAKeyPairSource: native.GenerateKeyPair,
			},
			isRaw:      true,
			shouldSkip: func() bool { return false },
		},
		{
			desc:   "softhsm",
			config: softHSMConfig,
			shouldSkip: func() bool {
				if skipSoftHSM {
					log.Println("Skipping softhsm test because SOFTHSM2_PATH is not set.")
					return true
				}
				return false
			},
		},
		{
			desc: "yubihsm",
			config: keystore.Config{
				Path:       os.Getenv("YUBIHSM_PKCS11_PATH"),
				SlotNumber: &yubiSlotNumber,
				Pin:        "0001password",
				HostUUID:   "server1",
			},
			shouldSkip: func() bool {
				if os.Getenv("YUBIHSM_PKCS11_CONF") == "" || os.Getenv("YUBIHSM_PKCS11_PATH") == "" {
					log.Println("Skipping yubihsm test because YUBIHSM_PKCS11_CONF or YUBIHSM_PKCS11_PATH is not set.")
					return true
				}
				return false
			},
		},
		{
			desc: "cloudhsm",
			config: keystore.Config{
				Path:       "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
				TokenLabel: "cavium",
				Pin:        os.Getenv("CLOUDHSM_PIN"),
				HostUUID:   "server1",
			},
			shouldSkip: func() bool {
				if os.Getenv("CLOUDHSM_PIN") == "" {
					log.Println("Skipping cloudhsm test because CLOUDHSM_PIN is not set.")
					return true
				}
				return false
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			if tc.shouldSkip() {
				t.SkipNow()
				return
			}

			// create the keystore
			keyStore, err := keystore.NewKeyStore(tc.config)
			require.NoError(t, err)

			// create a key
			key, signer, err := keyStore.GenerateRSA()
			require.NoError(t, err)
			require.NotNil(t, key)
			require.NotNil(t, signer)

			// delete the key when we're done with it
			t.Cleanup(func() { require.NoError(t, keyStore.DeleteKey(key)) })

			// get a signer from the key
			signer, err = keyStore.GetSigner(key)
			require.NoError(t, err)
			require.NotNil(t, signer)

			// try signing something
			message := []byte("Lorem ipsum dolor sit amet...")
			hashed := sha256.Sum256(message)
			signature, err := signer.Sign(rand.Reader, hashed[:], crypto.SHA256)
			require.NoError(t, err)
			require.NotEmpty(t, signature)
			// make sure we can verify the signature with a "known good" rsa implementation
			err = rsa.VerifyPKCS1v15(signer.Public().(*rsa.PublicKey), crypto.SHA256, hashed[:], signature)
			require.NoError(t, err)

			// make sure we can get the ssh public key
			sshSigner, err := ssh.NewSignerFromSigner(signer)
			require.NoError(t, err)
			sshPublicKey := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())

			// make sure we can get a tls cert
			tlsCert, err := tlsca.GenerateSelfSignedCAWithSigner(
				signer,
				pkix.Name{
					CommonName:   "server1",
					Organization: []string{"server1"},
				}, nil, defaults.CATTL)
			require.NoError(t, err)
			require.NotNil(t, tlsCert)

			// test CA with multiple active keypairs
			ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: "example.com",
				ActiveKeys: types.CAKeySet{
					SSH: []*types.SSHKeyPair{
						testPKCS11SSHKeyPair,
						&types.SSHKeyPair{
							PrivateKey:     key,
							PrivateKeyType: keystore.KeyType(key),
							PublicKey:      sshPublicKey,
						},
					},
					TLS: []*types.TLSKeyPair{
						testPKCS11TLSKeyPair,
						&types.TLSKeyPair{
							Key:     key,
							KeyType: keystore.KeyType(key),
							Cert:    tlsCert,
						},
					},
					JWT: []*types.JWTKeyPair{
						testPKCS11JWTKeyPair,
						&types.JWTKeyPair{
							PrivateKey:     key,
							PrivateKeyType: keystore.KeyType(key),
							PublicKey:      sshPublicKey,
						},
					},
				},
			})
			require.NoError(t, err)

			// test that keyStore is able to select the correct key and get a signer
			sshSigner, err = keyStore.GetSSHSigner(ca)
			require.NoError(t, err)
			require.NotNil(t, sshSigner)

			tlsCert, tlsSigner, err := keyStore.GetTLSCertAndSigner(ca)
			require.NoError(t, err)
			require.NotNil(t, tlsCert)
			require.NotEqual(t, testPKCS11TLSKeyPair.Cert, tlsCert)
			require.NotNil(t, tlsSigner)

			jwtSigner, err := keyStore.GetJWTSigner(ca)
			require.NoError(t, err)
			require.NotNil(t, jwtSigner)

			// test CA with only raw keys
			ca, err = types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: "example.com",
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

			if !tc.isRaw {
				// hsm keyStore should not get any signer from raw keys
				_, err = keyStore.GetSSHSigner(ca)
				require.True(t, trace.IsNotFound(err))

				_, _, err = keyStore.GetTLSCertAndSigner(ca)
				require.True(t, trace.IsNotFound(err))

				_, err = keyStore.GetJWTSigner(ca)
				require.True(t, trace.IsNotFound(err))
			} else {
				// raw keyStore should be able to get a signer
				sshSigner, err = keyStore.GetSSHSigner(ca)
				require.NoError(t, err)
				require.NotNil(t, sshSigner)

				tlsCert, tlsSigner, err = keyStore.GetTLSCertAndSigner(ca)
				require.NoError(t, err)
				require.NotNil(t, tlsCert)
				require.NotNil(t, tlsSigner)

				jwtSigner, err = keyStore.GetJWTSigner(ca)
				require.NoError(t, err)
				require.NotNil(t, jwtSigner)
			}
		})
	}
}

func TestLicenseRequirement(t *testing.T) {
	// we need the SoftHSM2 tests to be enabled so that the HSM keystore can be
	// selected
	if os.Getenv("SOFTHSM2_PATH") == "" {
		t.SkipNow()
	}

	config := keystore.SetupSoftHSMTest(t)
	config.HostUUID = "server1"

	// should fail to create the keystore with default modules
	_, err := keystore.NewKeyStore(config)
	require.Error(t, err)

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(keystore.TestModules{})

	// should succeed when HSM feature is enabled
	_, err = keystore.NewKeyStore(config)
	require.NoError(t, err)
}
