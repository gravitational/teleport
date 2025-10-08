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

package join_test

import (
	"context"
	"crypto"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/tlsca"
)

func testBoundKeypair(t *testing.T) (crypto.Signer, string) {
	key, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	return key.Signer, strings.TrimSpace(string(key.MarshalSSHPublicKey()))
}

// parseJoinState parses a join state token without verification, for testing
// purposes only.
func parseJoinState(t *testing.T, state []byte) *boundkeypair.JoinState {
	token, err := jwt.ParseSigned(string(state))
	require.NoError(t, err)

	var doc boundkeypair.JoinState
	require.NoError(t, token.UnsafeClaimsWithoutVerification(&doc))

	return &doc
}

func newTestTLSServer(t *testing.T, clock clockwork.Clock) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})
	return srv
}

func TestJoinBoundKeypair(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	correctSigner, correctPublicKey := testBoundKeypair(t)
	rotatedSigner, rotatedPublicKey := testBoundKeypair(t)
	incorrectSigner, incorrectPublicKey := testBoundKeypair(t)

	signers := map[string]crypto.Signer{
		correctPublicKey:   correctSigner,
		rotatedPublicKey:   rotatedSigner,
		incorrectPublicKey: incorrectSigner,
	}

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())
	startTime := clock.Now()

	srv := newTestTLSServer(t, clock)
	authServer := srv.Auth()

	_, err := authtest.CreateRole(ctx, authServer, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{"example"},
			},
		},
	})
	require.NoError(t, err)

	jwtCA, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.BoundKeypairCA,
		DomainName: srv.ClusterName(),
	}, /* loadKeys */ true)
	require.NoError(t, err)

	jwtSigner, err := authServer.GetKeyStore().GetJWTSigner(ctx, jwtCA)
	require.NoError(t, err)

	// An invalid signer for signing "fake" JWTs.
	invalidJWTSigner, _ := testBoundKeypair(t)

	makeToken := func(mutators ...func(v2 *types.ProvisionTokenV2)) types.ProvisionTokenV2 {
		token := types.ProvisionTokenV2{
			Spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBoundKeypair,
				Roles:      []types.SystemRole{types.RoleBot},
				BotName:    "test",
				BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
					Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{},
					Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
						Mode: boundkeypair.RecoveryModeInsecure,
					},
				},
			},
			Status: &types.ProvisionTokenStatusV2{
				BoundKeypair: &types.ProvisionTokenStatusV2BoundKeypair{},
			},
		}
		for _, mutator := range mutators {
			mutator(&token)
		}
		return token
	}

	withRecovery := func(mode string, count, limit uint32, botInstanceID string) func(*types.ProvisionTokenV2) {
		return func(v2 *types.ProvisionTokenV2) {
			v2.Spec.BoundKeypair.Recovery.Mode = mode
			v2.Spec.BoundKeypair.Recovery.Limit = limit
			v2.Status.BoundKeypair.RecoveryCount = count
			v2.Status.BoundKeypair.BoundBotInstanceID = botInstanceID
		}
	}

	withInitialKey := func(key string) func(*types.ProvisionTokenV2) {
		return func(v2 *types.ProvisionTokenV2) {
			v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = key
		}
	}

	withBoundKey := func(key string) func(*types.ProvisionTokenV2) {
		return func(v2 *types.ProvisionTokenV2) {
			v2.Status.BoundKeypair.BoundPublicKey = key
		}
	}

	makeJoinState := func(signer crypto.Signer, mutators ...func(s *boundkeypair.JoinStateParams)) string {
		params := &boundkeypair.JoinStateParams{
			Clock:       srv.Clock(),
			ClusterName: srv.ClusterName(),
		}

		for _, mutator := range mutators {
			mutator(params)
		}

		state, err := boundkeypair.IssueJoinState(signer, params)
		require.NoError(t, err)

		return state
	}

	withToken := func(mutators ...func(v2 *types.ProvisionTokenV2)) func(*boundkeypair.JoinStateParams) {
		return func(jsp *boundkeypair.JoinStateParams) {
			token := makeToken(mutators...)
			jsp.Token = &token
		}
	}

	type solver struct {
		signingKeys  []string
		rotationKeys []string

		rotationCount  int
		challengeCount int
		solutionKeys   []string

		getSigner         joinclient.GetSignerFunc
		requestNewKeypair joinclient.KeygenFunc
	}

	makeSolver := func(mutators ...func(s *solver)) *solver {
		s := &solver{}
		for _, mutator := range mutators {
			mutator(s)
		}

		s.getSigner = func(pubKey string) (crypto.Signer, error) {
			// getSigner will return signers in order from s.signingKeys, these
			// may not match pubKey requested but that will test signatures
			// using an incorrect key.
			s.challengeCount++
			challengeNum := s.challengeCount
			if s.challengeCount > len(s.signingKeys) {
				return nil, trace.Errorf("unabled to sign (solver has %d signing keys, %d challenges requested)", len(s.signingKeys), challengeNum)
			}
			signerPubKey := s.signingKeys[challengeNum-1]
			signer, ok := signers[signerPubKey]
			if !ok {
				return nil, trace.NotFound("key not found: %s", signerPubKey)
			}
			s.solutionKeys = append(s.solutionKeys, signerPubKey)
			return signer, nil
		}

		s.requestNewKeypair = func(_ context.Context, _ cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
			// requestNewKeypair will return signers in order from
			// s.rotationKeys.
			s.rotationCount++
			if s.rotationCount > len(s.rotationKeys) {
				return nil, trace.BadParameter("can't generate key")
			}
			nextPubKey := s.rotationKeys[s.rotationCount-1]
			signer, ok := signers[nextPubKey]
			if !ok {
				return nil, trace.NotFound("signer for rotatedPubKey not found")
			}
			return signer, nil
		}

		return s
	}

	withSigningKeys := func(pubKeys ...string) func(s *solver) {
		return func(s *solver) {
			s.signingKeys = pubKeys
		}
	}

	withRotationKeys := func(pubKeys ...string) func(s *solver) {
		return func(s *solver) {
			s.rotationKeys = pubKeys
		}
	}

	// Advance the clock a bit. Tests may reference `startTime` for a past
	// reference point.
	clock.Advance(time.Hour)

	// Make an unauthenticated auth client that will be used for joining.
	nopClient, err := srv.NewClient(authtest.TestNop())
	require.NoError(t, err)

	tests := []struct {
		name string

		token             types.ProvisionTokenV2
		initialJoinSecret string
		previousJoinState string
		solver            *solver

		assertError       require.ErrorAssertionFunc
		assertResponse    func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult)
		assertSolverState func(t *testing.T, s *solver)
	}{
		{
			// an initial key but no bound key, and no bound bot instance. aka,
			// initial join with preregistered key
			name: "initial-join-success",

			token:  makeToken(withInitialKey(correctPublicKey)),
			solver: makeSolver(withSigningKeys(correctPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *joinclient.BoundKeypairResult) {
				// join count should be incremented
				require.Equal(t, uint32(1), v2.Status.BoundKeypair.RecoveryCount)
				require.NotEmpty(t, v2.Status.BoundKeypair.BoundBotInstanceID)
				require.NotEmpty(t, v2.Status.BoundKeypair.BoundPublicKey)
			},
		},
		{
			// no bound key, no bound bot instance, aka initial join without
			// secret
			name: "initial-join-with-wrong-key",

			token:  makeToken(withInitialKey(correctPublicKey)),
			solver: makeSolver(withSigningKeys(incorrectPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "failed to complete challenge")
			},
		},
		{
			// bound key but no valid incoming bot instance, i.e. the certs
			// expired and triggered a hard rejoin
			name: "rejoin-success",

			token: makeToken(withBoundKey(correctPublicKey), func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *joinclient.BoundKeypairResult) {
				require.Equal(t, uint32(1), v2.Status.BoundKeypair.RecoveryCount)

				// Should generate a new bot instance
				require.NotEmpty(t, v2.Status.BoundKeypair.BoundBotInstanceID)
				require.NotEqual(t, "asdf", v2.Status.BoundKeypair.BoundBotInstanceID)
			},
		},
		{
			// Bad state: somehow a key was registered without a bot instance.
			// This should fail and prompt the user to recreate the token.
			name: "bound-key-no-instance",

			token:  makeToken(withBoundKey(correctPublicKey)),
			solver: makeSolver(withSigningKeys(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "bad backend state")
			},
		},
		{
			name:        "standard-initial-recovery-success",
			token:       makeToken(withRecovery("standard", 0, 1, ""), withInitialKey(correctPublicKey)),
			solver:      makeSolver(withSigningKeys(correctPublicKey)),
			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, uint32(1), v2.Status.BoundKeypair.RecoveryCount)

				require.NotNil(t, res)
				require.NotEmpty(t, res.JoinState)
			},
		},
		{
			name:              "standard-success-second-recovery",
			token:             makeToken(withRecovery("standard", 1, 2, "id"), withInitialKey(correctPublicKey)),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError:       require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, uint32(2), v2.Status.BoundKeypair.RecoveryCount)
				require.NotNil(t, res)
				state := parseJoinState(t, res.JoinState)
				require.Equal(t, v2.Status.BoundKeypair.RecoveryCount, state.RecoverySequence)
			},
		},
		{
			name:   "standard-failure-missing-join-state",
			token:  makeToken(withRecovery("standard", 1, 2, "id"), withBoundKey(correctPublicKey)),
			solver: makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:              "standard-failure-limit-exhausted",
			token:             makeToken(withRecovery("standard", 2, 2, "id")),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("standard", 2, 2, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "no recovery attempts remaining")
			},
		},
		{
			// Attempts to join with an outdated join state document should fail.
			name:              "standard-failure-recovery-count-mismatch",
			token:             makeToken(withRecovery("standard", 2, 3, "id"), withBoundKey(correctPublicKey)),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("standard", 1, 3, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:              "standard-failure-invalid-jwt",
			token:             makeToken(withRecovery("standard", 1, 2, "id"), withBoundKey(correctPublicKey)),
			previousJoinState: "asdf",
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:              "standard-failure-invalid-jwt-signature",
			token:             makeToken(withRecovery("standard", 1, 2, "id"), withBoundKey(correctPublicKey)),
			previousJoinState: makeJoinState(invalidJWTSigner, withToken(withRecovery("standard", 1, 2, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:              "standard-failure-invalid-instance-id",
			token:             makeToken(withRecovery("standard", 1, 2, "foo"), withBoundKey(correctPublicKey)),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:  "standard-failure-invalid-cluster",
			token: makeToken(withRecovery("standard", 1, 2, "foo"), withBoundKey(correctPublicKey)),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id")), func(s *boundkeypair.JoinStateParams) {
				s.ClusterName = "wrong-cluster"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey)),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:              "relaxed-success-count-over-limit",
			token:             makeToken(withRecovery("relaxed", 1, 0, "id"), withBoundKey(correctPublicKey)),
			previousJoinState: makeJoinState(jwtSigner, withToken(withRecovery("relaxed", 1, 0, "id"))),
			solver:            makeSolver(withSigningKeys(correctPublicKey)),
			assertError:       require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, uint32(2), v2.Status.BoundKeypair.RecoveryCount)

				require.NotNil(t, res)
				require.NotEmpty(t, res.JoinState)

				state := parseJoinState(t, res.JoinState)
				require.Equal(t, v2.Status.BoundKeypair.RecoveryCount, state.RecoverySequence)
			},
		},
		{
			// Initial rotation, i.e. `LastRotatedAt` isn't set. This should
			// trigger as soon as the `RotateAfter` threshold has been crossed.
			name: "first-rotation-success",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.RotateAfter = &startTime

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey, rotatedPublicKey), withRotationKeys(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, rotatedPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, rotatedPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey, rotatedPublicKey}, s.solutionKeys)
			},
		},
		{
			// Initial rotation timestamp hasn't been reached
			name: "first-rotation-skipped",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				rotateAfter := clock.Now().Add(time.Minute)
				v2.Spec.BoundKeypair.RotateAfter = &rotateAfter

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey), withRotationKeys(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, correctPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutionKeys)
			},
		},
		{
			// This should only trigger after `RotateAfter` has been crossed and
			// `LastRotatedAt` isn't after it.
			name: "second-rotation-success",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				rotateAfter := startTime.Add(10 * time.Minute)
				v2.Spec.BoundKeypair.RotateAfter = &rotateAfter

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
				v2.Status.BoundKeypair.LastRotatedAt = &startTime
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey, rotatedPublicKey), withRotationKeys(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, rotatedPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, rotatedPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey, rotatedPublicKey}, s.solutionKeys)
			},
		},
		{
			// We shouldn't try to rotate again if LastRotatedAt is greater than
			// RotateAfter.
			name: "second-rotation-skipped",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.RotateAfter = &startTime

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"

				rotatedAt := startTime.Add(10 * time.Minute)
				v2.Status.BoundKeypair.LastRotatedAt = &rotatedAt
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey), withRotationKeys(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, correctPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutionKeys)
			},
		},
		{
			// If the client doesn't complete rotation, an error should be
			// returned and the key should not change on the server.
			name: "rotation-failure",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.RotateAfter = &startTime

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "requesting new keypair")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
		{
			name: "rotation-same-key-not-allowed",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.RotateAfter = &startTime

				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			solver: makeSolver(withSigningKeys(correctPublicKey, correctPublicKey), withRotationKeys(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "public key may not be reused after rotation")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)

				// note: the client does complete the challenge for the
				// duplicate key, but the attempt will ultimately be rejected
				require.Equal(t, []string{correctPublicKey, correctPublicKey}, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
		{
			name: "registration-success",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = ""
				v2.Spec.BoundKeypair.Onboarding.RegistrationSecret = "secret"
			}),
			initialJoinSecret: "secret",
			solver:            makeSolver(withSigningKeys(correctPublicKey), withRotationKeys(correctPublicKey)),

			assertError: require.NoError,
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)

				// we'll only be asked for one challenge
				require.Equal(t, []string{correctPublicKey}, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, correctPublicKey, res.BoundPublicKey)
			},
		},
		{
			name: "registration-failure-wrong-secret",
			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = ""
				v2.Spec.BoundKeypair.Onboarding.RegistrationSecret = "secret"
			}),
			initialJoinSecret: "asdf",
			solver:            makeSolver(),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Empty(t, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Empty(t, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
		{
			// in this case, the server will generate a registration secret
			// automatically since nothing was set in .Onboarding. we won't know
			// it in the test, but will know it tried to check the provided
			// secret due to the error message.
			name: "registration-failure-wrong-secret-autogenerated",
			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = ""
				v2.Spec.BoundKeypair.Onboarding.RegistrationSecret = ""
			}),
			initialJoinSecret: "asdf",
			solver:            makeSolver(),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Empty(t, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Empty(t, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
		{
			// Joining with the a secret when a key was expected should be
			// handled as if the client couldn't complete the challenge (which
			// it can't)
			name: "registration-failure-expected-key",
			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = correctPublicKey
			}),
			initialJoinSecret: "asdf",
			solver:            makeSolver(withSigningKeys(rotatedPublicKey), withRotationKeys(rotatedPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "failed to complete challenge")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{rotatedPublicKey}, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Empty(t, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
		{
			name: "registration-failure-secret-expired",
			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Spec.BoundKeypair.Onboarding.InitialPublicKey = ""
				v2.Spec.BoundKeypair.Onboarding.RegistrationSecret = "secret"
				v2.Spec.BoundKeypair.Onboarding.MustRegisterBefore = &startTime
			}),
			initialJoinSecret: "secret",
			solver:            makeSolver(),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *solver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Empty(t, s.solutionKeys)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult) {
				require.Empty(t, v2.Status.BoundKeypair.BoundPublicKey)
				require.Nil(t, res)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpecAndStatus(
				tt.name, time.Now().Add(2*time.Hour), tt.token.Spec, tt.token.Status,
			)
			require.NoError(t, err)

			// note: we only override CreateToken in ServerWithRoles, so we'll
			// need to call CreateBoundKeypairToken() directly to ensure
			// computed fields (i.e. registration secrets) are handled properly.
			require.NoError(t, authServer.CreateBoundKeypairToken(ctx, token))

			joinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
				Token: token.GetName(),
				ID: state.IdentityID{
					Role: types.RoleBot,
				},
				AuthClient: nopClient,
				BoundKeypairParams: &joinclient.BoundKeypairParams{
					RegistrationSecret: tt.initialJoinSecret,
					PreviousJoinState:  []byte(tt.previousJoinState),
					GetSigner:          tt.solver.getSigner,
					RequestNewKeypair:  tt.solver.requestNewKeypair,
				},
			})
			tt.assertError(t, err)

			if tt.assertResponse != nil {
				pt, err := authServer.GetToken(ctx, tt.name)
				require.NoError(t, err)
				require.IsType(t, (*types.ProvisionTokenV2)(nil), pt)
				var boundKeypairResult *joinclient.BoundKeypairResult
				if joinResult != nil {
					boundKeypairResult = joinResult.BoundKeypair
				}
				tt.assertResponse(t, pt.(*types.ProvisionTokenV2), boundKeypairResult)
			}

			if tt.assertSolverState != nil {
				tt.assertSolverState(t, tt.solver)
			}
		})
	}

	rejoinTests := []struct {
		name string

		mutateToken func(*types.ProvisionTokenV2)
		solver      *solver

		assertError    require.ErrorAssertionFunc
		assertResponse func(t *testing.T, v2 *types.ProvisionTokenV2, res *joinclient.BoundKeypairResult)
	}{
		{
			// bound key, valid bound bot instance, aka "soft join"
			name: "reauth-success",

			mutateToken: func(v2 *types.ProvisionTokenV2) {},
			solver:      makeSolver(withSigningKeys(correctPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *joinclient.BoundKeypairResult) {
				// join count should not be incremented, it should be 1 from the initial join.
				require.Equal(t, uint32(1), v2.Status.BoundKeypair.RecoveryCount)
			},
		},
		{
			// bound key, seemingly valid bot instance, but wrong key
			// (should be impossible, but should fail anyway)
			name: "reauth-with-wrong-key",

			solver: makeSolver(withSigningKeys(incorrectPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "validating challenge response")
			},
		},
		{
			// The client somehow presents certs that refer to a different
			// instance, maybe tried switching auth methods.
			name: "bound-key-wrong-instance",

			mutateToken: func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundBotInstanceID = "qwerty"
			},
			solver: makeSolver(withSigningKeys(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "bot instance mismatch")
			},
		},
	}
	for _, tt := range rejoinTests {
		t.Run(tt.name, func(t *testing.T) {
			token := makeToken(withInitialKey(correctPublicKey))
			token.SetName(tt.name)
			initialSolver := makeSolver(withSigningKeys(correctPublicKey))
			require.NoError(t, authServer.CreateBoundKeypairToken(ctx, &token))

			initialJoinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
				Token: token.GetName(),
				ID: state.IdentityID{
					Role: types.RoleBot,
				},
				AuthClient: nopClient,
				BoundKeypairParams: &joinclient.BoundKeypairParams{
					GetSigner:         initialSolver.getSigner,
					RequestNewKeypair: initialSolver.requestNewKeypair,
				},
			})
			require.NoError(t, err)
			botClient, err := clientFromJoinResult(srv, initialJoinResult)
			require.NoError(t, err)

			if tt.mutateToken != nil {
				pt, err := authServer.GetToken(ctx, token.GetName())
				require.NoError(t, err)
				require.IsType(t, (*types.ProvisionTokenV2)(nil), pt)
				tt.mutateToken(pt.(*types.ProvisionTokenV2))
				require.NoError(t, authServer.UpsertToken(ctx, pt))
			}

			rejoinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
				Token: token.GetName(),
				ID: state.IdentityID{
					Role: types.RoleBot,
				},
				AuthClient: botClient,
				BoundKeypairParams: &joinclient.BoundKeypairParams{
					GetSigner:         tt.solver.getSigner,
					RequestNewKeypair: tt.solver.requestNewKeypair,
				},
			})
			tt.assertError(t, err)

			if tt.assertResponse != nil {
				pt, err := authServer.GetToken(ctx, token.GetName())
				require.NoError(t, err)
				require.IsType(t, (*types.ProvisionTokenV2)(nil), pt)
				var boundKeypairResult *joinclient.BoundKeypairResult
				if rejoinResult != nil {
					boundKeypairResult = rejoinResult.BoundKeypair
				}
				tt.assertResponse(t, pt.(*types.ProvisionTokenV2), boundKeypairResult)
			}
		})
	}
}

