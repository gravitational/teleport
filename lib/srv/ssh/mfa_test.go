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

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	teleportUsername = "alice"
)

func TestNewMFAPromptVerifier_InvalidParams(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name          string
		verifier      srvssh.ValidatedMFAChallengeVerifier
		sourceCluster string
		username      string
		sessionID     []byte
		wantErr       error
	}{
		{
			name:          "nil verifier",
			verifier:      nil,
			sourceCluster: sourceCluster,
			username:      teleportUsername,
			sessionID:     []byte(sessionID),
			wantErr:       trace.BadParameter("params Verifier must be set"),
		},
		{
			name:          "empty sourceCluster",
			verifier:      &mockValidatedMFAChallengeVerifier{},
			sourceCluster: "",
			username:      teleportUsername,
			sessionID:     []byte(sessionID),
			wantErr:       trace.BadParameter("params SourceCluster must be set"),
		},
		{
			name:          "empty username",
			verifier:      &mockValidatedMFAChallengeVerifier{},
			sourceCluster: sourceCluster,
			username:      "",
			sessionID:     []byte(sessionID),
			wantErr:       trace.BadParameter("params Username must be set"),
		},
		{
			name:          "nil sessionID",
			verifier:      &mockValidatedMFAChallengeVerifier{},
			sourceCluster: sourceCluster,
			username:      teleportUsername,
			sessionID:     nil,
			wantErr:       trace.BadParameter("params SessionID must be set and be non-empty"),
		},
		{
			name:          "empty sessionID",
			verifier:      &mockValidatedMFAChallengeVerifier{},
			sourceCluster: sourceCluster,
			username:      teleportUsername,
			sessionID:     []byte(""),
			wantErr:       trace.BadParameter("params SessionID must be set and be non-empty"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := srvssh.NewMFAPromptVerifier(
				testCase.verifier,
				testCase.sourceCluster,
				testCase.username,
				testCase.sessionID,
			)
			require.ErrorIs(t, err, testCase.wantErr)
		})
	}
}

func TestMFAPromptVerifier_MarshalPrompt(t *testing.T) {
	t.Parallel()

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
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

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
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
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
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
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &sshpb.MFAPromptResponse{Response: nil}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	err = verifier.VerifyAnswer(t.Context(), string(respJSON))
	require.ErrorIs(t, err, trace.BadParameter("missing Response in MFAPromptResponse"))
}

func TestMFAPromptVerifier_VerifyAnswer_EmptyChallengeName(t *testing.T) {
	t.Parallel()

	verifier, err := srvssh.NewMFAPromptVerifier(
		&mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
		sourceCluster,
		teleportUsername,
		[]byte(sessionID),
	)
	require.NoError(t, err)

	resp := &sshpb.MFAPromptResponse{
		Response: &sshpb.MFAPromptResponse_Reference{
			Reference: &sshpb.MFAPromptResponseReference{
				ChallengeName: "",
			},
		},
	}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	err = verifier.VerifyAnswer(t.Context(), string(respJSON))
	require.ErrorIs(t, err, trace.BadParameter("missing ChallengeName in MFAPromptResponseReference"))
}

type mockValidatedMFAChallengeVerifier struct {
	expectedChallengeName string
	err                   error
}

func (m *mockValidatedMFAChallengeVerifier) VerifyValidatedMFAChallenge(
	_ context.Context,
	req *mfav1.VerifyValidatedMFAChallengeRequest,
	_ ...grpc.CallOption,
) (*mfav1.VerifyValidatedMFAChallengeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.expectedChallengeName != "" && req.Name != m.expectedChallengeName {
		return nil, trace.Errorf(
			"unexpected challenge name: got %q, want %q",
			req.Name,
			m.expectedChallengeName,
		)
	}

	return &mfav1.VerifyValidatedMFAChallengeResponse{}, nil
}
