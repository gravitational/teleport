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
	"golang.org/x/crypto/ssh"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
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

// TestCommandApprovalResponseSSHRoundTrip verifies the response survives the
// wire encoding actually used (ssh.Marshal, see
// TestCommandApprovalResponseSurvivesContextFromRequest for why JSON is unsafe).
func TestCommandApprovalResponseSSHRoundTrip(t *testing.T) {
	resp := CommandApprovalResponse{
		ID:       "sess-1",
		Approved: true,
		Reason:   "looks fine",
	}

	data := ssh.Marshal(resp)

	var got CommandApprovalResponse
	require.NoError(t, ssh.Unmarshal(data, &got))
	require.Equal(t, resp, got)

	// An empty Reason round-trips as an empty string.
	emptyReason := CommandApprovalResponse{ID: "sess-2", Approved: false}
	data = ssh.Marshal(emptyReason)

	var gotEmpty CommandApprovalResponse
	require.NoError(t, ssh.Unmarshal(data, &gotEmpty))
	require.Equal(t, emptyReason, gotEmpty)
}

// TestCommandApprovalResponseSurvivesContextFromRequest is a regression guard
// for the bug where an approved command tore down the moderator's session with
// "unexpected end of JSON input".
//
// The approval response is sent as a session channel-request payload. On the
// server, every session request passes through tracessh.ContextFromRequest,
// which leniently json.Unmarshals the payload as a trace Envelope and, on
// success, OVERWRITES req.Payload with the (empty) envelope payload. A raw JSON
// response payload like {"id":...,"approved":true} parses as a valid (empty)
// Envelope, so the real payload is wiped before the handler ever sees it.
//
// Encoding the response with ssh.Marshal (binary SSH wire format) is not valid
// JSON, so ContextFromRequest's json.Unmarshal fails and leaves req.Payload
// intact — exactly the pattern the file-transfer decision uses.
func TestCommandApprovalResponseSurvivesContextFromRequest(t *testing.T) {
	resp := CommandApprovalResponse{ID: "sess-1", Approved: true, Reason: "ok"}

	// BUG: a JSON-encoded response is wiped by ContextFromRequest.
	jsonPayload, err := json.Marshal(resp)
	require.NoError(t, err)
	require.NotEmpty(t, jsonPayload)

	req := &ssh.Request{Payload: jsonPayload}
	tracessh.ContextFromRequest(req)
	require.Empty(t, req.Payload,
		"raw JSON payload should be wiped by ContextFromRequest (documents the bug)")
	// A subsequent json.Unmarshal of the now-empty payload is what produced
	// "unexpected end of JSON input".
	var wiped CommandApprovalResponse
	require.Error(t, json.Unmarshal(req.Payload, &wiped))

	// FIX: an ssh.Marshal-encoded response survives ContextFromRequest.
	sshPayload := ssh.Marshal(resp)
	require.NotEmpty(t, sshPayload)

	req2 := &ssh.Request{Payload: sshPayload}
	tracessh.ContextFromRequest(req2)
	require.Equal(t, sshPayload, req2.Payload,
		"ssh.Marshal payload must survive ContextFromRequest unchanged")

	var got CommandApprovalResponse
	require.NoError(t, ssh.Unmarshal(req2.Payload, &got))
	require.Equal(t, resp, got)
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