func TestJoinBoundKeypair_GenerationCounter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	correctSigner, correctPublicKey := testBoundKeypair(t)

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())

	srv := newTestTLSServer(t, clock)
	authServer := srv.Auth()

	_, err := authtest.CreateRole(ctx, authServer, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{"example"},
			},
		},
	})
	require.NoError(t, err)

	token, err := types.NewProvisionTokenFromSpecAndStatus(
		"bound-keypair-test",
		time.Now().Add(2*time.Hour),
		types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodBoundKeypair,
			Roles:      []types.SystemRole{types.RoleBot},
			BotName:    "test",
			BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
				Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
					InitialPublicKey: correctPublicKey,
				},
				Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
					Limit: 3,
				},
			},
		},
		&types.ProvisionTokenStatusV2{},
	)
	require.NoError(t, err)
	require.NoError(t, authServer.CreateBoundKeypairToken(ctx, token))

	// Make an unauthenticated auth client that will be used for joining.
	nopClient, err := srv.NewClient(authtest.TestNop())
	require.NoError(t, err)

	joinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)

	firstInstance, generation := testExtractBotParamsFromCerts(t, joinResult.Certs)
	require.Equal(t, uint64(1), generation)

	// Rejoin several times.
	for i := range 10 {
		botClient, err := clientFromJoinResult(srv, joinResult)
		require.NoError(t, err)

		joinResult, err = joinclient.Join(t.Context(), joinclient.JoinParams{
			Token: token.GetName(),
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			AuthClient: botClient,
			BoundKeypairParams: &joinclient.BoundKeypairParams{
				PreviousJoinState: joinResult.BoundKeypair.JoinState,
				GetSigner: func(pubKey string) (crypto.Signer, error) {
					return correctSigner, nil
				},
				RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
					return nil, trace.Errorf("no rotation expected")
				},
			},
		})
		require.NoError(t, err)

		instance, generation := testExtractBotParamsFromCerts(t, joinResult.Certs)
		require.Equal(t, uint64(i+2), generation)
		require.Equal(t, firstInstance, instance)
	}

	// Perform a recovery to get a new instance and reset the counter.
	// A recovery entails joining with an unauthenticated client and the
	// correct previous join state and bound key.
	joinResult, err = joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: joinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)

	secondInstance, generation := testExtractBotParamsFromCerts(t, joinResult.Certs)
	require.Equal(t, uint64(1), generation)
	require.NotEqual(t, firstInstance, secondInstance)

	// Save an old client with outdated generation counter
	oldClient, err := clientFromJoinResult(srv, joinResult)
	require.NoError(t, err)

	// Rejoin several more times.
	for i := range 10 {
		botClient, err := clientFromJoinResult(srv, joinResult)
		require.NoError(t, err)

		joinResult, err = joinclient.Join(t.Context(), joinclient.JoinParams{
			Token: token.GetName(),
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			AuthClient: botClient,
			BoundKeypairParams: &joinclient.BoundKeypairParams{
				PreviousJoinState: joinResult.BoundKeypair.JoinState,
				GetSigner: func(pubKey string) (crypto.Signer, error) {
					return correctSigner, nil
				},
				RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
					return nil, trace.Errorf("no rotation expected")
				},
			},
		})
		require.NoError(t, err)

		instance, generation := testExtractBotParamsFromCerts(t, joinResult.Certs)
		require.Equal(t, uint64(i+2), generation)
		require.Equal(t, secondInstance, instance)
	}

	// Try an API call with the bot certs.
	botClient, err := clientFromJoinResult(srv, joinResult)
	require.NoError(t, err)
	_, err = botClient.Ping(ctx)
	require.NoError(t, err)

	// Try to rejoin with an old client using the old generation count.
	_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: oldClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: joinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.Error(t, err)

	// The token should now be locked.
	locks, err := srv.Auth().GetLocks(ctx, true, types.LockTarget{
		JoinToken: token.GetName(),
	})
	require.NoError(t, err)
	require.Len(t, locks, 1, "only one lock should be generated")
	require.Contains(t, locks[0].Message(), "certificate generation mismatch")

	// Using the previously working client, make sure API calls no longer work.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = botClient.Ping(ctx)
		assert.ErrorContains(t, err, "access denied")
	}, 5*time.Second, 100*time.Millisecond)

	// Try rejoining again now that we know the lock is properly in force.
	_, err = joinclient.Join(ctx, joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: botClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: joinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.ErrorContains(t, err, "have been locked due to a certificate generation mismatch")
}

