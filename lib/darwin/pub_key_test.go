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

package darwin_test

import (
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/lib/darwin"
)

func TestECDSAPublicKeyFromRaw(t *testing.T) {
	privKey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "GenerateKey failed")

	pubKey := privKey.PublicKey

	// Marshal key into the raw Apple format.
	rawAppleKey := make([]byte, 1+32+32)
	rawAppleKey[0] = 0x04
	pubKey.X.FillBytes(rawAppleKey[1:33])
	pubKey.Y.FillBytes(rawAppleKey[33:])

	got, err := darwin.ECDSAPublicKeyFromRaw(rawAppleKey)
	require.NoError(t, err, "ECDSAPublicKeyFromRaw failed")
	assert.Equal(t, pubKey, *got, "ECDSAPublicKeyFromRaw mismatch")
}
