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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// fakeBroadcaster captures the broadcast request and signals when it arrives so
// tests can grab the generated correlation ID without polling on wall-clock
// time.
type fakeBroadcaster struct {
	err error

	gotCh chan CommandApprovalRequest
}

func newFakeBroadcaster(err error) *fakeBroadcaster {
	return &fakeBroadcaster{
		err:   err,
		gotCh: make(chan CommandApprovalRequest, 1),
	}
}

func (f *fakeBroadcaster) BroadcastApprovalRequest(req CommandApprovalRequest) error {
	if f.err != nil {
		return f.err
	}
	f.gotCh <- req
	return nil
}

// waitRequest blocks until a request has been broadcast and returns it.
func (f *fakeBroadcaster) waitRequest(t *testing.T) CommandApprovalRequest {
	t.Helper()
	select {
	case req := <-f.gotCh:
		return req
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for broadcast")
		return CommandApprovalRequest{}
	}
}

func testRequest() CommandRequest {
	return CommandRequest{
		SessionID:   "session-1",
		Command:     "rm -rf /",
		Participant: "alice",
		Login:       "root",
		ServerID:    "server-1",
		Kind:        types.SSHSessionKind,
	}
}

func TestHumanApproverFirstResponseWins(t *testing.T) {
	b := newFakeBroadcaster(nil)
	a := newHumanApprover(b)

	go func() {
		req := b.waitRequest(t)
		a.submit(req.ID, true, "ok", "bob", types.SessionModeratorMode)
	}()

	dec := a.Approve(context.Background(), testRequest())
	require.True(t, dec.Approved)
	require.Equal(t, "bob", dec.Approver)
	require.Equal(t, "ok", dec.Reason)
	require.Equal(t, ModeHuman, dec.Mode)
}

func TestHumanApproverRejectsNonModeratorResponse(t *testing.T) {
	b := newFakeBroadcaster(nil)
	a := newHumanApprover(b)

	go func() {
		req := b.waitRequest(t)
		// A peer's approval must be ignored server-side.
		a.submit(req.ID, true, "peer-approve", "mallory", types.SessionPeerMode)
		// The moderator's deny is the first valid response and must win.
		a.submit(req.ID, false, "denied", "bob", types.SessionModeratorMode)
	}()

	dec := a.Approve(context.Background(), testRequest())
	require.False(t, dec.Approved)
	require.Equal(t, "bob", dec.Approver)
	require.Equal(t, "denied", dec.Reason)
	require.Equal(t, ModeHuman, dec.Mode)
}

func TestHumanApproverFailsClosedOnBroadcastError(t *testing.T) {
	b := newFakeBroadcaster(errors.New("network down"))
	a := newHumanApprover(b)

	dec := a.Approve(context.Background(), testRequest())
	require.False(t, dec.Approved)
	require.Equal(t, ApproverSystem, dec.Approver)
	require.NotEmpty(t, dec.Reason)
}

func TestHumanApproverUnknownIDIgnored(t *testing.T) {
	b := newFakeBroadcaster(nil)
	a := newHumanApprover(b)

	go func() {
		req := b.waitRequest(t)
		// A bogus ID must not resolve any pending request.
		a.submit("bogus-id", true, "nope", "bob", types.SessionModeratorMode)
		// The real ID resolves it.
		a.submit(req.ID, true, "ok", "bob", types.SessionModeratorMode)
	}()

	dec := a.Approve(context.Background(), testRequest())
	require.True(t, dec.Approved)
	require.Equal(t, "bob", dec.Approver)
}

func TestHumanApproverContextCancelDenies(t *testing.T) {
	b := newFakeBroadcaster(nil)
	a := newHumanApprover(b)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		b.waitRequest(t)
		cancel()
	}()

	dec := a.Approve(ctx, testRequest())
	require.False(t, dec.Approved)
	require.Equal(t, ApproverSystem, dec.Approver)
	require.Contains(t, dec.Reason, "fail-closed")
}

func TestHumanApproverSetsExpiresFromDeadline(t *testing.T) {
	b := newFakeBroadcaster(nil)
	a := newHumanApprover(b)

	deadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	go func() {
		req := b.waitRequest(t)
		require.WithinDuration(t, deadline, req.ExpiresAt, time.Second)
		a.submit(req.ID, true, "ok", "bob", types.SessionModeratorMode)
	}()

	dec := a.Approve(ctx, testRequest())
	require.True(t, dec.Approved)
}
