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
	"sync"
	"testing"
	"time"

	pivgo "github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/piv"
	"github.com/gravitational/teleport/api/utils/prompt"
)

// TestNewHardwarePrivateKey_Interactive tests generation and retrieval of YubiKey private keys.
func TestNewHardwarePrivateKey_Interactive(t *testing.T) {
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

	t.Cleanup(func() { resetYubikey(t, y) })

	// Test creating a new key, retrieving it, and using it for signatures.
	testNewPrivateKey := func(t *testing.T, config hardwarekey.PrivateKeyConfig) {
		t.Helper()

		// NewHardwarePrivateKey should generate a new hardware private key.
		priv, err := keys.NewHardwarePrivateKey(ctx, s, config)
		require.NoError(t, err)
		hwSigner, ok := priv.Signer.(*hardwarekey.Signer)
		require.True(t, ok, "expected hardwarekey.Signer but got %T", priv.Signer)

		// Check that config was applied correctly.
		require.Equal(t, config.Policy, hwSigner.Ref.Policy)
		if config.CustomSlot != "" {
			expectSlot, err := config.CustomSlot.Parse()
			require.NoError(t, err)
			require.Equal(t, expectSlot, hwSigner.Ref.SlotKey)
		}

		// test Hardware Key methods
		require.Equal(t, config.Policy, priv.GetPrivateKeyPolicy().GetPromptPolicy())
		require.NotNil(t, priv.GetAttestationStatement())
		require.True(t, priv.IsHardware())

		// Test bogus sign (warmup).
		require.NoError(t, priv.WarmupHardwareKey(ctx))

		// Another call to NewHardwarePrivateKey should retrieve the previously generated key.
		retrievePriv, err := keys.NewHardwarePrivateKey(ctx, s, config)
		require.NoError(t, err)
		require.Equal(t, priv.Public(), retrievePriv.Public())

		// parsing the key's private key PEM should produce the same key as well.
		retrievePriv, err = keys.ParsePrivateKey(priv.PrivateKeyPEM(), keys.WithHardwareKeyService(s))
		require.NoError(t, err)
		require.Equal(t, priv.Public(), retrievePriv.Public())
	}

	t.Run("PromptPolicies", func(t *testing.T) {
		// Warmup the hardware key to prompt touch at the start of the test,
		// rather than having this interaction later.
		priv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
			Policy: hardwarekey.PromptPolicy{TouchRequired: true},
		})
		require.NoError(t, err)
		require.NoError(t, priv.WarmupHardwareKey(ctx))

		resetYubikey(t, y)

		// Set pin.
		const testPIN = "123123"
		require.NoError(t, y.SetPIN(pivgo.DefaultPIN, testPIN))

		for _, policy := range []hardwarekey.PromptPolicy{
			hardwarekey.PromptPolicyNone,
			hardwarekey.PromptPolicyTouch,
			hardwarekey.PromptPolicyPIN,
			hardwarekey.PromptPolicyTouchAndPIN,
		} {
			t.Run(fmt.Sprintf("policy:%+v", policy), func(t *testing.T) {
				// Handle pin prompts (1 for generating the key, 1 for signing).
				if policy.PINRequired {
					promptReader.AddString(testPIN).AddString(testPIN)
				}

				testNewPrivateKey(t, hardwarekey.PrivateKeyConfig{
					Policy: policy,
				})
			})
		}
	})

	t.Run("CustomSlot", func(t *testing.T) {
		resetYubikey(t, y)
		testNewPrivateKey(t, hardwarekey.PrivateKeyConfig{
			CustomSlot: "9c",
		})
	})

	t.Run("Algorithms", func(t *testing.T) {
		for algorithm, config := range map[string]hardwarekey.PrivateKeyConfig{
			"EC256": {
				CustomSlot: "9a",
				Algorithm:  hardwarekey.SignatureAlgorithmEC256,
			},
			"RSA2048": {
				CustomSlot: "9c",
				Algorithm:  hardwarekey.SignatureAlgorithmRSA2048,
			},
		} {
			t.Run(fmt.Sprintf("algorithm:%v", algorithm), func(t *testing.T) {
				resetYubikey(t, y)
				testNewPrivateKey(t, config)
			})
		}
	})
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

func TestPINCaching(t *testing.T) {
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

	// Set pin.
	const testPIN = "123123"
	require.NoError(t, y.SetPIN(pivgo.DefaultPIN, testPIN))
	const wrongPIN = "123321"

	// Generate a key with PINPolicyAlways so that the PIN isn't cached internally on the YubiKey.
	pivSlot := pivgo.SlotAuthentication
	err = y.GenerateKey(pivSlot, pivgo.Key{
		Algorithm:   pivgo.AlgorithmEC384,
		PINPolicy:   pivgo.PINPolicyAlways,
		TouchPolicy: pivgo.TouchPolicyNever,
	})
	require.NoError(t, err)

	// Providing the wrong PIN should fail without caching it.
	promptReader.AddString(wrongPIN)
	_, err = keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
		Policy:      hardwarekey.PromptPolicyPIN,
		PINCacheTTL: time.Second,
		CustomSlot:  hardwarekey.PIVSlotKeyString(pivSlot.String()),
	})
	require.Error(t, err)

	// Retrieve the key with the right PIN and cache it.
	promptReader.AddString(testPIN)
	priv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
		Policy:      hardwarekey.PromptPolicyPIN,
		PINCacheTTL: time.Second,
		CustomSlot:  hardwarekey.PIVSlotKeyString(pivSlot.String()),
	})
	require.NoError(t, err)

	// The PIN is cached, no prompt needed.
	err = priv.WarmupHardwareKey(ctx)
	require.NoError(t, err)

	// Wait for the cache to expire.
	time.Sleep(time.Second)

	// Signing should fail with the wrong PIN without caching it.
	promptReader.AddString(wrongPIN)
	err = priv.WarmupHardwareKey(ctx)
	require.Error(t, err)

	// Signing with the right PIN should cache it.
	promptReader.AddString(testPIN)
	err = priv.WarmupHardwareKey(ctx)
	require.Error(t, err)

	// The PIN is cached, no prompt needed.
	err = priv.WarmupHardwareKey(ctx)
	require.Error(t, err)
}

func TestConcurrentSignature(t *testing.T) {
	// This test will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()
	promptReader := prompt.NewFakeReader()
	prompt := hardwarekey.NewCLIPrompt(os.Stderr, promptReader)
	s := piv.NewYubiKeyService(prompt)

	y, err := piv.FindYubiKey(0)
	require.NoError(t, err)

	resetYubikey(t, y)
	t.Cleanup(func() { resetYubikey(t, y) })

	// Set pin.
	const testPIN = "123123"
	require.NoError(t, y.SetPIN(pivgo.DefaultPIN, testPIN))

	promptReader.AddString(testPIN)
	priv, err := keys.NewHardwarePrivateKey(ctx, s, hardwarekey.PrivateKeyConfig{
		// Use PIN policy to slow down the signatures a bit so that they are concurrent.
		Policy: hardwarekey.PromptPolicyPIN,
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = priv.WarmupHardwareKey(ctx)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

// resetYubikey connects to the first yubiKey and resets it to defaults.
func resetYubikey(t *testing.T, y *piv.YubiKey) {
	t.Helper()
	require.NoError(t, y.Reset())
}
