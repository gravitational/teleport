/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package mfa

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	teleportUsername = "alice"
)

func TestNewVerifier_InvalidParams(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name              string
		challengeVerifier challengeVerifier
		sourceCluster     string
		username          string
		sessionID         []byte
		wantErr           error
	}{
		{
			name:              "nil challengeVerifier",
			challengeVerifier: nil,
			sourceCluster:     sourceCluster,
			username:          teleportUsername,
			sessionID:         []byte(sessionID),
			wantErr:           trace.BadParameter("params ChallengeVerifier must be set"),
		},
		{
			name:              "empty sourceCluster",
			challengeVerifier: &mockValidatedMFAChallengeVerifier{},
			sourceCluster:     "",
			username:          teleportUsername,
			sessionID:         []byte(sessionID),
			wantErr:           trace.BadParameter("params SourceCluster must be set"),
		},
		{
			name:              "empty username",
			challengeVerifier: &mockValidatedMFAChallengeVerifier{},
			sourceCluster:     sourceCluster,
			username:          "",
			sessionID:         []byte(sessionID),
			wantErr:           trace.BadParameter("params Username must be set"),
		},
		{
			name:              "nil sessionID",
			challengeVerifier: &mockValidatedMFAChallengeVerifier{},
			sourceCluster:     sourceCluster,
			username:          teleportUsername,
			sessionID:         nil,
			wantErr:           trace.BadParameter("params SessionID must be set and be non-empty"),
		},
		{
			name:              "empty sessionID",
			challengeVerifier: &mockValidatedMFAChallengeVerifier{},
			sourceCluster:     sourceCluster,
			username:          teleportUsername,
			sessionID:         []byte(""),
			wantErr:           trace.BadParameter("params SessionID must be set and be non-empty"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewVerifier(tc.challengeVerifier, tc.sourceCluster, tc.username, tc.sessionID)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestVerifier_Verify_Success(t *testing.T) {
	t.Parallel()

	mockVerify := func(_ context.Context, req *mfav2.VerifyValidatedMFAChallengeRequest, _ ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
		if req.GetName() != challengeName {
			return nil, trace.Errorf("unexpected challenge name: got %q, want %q", req.GetName(), challengeName)
		}
		return &mfav2.VerifyValidatedMFAChallengeResponse{}, nil
	}

	v, err := NewVerifier(
		&mockValidatedMFAChallengeVerifier{verifyValidatedMFAChallenge: mockVerify},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	err = v.Verify(
		t.Context(),
		challengeName,
		func() *mfav2.SessionIdentifyingPayload {
			return mfav2.SessionIdentifyingPayload_builder{
				TlsSessionId: []byte(sessionID),
			}.Build()
		},
	)
	require.NoError(t, err)
}

func TestVerifier_Verify_Error(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		mockVerify    func(context.Context, *mfav2.VerifyValidatedMFAChallengeRequest, ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
		challengeName string
		wantErr       error
	}{
		{
			name: "empty challenge name",
			mockVerify: func(_ context.Context, _ *mfav2.VerifyValidatedMFAChallengeRequest, _ ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
				return nil, nil
			},
			challengeName: "",
			wantErr:       trace.BadParameter("missing ChallengeName in MFAPromptResponseReference"),
		},
		{
			name: "verifier error",
			mockVerify: func(_ context.Context, _ *mfav2.VerifyValidatedMFAChallengeRequest, _ ...grpc.CallOption) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
				return nil, trace.NotFound("challenge expired")
			},
			challengeName: challengeName,
			wantErr:       trace.NotFound("challenge expired"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			v, err := NewVerifier(
				&mockValidatedMFAChallengeVerifier{verifyValidatedMFAChallenge: tc.mockVerify},
				sourceCluster,
				teleportUsername,
				[]byte(sessionID),
			)
			require.NoError(t, err)

			err = v.Verify(
				t.Context(),
				tc.challengeName,
				func() *mfav2.SessionIdentifyingPayload {
					return mfav2.SessionIdentifyingPayload_builder{
						TlsSessionId: []byte(sessionID),
					}.Build()
				},
			)

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestVerifier_SessionID(t *testing.T) {
	t.Parallel()

	v, err := NewVerifier(
		&mockValidatedMFAChallengeVerifier{},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)
	require.Equal(t, []byte(sessionID), v.SessionID())
}

type mockValidatedMFAChallengeVerifier struct {
	verifyValidatedMFAChallenge func(
		context.Context,
		*mfav2.VerifyValidatedMFAChallengeRequest,
		...grpc.CallOption,
	) (*mfav2.VerifyValidatedMFAChallengeResponse, error)
}

func (m *mockValidatedMFAChallengeVerifier) VerifyValidatedMFAChallenge(
	ctx context.Context,
	req *mfav2.VerifyValidatedMFAChallengeRequest,
	opts ...grpc.CallOption,
) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
	return m.verifyValidatedMFAChallenge(ctx, req, opts...)
}
