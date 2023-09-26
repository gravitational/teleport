//go:build piv

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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
)

// TestHardwareSigner tests the HardwareSigner interface with hardware keys.
func TestHardwareSigner(t *testing.T) {
	// The rest of the  test expects a yubiKey to be connected with default PIV settings
	// and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()
	resetYubikey(ctx, t)

	// Generate a new YubiKeyPrivateKey. It should return a valid attestation statement and key policy.
	priv, err := keys.GetOrGenerateYubiKeyPrivateKey(false)
	require.NoError(t, err)

	att, err := keys.GetAttestationStatement(priv)
	require.NoError(t, err)
	require.NotNil(t, att)

	policy := keys.GetPrivateKeyPolicy(priv)
	require.Equal(t, keys.PrivateKeyPolicyHardwareKey, policy)
}

// TestNonHardwareSigner tests the HardwareSigner interface with non-hardware keys.
func TestNonHardwareSigner(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	key, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)

	att, err := keys.GetAttestationStatement(key)
	require.NoError(t, err)
	require.Nil(t, att)

	policy := keys.GetPrivateKeyPolicy(key)
	require.Equal(t, keys.PrivateKeyPolicyNone, policy)
}