// TestJoinBoundKeypair_JoinStateFailure tests that join state verification
// will trigger a lock if the original client and a secondary client both
// attempt to recover in sequence.
func TestJoinBoundKeypair_JoinStateFailure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	correctSigner, correctPublicKey := testBoundKeypair(t)

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())

	srv := newTestTLSServer(t, clock)
	authServer := srv.Auth()

	_, err := authtest.CreateRole(ctx, authServer, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{"example"},
			},
		},
	})
	require.NoError(t, err)

	token, err := types.NewProvisionTokenFromSpecAndStatus(
		"bound-keypair-test",
		time.Now().Add(2*time.Hour),
		types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodBoundKeypair,
			Roles:      []types.SystemRole{types.RoleBot},
			BotName:    "test",
			BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
				Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
					InitialPublicKey: correctPublicKey,
				},
				Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
					Limit: 3,
				},
			},
		},
		&types.ProvisionTokenStatusV2{},
	)
	require.NoError(t, err)
	require.NoError(t, authServer.CreateBoundKeypairToken(ctx, token))

	// Make an unauthenticated auth client that will be used for joining.
	nopClient, err := srv.NewClient(authtest.TestNop())
	require.NoError(t, err)

	originalJoinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)

	// Perform a recovery, this time with a join state.
	recoverResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)

	recoveredBotClient, err := clientFromJoinResult(srv, recoverResult)
	require.NoError(t, err)

	// Try an API call with these certs.
	_, err = recoveredBotClient.Ping(ctx)
	require.NoError(t, err)

	// Try to recover again, but with the original join state.
	_, err = joinclient.Join(ctx, joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.Error(t, err)

	// The token should now be locked - but only once.
	locks, err := srv.Auth().GetLocks(ctx, true, types.LockTarget{
		JoinToken: "bound-keypair-test",
	})
	require.NoError(t, err)
	require.Len(t, locks, 1, "only one lock should be generated")
	require.Contains(t, locks[0].Message(), "failed to verify its join state")

	// The previously working client should be locked.
	require.Eventually(t, func() bool {
		_, err = recoveredBotClient.Ping(ctx)
		return err != nil && strings.Contains(err.Error(), "access denied")
	}, 5*time.Second, 100*time.Millisecond)

	// Repeat the recovery attempt but with an Eventually() to consistently
	// check the error message. Depending on exact timing / cache propagation /
	// etc the lock may or may not be in force, but we also need to be
	// absolutely certain to try to generate at least 2 locking events.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = joinclient.Join(ctx, joinclient.JoinParams{
			Token: token.GetName(),
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			AuthClient: nopClient,
			BoundKeypairParams: &joinclient.BoundKeypairParams{
				PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
				GetSigner: func(pubKey string) (crypto.Signer, error) {
					return correctSigner, nil
				},
				RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
					return nil, trace.Errorf("no rotation expected")
				},
			},
		})
		require.ErrorContains(t, err, "a client failed to verify its join state")
	}, 5*time.Second, 100*time.Millisecond)
}

