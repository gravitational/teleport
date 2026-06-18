// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package approval

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCommandApprovalRequestJSONRoundTrip(t *testing.T) {
	req := CommandApprovalRequest{
		ID:          "sess-1",
		SessionID:   "sess",
		Command:     "rm -rf /",
		Participant: "alice",
		Login:       "root",
		ServerID:    "server-123",
		ExpiresAt:   time.Unix(1700000000, 0).UTC(),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var got CommandApprovalRequest
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, req, got)
}

func TestCommandApprovalResponseJSONRoundTrip(t *testing.T) {
	resp := CommandApprovalResponse{
		ID:       "sess-1",
		Approved: true,
		Reason:   "looks fine",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var got CommandApprovalResponse
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, resp, got)

	// When Reason is empty, the marshaled JSON must omit it.
	emptyReason := CommandApprovalResponse{ID: "sess-2", Approved: false}
	data, err = json.Marshal(emptyReason)
	require.NoError(t, err)
	require.NotContains(t, string(data), "reason")

	var gotEmpty CommandApprovalResponse
	require.NoError(t, json.Unmarshal(data, &gotEmpty))
	require.Equal(t, emptyReason, gotEmpty)
}

func TestApprovalIDGenUniqueAndMonotonic(t *testing.T) {
	var gen approvalIDGen

	ids := []string{
		gen.next("sess"),
		gen.next("sess"),
		gen.next("sess"),
	}

	require.Equal(t, []string{"sess-1", "sess-2", "sess-3"}, ids)

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		require.NotContains(t, seen, id)
		seen[id] = struct{}{}
	}

	// sanity: the prefix is preserved.
	require.True(t, strings.HasPrefix(ids[0], "sess-"))
}
