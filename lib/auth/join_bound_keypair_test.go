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

package auth

import (
	"context"
	"crypto"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/boundkeypair"
	"github.com/gravitational/teleport/lib/boundkeypair/boundkeypairexperiment"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockBoundKeypairValidator struct {
	subject     string
	clusterName string
	publicKey   crypto.PublicKey
}

func (v *mockBoundKeypairValidator) IssueChallenge() (*boundkeypair.ChallengeDocument, error) {
	return &boundkeypair.ChallengeDocument{
		Nonce: "fake",
	}, nil
}

func (v *mockBoundKeypairValidator) ValidateChallengeResponse(issued *boundkeypair.ChallengeDocument, compactResponse string) error {
	// For testing, the solver will just reply with the marshaled public key, so
	// we'll parse and compare it.
	key, err := sshutils.CryptoPublicKey([]byte(compactResponse))
	if err != nil {
		return trace.Wrap(err, "parsing bound public key")
	}

	equal, ok := v.publicKey.(interface {
		Equal(x crypto.PublicKey) bool
	})
	if !ok {
		return trace.BadParameter("unsupported public key type %T", key)
	}

	if !equal.Equal(key) {
		return trace.AccessDenied("incorrect public key")
	}

	return nil
}

func testBoundKeypair(t *testing.T) (crypto.Signer, string) {
	key, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	return key.Signer, string(key.MarshalSSHPublicKey())
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

func TestServer_RegisterUsingBoundKeypairMethod(t *testing.T) {
	ctx := context.Background()

	// TODO: This prevents parallel execution; remove along with the experiment.
	boundkeypairexperiment.SetEnabled(true)

	_, correctPublicKey := testBoundKeypair(t)
	_, rotatedPublicKey := testBoundKeypair(t)
	_, incorrectPublicKey := testBoundKeypair(t)

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())
	startTime := clock.Now()

	srv := newTestTLSServer(t, withClock(clock))
	auth := srv.Auth()
	auth.createBoundKeypairValidator = func(subject, clusterName string, publicKey crypto.PublicKey) (boundKeypairValidator, error) {
		return &mockBoundKeypairValidator{
			subject:     subject,
			clusterName: clusterName,
			publicKey:   publicKey,
		}, nil
	}

	_, err := CreateRole(ctx, auth, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(TestAdmin())
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

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	jwtCA, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.BoundKeypairCA,
		DomainName: srv.ClusterName(),
	}, /* loadKeys */ true)
	require.NoError(t, err)

	jwtSigner, err := auth.GetKeyStore().GetJWTSigner(ctx, jwtCA)
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
					Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
						InitialPublicKey: correctPublicKey,
					},
					Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
						// Only insecure is supported for now.
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

	makeInitReq := func(mutators ...func(r *proto.RegisterUsingBoundKeypairInitialRequest)) *proto.RegisterUsingBoundKeypairInitialRequest {
		req := &proto.RegisterUsingBoundKeypairInitialRequest{
			JoinRequest: &types.RegisterUsingTokenRequest{
				HostID:       "host-id",
				Role:         types.RoleBot,
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: sshPublicKey,
			},
		}
		for _, mutator := range mutators {
			mutator(req)
		}
		return req
	}

	withJoinState := func(signer crypto.Signer, mutators ...func(s *boundkeypair.JoinStateParams)) func(*proto.RegisterUsingBoundKeypairInitialRequest) {
		return func(req *proto.RegisterUsingBoundKeypairInitialRequest) {
			state := makeJoinState(signer, mutators...)
			req.PreviousJoinState = []byte(state)
		}
	}

	type wrappedSolver struct {
		rotatedPubKey string

		rotationCount  uint32
		challengeCount uint32
		solutions      []string

		wrapped client.RegisterUsingBoundKeypairChallengeResponseFunc
	}

	makeSolver := func(initialPubKey string, mutators ...func(s *wrappedSolver)) *wrappedSolver {
		wrapper := &wrappedSolver{
			solutions: []string{},
		}
		for _, mutator := range mutators {
			mutator(wrapper)
		}

		wrapper.wrapped = func(challenge *proto.RegisterUsingBoundKeypairMethodResponse) (*proto.RegisterUsingBoundKeypairMethodRequest, error) {
			switch r := challenge.Response.(type) {
			case *proto.RegisterUsingBoundKeypairMethodResponse_Challenge:
				wrapper.challengeCount++

				switch r.Challenge.PublicKey {
				case initialPubKey:
				case wrapper.rotatedPubKey:
				default:
					return nil, trace.BadParameter("wrong public key")
				}

				wrapper.solutions = append(wrapper.solutions, r.Challenge.PublicKey)

				return &proto.RegisterUsingBoundKeypairMethodRequest{
					Payload: &proto.RegisterUsingBoundKeypairMethodRequest_ChallengeResponse{
						ChallengeResponse: &proto.RegisterUsingBoundKeypairChallengeResponse{
							// For testing purposes, we'll just reply with the
							// public key, to avoid needing to parse the JWT.
							Solution: []byte(r.Challenge.PublicKey),
						},
					},
				}, nil
			case *proto.RegisterUsingBoundKeypairMethodResponse_Rotation:
				wrapper.rotationCount++
				if wrapper.rotatedPubKey == "" {
					return nil, trace.BadParameter("can't generate key")
				}

				return &proto.RegisterUsingBoundKeypairMethodRequest{
					Payload: &proto.RegisterUsingBoundKeypairMethodRequest_RotationResponse{
						RotationResponse: &proto.RegisterUsingBoundKeypairRotationResponse{
							PublicKey: wrapper.rotatedPubKey,
						},
					},
				}, nil
			default:
				return nil, trace.BadParameter("invalid response type")
			}
		}

		return wrapper
	}

	withRotatedPubKey := func(pubKey string) func(s *wrappedSolver) {
		return func(s *wrappedSolver) {
			s.rotatedPubKey = pubKey
		}
	}

	// Advance the clock a bit. Tests may reference `startTime` for a past
	// reference point.
	clock.Advance(time.Hour)

	tests := []struct {
		name string

		token   types.ProvisionTokenV2
		initReq *proto.RegisterUsingBoundKeypairInitialRequest
		solver  *wrappedSolver

		assertError       require.ErrorAssertionFunc
		assertResponse    func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse)
		assertSolverState func(t *testing.T, s *wrappedSolver)
	}{
		{
			// no bound key, no bound bot instance, aka initial join without
			// secret
			name: "initial-join-success",

			token:   makeToken(),
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *client.BoundKeypairRegistrationResponse) {
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

			token:   makeToken(),
			initReq: makeInitReq(),
			solver:  makeSolver(incorrectPublicKey),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "wrong public key")
			},
		},
		{
			// bound key, valid bound bot instance, aka "soft join"
			name: "reauth-success",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.JoinRequest.BotInstanceID = "asdf"
			}),
			solver: makeSolver(correctPublicKey),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *client.BoundKeypairRegistrationResponse) {
				// join count should not be incremented
				require.Equal(t, uint32(0), v2.Status.BoundKeypair.RecoveryCount)
			},
		},
		{
			// bound key, seemingly valid bot instance, but wrong key
			// (should be impossible, but should fail anyway)
			name: "reauth-with-wrong-key",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.JoinRequest.BotInstanceID = "asdf"
			}),
			solver: makeSolver(incorrectPublicKey),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "wrong public key")
			},
		},
		{
			// bound key but no valid incoming bot instance, i.e. the certs
			// expired and triggered a hard rejoin
			name: "rejoin-success",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "asdf"
			}),
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, _ *client.BoundKeypairRegistrationResponse) {
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

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
			}),
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "bad backend state")
			},
		},
		{
			// The client somehow presents certs that refer to a different
			// instance, maybe tried switching auth methods.
			name: "bound-key-wrong-instance",

			token: makeToken(func(v2 *types.ProvisionTokenV2) {
				v2.Status.BoundKeypair.BoundPublicKey = correctPublicKey
				v2.Status.BoundKeypair.BoundBotInstanceID = "qwerty"
			}),
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.JoinRequest.BotInstanceID = "asdf"
			}),
			solver: makeSolver(correctPublicKey),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				require.ErrorContains(tt, err, "bot instance mismatch")
			},
		},
		{
			name:        "standard-initial-recovery-success",
			token:       makeToken(withRecovery("standard", 0, 1, "")),
			initReq:     makeInitReq(),
			solver:      makeSolver(correctPublicKey),
			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, uint32(1), v2.Status.BoundKeypair.RecoveryCount)

				require.NotNil(t, res)
				require.NotEmpty(t, res.JoinState)
			},
		},
		{
			name:        "standard-success-second-recovery",
			token:       makeToken(withRecovery("standard", 1, 2, "id")),
			initReq:     makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id")))),
			solver:      makeSolver(correctPublicKey),
			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, uint32(2), v2.Status.BoundKeypair.RecoveryCount)
				require.NotNil(t, res)
				state := parseJoinState(t, res.JoinState)
				require.Equal(t, v2.Status.BoundKeypair.RecoveryCount, state.RecoverySequence)
			},
		},
		{
			name:    "standard-failure-missing-join-state",
			token:   makeToken(withRecovery("standard", 1, 2, "id")),
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "previous join state is required")
			},
		},
		{
			name:    "standard-failure-limit-exhausted",
			token:   makeToken(withRecovery("standard", 2, 2, "id")),
			initReq: makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("standard", 2, 2, "id")))),
			solver:  makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "no recovery attempts remaining")
			},
		},
		{
			// Attempts to join with an outdated join state document should fail.
			name:    "standard-failure-recovery-count-mismatch",
			token:   makeToken(withRecovery("standard", 2, 3, "id")),
			initReq: makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("standard", 1, 3, "id")))),
			solver:  makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:  "standard-failure-invalid-jwt",
			token: makeToken(withRecovery("standard", 1, 2, "id")),
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.PreviousJoinState = []byte("asdf")
			}),
			solver: makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:    "standard-failure-invalid-jwt-signature",
			token:   makeToken(withRecovery("standard", 1, 2, "id")),
			initReq: makeInitReq(withJoinState(invalidJWTSigner, withToken(withRecovery("standard", 1, 2, "id")))),
			solver:  makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:    "standard-failure-invalid-instance-id",
			token:   makeToken(withRecovery("standard", 1, 2, "foo")),
			initReq: makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id")))),
			solver:  makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:  "standard-failure-invalid-cluster",
			token: makeToken(withRecovery("standard", 1, 2, "foo")),
			initReq: makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("standard", 1, 2, "id")), func(s *boundkeypair.JoinStateParams) {
				s.ClusterName = "wrong-cluster"
			})),
			solver: makeSolver(correctPublicKey),
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "join state verification failed")
			},
		},
		{
			name:        "relaxed-success-count-over-limit",
			token:       makeToken(withRecovery("relaxed", 1, 0, "id")),
			initReq:     makeInitReq(withJoinState(jwtSigner, withToken(withRecovery("relaxed", 1, 0, "id")))),
			solver:      makeSolver(correctPublicKey),
			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey, withRotatedPubKey(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, rotatedPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, rotatedPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey, rotatedPublicKey}, s.solutions)
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey, withRotatedPubKey(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, correctPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutions)
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey, withRotatedPubKey(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, rotatedPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, rotatedPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey, rotatedPublicKey}, s.solutions)
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey, withRotatedPubKey(rotatedPublicKey)),

			assertError: require.NoError,
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
				require.Equal(t, correctPublicKey, v2.Status.BoundKeypair.BoundPublicKey)
				require.Equal(t, correctPublicKey, res.BoundPublicKey)
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutions)
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "requesting a new public key")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)
				require.Equal(t, []string{correctPublicKey}, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(),
			solver:  makeSolver(correctPublicKey, withRotatedPubKey(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "public key may not be reused after rotation")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 2, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)

				// note: the client does complete the challenge for the
				// duplicate key, but the attempt will ultimately be rejected
				require.Equal(t, []string{correctPublicKey, correctPublicKey}, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				// note that we'll need to specify a secret here since there's
				// not a good way to plumb the auto-generated secret back to the
				// test.
				r.InitialJoinSecret = "secret"
			}),
			solver: makeSolver("", withRotatedPubKey(correctPublicKey)),

			assertError: require.NoError,
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 1, s.rotationCount)

				// we'll only be asked for one challenge
				require.Equal(t, []string{correctPublicKey}, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.InitialJoinSecret = "asdf"
			}),
			solver: makeSolver("", withRotatedPubKey(correctPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Empty(t, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.InitialJoinSecret = "asdf"
			}),
			solver: makeSolver(""),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{}, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.InitialJoinSecret = "asdf"
			}),
			solver: makeSolver("", withRotatedPubKey(rotatedPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				// note: error generated by our mock solver above
				require.ErrorContains(t, err, "wrong public key")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 1, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Equal(t, []string{}, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			initReq: makeInitReq(func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.InitialJoinSecret = "secret"
			}),
			solver: makeSolver("", withRotatedPubKey(rotatedPublicKey)),

			assertError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "a valid registration secret is required")
			},
			assertSolverState: func(t *testing.T, s *wrappedSolver) {
				require.EqualValues(t, 0, s.challengeCount)
				require.EqualValues(t, 0, s.rotationCount)
				require.Empty(t, s.solutions)
			},
			assertResponse: func(t *testing.T, v2 *types.ProvisionTokenV2, res *client.BoundKeypairRegistrationResponse) {
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
			require.NoError(t, auth.CreateBoundKeypairToken(ctx, token))
			tt.initReq.JoinRequest.Token = tt.name

			response, err := auth.RegisterUsingBoundKeypairMethod(ctx, tt.initReq, tt.solver.wrapped)
			tt.assertError(t, err)

			if tt.assertResponse != nil {
				pt, err := auth.GetToken(ctx, tt.name)
				require.NoError(t, err)

				ptv2, ok := pt.(*types.ProvisionTokenV2)
				require.True(t, ok)

				tt.assertResponse(t, ptv2, response)
			}

			if tt.assertSolverState != nil {
				tt.assertSolverState(t, tt.solver)
			}
		})
	}
}

