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

package hardwarekey

import (
	"crypto/x509"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
)

// TestAttestHardwareKey tests AttestHardwareKey.
func TestAttestHardwareKey_Interactive(t *testing.T) {
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestAttestHardwareKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	if !testing.Verbose() {
		t.Fatal("This test is interactive and must be called with the -v verbose flag to see touch prompts.")
	}
	fmt.Println("This test is interactive, tap your YubiKey when prompted.")

	for _, policy := range []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKey,
		keys.PrivateKeyPolicyHardwareKeyTouch,
	} {
		t.Run(fmt.Sprintf("policy:%v", policy), func(t *testing.T) {
			priv, err := keys.GetOrGenerateYubiKeyPrivateKey(policy == keys.PrivateKeyPolicyHardwareKeyTouch)
			require.NoError(t, err)

			att, err := keys.GetAttestationStatement(priv)
			require.NoError(t, err)

			attData, err := attestHardwareKey(att)
			require.NoError(t, err)
			require.Equal(t, policy, attData.PrivateKeyPolicy)

			pub, err := x509.ParsePKIXPublicKey(attData.PublicKeyDER)
			require.NoError(t, err)
			require.Equal(t, priv.Public(), pub)
		})
	}
}
