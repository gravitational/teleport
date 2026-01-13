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
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
)

const (
	sessionID        = "test-session-id"
	challengeName    = "test-challenge-name"
	sourceCluster    = "test-cluster"
	osUsername       = "nonroot"
	teleportUsername = "alice"
)

var authPromptsWithMFAOnly = []*sshpb.AuthPrompt{{
	Prompt: &sshpb.AuthPrompt_MfaPrompt{
		MfaPrompt: &sshpb.MFAPrompt{},
	},
}}

func TestKeyboardInteractiveCallback_SuccessfulMFA(t *testing.T) {
	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeSuccess(challengeName),
		Permissions:   &ssh.Permissions{Extensions: map[string]string{"foo": "bar"}},
		Verifier:      &mockValidatedMFAChallengeVerifier{expectedChallengeName: challengeName},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.NoError(t, err)
	require.Equal(t, params.Permissions, perms)
}

func TestKeyboardInteractiveCallback_FailedMFA(t *testing.T) {
	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeFailure("a-wild-error-appeared!"),
		Permissions:   &ssh.Permissions{},
		Verifier:      &mockValidatedMFAChallengeVerifier{},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorContains(t, err, "a-wild-error-appeared!")
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_NonProtoAnswer(t *testing.T) {
	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeRaw([]string{"non-proto-answer"}),
		Permissions:   &ssh.Permissions{},
		Verifier:      &mockValidatedMFAChallengeVerifier{},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorContains(t, err, "invalid value non-proto-answer")
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_TooManyAnswers(t *testing.T) {
	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeRaw([]string{"answer1", "answer2"}),
		Permissions:   &ssh.Permissions{},
		Verifier:      &mockValidatedMFAChallengeVerifier{},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorIs(t, err, trace.BadParameter("expected exactly 1 answers, got 2 answers"))
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_NilReferenceField(t *testing.T) {
	resp := &sshpb.MFAPromptResponse{
		Response: nil,
	}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeRaw([]string{string(respJSON)}),
		Permissions:   &ssh.Permissions{},
		Verifier:      &mockValidatedMFAChallengeVerifier{},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorIs(t, err, trace.BadParameter("received sshpb.MFAPromptResponse with nil Response field"))
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_EmptyReferenceField(t *testing.T) {
	resp := &sshpb.MFAPromptResponse{
		Response: &sshpb.MFAPromptResponse_Reference{
			Reference: &sshpb.MFAPromptResponseReference{
				ChallengeName: "",
			},
		},
	}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	params := srvssh.KeyboardInteractiveCallbackParams{
		Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
		Challenge:     mockKeyboardInteractiveChallengeRaw([]string{string(respJSON)}),
		Permissions:   &ssh.Permissions{},
		Verifier:      &mockValidatedMFAChallengeVerifier{},
		SourceCluster: sourceCluster,
		Username:      teleportUsername,
		Prompts:       authPromptsWithMFAOnly,
	}

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorIs(t, err, trace.BadParameter("received sshpb.MFAPromptResponseReference with empty ChallengeName field"))
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_CheckParams(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		modify  func(params *srvssh.KeyboardInteractiveCallbackParams)
		wantErr error
	}{
		{
			name: "missing Metadata",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Metadata = nil
			},
			wantErr: trace.BadParameter("params Metadata must be set"),
		},
		{
			name: "missing Challenge",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Challenge = nil
			},
			wantErr: trace.BadParameter("params Challenge must be set"),
		},
		{
			name: "missing Permissions",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Permissions = nil
			},
			wantErr: trace.BadParameter("params Permissions must be set"),
		},
		{
			name: "missing Verifier",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Verifier = nil
			},
			wantErr: trace.BadParameter("params Verifier must be set"),
		},
		{
			name: "missing SourceCluster",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.SourceCluster = ""
			},
			wantErr: trace.BadParameter("params SourceCluster must be set"),
		},
		{
			name: "missing Username",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Username = ""
			},
			wantErr: trace.BadParameter("params Username must be set"),
		},
		{
			name: "missing Prompts",
			modify: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Prompts = nil
			},
			wantErr: trace.BadParameter("params Prompts must be set and contain at least one prompt"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			params := srvssh.KeyboardInteractiveCallbackParams{
				Metadata:      &mockConnMetadata{sessionID: []byte(sessionID), user: osUsername},
				Challenge:     mockKeyboardInteractiveChallengeSuccess(challengeName),
				Permissions:   &ssh.Permissions{},
				Verifier:      &mockValidatedMFAChallengeVerifier{},
				SourceCluster: sourceCluster,
				Username:      teleportUsername,
				Prompts:       authPromptsWithMFAOnly,
			}

			testCase.modify(&params)

			perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
			require.ErrorIs(t, err, testCase.wantErr)
			require.Nil(t, perms)
		})
	}
}

type mockConnMetadata struct {
	ssh.ConnMetadata

	sessionID []byte
	user      string
}

func (m *mockConnMetadata) SessionID() []byte { return m.sessionID }
func (m *mockConnMetadata) User() string      { return m.user }

type mockValidatedMFAChallengeVerifier struct {
	expectedChallengeName string
	err                   error
}

func (m *mockValidatedMFAChallengeVerifier) VerifyValidatedMFAChallenge(_ context.Context, req *mfav1.VerifyValidatedMFAChallengeRequest) error {
	if m.err != nil {
		return m.err
	}

	if m.expectedChallengeName != "" && req.Name != m.expectedChallengeName {
		return trace.Errorf("unexpected challenge name: got %q, want %q", req.Name, m.expectedChallengeName)
	}

	return nil
}

// mockKeyboardInteractiveChallengeSuccess returns a KeyboardInteractiveChallenge that simulates a successful MFA prompt response.
func mockKeyboardInteractiveChallengeSuccess(challengeName string) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
		resp := &sshpb.MFAPromptResponse{
			Response: &sshpb.MFAPromptResponse_Reference{
				Reference: &sshpb.MFAPromptResponseReference{
					ChallengeName: challengeName,
				},
			},
		}

		respJSON, err := protojson.Marshal(resp)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return []string{string(respJSON)}, nil
	}
}

// mockKeyboardInteractiveChallengeFailure returns a KeyboardInteractiveChallenge that simulates a failure.
func mockKeyboardInteractiveChallengeFailure(errMsg string) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
		return nil, errors.New(errMsg)
	}
}

// mockKeyboardInteractiveChallengeRaw returns a KeyboardInteractiveChallenge that returns the provided answers as-is.
func mockKeyboardInteractiveChallengeRaw(answers []string) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, _ []string, _ []bool) ([]string, error) {
		return answers, nil
	}
}
