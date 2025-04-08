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

package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalJWK(t *testing.T) {
	pubBytes, _, err := GenerateKeyPair()
	require.NoError(t, err)

	jwk, err := MarshalJWK(pubBytes)
	require.NoError(t, err)

	// Required for integrating with AWS OpenID Connect Identity Provider.
	require.Equal(t, "sig", jwk.Use)
}

func TestKeyIDHasConsistentOutputForAnInput(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	publicKey := privateKey.Public().(*rsa.PublicKey)
	id1 := KeyID(publicKey)
	id2 := KeyID(publicKey)
	require.NotEmpty(t, id1)
	require.Equal(t, id1, id2)

	expectedLength := base64.RawURLEncoding.EncodedLen(sha256.Size)
	require.Len(t, id1, expectedLength, "expected key id to always be %d characters long", expectedLength)
}

func TestKeyIDHasDistinctOutputForDifferingInputs(t *testing.T) {
	t.Parallel()

	privateKey1, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	privateKey2, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	publicKey1 := privateKey1.Public().(*rsa.PublicKey)
	publicKey2 := privateKey2.Public().(*rsa.PublicKey)
	id1 := KeyID(publicKey1)
	id2 := KeyID(publicKey2)
	require.NotEmpty(t, id1)
	require.NotEmpty(t, id2)
	require.NotEqual(t, id1, id2)
}

// TestKeyIDCompatibility ensures we do not introduce a change in the KeyID algorithm for existing keys.
// It does so by ensuring that a pre-generated public key results in the expected value.
func TestKeyIDCompatibility(t *testing.T) {
	n, ok := new(big.Int).
		SetString("10804584566601725083798733714540307814537881454603593919227265169397611763416631197061041949793088023127406259586903197568870611092333639226643589004457719", 10)
	require.True(t, ok, "failed to create a bigint")
	publicKey := &rsa.PublicKey{
		E: 65537,
		N: n,
	}
	require.Equal(t, "GDLHLDvPUYmNLVU3WgshDX7bAw8xEmML8ypeE9KRAEQ", KeyID(publicKey))
}
