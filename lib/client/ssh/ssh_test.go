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
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"

	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	clientssh "github.com/gravitational/teleport/lib/client/ssh"
)

func TestKeyboardInteractive_SuccessfulMFA(t *testing.T) {
	const challengeName = "test-challenge-name"

	authMethod := clientssh.KeyboardInteractive(
		t.Context(),
		func(_ context.Context, _ []byte) (string, error) {
			return challengeName, nil
		},
		&mockConnMetadata{
			sessionID: []byte("test-session-id"),
		},
	)

	mfaPrompt := sshpb.AuthPrompt_builder{
		MfaPrompt: &sshpb.MFAPrompt{},
	}.Build()
	mfaPromptJSON, err := protojson.Marshal(mfaPrompt)
	require.NoError(t, err)

	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{string(mfaPromptJSON)}, []bool{})
	require.NoError(t, err)
	require.Len(t, answers, 1, "expected exactly 1 answer in response")

	resp := &sshpb.MFAPromptResponse{}
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(answers[0]), resp)
	require.NoError(t, err)
	require.Equal(t, challengeName, resp.GetReference().GetChallengeName())
}

func TestKeyboardInteractive_FailedMFA(t *testing.T) {
	authMethod := clientssh.KeyboardInteractive(
		t.Context(),
		func(_ context.Context, _ []byte) (string, error) {
			return "", trace.Errorf("a-wild-error-appeared!")
		},
		&mockConnMetadata{
			sessionID: []byte("test-session-id"),
		},
	)

	mfaPrompt := sshpb.AuthPrompt_builder{
		MfaPrompt: &sshpb.MFAPrompt{},
	}.Build()
	mfaPromptJSON, err := protojson.Marshal(mfaPrompt)
	require.NoError(t, err)

	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{string(mfaPromptJSON)}, []bool{})
	require.ErrorContains(t, err, "a-wild-error-appeared!")
	require.Nil(t, answers)
}

func TestKeyboardInteractive_InvalidAuthPrompt_NonProtoQuestion(t *testing.T) {
	authMethod := clientssh.KeyboardInteractive(t.Context(), nil, nil)

	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{"invalid-auth-prompt"}, []bool{})
	require.ErrorContains(t, err, "invalid value invalid-auth-prompt")
	require.Nil(t, answers)
}

func TestKeyboardInteractive_InvalidAuthPrompt_NilPromptField(t *testing.T) {
	authMethod := clientssh.KeyboardInteractive(t.Context(), nil, nil)

	mfaPrompt := &sshpb.AuthPrompt{
		Prompt: nil,
	}
	mfaPromptJSON, err := protojson.Marshal(mfaPrompt)
	require.NoError(t, err)

	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{string(mfaPromptJSON)}, []bool{})
	require.ErrorIs(t, err, trace.BadParameter("received sshpb.AuthPrompt with nil Prompt field"))
	require.Nil(t, answers)
}

func TestAuthCallback_ReturnsKeyboardInteractive(t *testing.T) {
	t.Parallel()

	callback := clientssh.AuthCallback(
		t.Context(),
		func() (clientssh.Performer, error) {
			return func(_ context.Context, _ []byte) (string, error) {
				return "test-challenge", nil
			}, nil
		},
	)

	authMethod, err := callback(
		&ssh.ClientAuthContext{
			PartialSuccessMethods: []string{"publickey"},
			AllowedMethods:        []string{"keyboard-interactive"},
			Metadata:              &mockConnMetadata{sessionID: []byte("test-session-id")},
		},
	)
	require.NoError(t, err)

	_, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)
}

func TestAuthCallback_ReturnsNil_KeyboardInteractiveNotAllowed(t *testing.T) {
	t.Parallel()

	callback := clientssh.AuthCallback(
		t.Context(),
		func() (clientssh.Performer, error) {
			return nil, nil
		},
	)

	authMethod, err := callback(
		&ssh.ClientAuthContext{
			PartialSuccessMethods: []string{"publickey"},
			AllowedMethods:        []string{"password"},
		},
	)
	require.NoError(t, err)
	require.Nil(t, authMethod)
}

func TestAuthCallback_ReturnsNil_NoPublickeyPartialSuccess(t *testing.T) {
	t.Parallel()

	callback := clientssh.AuthCallback(
		t.Context(), func() (clientssh.Performer, error) {
			return nil, nil
		},
	)

	authMethod, err := callback(
		&ssh.ClientAuthContext{
			PartialSuccessMethods: []string{"password"},
			AllowedMethods:        []string{"keyboard-interactive"},
		},
	)
	require.NoError(t, err)
	require.Nil(t, authMethod)
}

func TestAuthCallback_PerformerErrorPropagates(t *testing.T) {
	t.Parallel()

	callback := clientssh.AuthCallback(
		t.Context(),
		func() (clientssh.Performer, error) {
			return nil, trace.BadParameter("some performer error")
		},
	)

	authMethod, err := callback(
		&ssh.ClientAuthContext{
			PartialSuccessMethods: []string{"publickey"},
			AllowedMethods:        []string{"keyboard-interactive"},
		},
	)
	require.ErrorIs(t, err, trace.BadParameter("some performer error"))
	require.Nil(t, authMethod)
}

type mockConnMetadata struct {
	ssh.ConnMetadata

	sessionID []byte
}

func (m *mockConnMetadata) SessionID() []byte { return m.sessionID }
