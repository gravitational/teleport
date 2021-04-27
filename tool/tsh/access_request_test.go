/*
Copyright 2021 Gravitational, Inc.

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

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func newKeygen(t *testing.T, precompute int) *native.Keygen {
	fakeClock := clockwork.NewFakeClockAt(time.Date(2016, 9, 8, 7, 6, 5, 0, time.UTC))

	keygen := native.New(
		context.TODO(),
		native.PrecomputeKeys(precompute),
		native.SetClock(fakeClock))

	t.Cleanup(func() { keygen.Close() })

	return keygen
}

func newKey(keygen *native.Keygen, roles []string) (*client.Key, error) {
	priv, pub, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, err
	}
	cert, err := keygen.GenerateUserCertWithoutValidation(
		services.UserCertParams{
			PrivateCASigningKey:   priv,
			CASigningAlg:          defaults.CASignatureAlgorithm,
			PublicUserKey:         pub,
			Username:              "some-user",
			AllowedLogins:         []string{},
			TTL:                   time.Hour,
			Roles:                 roles,
			CertificateFormat:     teleport.CertificateFormatStandard,
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		})
	if err != nil {
		return nil, err
	}

	key := &client.Key{
		Priv: priv,
		Pub:  pub,
		Cert: cert,
	}

	return key, nil
}

func TestPassingNilToIsNotFoundReturnsFalse(t *testing.T) {
	require.False(t, trace.IsNotFound(nil))
}

func TestKeysTriggerRoleChange(t *testing.T) {
	// We can run this top-level test in parallel with the other parallel tests,
	// but given that all the sub-tests interact with the same Keygen, they
	// should probably run in series within this parent.
	t.Parallel()

	testCases := []struct {
		desc         string
		rolesOld     []string
		rolesNew     []string
		expectChange bool
	}{
		{
			desc:         "Same",
			rolesOld:     []string{"alpha", "beta", "gamma"},
			rolesNew:     []string{"alpha", "beta", "gamma"},
			expectChange: false,
		}, {
			desc:         "Out of order",
			rolesOld:     []string{"beta", "gamma", "alpha"},
			rolesNew:     []string{"alpha", "beta", "gamma"},
			expectChange: false,
		}, {
			desc:         "Empty",
			rolesOld:     []string{},
			rolesNew:     []string{},
			expectChange: false,
		}, {
			desc:         "Removal",
			rolesOld:     []string{"alpha", "beta", "gamma", "delta"},
			rolesNew:     []string{"alpha", "beta", "gamma"},
			expectChange: true,
		}, {
			desc:         "Addition",
			rolesOld:     []string{"alpha", "beta", "gamma"},
			rolesNew:     []string{"alpha", "beta", "gamma", "delta"},
			expectChange: true,
		}, {
			desc:         "Total Replacement",
			rolesNew:     []string{"alpha", "beta", "gamma"},
			rolesOld:     []string{"delta", "epsilon", "zeta"},
			expectChange: true,
		}, {
			desc:         "Duplicate coalescing (old)",
			rolesOld:     []string{"alpha", "alpha", "beta", "gamma"},
			rolesNew:     []string{"alpha", "beta", "gamma"},
			expectChange: false,
		}, {
			desc:         "Duplicate coalescing (new)",
			rolesOld:     []string{"alpha", "beta", "gamma"},
			rolesNew:     []string{"alpha", "beta", "gamma", "gamma"},
			expectChange: false,
		}, {
			desc:         "Duplicate replacement",
			rolesOld:     []string{"alpha", "alpha", "gamma"},
			rolesNew:     []string{"alpha", "beta", "gamma"},
			expectChange: true,
		}, {
			desc:         "Duplicate insertion",
			rolesOld:     []string{"alpha", "beta", "gamma"},
			rolesNew:     []string{"alpha", "alpha", "gamma"},
			expectChange: true,
		},
	}
	keyGen := newKeygen(t, len(testCases))

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			oldKey, err := newKey(keyGen, testCase.rolesNew)
			require.NoError(t, err)

			newKey, err := newKey(keyGen, testCase.rolesOld)
			require.NoError(t, err)

			changed, err := rolesHaveChanged(oldKey, newKey)
			require.NoError(t, err)
			require.Equal(t, changed, testCase.expectChange)
		})
	}
}
