/*
Copyright 2019-2021 Gravitational, Inc.

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

package sshutils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
)

// TestCheckerValidateFIPS makes sure the public key is a valid algorithm
// that Teleport supports while in FIPS mode.
func TestCheckerValidateFIPS(t *testing.T) {
	regularChecker := CertChecker{}
	fipsChecker := CertChecker{
		FIPS: true,
	}

	//nolint:forbidigo // Generating RSA keys allowed for key check test.
	rsaKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)
	//nolint:forbidigo // Generating RSA keys allowed for key check test.
	smallRSAKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	ellipticKeyP224, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(t, err)
	ellipticKeyP256, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ellipticKeyP384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	for _, tc := range []struct {
		desc            string
		key             crypto.Signer
		expectSSHError  bool
		expectFIPSError bool
	}{
		{
			desc: "RSA2048",
			key:  rsaKey,
		},
		{
			desc:            "RSA1024",
			key:             smallRSAKey,
			expectFIPSError: true,
		},
		{
			desc:            "ECDSAP224",
			key:             ellipticKeyP224,
			expectSSHError:  true,
			expectFIPSError: true,
		},
		{
			desc: "ECDSAP256",
			key:  ellipticKeyP256,
		},
		{
			desc: "ECDSAP384",
			key:  ellipticKeyP384,
		},
		{
			desc:            "Ed25519",
			key:             ed25519Key,
			expectFIPSError: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cryptoKey := tc.key.Public()
			sshKey, err := ssh.NewPublicKey(cryptoKey)
			if tc.expectSSHError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			err = regularChecker.validateFIPS(sshKey)
			assert.NoError(t, err)

			err = fipsChecker.validateFIPS(sshKey)
			if tc.expectFIPSError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
