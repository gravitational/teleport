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
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIsRequiredPolicyMet(t *testing.T) {
	for _, tc := range []struct {
		requiredPolicy   PrivateKeyPolicy
		verifiedPolicies []PrivateKeyPolicy
	}{
		{
			requiredPolicy:   PrivateKeyPolicyNone,
			verifiedPolicies: privateKeyPolicies,
		}, {
			requiredPolicy:   PrivateKeyPolicyHardwareKey,
			verifiedPolicies: hardwareKeyPolicies,
		}, {
			requiredPolicy:   PrivateKeyPolicyHardwareKeyTouch,
			verifiedPolicies: hardwareKeyTouchPolicies,
		},
	} {
		t.Run(string(PrivateKeyPolicyHardwareKey), func(t *testing.T) {
			for _, keyPolicy := range privateKeyPolicies {
				if IsRequiredPolicyMet(tc.requiredPolicy, keyPolicy) {
					require.Contains(t, tc.verifiedPolicies, keyPolicy, "Policy %q does not meet %q but IsRequirePolicyMet(%v, %v) returned true", keyPolicy, tc.requiredPolicy, keyPolicy, tc.requiredPolicy)
				} else {
					require.NotContains(t, tc.verifiedPolicies, keyPolicy, "Policy %q does meet %q but IsRequirePolicyMet(%v, %v) returned false", keyPolicy, tc.requiredPolicy, keyPolicy, tc.requiredPolicy)
				}
			}
		})
	}
}

func TestGetPolicyFromSet(t *testing.T) {
	testCases := []struct {
		name            string
		policySet       []PrivateKeyPolicy
		expectSetPolicy PrivateKeyPolicy
	}{
		{
			name: "none",
			policySet: []PrivateKeyPolicy{
				PrivateKeyPolicyNone,
				PrivateKeyPolicyNone,
			},
			expectSetPolicy: PrivateKeyPolicyNone,
		}, {
			name: "hardware key policy",
			policySet: []PrivateKeyPolicy{
				PrivateKeyPolicyNone,
				PrivateKeyPolicyHardwareKey,
			},
			expectSetPolicy: PrivateKeyPolicyHardwareKey,
		}, {
			name: "touch policy",
			policySet: []PrivateKeyPolicy{
				PrivateKeyPolicyNone,
				PrivateKeyPolicyHardwareKey,
				PrivateKeyPolicyHardwareKeyTouch,
			},
			expectSetPolicy: PrivateKeyPolicyHardwareKeyTouch,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectSetPolicy, GetPolicyFromSet(tc.policySet))

			// reversing the policy set shouldn't change the output
			for i, j := 0, len(tc.policySet)-1; i < j; i, j = i+1, j-1 {
				tc.policySet[i], tc.policySet[j] = tc.policySet[j], tc.policySet[i]
			}
			require.Equal(t, tc.expectSetPolicy, GetPolicyFromSet(tc.policySet))
		})
	}
}

// TestPrivateKeyPolicyError tests private key policy error logic.
func TestPrivateKeyPolicyError(t *testing.T) {
	type testCase struct {
		desc                    string
		errIn                   error
		expectIsKeyPolicy       bool
		expectParseKeyPolicyErr bool
		expectKeyPolicy         PrivateKeyPolicy
	}

	testCases := []testCase{
		{
			desc:                    "random error",
			errIn:                   trace.BadParameter("random error"),
			expectIsKeyPolicy:       false,
			expectParseKeyPolicyErr: true,
		}, {
			desc:                    "unknown_key_policy",
			errIn:                   NewPrivateKeyPolicyError("unknown_key_policy"),
			expectIsKeyPolicy:       true,
			expectParseKeyPolicyErr: true,
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
	}

	for _, policy := range privateKeyPolicies {
		testCases = append(testCases, testCase{
			desc:              fmt.Sprintf("valid key policy: %v", policy),
			errIn:             NewPrivateKeyPolicyError(policy),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   policy,
		})
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expectIsKeyPolicy, IsPrivateKeyPolicyError(tc.errIn))

			keyPolicy, err := ParsePrivateKeyPolicyError(tc.errIn)
			if tc.expectParseKeyPolicyErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectKeyPolicy, keyPolicy)
			}
		})
	}
}
