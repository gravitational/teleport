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

	srvssh "github.com/gravitational/teleport/lib/srv/ssh"
)

var keyboardInteractiveCallbackParams = srvssh.KeyboardInteractiveCallbackParams{
	Metadata:    &mockConnMetadata{sessionID: []byte(sessionID), user: "nonroot"},
	Challenge:   mockKeyboardInteractiveChallengeRaw([]string{"test-answer"}),
	Permissions: &ssh.Permissions{Extensions: map[string]string{"foo": "bar"}},
	PromptVerifiers: []srvssh.PromptVerifier{
		&mockPromptVerifier{
			Prompt:         "test-prompt",
			Echo:           false,
			ExpectedAnswer: "test-answer",
		},
	},
}

func TestKeyboardInteractiveCallback_Success(t *testing.T) {
	t.Parallel()

	params := keyboardInteractiveCallbackParams

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.NoError(t, err)
	require.Equal(t, params.Permissions, perms)
}

func TestKeyboardInteractiveCallback_Failed(t *testing.T) {
	t.Parallel()

	params := keyboardInteractiveCallbackParams
	params.Challenge = mockKeyboardInteractiveChallengeFailure(trace.BadParameter("a-wild-error-appeared!"))

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorIs(t, err, trace.BadParameter("a-wild-error-appeared!"))
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_NonProtoAnswer(t *testing.T) {
	t.Parallel()

	params := keyboardInteractiveCallbackParams
	params.Challenge = mockKeyboardInteractiveChallengeRaw([]string{"non-proto-answer"})

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorContains(t, err, `got "non-proto-answer", want "test-answer"`)
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_TooManyAnswers(t *testing.T) {
	t.Parallel()

	params := keyboardInteractiveCallbackParams
	params.Challenge = mockKeyboardInteractiveChallengeRaw([]string{"answer1", "answer2"})

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.ErrorIs(t, err, trace.BadParameter("expected exactly 1 answer(s), got 2 answer(s)"))
	require.Nil(t, perms)
}

func TestKeyboardInteractiveCallback_MultiplePromptVerifiers(t *testing.T) {
	t.Parallel()

	params := keyboardInteractiveCallbackParams
	params.PromptVerifiers = []srvssh.PromptVerifier{
		&mockPromptVerifier{
			Prompt:         "test-prompt-1",
			Echo:           false,
			ExpectedAnswer: "test-answer-1",
		},
		&mockPromptVerifier{
			Prompt:         "test-prompt-2",
			Echo:           false,
			ExpectedAnswer: "test-answer-2",
		},
	}
	params.Challenge = mockKeyboardInteractiveChallengeRaw([]string{"test-answer-1", "test-answer-2"})

	perms, err := srvssh.KeyboardInteractiveCallback(t.Context(), params)
	require.NoError(t, err)
	require.Equal(t, params.Permissions, perms)
}

func TestKeyboardInteractiveCallback_CheckParams(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		mutate  func(params *srvssh.KeyboardInteractiveCallbackParams)
		wantErr error
	}{
		{
			name: "missing Metadata",
			mutate: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Metadata = nil
			},
			wantErr: trace.BadParameter("params Metadata must be set"),
		},
		{
			name: "missing Challenge",
			mutate: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Challenge = nil
			},
			wantErr: trace.BadParameter("params Challenge must be set"),
		},
		{
			name: "missing Permissions",
			mutate: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.Permissions = nil
			},
			wantErr: trace.BadParameter("params Permissions must be set"),
		},
		{
			name: "missing PromptVerifiers",
			mutate: func(params *srvssh.KeyboardInteractiveCallbackParams) {
				params.PromptVerifiers = nil
			},
			wantErr: trace.BadParameter("params PromptVerifiers must be set and contain at least one PromptVerifier"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			params := keyboardInteractiveCallbackParams

			testCase.mutate(&params)

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

func mockKeyboardInteractiveChallengeRaw(answers []string) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, _ []string, _ []bool) ([]string, error) {
		return answers, nil
	}
}

func mockKeyboardInteractiveChallengeFailure(err error) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, _ []string, _ []bool) ([]string, error) {
		return nil, err
	}
}

type mockPromptVerifier struct {
	Prompt         string
	Echo           bool
	ExpectedAnswer string
}

var _ srvssh.PromptVerifier = (*mockPromptVerifier)(nil)

func (m *mockPromptVerifier) MarshalPrompt() (string, bool, error) {
	return m.Prompt, m.Echo, nil
}

func (m *mockPromptVerifier) VerifyAnswer(ctx context.Context, answer string) error {
	if answer != m.ExpectedAnswer {
		return trace.BadParameter("got %q, want %q", answer, m.ExpectedAnswer)
	}

	return nil
}
