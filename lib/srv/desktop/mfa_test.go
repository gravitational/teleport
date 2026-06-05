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

package desktop_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/lib/srv/desktop"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	teleportUsername = "alice"
)

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

func TestNewAuthPrompt(t *testing.T) {
	t.Parallel()

	prompt := desktop.NewAuthPrompt()
	require.NotNil(t, prompt)
	require.NotNil(t, prompt.GetMfaPrompt())
}

func TestMFAPromptVerifier_VerifyResponse_Success(t *testing.T) {
	t.Parallel()

	cv := &mockValidatedMFAChallengeVerifier{
		verifyValidatedMFAChallenge: func(
			_ context.Context,
			req *mfav2.VerifyValidatedMFAChallengeRequest,
			_ ...grpc.CallOption,
		) (*mfav2.VerifyValidatedMFAChallengeResponse, error) {
			if req.GetName() != challengeName {
				return nil, trace.Errorf("unexpected challenge name: got %q, want %q", req.GetName(), challengeName)
			}
			return &mfav2.VerifyValidatedMFAChallengeResponse{}, nil
		},
	}

	verifier, err := desktop.NewMFAPromptVerifier(
		cv,
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &tdpbv1.MFAPromptResponse{
		Response: &tdpbv1.MFAPromptResponse_Reference{
			Reference: &tdpbv1.MFAPromptResponseReference{
				ChallengeName: challengeName,
			},
		},
	}

	err = verifier.VerifyResponse(t.Context(), resp)
	require.NoError(t, err)
}

func TestMFAPromptVerifier_VerifyResponse_MissingResponse(t *testing.T) {
	t.Parallel()

	verifier, err := desktop.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &tdpbv1.MFAPromptResponse{Response: nil}

	err = verifier.VerifyResponse(t.Context(), resp)
	require.ErrorIs(t, err, trace.BadParameter("missing or unknown MFAPromptResponse type: <nil>"))
}
