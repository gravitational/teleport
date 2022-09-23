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

// TestParsePrivateKeyPolicyError tests ParsePrivateKeyPolicyError.
func TestParsePrivateKeyPolicyError(t *testing.T) {
	for _, tc := range []struct {
		desc               string
		errIn              error
		expectKeyPolicyErr bool
		expectKeyPolicy    PrivateKeyPolicy
	}{
		{
			desc:               "random error",
			errIn:              trace.BadParameter("random error"),
			expectKeyPolicyErr: true,
		}, {
			desc:               "unkown_key_policy",
			errIn:              newPrivateKeyPolicyError("unkown_key_policy"),
			expectKeyPolicyErr: true,
		}, {
			desc:            string(PrivateKeyPolicyNone),
			errIn:           newPrivateKeyPolicyError(PrivateKeyPolicyNone),
			expectKeyPolicy: PrivateKeyPolicyNone,
		}, {
			desc:            string(PrivateKeyPolicyHardwareKey),
			errIn:           newPrivateKeyPolicyError(PrivateKeyPolicyHardwareKey),
			expectKeyPolicy: PrivateKeyPolicyHardwareKey,
		}, {
			desc:            string(PrivateKeyPolicyHardwareKeyTouch),
			errIn:           newPrivateKeyPolicyError(PrivateKeyPolicyHardwareKeyTouch),
			expectKeyPolicy: PrivateKeyPolicyHardwareKeyTouch,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
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
