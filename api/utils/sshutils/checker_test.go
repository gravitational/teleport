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
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
)

// TestCheckerValidate checks what algorithm are supported in regular (non-FIPS) mode.
func TestCheckerValidate(t *testing.T) {
	checker := CertChecker{}

	rsaKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)
	smallRSAKey, err := cryptopatch.GenerateRSAKey(rand.Reader, 1024)
	require.NoError(t, err)
	ellipticKey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// 2048-bit RSA keys are valid.
	cryptoKey := rsaKey.Public()
	sshKey, err := ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.NoError(t, err)

	// 1024-bit RSA keys are valid.
	cryptoKey = smallRSAKey.Public()
	sshKey, err = ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.NoError(t, err)

	// ECDSA keys are valid.
	cryptoKey = ellipticKey.Public()
	sshKey, err = ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.NoError(t, err)
}

// TestCheckerValidateFIPS makes sure the public key is a valid algorithm
// that Teleport supports while in FIPS mode.
func TestCheckerValidateFIPS(t *testing.T) {
	checker := CertChecker{
		FIPS: true,
	}

	rsaKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)
	smallRSAKey, err := cryptopatch.GenerateRSAKey(rand.Reader, 1024)
	require.NoError(t, err)
	ellipticKey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// 2048-bit RSA keys are valid.
	cryptoKey := rsaKey.Public()
	sshKey, err := ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.NoError(t, err)

	// 1024-bit RSA keys are not valid.
	cryptoKey = smallRSAKey.Public()
	sshKey, err = ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.Error(t, err)

	// ECDSA keys are not valid.
	cryptoKey = ellipticKey.Public()
	sshKey, err = ssh.NewPublicKey(cryptoKey)
	require.NoError(t, err)
	err = checker.validateFIPS(sshKey)
	require.Error(t, err)
}
