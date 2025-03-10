/*
Copyright 2022 Gravitational, Inc.

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

package keys_test

import (
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/api/utils/keys"
)

// TestNonHardwareSigner tests the HardwareSigner interface with non-hardware keys.
//
// HardwareSigners require the piv go tag and should be tested individually in tests
// like `TestGetYubiKeyPrivateKey_Interactive`.
func TestNonHardwareSigner(t *testing.T) {
	priv, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	key, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)

	require.Nil(t, key.GetAttestationStatement())
	require.Equal(t, keys.PrivateKeyPolicyNone, key.GetPrivateKeyPolicy())
}
