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

	// Extract the KeyboardInteractiveChallenge function from the AuthMethod so it can be tested.
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

	// Extract the KeyboardInteractiveChallenge function from the AuthMethod so it can be tested.
	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{string(mfaPromptJSON)}, []bool{})
	require.ErrorContains(t, err, "a-wild-error-appeared!")
	require.Nil(t, answers)
}

func TestKeyboardInteractive_InvalidAuthPrompt_NonProtoQuestion(t *testing.T) {
	authMethod := clientssh.KeyboardInteractive(t.Context(), nil, nil)

	// Extract the KeyboardInteractiveChallenge function from the AuthMethod so it can be tested.
	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{"invalid-auth-prompt"}, []bool{})
	require.ErrorContains(t, err, "invalid value invalid-auth-prompt")
	require.Nil(t, answers)
}

func TestKeyboardInteractive_InvalidAuthPrompt_NilPromptField(t *testing.T) {
	authMethod := clientssh.KeyboardInteractive(t.Context(), nil, nil)

	mfaPrompt := sshpb.AuthPrompt_builder{}.Build()
	mfaPromptJSON, err := protojson.Marshal(mfaPrompt)
	require.NoError(t, err)

	// Extract the KeyboardInteractiveChallenge function from the AuthMethod so it can be tested.
	ki, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
	require.True(t, ok)

	answers, err := ki("", "", []string{string(mfaPromptJSON)}, []bool{})
	require.ErrorIs(t, err, trace.BadParameter("received sshpb.AuthPrompt with nil Prompt field"))
	require.Nil(t, answers)
}

const (
	keyboardInteractiveMethod = "keyboard-interactive"
	passwordMethod            = "password"
	publicKeyMethod           = "publickey"
)

func TestAuthCallback(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name             string
		config           clientssh.AuthCallbackConfig
		authCtx          *ssh.ClientAuthContext
		expectNil        bool
		expectKICallback bool
	}{
		{
			name: "returns keyboard-interactive callback on partial success",
			config: clientssh.AuthCallbackConfig{
				MFAPerformer: func(_ context.Context, _ []byte) (string, error) {
					return "test-challenge", nil
				},
			},
			authCtx: &ssh.ClientAuthContext{
				PartialSuccessMethods: []string{publicKeyMethod},
				AllowedMethods:        []string{keyboardInteractiveMethod},
				Metadata:              &mockConnMetadata{sessionID: []byte("test-session-id")},
			},
			expectKICallback: true,
		},
		{
			name: "returns nil when keyboard-interactive not allowed",
			authCtx: &ssh.ClientAuthContext{
				PartialSuccessMethods: []string{publicKeyMethod},
				AllowedMethods:        []string{passwordMethod},
			},
			expectNil: true,
		},
		{
			name: "returns nil when MFAPerformer is not set",
			config: clientssh.AuthCallbackConfig{
				MFAPerformer: nil,
			},
			authCtx: &ssh.ClientAuthContext{
				PartialSuccessMethods: []string{publicKeyMethod},
				AllowedMethods:        []string{keyboardInteractiveMethod},
			},
			expectNil: true,
		},
		{
			name: "returns nil when no publickey partial success",
			authCtx: &ssh.ClientAuthContext{
				PartialSuccessMethods: []string{passwordMethod},
				AllowedMethods:        []string{keyboardInteractiveMethod},
			},
			expectNil: true,
		},
		{
			name: "returns nil when keyboard-interactive already tried",
			authCtx: &ssh.ClientAuthContext{
				PartialSuccessMethods: []string{publicKeyMethod},
				AllowedMethods:        []string{keyboardInteractiveMethod},
				TriedMethods:          []string{keyboardInteractiveMethod},
			},
			expectNil: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callback := clientssh.AuthCallback(t.Context(), tt.config)

			authMethod, err := callback(tt.authCtx)
			require.NoError(t, err)

			if tt.expectNil {
				require.Nil(t, authMethod)
			}

			if tt.expectKICallback {
				_, ok := authMethod.(ssh.KeyboardInteractiveChallenge)
				require.True(t, ok)
			}
		})
	}
}

type mockConnMetadata struct {
	ssh.ConnMetadata

	sessionID []byte
}

func (m *mockConnMetadata) SessionID() []byte { return m.sessionID }
