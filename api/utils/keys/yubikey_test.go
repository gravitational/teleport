//go:build libpcsclite
// +build libpcsclite

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

	"github.com/stretchr/testify/require"
)

// TestGetOrGenerateYubiKeyPrivateKey tests GetOrGenerateYubiKeyPrivateKey.
func TestGetOrGenerateYubiKeyPrivateKey(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()

	// Connect to the first yubiKey and reset it.
	y, err := findYubiKey(ctx, 0)
	require.NoError(t, err)
	yk, err := y.open(ctx)
	require.NoError(t, err)
	require.NoError(t, yk.Reset())
	require.NoError(t, yk.Close())

	// Generate a new YubiKeyPrivateKey.
	priv, err := GetOrGenerateYubiKeyPrivateKey(ctx, false)
	require.NoError(t, err)

	// Test creating a self signed certificate with the key.
	_, err = selfSignedTeleportClientCertificate(priv, priv.Public())
	require.NoError(t, err)

	// Another call to GetOrGenerateYubiKeyPrivateKey should retrieve the previously generated key.
	retrievePriv, err := GetOrGenerateYubiKeyPrivateKey(ctx, false)
	require.NoError(t, err)
	require.Equal(t, priv, retrievePriv)

	// parsing the key's private key PEM should produce the same key as well.
	retrieveKey, err := ParsePrivateKey(priv.PrivateKeyPEM())
	require.NoError(t, err)
	require.Equal(t, priv, retrieveKey)
}