type mockSolver struct {
	publicKey string
}

func (m *mockSolver) solver() client.RegisterUsingBoundKeypairChallengeResponseFunc {
	return func(challenge *proto.RegisterUsingBoundKeypairMethodResponse) (*proto.RegisterUsingBoundKeypairMethodRequest, error) {
		switch r := challenge.Response.(type) {
		case *proto.RegisterUsingBoundKeypairMethodResponse_Rotation:
			return &proto.RegisterUsingBoundKeypairMethodRequest{
				Payload: &proto.RegisterUsingBoundKeypairMethodRequest_RotationResponse{
					RotationResponse: &proto.RegisterUsingBoundKeypairRotationResponse{
						PublicKey: m.publicKey,
					},
				},
			}, nil
		case *proto.RegisterUsingBoundKeypairMethodResponse_Challenge:
			return &proto.RegisterUsingBoundKeypairMethodRequest{
				Payload: &proto.RegisterUsingBoundKeypairMethodRequest_ChallengeResponse{
					ChallengeResponse: &proto.RegisterUsingBoundKeypairChallengeResponse{
						// For testing purposes, we'll just reply with the
						// public key, to avoid needing to parse the JWT.
						Solution: []byte(r.Challenge.PublicKey),
					},
				},
			}, nil
		default:
			return nil, trace.BadParameter("not supported")

		}
	}
}

