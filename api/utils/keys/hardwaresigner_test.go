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

	"github.com/stretchr/testify/require"
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
	priv, err := GetOrGenerateYubiKeyPrivateKey(false)
	require.NoError(t, err)

	require.NotNil(t, priv.GetAttestationStatement())
	require.Equal(t, PrivateKeyPolicyHardwareKey, priv.GetPrivateKeyPolicy())
}

// TestNonHardwareSigner tests the HardwareSigner interface with non-hardware keys.
func TestNonHardwareSigner(t *testing.T) {
	// Non-hardware keys should return a nil attestation statement and PrivateKeyPolicyNone.
	priv, err := ParsePrivateKey(rsaKeyPEM)
	require.NoError(t, err)

	require.Nil(t, priv.GetAttestationStatement())
	require.Equal(t, PrivateKeyPolicyNone, priv.GetPrivateKeyPolicy())
}
