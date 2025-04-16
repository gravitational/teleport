//go:build piv

// Copyright 2025 Gravitational, Inc.
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

package piv_test

import (
	"bytes"
	"context"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"testing"

	pivgo "github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/piv"
	"github.com/gravitational/teleport/api/utils/prompt"
)

// TestGetYubiKeyPrivateKey_Interactive tests generation and retrieval of YubiKey private keys.
func TestGetYubiKeyPrivateKey_Interactive(t *testing.T) {
	// This test will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	if !testing.Verbose() {
		t.Fatal("This test is interactive and must be called with the -v verbose flag to see touch prompts.")
	}
	fmt.Println("This test is interactive, tap your YubiKey when prompted.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	promptReader := prompt.NewFakeReader()
	prompt := hardwarekey.NewCLIPrompt(os.Stderr, promptReader)
	s := piv.NewYubiKeyService(prompt)

	y, err := piv.FindYubiKey(0)
	require.NoError(t, err)

	resetYubikey(t, y)
	t.Cleanup(func() { resetYubikey(t, y) })

	// Warmup the hardware key to prompt touch at the start of the test,
	// rather than having this interaction later.
	priv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicy{TouchRequired: true},
	})
	require.NoError(t, err)
	require.NoError(t, priv.WarmupHardwareKey(ctx))

	// Set pin and handle expected prompts.
	setupPINPrompt := func(t *testing.T) {
		const testPIN = "123123"
		require.NoError(t, y.SetPIN(pivgo.DefaultPIN, testPIN))
		promptReader.AddString(testPIN).AddString(testPIN)
	}

	for _, policy := range []hardwarekey.PromptPolicy{
		hardwarekey.PromptPolicyNone,
		hardwarekey.PromptPolicyTouch,
		hardwarekey.PromptPolicyPIN,
		hardwarekey.PromptPolicyTouchAndPIN,
	} {
		for _, customSlot := range []bool{true, false} {
			t.Run(fmt.Sprintf("policy:%+v", policy), func(t *testing.T) {
				t.Run(fmt.Sprintf("custom slot:%v", customSlot), func(t *testing.T) {
					setupPINPrompt(t)
					t.Cleanup(func() {
						resetYubikey(t, y)
					})

					var slot hardwarekey.PIVSlotKeyString = ""
					if customSlot {
						slot = "9a"
					}

					// NewHardwarePrivateKey should generate a new hardware private key.
					priv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
						CustomSlot: slot,
						Policy:     policy,
					})
					require.NoError(t, err)

					// test HardwareSigner methods
					require.Equal(t, policy, priv.GetPrivateKeyPolicy().GetPromptPolicy())
					require.NotNil(t, priv.GetAttestationStatement())
					require.True(t, priv.IsHardware())

					// Test bogus sign (warmup).
					require.NoError(t, priv.WarmupHardwareKey(ctx))

					// Another call to NewHardwarePrivateKey should retrieve the previously generated key.
					retrievePriv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
						CustomSlot: slot,
						Policy:     policy,
					})
					require.NoError(t, err)
					require.Equal(t, priv.Public(), retrievePriv.Public())

					// parsing the key's private key PEM should produce the same key as well.
					retrievePriv, err = keys.ParsePrivateKey(priv.PrivateKeyPEM(), keys.WithHardwareKeyService(s))
					require.NoError(t, err)
					require.Equal(t, priv.Public(), retrievePriv.Public())
				})
			})
		}
	}
}

func TestOverwritePrompt(t *testing.T) {
	// This test will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()

	promptWriter := bytes.NewBuffer([]byte{})
	promptReader := prompt.NewFakeReader()
	prompt := hardwarekey.NewCLIPrompt(promptWriter, promptReader)
	s := piv.NewYubiKeyService(prompt)

	y, err := piv.FindYubiKey(0)
	require.NoError(t, err)

	resetYubikey(t, y)
	t.Cleanup(func() { resetYubikey(t, y) })

	// Get the default slot used for hardware_key_touch.
	touchSlot := pivgo.SlotSignature

	testOverwritePrompt := func(t *testing.T) {
		// Fail to overwrite slot when user denies
		promptReader.AddString("n")
		_, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
			Policy: hardwarekey.PromptPolicy{TouchRequired: true},
		})
		require.True(t, trace.IsCompareFailed(err), "Expected compare failed error but got %v", err)

		// Successfully overwrite slot when user accepts
		promptReader.AddString("y")
		_, err = keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
			Policy: hardwarekey.PromptPolicy{TouchRequired: true},
		})
		require.NoError(t, err)
	}

	t.Run("invalid metadata cert", func(t *testing.T) {
		resetYubikey(t, y)

		// Set a non-teleport certificate in the slot.
		err = y.SetMetadataCertificate(touchSlot, pkix.Name{Organization: []string{"not-teleport"}})
		require.NoError(t, err)

		testOverwritePrompt(t)
	})

	t.Run("invalid key policies", func(t *testing.T) {
		resetYubikey(t, y)

		// Generate a key that does not require touch in the slot that Teleport expects to require touch.
		_, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
			CustomSlot: hardwarekey.PIVSlotKeyString(touchSlot.String()),
			Policy:     hardwarekey.PromptPolicy{TouchRequired: false},
		})
		require.NoError(t, err)

		testOverwritePrompt(t)
	})
}

// resetYubikey connects to the first yubiKey and resets it to defaults.
func resetYubikey(t *testing.T, y *piv.YubiKey) {
	t.Helper()
	require.NoError(t, y.Reset())
}
