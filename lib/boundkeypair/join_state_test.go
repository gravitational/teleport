/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package boundkeypair

import (
	"crypto"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
)

type mockCA struct {
	types.CertAuthority

	keys []*types.JWTKeyPair
}

func (m *mockCA) GetTrustedJWTKeyPairs() []*types.JWTKeyPair {
	return m.keys
}

func newJWTKeyPair(t *testing.T) (crypto.Signer, *types.JWTKeyPair) {
	t.Helper()

	signer := newTestKeypair(t)
	private, err := keys.MarshalPrivateKey(signer)
	require.NoError(t, err)

	public, err := keys.MarshalPublicKey(signer.Public())
	require.NoError(t, err)

	return signer, &types.JWTKeyPair{
		PublicKey:      public,
		PrivateKey:     private,
		PrivateKeyType: types.PrivateKeyType_RAW,
	}
}

func TestIssueAndVerifyJoinState(t *testing.T) {
	activeSigner, activeKeypair := newJWTKeyPair(t)
	standbySigner, standbyKeypair := newJWTKeyPair(t)
	invalidSigner, _ := newJWTKeyPair(t)

	clock := clockwork.NewFakeClock()

	ca := &mockCA{
		keys: []*types.JWTKeyPair{
			activeKeypair,
			standbyKeypair,
		},
	}

	makeParams := func(mutators ...func(*JoinStateParams)) *JoinStateParams {
		token := &types.ProvisionTokenV2{
			Spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBoundKeypair,
				Roles:      []types.SystemRole{types.RoleBot},
				BotName:    "test",
				BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
					Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
						InitialPublicKey: "abcd",
					},
					Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
						Mode: string(RecoveryModeStandard),
					},
				},
			},
			Status: &types.ProvisionTokenStatusV2{
				BoundKeypair: &types.ProvisionTokenStatusV2BoundKeypair{},
			},
		}

		params := &JoinStateParams{
			Clock:       clock,
			ClusterName: "example.com",
			Token:       token,
		}

		for _, mutator := range mutators {
			mutator(params)
		}

		return params
	}

	withRecovery := func(count, limit uint32) func(*JoinStateParams) {
		return func(params *JoinStateParams) {
			params.Token.Status.BoundKeypair.RecoveryCount = count
			params.Token.Spec.BoundKeypair.Recovery.Limit = limit
		}
	}

	withInstanceID := func(id string) func(*JoinStateParams) {
		return func(params *JoinStateParams) {
			params.Token.Status.BoundKeypair.BoundBotInstanceID = id
		}
	}

	makeIssuer := func(signer crypto.Signer, params *JoinStateParams) func(*testing.T, clockwork.Clock) string {
		return func(t *testing.T, clock clockwork.Clock) string {
			params.Clock = clock

			state, err := IssueJoinState(signer, params)
			require.NoError(t, err)

			return state
		}
	}

	tests := []struct {
		name string

		issue        func(t *testing.T, clock clockwork.Clock) string
		verifyParams *JoinStateParams

		clockMod func(clock *clockwork.FakeClock)

		assertError   require.ErrorAssertionFunc
		assertSuccess func(t *testing.T, s *JoinState)
	}{
		{
			name:         "success",
			issue:        makeIssuer(activeSigner, makeParams(withRecovery(0, 1))),
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError:  require.NoError,
		},
		{
			name:         "success with alternate signer",
			issue:        makeIssuer(standbySigner, makeParams(withRecovery(0, 1))),
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError:  require.NoError,
		},
		{
			name: "invalid join state",
			issue: func(t *testing.T, _ clockwork.Clock) string {
				return "asdf"
			},
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "parsing serialized join state")
			},
		},
		{
			name:         "invalid count",
			issue:        makeIssuer(activeSigner, makeParams(withRecovery(0, 1))),
			verifyParams: makeParams(withRecovery(1, 1)),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "recovery counter mismatch")
			},
		},
		{
			name:         "invalid instance ID",
			issue:        makeIssuer(activeSigner, makeParams(withRecovery(0, 1), withInstanceID("foo"))),
			verifyParams: makeParams(withRecovery(0, 1), withInstanceID("bar")),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "bot instance mismatch")
			},
		},
		{
			name:         "untrusted signer",
			issue:        makeIssuer(invalidSigner, makeParams(withRecovery(0, 1), withInstanceID("foo"))),
			verifyParams: makeParams(withRecovery(0, 1), withInstanceID("bar")),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "join state could not be verified")
			},
		},
		{
			name:         "issued too early",
			issue:        makeIssuer(activeSigner, makeParams(withRecovery(0, 1))),
			verifyParams: makeParams(withRecovery(0, 1)),
			clockMod: func(clock *clockwork.FakeClock) {
				clock.Advance(-10 * time.Minute)
			},
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "token not valid yet")
			},
		},
		{
			name: "cluster name must match",
			issue: makeIssuer(activeSigner, makeParams(withRecovery(0, 1), func(jsp *JoinStateParams) {
				jsp.ClusterName = "invalid"
			})),
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "invalid issuer claim")
			},
		},
		{
			name: "subject must match",
			issue: makeIssuer(activeSigner, makeParams(withRecovery(0, 1), func(jsp *JoinStateParams) {
				jsp.Token.Spec.BotName = "invalid"
			})),
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "invalid subject claim")
			},
		},
		{
			name: "informational parameters can be modified",
			issue: makeIssuer(activeSigner, makeParams(withRecovery(0, 1), func(jsp *JoinStateParams) {
				jsp.Token.Spec.BoundKeypair.Recovery.Mode = "relaxed"
				jsp.Token.Spec.BoundKeypair.Recovery.Limit = 123
			})),
			verifyParams: makeParams(withRecovery(0, 1)),
			assertError:  require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()

			signedState := tt.issue(t, clock)

			if tt.clockMod != nil {
				tt.clockMod(clock)
			}

			tt.verifyParams.Clock = clock
			_, err := VerifyJoinState(ca, signedState, tt.verifyParams)
			tt.assertError(t, err)
		})
	}
}