func newMockSolver(t *testing.T, pubKey string) *mockSolver {
	t.Helper()

	return &mockSolver{
		publicKey: pubKey,
	}
}

func testExtractBotParamsFromCerts(t *testing.T, certs *proto.Certs) (string, uint64) {
	t.Helper()

	parsed, err := tlsca.ParseCertificatePEM(certs.TLS)
	require.NoError(t, err)
	ident, err := tlsca.FromSubject(parsed.Subject, parsed.NotAfter)
	require.NoError(t, err)

	return ident.BotInstanceID, ident.Generation
}

func TestServer_RegisterUsingBoundKeypairMethod_GenerationCounter(t *testing.T) {
	ctx := context.Background()

	// TODO: This prevents parallel execution; remove along with the experiment.
	boundkeypairexperiment.SetEnabled(true)

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	_, correctPublicKey := testBoundKeypair(t)

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())

	srv := newTestTLSServer(t, withClock(clock))
	auth := srv.Auth()
	auth.createBoundKeypairValidator = func(subject, clusterName string, publicKey crypto.PublicKey) (boundKeypairValidator, error) {
		return &mockBoundKeypairValidator{
			subject:     subject,
			clusterName: clusterName,
			publicKey:   publicKey,
		}, nil
	}

	_, err = CreateRole(ctx, auth, "example", types.RoleSpecV6{})
	require.NoError(t, err)

	adminClient, err := srv.NewClient(TestAdmin())
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
					Limit: 2,
				},
			},
		},
		&types.ProvisionTokenStatusV2{},
	)
	require.NoError(t, err)
	require.NoError(t, auth.CreateBoundKeypairToken(ctx, token))

	makeInitReq := func(mutators ...func(r *proto.RegisterUsingBoundKeypairInitialRequest)) *proto.RegisterUsingBoundKeypairInitialRequest {
		req := &proto.RegisterUsingBoundKeypairInitialRequest{
			JoinRequest: &types.RegisterUsingTokenRequest{
				HostID:       "host-id",
				Role:         types.RoleBot,
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: sshPublicKey,
				Token:        "bound-keypair-test",
			},
		}
		for _, mutator := range mutators {
			mutator(req)
		}
		return req
	}

	withJoinState := func(state []byte) func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
		return func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
			r.PreviousJoinState = state
		}
	}

	withBotParamsFromIdent := func(t *testing.T, certs *proto.Certs) func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
		id, gen := testExtractBotParamsFromCerts(t, certs)

		return func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
			r.JoinRequest.BotInstanceID = id
			r.JoinRequest.BotGeneration = int32(gen)
		}
	}

	solver := newMockSolver(t, correctPublicKey)
	response, err := auth.RegisterUsingBoundKeypairMethod(ctx, makeInitReq(), solver.solver())
	require.NoError(t, err)

	instance, generation := testExtractBotParamsFromCerts(t, response.Certs)
	require.Equal(t, uint64(1), generation)

	firstInstance := instance

	// Register several times.
	for i := range 10 {
		response, err = auth.RegisterUsingBoundKeypairMethod(
			ctx,
			makeInitReq(withJoinState(response.JoinState), withBotParamsFromIdent(t, response.Certs)),
			solver.solver(),
		)
		require.NoError(t, err)

		instance, generation := testExtractBotParamsFromCerts(t, response.Certs)
		require.Equal(t, uint64(i+2), generation)
		require.Equal(t, firstInstance, instance)
	}

	// Perform a recovery to get a new instance and reset the counter.
	response, err = auth.RegisterUsingBoundKeypairMethod(ctx, makeInitReq(withJoinState(response.JoinState)), solver.solver())
	require.NoError(t, err)

	instance, generation = testExtractBotParamsFromCerts(t, response.Certs)
	require.Equal(t, uint64(1), generation, "generation counter should reset")
	require.NotEqual(t, instance, firstInstance)

	secondInstance := instance

	// Register several more times.
	for i := range 10 {
		response, err = auth.RegisterUsingBoundKeypairMethod(
			ctx,
			makeInitReq(withJoinState(response.JoinState), withBotParamsFromIdent(t, response.Certs)),
			solver.solver(),
		)
		require.NoError(t, err)

		instance, generation := testExtractBotParamsFromCerts(t, response.Certs)
		require.Equal(t, uint64(i+2), generation)
		require.Equal(t, secondInstance, instance)
	}

	// Provide an incorrect generation counter value.
	response, err = auth.RegisterUsingBoundKeypairMethod(
		ctx,
		makeInitReq(
			withJoinState(response.JoinState),
			withBotParamsFromIdent(t, response.Certs),
			func(r *proto.RegisterUsingBoundKeypairInitialRequest) {
				r.JoinRequest.BotGeneration = 1
			},
		),
		solver.solver(),
	)
	require.Nil(t, response)
	require.ErrorContains(t, err, "renewable cert generation mismatch")

	// The bot user should now be locked.
	locks, err := srv.Auth().GetLocks(ctx, true, types.LockTarget{
		User: "bot-test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, locks)
}
