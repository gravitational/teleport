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

package client

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/approval"
)

func TestRenderApprovalPrompt(t *testing.T) {
	req := approval.CommandApprovalRequest{
		ID:          "session-1",
		SessionID:   "session",
		Command:     "rm -rf /important/data",
		Participant: "alice",
		Login:       "root",
		ServerID:    "node-1",
	}

	out := renderApprovalPrompt(req)

	require.Contains(t, out, req.Command, "prompt should contain the command")
	require.Contains(t, out, req.Participant, "prompt should contain the participant")
	require.Contains(t, out, req.Login, "prompt should contain the target login")
	require.Contains(t, out, "[a]", "prompt should contain the approve hint")
	require.Contains(t, out, "[d]", "prompt should contain the deny hint")
	require.Contains(t, out, "approve", "prompt should describe the approve action")
	require.Contains(t, out, "deny", "prompt should describe the deny action")

	// With a zero ExpiresAt, the prompt omits the expiry hint.
	require.NotContains(t, out, "expires at", "prompt should omit expiry hint when ExpiresAt is zero")

	// Raw terminal mode requires carriage returns; ensure no bare newlines.
	for _, line := range strings.Split(out, "\r\n") {
		require.NotContains(t, line, "\n", "lines must use \\r\\n endings")
	}

	// With a non-zero ExpiresAt, the prompt includes the formatted deadline.
	expiry := time.Date(2026, time.June, 18, 15, 4, 0, 0, time.UTC)
	req.ExpiresAt = expiry
	out = renderApprovalPrompt(req)
	require.Contains(t, out, "expires at", "prompt should include expiry hint when ExpiresAt is set")
	require.Contains(t, out, expiry.Format(time.Kitchen), "prompt should include the formatted expiry time")

	for _, line := range strings.Split(out, "\r\n") {
		require.NotContains(t, line, "\n", "lines must use \\r\\n endings")
	}
}

func TestPendingApproval(t *testing.T) {
	var p pendingApproval

	// take on an empty holder reports nothing pending.
	_, ok := p.take()
	require.False(t, ok, "empty holder should report no pending approval")

	req := approval.CommandApprovalRequest{ID: "session-1", Command: "ls"}
	p.set(req)

	// take returns the stored request and clears it.
	got, ok := p.take()
	require.True(t, ok, "holder should report a pending approval after set")
	require.Equal(t, req, got)

	// a second take is now empty (take cleared it).
	_, ok = p.take()
	require.False(t, ok, "take should clear the pending approval")

	// clear removes a pending request.
	p.set(req)
	p.clear()
	_, ok = p.take()
	require.False(t, ok, "clear should remove the pending approval")
}
