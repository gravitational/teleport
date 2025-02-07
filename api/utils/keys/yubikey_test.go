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
	"crypto/rand"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"testing"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/prompt"
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

	y, err := keys.FindYubiKey(0, nil)
	require.NoError(t, err)

	t.Cleanup(func() { resetYubikey(t, y) })

	for _, policy := range []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKey,
		keys.PrivateKeyPolicyHardwareKeyTouch,
		keys.PrivateKeyPolicyHardwareKeyPIN,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
	} {
		for _, customSlot := range []bool{true, false} {
			t.Run(fmt.Sprintf("policy:%q", policy), func(t *testing.T) {
				t.Run(fmt.Sprintf("custom slot:%v", customSlot), func(t *testing.T) {
					resetYubikey(t, y)
					setupPINPrompt(t, y)

					var slot keys.PIVSlot = ""
					if customSlot {
						slot = "9a"
					}

					// GetYubiKeyPrivateKey should generate a new YubiKeyPrivateKey.
					priv, err := keys.NewYubiKeyPrivateKey(ctx, policy, slot, nil)
					require.NoError(t, err)

					// test HardwareSigner methods
					require.Equal(t, policy, priv.GetPrivateKeyPolicy())
					require.NotNil(t, priv.GetAttestationStatement())

					// Test Sign.
					digest := []byte{100}
					_, err = priv.Sign(rand.Reader, digest, nil)
					require.NoError(t, err)

					// Another call to GetYubiKeyPrivateKey should retrieve the previously generated key.
					retrievePriv, err := keys.NewYubiKeyPrivateKey(ctx, policy, slot, nil)
					require.NoError(t, err)
					require.Equal(t, priv.Public(), retrievePriv.Public())

					// parsing the key's private key PEM should produce the same key as well.
					retrievePriv, err = keys.ParsePrivateKey(priv.PrivateKeyPEM())
					require.NoError(t, err)
					require.Equal(t, priv.Public(), retrievePriv.Public())
				})
			})
		}
	}
}

func TestOverwritePrompt(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()

	y, err := keys.FindYubiKey(0, nil)
	require.NoError(t, err)

	t.Cleanup(func() { resetYubikey(t, y) })

	// Use a custom slot.
	pivSlot, err := keys.GetDefaultKeySlot(keys.PrivateKeyPolicyHardwareKeyTouch)
	require.NoError(t, err)

	testOverwritePrompt := func(t *testing.T) {
		// Fail to overwrite slot when user denies
		prompt.SetStdin(prompt.NewFakeReader().AddString("n"))
		_, err := keys.NewYubiKeyPrivateKey(ctx, keys.PrivateKeyPolicyHardwareKeyTouch, "" /* slot */, nil)
		require.True(t, trace.IsCompareFailed(err), "Expected compare failed error but got %v", err)

		// Successfully overwrite slot when user accepts
		prompt.SetStdin(prompt.NewFakeReader().AddString("y"))
		_, err = keys.NewYubiKeyPrivateKey(ctx, keys.PrivateKeyPolicyHardwareKeyTouch, "" /* slot */, nil)
		require.NoError(t, err)
	}

	t.Run("invalid metadata cert", func(t *testing.T) {
		resetYubikey(t, y)

		// Set a non-teleport certificate in the slot.
		err = y.SetMetadataCertificate(pivSlot, pkix.Name{Organization: []string{"not-teleport"}})
		require.NoError(t, err)

		testOverwritePrompt(t)
	})

	t.Run("invalid key policies", func(t *testing.T) {
		resetYubikey(t, y)

		// Generate a key that does not require touch in the slot that Teleport expects to require touch.
		_, err := keys.NewYubiKeyPrivateKey(ctx, keys.PrivateKeyPolicyHardwareKey, keys.PIVSlot(pivSlot.String()), nil)
		require.NoError(t, err)

		testOverwritePrompt(t)
	})
}

// resetYubikey connects to the first yubiKey and resets it to defaults.
func resetYubikey(t *testing.T, y *keys.YubiKey) {
	t.Helper()
	require.NoError(t, y.Reset())
}

func setupPINPrompt(t *testing.T, y *keys.YubiKey) {
	t.Helper()

	// Set pin for tests.
	const testPIN = "123123"
	require.NoError(t, y.SetPIN(piv.DefaultPIN, testPIN))

	// Handle PIN prompt.
	oldStdin := prompt.Stdin()
	t.Cleanup(func() { prompt.SetStdin(oldStdin) })
	prompt.SetStdin(prompt.NewFakeReader().AddString(testPIN).AddString(testPIN))
}