// TestJoinBoundKeypair_JoinStateFailureDuringRenewal, similar to
// _JoinStateFailure above, exercises the case where the original client still
// has valid certs and isn't attempting a recovery of its own.
func TestJoinBoundKeypair_JoinStateFailureDuringRenewal(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	correctSigner, correctPublicKey := testBoundKeypair(t)

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())

	srv := newTestTLSServer(t, clock)
	authServer := srv.Auth()

	_, err := authtest.CreateRole(ctx, authServer, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test",
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: []string{"example"},
			},
		},
	})
	require.NoError(t, err)

	token, err := types.NewProvisionTokenFromSpecAndStatus(
		"bound-keypair-test",
		time.Now().Add(2*time.Hour),
		types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodBoundKeypair,
			Roles:      []types.SystemRole{types.RoleBot},
			BotName:    "test",
			BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
				Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
					InitialPublicKey: correctPublicKey,
				},
				Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
					Limit: 3,
				},
			},
		},
		&types.ProvisionTokenStatusV2{},
	)
	require.NoError(t, err)
	require.NoError(t, authServer.CreateBoundKeypairToken(ctx, token))

	// Make an unauthenticated auth client that will be used for joining.
	nopClient, err := srv.NewClient(authtest.TestNop())
	require.NoError(t, err)

	// Perform the initial registration
	originalJoinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)
	originalBotClient, err := clientFromJoinResult(srv, originalJoinResult)
	require.NoError(t, err)

	// Perform a recovery, this time with a join state, simulating an attacker
	// that has copied the certs.
	attackerResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: nopClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.NoError(t, err)

	attackerClient, err := clientFromJoinResult(srv, attackerResult)
	require.NoError(t, err)

	// Try an API call with these certs.
	_, err = attackerClient.Ping(ctx)
	require.NoError(t, err)

	// Simulate the original valid bot trying to rejoin to renew its
	// certificates, with an authenticated client and the valid original join
	// state.
	_, err = joinclient.Join(ctx, joinclient.JoinParams{
		Token: token.GetName(),
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthClient: originalBotClient,
		BoundKeypairParams: &joinclient.BoundKeypairParams{
			PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
			GetSigner: func(pubKey string) (crypto.Signer, error) {
				return correctSigner, nil
			},
			RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
				return nil, trace.Errorf("no rotation expected")
			},
		},
	})
	require.Error(t, err)

	// The token should now be locked - but only once.
	locks, err := srv.Auth().GetLocks(ctx, true, types.LockTarget{
		JoinToken: "bound-keypair-test",
	})
	require.NoError(t, err)
	require.Len(t, locks, 1, "only one lock should be generated")
	require.Contains(t, locks[0].Message(), "failed to verify its join state")

	// The previously working client should be locked.
	require.Eventually(t, func() bool {
		_, err = attackerClient.Ping(ctx)
		return err != nil && strings.Contains(err.Error(), "access denied")
	}, 5*time.Second, 100*time.Millisecond)

	// Repeat the valid bot rejoin attempt but with an Eventually() to
	// consistently check the error message. Depending on exact timing / cache
	// propagation / etc the lock may or may not be in force, but we also need
	// to be absolutely certain to try to generate at least 2 locking events.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = joinclient.Join(ctx, joinclient.JoinParams{
			Token: token.GetName(),
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			AuthClient: originalBotClient,
			BoundKeypairParams: &joinclient.BoundKeypairParams{
				PreviousJoinState: originalJoinResult.BoundKeypair.JoinState,
				GetSigner: func(pubKey string) (crypto.Signer, error) {
					return correctSigner, nil
				},
				RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
					return nil, trace.Errorf("no rotation expected")
				},
			},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "a client failed to verify its join state")
	}, 5*time.Second, 100*time.Millisecond)
}

func clientFromJoinResult(srv *authtest.TLSServer, joinResult *joinclient.JoinResult) (*authclient.Client, error) {
	certPemBlock, _ := pem.Decode(joinResult.Certs.TLS)
	return srv.NewClientWithCert(tls.Certificate{
		Certificate: [][]byte{certPemBlock.Bytes},
		PrivateKey:  joinResult.PrivateKey,
	})
}

type tHelper interface {
	Helper()
}

func testExtractBotParamsFromCerts(t require.TestingT, certs *proto.Certs) (string, uint64) {
	// we might not have .Helper() available for require.CollectT
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	parsed, err := tlsca.ParseCertificatePEM(certs.TLS)
	require.NoError(t, err)
	ident, err := tlsca.FromSubject(parsed.Subject, parsed.NotAfter)
	require.NoError(t, err)

	return ident.BotInstanceID, ident.Generation
}
