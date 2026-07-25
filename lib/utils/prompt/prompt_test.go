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

package prompt_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/prompt"
)

func TestSelectPromptView(t *testing.T) {
	t.Parallel()

	model := newTestSelectModel()

	require.Equal(t, "Pick a service:\n\n> proxy\n  auth\n  node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View())
}

func TestSelectPromptKeyboardSelection(t *testing.T) {
	t.Parallel()

	model := newTestSelectModel()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n  proxy\n> auth\n  node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View())

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n  proxy\n  auth\n> node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View())

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n  proxy\n  auth\n> node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View(), "cursor should stay on the last option")

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n  proxy\n> auth\n  node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View())

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n> proxy\n  auth\n  node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View())

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)
	require.Equal(t, "Pick a service:\n\n> proxy\n  auth\n  node\n\n(Use arrows or hjkl to navigate, enter to select, q to quit)\n", model.View(), "cursor should stay on the first option")

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(prompt.SelectModel[string])
	require.Nil(t, cmd)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(prompt.SelectModel[string])
	requireQuitCommand(t, cmd)
	require.Equal(t, "Selected: auth\n", model.View())
}

func TestSelectPromptKeyboardCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      tea.KeyMsg
		expected string
	}{
		{
			name:     "q",
			key:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			expected: "Prompt canceled by user.\n",
		},
		{
			name:     "ctrl-c",
			key:      tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: "Prompt interrupted.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := newTestSelectModel()

			updated, cmd := model.Update(tt.key)
			model = updated.(prompt.SelectModel[string])

			requireQuitCommand(t, cmd)
			require.Equal(t, tt.expected, model.View())
		})
	}
}

func TestSelectPromptContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("init has no cancellation command", func(t *testing.T) {
		t.Parallel()

		model := newTestSelectModel()

		require.Nil(t, model.Init())
	})

	t.Run("run returns pre-canceled context error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		got, err := newTestSelectModel().Run(ctx)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, got)
	})
}

func newTestSelectModel() prompt.SelectModel[string] {
	return prompt.NewSelectPrompt("Pick a service", []string{"proxy", "auth", "node"}, func(option string) string {
		return option
	})
}

func requireQuitCommand(t *testing.T, cmd tea.Cmd) {
	t.Helper()

	require.NotNil(t, cmd)
	require.IsType(t, tea.QuitMsg{}, cmd())
}
