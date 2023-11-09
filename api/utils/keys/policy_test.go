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
	"fmt"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
)

var (
	privateKeyPolicies = []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyNone,
		keys.PrivateKeyPolicyHardwareKey,
		keys.PrivateKeyPolicyHardwareKeyTouch,
		keys.PrivateKeyPolicyHardwareKeyPIN,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		keys.PrivateKeyPolicyWebSession,
	}
	hardwareKeyPolicies = []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKey,
		keys.PrivateKeyPolicyHardwareKeyTouch,
		keys.PrivateKeyPolicyHardwareKeyPIN,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		keys.PrivateKeyPolicyWebSession,
	}
	hardwareKeyTouchPolicies = []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKeyTouch,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		keys.PrivateKeyPolicyWebSession,
	}
	hardwareKeyPINPolicies = []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKeyPIN,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		keys.PrivateKeyPolicyWebSession,
	}
	hardwareKeyTouchAndPINPolicies = []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		keys.PrivateKeyPolicyWebSession,
	}
)

func TestIsRequiredPolicyMet(t *testing.T) {
	privateKeyPolicies := []keys.PrivateKeyPolicy{
		keys.PrivateKeyPolicyNone,
		keys.PrivateKeyPolicyHardwareKey,
		keys.PrivateKeyPolicyHardwareKeyTouch,
		keys.PrivateKeyPolicyHardwareKeyPIN,
		keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
	}
	for _, tc := range []struct {
		requiredPolicy     keys.PrivateKeyPolicy
		satisfyingPolicies []keys.PrivateKeyPolicy
	}{
		{
			requiredPolicy:     keys.PrivateKeyPolicyNone,
			satisfyingPolicies: privateKeyPolicies,
		}, {
			requiredPolicy:     keys.PrivateKeyPolicyHardwareKey,
			satisfyingPolicies: hardwareKeyPolicies,
		}, {
			requiredPolicy:     keys.PrivateKeyPolicyHardwareKeyTouch,
			satisfyingPolicies: hardwareKeyTouchPolicies,
		}, {
			requiredPolicy:     keys.PrivateKeyPolicyHardwareKeyPIN,
			satisfyingPolicies: hardwareKeyPINPolicies,
		}, {
			requiredPolicy:     keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
			satisfyingPolicies: hardwareKeyTouchAndPINPolicies,
		},
	} {
		t.Run(string(tc.requiredPolicy), func(t *testing.T) {
			for _, keyPolicy := range privateKeyPolicies {
				if tc.requiredPolicy.IsSatisfiedBy(keyPolicy) {
					require.Contains(t, tc.satisfyingPolicies, keyPolicy, "Policy %q does not meet %q but IsRequirePolicyMet(%v, %v) returned true", keyPolicy, tc.requiredPolicy, tc.requiredPolicy, keyPolicy)
				} else {
					require.NotContains(t, tc.satisfyingPolicies, keyPolicy, "Policy %q does meet %q but IsRequirePolicyMet(%v, %v) returned false", keyPolicy, tc.requiredPolicy, tc.requiredPolicy, keyPolicy)
				}
			}
		})
	}
}

func TestGetPolicyFromSet(t *testing.T) {
	testCases := []struct {
		name       string
		policySet  []keys.PrivateKeyPolicy
		wantPolicy keys.PrivateKeyPolicy
	}{
		{
			name: "none",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyNone,
			},
			wantPolicy: keys.PrivateKeyPolicyNone,
		}, {
			name: "hardware key policy",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyHardwareKey,
			},
			wantPolicy: keys.PrivateKeyPolicyHardwareKey,
		}, {
			name: "touch policy",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyHardwareKey,
				keys.PrivateKeyPolicyHardwareKeyTouch,
			},
			wantPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
		}, {
			name: "pin policy",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyHardwareKey,
				keys.PrivateKeyPolicyHardwareKeyPIN,
			},
			wantPolicy: keys.PrivateKeyPolicyHardwareKeyPIN,
		}, {
			name: "touch policy and pin policy",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyHardwareKey,
				keys.PrivateKeyPolicyHardwareKeyPIN,
				keys.PrivateKeyPolicyHardwareKeyTouch,
			},
			wantPolicy: keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		}, {
			name: "touch and pin policy",
			policySet: []keys.PrivateKeyPolicy{
				keys.PrivateKeyPolicyNone,
				keys.PrivateKeyPolicyHardwareKey,
				keys.PrivateKeyPolicyHardwareKeyTouch,
				keys.PrivateKeyPolicyHardwareKeyPIN,
				keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
			},
			wantPolicy: keys.PrivateKeyPolicyHardwareKeyTouchAndPIN,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requiredPolicy, err := keys.PolicyThatSatisfiesSet(tc.policySet)
			require.NoError(t, err)
			require.Equal(t, tc.wantPolicy, requiredPolicy)

			// reversing the policy set shouldn't change the output
			slices.Reverse(tc.policySet)

			requiredPolicy, err = keys.PolicyThatSatisfiesSet(tc.policySet)
			require.NoError(t, err)
			require.Equal(t, tc.wantPolicy, requiredPolicy)
		})
	}
}

// TestParsePrivateKeyPolicyError tests private key policy error parsing and checking.
func TestParsePrivateKeyPolicyError(t *testing.T) {
	type testCase struct {
		desc                    string
		errIn                   error
		expectIsKeyPolicy       bool
		expectParseKeyPolicyErr bool
		expectKeyPolicy         keys.PrivateKeyPolicy
	}

	testCases := []testCase{
		{
			desc:                    "random error",
			errIn:                   trace.BadParameter("random error"),
			expectIsKeyPolicy:       false,
			expectParseKeyPolicyErr: true,
		}, {
			desc:                    "unknown_key_policy",
			errIn:                   keys.NewPrivateKeyPolicyError("unknown_key_policy"),
			expectIsKeyPolicy:       true,
			expectParseKeyPolicyErr: true,
		}, {
			desc:              "wrapped policy error",
			errIn:             trace.Wrap(keys.NewPrivateKeyPolicyError(keys.PrivateKeyPolicyHardwareKeyTouch), "wrapped err"),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   keys.PrivateKeyPolicyHardwareKeyTouch,
		}, {
			desc:              "policy error string contained in error",
			errIn:             trace.Errorf("ssh: rejected: administratively prohibited (%s)", keys.NewPrivateKeyPolicyError(keys.PrivateKeyPolicyHardwareKeyTouch).Error()),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   keys.PrivateKeyPolicyHardwareKeyTouch,
		},
	}

	for _, policy := range privateKeyPolicies {
		testCases = append(testCases, testCase{
			desc:              fmt.Sprintf("valid key policy: %v", policy),
			errIn:             keys.NewPrivateKeyPolicyError(policy),
			expectIsKeyPolicy: true,
			expectKeyPolicy:   policy,
		})
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expectIsKeyPolicy, keys.IsPrivateKeyPolicyError(tc.errIn))

			keyPolicy, err := keys.ParsePrivateKeyPolicyError(tc.errIn)
			if tc.expectParseKeyPolicyErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectKeyPolicy, keyPolicy)
			}
		})
	}
}
