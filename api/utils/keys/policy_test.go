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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestVerifyPolicy tests VerifyPolicy.
func TestVerifyPolicy(t *testing.T) {
	t.Run("key policy none", func(t *testing.T) {
		require.NoError(t, PrivateKeyPolicyNone.VerifyPolicy(PrivateKeyPolicyNone))
		require.NoError(t, PrivateKeyPolicyNone.VerifyPolicy(PrivateKeyPolicyHardwareKey))
		require.NoError(t, PrivateKeyPolicyNone.VerifyPolicy(PrivateKeyPolicyHardwareKeyTouch))
	})
	t.Run("key policy hardware_key", func(t *testing.T) {
		require.Error(t, PrivateKeyPolicyHardwareKey.VerifyPolicy(PrivateKeyPolicyNone))
		require.NoError(t, PrivateKeyPolicyHardwareKey.VerifyPolicy(PrivateKeyPolicyHardwareKey))
		require.NoError(t, PrivateKeyPolicyHardwareKey.VerifyPolicy(PrivateKeyPolicyHardwareKeyTouch))
	})
	t.Run("key policy hardware_key_touch", func(t *testing.T) {
		require.Error(t, PrivateKeyPolicyHardwareKeyTouch.VerifyPolicy(PrivateKeyPolicyNone))
		require.Error(t, PrivateKeyPolicyHardwareKeyTouch.VerifyPolicy(PrivateKeyPolicyHardwareKey))
		require.NoError(t, PrivateKeyPolicyHardwareKeyTouch.VerifyPolicy(PrivateKeyPolicyHardwareKeyTouch))
	})
}

// TestPrivateKeyPolicyError tests private key policy error logic.
func TestPrivateKeyPolicyError(t *testing.T) {
	for _, tc := range []struct {
		desc               string
		errIn              error
		expectIsKeyPolicy  bool
		expectKeyPolicyErr bool
		expectKeyPolicy    PrivateKeyPolicy
	}{
		{
			desc:               "random error",
			errIn:              trace.BadParameter("random error"),
			expectIsKeyPolicy:  false,
			expectKeyPolicyErr: true,
		}, {
			desc:               "unknown_key_policy",
			errIn:              NewPrivateKeyPolicyError("unknown_key_policy"),
			expectIsKeyPolicy:  true,
			expectKeyPolicyErr: true,
		}, {
			desc:              string(PrivateKeyPolicyNone),
			errIn:             NewPrivateKeyPolicyError(PrivateKeyPolicyNone),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   PrivateKeyPolicyNone,
		}, {
			desc:              string(PrivateKeyPolicyHardwareKey),
			errIn:             NewPrivateKeyPolicyError(PrivateKeyPolicyHardwareKey),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   PrivateKeyPolicyHardwareKey,
		}, {
			desc:              string(PrivateKeyPolicyHardwareKeyTouch),
			errIn:             NewPrivateKeyPolicyError(PrivateKeyPolicyHardwareKeyTouch),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   PrivateKeyPolicyHardwareKeyTouch,
		}, {
			desc:              "wrapped policy error",
			errIn:             trace.Wrap(NewPrivateKeyPolicyError(PrivateKeyPolicyHardwareKeyTouch), "wrapped err"),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   PrivateKeyPolicyHardwareKeyTouch,
		}, {
			desc:              "policy error string contained in error",
			errIn:             trace.Errorf("ssh: rejected: administratively prohibited (%s)", NewPrivateKeyPolicyError(PrivateKeyPolicyHardwareKeyTouch).Error()),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   PrivateKeyPolicyHardwareKeyTouch,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expectIsKeyPolicy, IsPrivateKeyPolicyError(tc.errIn))

			keyPolicy, err := ParsePrivateKeyPolicyError(tc.errIn)
			if tc.expectKeyPolicyErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectKeyPolicy, keyPolicy)
			}
		})
	}
}
