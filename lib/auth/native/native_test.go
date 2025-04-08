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

package native

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestPrecomputeMode verifies that package enters precompute mode when
// PrecomputeKeys is called.
func TestPrecomputeMode(t *testing.T) {
	t.Parallel()

	PrecomputeKeys()

	select {
	case <-precomputedKeys:
	case <-time.After(time.Second * 10):
		t.Fatal("Key precompute routine failed to start.")
	}
}

// TestGenerateRSAPKSC1Keypair tests that GeneratePrivateKey generates
// a valid PKCS1 rsa key.
func TestGeneratePKSC1RSAKey(t *testing.T) {
	t.Parallel()

	priv, err := GeneratePrivateKey()
	require.NoError(t, err)

	block, rest := pem.Decode(priv.PrivateKeyPEM())
	require.NoError(t, err)
	require.Empty(t, rest)

	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
}

func TestGenerateEICEKey_when_boringbinary(t *testing.T) {
	if !IsBoringBinary() {
		t.Skip()
	}

	publicKey, privateKey, err := GenerateEICEKey()
	require.NoError(t, err)

	// We expect an RSA Key because boringcrypto doesn't yet support generating ED25519 keys.
	require.IsType(t, rsa.PublicKey{}, publicKey)
	require.IsType(t, rsa.PrivateKey{}, privateKey)
}

func TestGenerateEICEKey(t *testing.T) {
	if IsBoringBinary() {
		t.Skip()
	}

	publicKey, privateKey, err := GenerateEICEKey()
	require.NoError(t, err)

	// We expect an ED25519 key
	require.IsType(t, ed25519.PublicKey{}, publicKey)
	require.IsType(t, ed25519.PrivateKey{}, privateKey)
}
