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

package hsm_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/hsm"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestHSM(t *testing.T) {
	yubiSlotNumber := 0
	testcases := []struct {
		desc         string
		clientConfig hsm.ClientConfig
		shouldSkip   func() bool
		setup        func(t *testing.T)
	}{
		{
			desc: "null client",
			clientConfig: hsm.ClientConfig{
				RSAKeyPairSource: native.GenerateKeyPair,
			},
			shouldSkip: func() bool { return false },
		},
		{
			desc: "softhsm",
			clientConfig: hsm.ClientConfig{
				Path:       os.Getenv("SOFTHSM2_PATH"),
				TokenLabel: "test",
				Pin:        "password",
				HostUUID:   "server1",
			},
			shouldSkip: func() bool {
				if os.Getenv("SOFTHSM2_PATH") == "" {
					log.Println("Skipping softhsm test because SOFTHSM2_PATH is not set.")
					return true
				}
				return false
			},
			setup: func(t *testing.T) {
				// create tokendir
				tokenDir, err := os.MkdirTemp("", "softhsm2-tokendir")
				require.NoError(t, err)

				// create config file
				configFile, err := os.CreateTemp("", "softhsm2.conf")
				os.Setenv("SOFTHSM2_CONF", configFile.Name())

				// write config file
				_, err = configFile.WriteString(fmt.Sprintf(
					"directories.tokendir = %s\nobjectstore.backend = file\nlog.level = DEBUG\n",
					tokenDir))
				require.NoError(t, err)
				require.NoError(t, configFile.Close())

				// create test token
				cmd := exec.Command("softhsm2-util", "--init-token", "--slot", "0", "--label", "test", "--so-pin", "password", "--pin", "password")
				require.NoError(t, cmd.Run())

				t.Cleanup(func() {
					require.NoError(t, os.Remove(configFile.Name()))
					require.NoError(t, os.RemoveAll(tokenDir))
				})
			},
		},
		{
			desc: "yubihsm",
			clientConfig: hsm.ClientConfig{
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
			clientConfig: hsm.ClientConfig{
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
				return
			}
			t.Parallel()

			if tc.setup != nil {
				tc.setup(t)
			}

			client, err := hsm.NewClient(&tc.clientConfig)
			require.NoError(t, err)

			key, signer, err := client.GenerateRSA()
			require.NoError(t, err)
			require.NotNil(t, key)
			require.NotNil(t, signer)

			signer, err = client.GetSigner(key)
			require.NoError(t, err)
			require.NotNil(t, signer)

			message := []byte("Lorem ipsum dolor sit amet...")
			hashed := sha256.Sum256(message)

			signature, err := signer.Sign(rand.Reader, hashed[:], crypto.SHA256)
			require.NoError(t, err)
			require.NotEmpty(t, signature)

			err = rsa.VerifyPKCS1v15(signer.Public().(*rsa.PublicKey), crypto.SHA256, hashed[:], signature)
			require.NoError(t, err)

			sshSigner, err := ssh.NewSignerFromSigner(signer)
			require.NoError(t, err)
			sshPublicKey := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())

			ca := &types.CertAuthorityV2{
				Kind:    types.KindCertAuthority,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "server1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.CertAuthoritySpecV2{
					ClusterName: "server1",
					ActiveKeys: types.CAKeySet{
						SSH: []*types.SSHKeyPair{
							&types.SSHKeyPair{
								PrivateKey:     key,
								PrivateKeyType: hsm.KeyType(key),
								PublicKey:      sshPublicKey,
							},
						},
						TLS: []*types.TLSKeyPair{
							&types.TLSKeyPair{
								Key:     key,
								KeyType: hsm.KeyType(key),
							},
						},
						JWT: []*types.JWTKeyPair{
							&types.JWTKeyPair{
								PrivateKey:     key,
								PrivateKeyType: hsm.KeyType(key),
								PublicKey:      sshPublicKey,
							},
						},
					},
				},
			}

			sshSigner, err = client.GetSSHSigner(ca)
			require.NoError(t, err)
			require.NotNil(t, sshSigner)

			_, tlsSigner, err := client.GetTLSCertAndSigner(ca)
			require.NoError(t, err)
			require.NotNil(t, tlsSigner)

			jwtSigner, err := client.GetJWTSigner(ca, clockwork.NewFakeClock())
			require.NoError(t, err)
			require.NotNil(t, jwtSigner)
		})
	}
}
