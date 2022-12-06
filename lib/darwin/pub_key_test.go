// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package darwin_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/darwin"
)

func TestECDSAPublicKeyFromRaw(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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
