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

package recordings

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
)

// TestLoadMoreFetchErrorIsShown verifies that when the "load more" fetch fails,
// the underlying error is surfaced to the user rather than silently swallowed.
func TestLoadMoreFetchErrorIsShown(t *testing.T) {
	const errText = "permission denied: cannot list more sessions"

	sessions := []*sessionsearchv1pb.SessionSummary{
		sessionsearchv1pb.SessionSummary_builder{SessionId: "abc-123"}.Build(),
	}
	fetcher := func(context.Context, string) ([]*sessionsearchv1pb.SessionSummary, string, error) {
		return nil, "", trace.BadParameter("%s", errText)
	}

	m := newModel(context.Background(), sessions, "next-token", nil, BatchFetcher(fetcher))

	// Give the model a size so the detail viewport renders content.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(*model)

	// Move selection down onto the load-more sentinel.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*model)
	_, ok := m.list.SelectedItem().(loadMoreItem)
	require.True(t, ok, "expected load-more item to be selected")

	// Press Enter to trigger the fetch, then run the returned command to obtain
	// the resulting message.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*model)
	require.NotNil(t, cmd, "expected a fetch command")

	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(*model)

	// The error must be surfaced in a popup, not silently swallowed.
	require.IsType(t, &errorPopupModel{}, m.popup,
		"a failed load-more fetch should open an error popup")
	require.Contains(t, m.View(), errText,
		"the load-more fetch error should be displayed to the user")
}
