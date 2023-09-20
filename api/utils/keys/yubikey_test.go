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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetYubiKeyPrivateKey_Interactive tests generation and retrieval of YubiKey private keys.
func TestGetYubiKeyPrivateKey_Interactive(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	if !testing.Verbose() {
		t.Fatal("This test is interactive and must be called with the -v verbose flag to see touch prompts.")
	}
	fmt.Println("This test is interactive, tap your YubiKey when prompted.")

	ctx := context.Background()
	resetYubikey(ctx, t)

	for _, policy := range []PrivateKeyPolicy{
		PrivateKeyPolicyHardwareKey,
		PrivateKeyPolicyHardwareKeyTouch,
	} {
		t.Run(fmt.Sprintf("policy:%q", policy), func(t *testing.T) {
			t.Cleanup(func() { resetYubikey(ctx, t) })

			// GetYubiKeyPrivateKey should generate a new YubiKeyPrivateKey.
			priv, err := GetOrGenerateYubiKeyPrivateKey(policy == PrivateKeyPolicyHardwareKeyTouch)
			require.NoError(t, err)

			// Test Sign.
			_, err = selfSignedTeleportClientCertificate(priv, priv.Public())
			require.NoError(t, err)

			// Another call to GetYubiKeyPrivateKey should retrieve the previously generated key.
			retrievePriv, err := GetOrGenerateYubiKeyPrivateKey(policy == PrivateKeyPolicyHardwareKeyTouch)
			require.NoError(t, err)
			require.Equal(t, priv.Public(), retrievePriv.Public())

			// parsing the key's private key PEM should produce the same key as well.
			retrievePriv, err = ParsePrivateKey(priv.PrivateKeyPEM())
			require.NoError(t, err)
			require.Equal(t, priv.Public(), retrievePriv.Public())
		})
	}
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
