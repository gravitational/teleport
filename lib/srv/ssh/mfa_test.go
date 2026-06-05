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

package ssh_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	teleportUsername = "alice"
)

func TestMFAPromptVerifier_MarshalPrompt(t *testing.T) {
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

	verifier, err := srvssh.NewMFAPromptVerifier(
		cv,
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	prompt, echo, err := verifier.MarshalPrompt()
	require.NoError(t, err)
	require.False(t, echo)
	require.Contains(t, prompt, srvssh.MFAPromptMessage)
}

func TestMFAPromptVerifier_VerifyAnswer_Success(t *testing.T) {
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

	verifier, err := srvssh.NewMFAPromptVerifier(
		cv,
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &sshpb.MFAPromptResponse{
		Response: &sshpb.MFAPromptResponse_Reference{
			Reference: &sshpb.MFAPromptResponseReference{
				ChallengeName: challengeName,
			},
		},
	}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	err = verifier.VerifyAnswer(t.Context(), string(respJSON))
	require.NoError(t, err)
}

func TestMFAPromptVerifier_VerifyAnswer_InvalidJSON(t *testing.T) {
	t.Parallel()

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	err = verifier.VerifyAnswer(t.Context(), "not-json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid value not-json")
}

func TestMFAPromptVerifier_VerifyAnswer_MissingResponse(t *testing.T) {
	t.Parallel()

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &sshpb.MFAPromptResponse{Response: nil}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	err = verifier.VerifyAnswer(t.Context(), string(respJSON))
	require.ErrorIs(t, err, trace.BadParameter("missing or unknown MFAPromptResponse type: <nil>"))
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
