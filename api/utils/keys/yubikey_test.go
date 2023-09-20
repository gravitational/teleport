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

package keys

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestGetOrGenerateYubiKeyPrivateKey tests GetOrGenerateYubiKeyPrivateKey.
func TestGetOrGenerateYubiKeyPrivateKey(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()
	resetYubikey(ctx, t)

	// Generate a new YubiKeyPrivateKey.
	priv, err := GetOrGenerateYubiKeyPrivateKey(false)
	require.NoError(t, err)

	// Test creating a self signed certificate with the key.
	_, err = selfSignedTeleportClientCertificate(priv, priv.Public())
	require.NoError(t, err)

	// Another call to GetOrGenerateYubiKeyPrivateKey should retrieve the previously generated key.
	retrievePriv, err := GetOrGenerateYubiKeyPrivateKey(false)
	require.NoError(t, err)
	require.Equal(t, priv.Public(), retrievePriv.Public())

	// parsing the key's private key PEM should produce the same key as well.
	retrievePriv, err = ParsePrivateKey(priv.PrivateKeyPEM())
	require.NoError(t, err)
	require.Equal(t, priv.Public(), retrievePriv.Public())
}

func TestInvalidKey(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()
	resetYubikey(ctx, t)

	// Generate a key that does not require touch in the slot that Teleport expects to require touch.
	y, err := findYubiKey(0)
	require.NoError(t, err)
	touchRequiredSlot, err := getDefaultKeySlot(PrivateKeyPolicyHardwareKeyTouch)
	require.NoError(t, err)
	err = y.generatePrivateKey(touchRequiredSlot, PrivateKeyPolicyHardwareKey)
	require.NoError(t, err)

	// Attempting to retrieve the YubiKeyPrivateKey as a key with touch required should fail.
	_, err = GetOrGenerateYubiKeyPrivateKey(true)
	require.True(t, trace.IsCompareFailed(err), "Expected compare failed error but got %v", err)
}

// resetYubikey connects to the first yubiKey and resets it to defaults.
func resetYubikey(ctx context.Context, t *testing.T) {
	t.Helper()
	y, err := findYubiKey(0)
	require.NoError(t, err)
	yk, err := y.open()
	require.NoError(t, err)
	require.NoError(t, yk.Reset())
	require.NoError(t, yk.Close())
}
