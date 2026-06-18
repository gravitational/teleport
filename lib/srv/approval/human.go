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
	"sync"

	"github.com/gravitational/teleport/api/types"
)

// Broadcaster delivers an approval request to all connected moderator clients.
type Broadcaster interface {
	BroadcastApprovalRequest(req CommandApprovalRequest) error
}

// HumanApprover is a CommandApprover that broadcasts each command to the
// session's moderator clients and blocks until the first valid moderator
// response. It is fail-closed: a broadcast failure or context cancellation
// yields a denying Decision.
//
// Responses are accepted only from participants in moderator mode; this guard
// is enforced server-side and never trusts a client's claimed mode.
type HumanApprover struct {
	broadcaster Broadcaster
	ids         approvalIDGen

	mu      sync.Mutex
	pending map[string]chan Decision
}

// NewHumanApprover returns a HumanApprover that broadcasts approval requests via
// b. Each HumanApprover owns its own ID generator, so IDs are unique only within
// this instance; callers use one HumanApprover per session.
func NewHumanApprover(b Broadcaster) *HumanApprover {
	return &HumanApprover{
		broadcaster: b,
		pending:     make(map[string]chan Decision),
	}
}

// Approve broadcasts req to moderator clients and blocks until the first valid
// moderator response, ctx cancellation, or a broadcast failure. It always
// removes the pending entry before returning.
func (h *HumanApprover) Approve(ctx context.Context, req CommandRequest) Decision {
	id := h.ids.next(req.SessionID)

	bReq := CommandApprovalRequest{
		ID:          id,
		SessionID:   req.SessionID,
		Command:     req.Command,
		Participant: req.Participant,
		Login:       req.Login,
		ServerID:    req.ServerID,
	}
	// If the caller imposed a deadline, surface it to moderator clients so they
	// know how long they have to respond. Otherwise leave it zero.
	if deadline, ok := ctx.Deadline(); ok {
		bReq.ExpiresAt = deadline
	}

	// Register the pending decision channel before broadcasting so a fast
	// response cannot race ahead of registration.
	ch := make(chan Decision, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
	}()

	if err := h.broadcaster.BroadcastApprovalRequest(bReq); err != nil {
		return Decision{
			Approved: false,
			Approver: ApproverSystem,
			Reason:   "failed to broadcast approval request (fail-closed): " + err.Error(),
		}
	}

	select {
	case dec := <-ch:
		return dec
	case <-ctx.Done():
		return Decision{
			Approved: false,
			Approver: ApproverSystem,
			Reason:   "approval timed out or was cancelled (fail-closed): " + ctx.Err().Error(),
		}
	}
}

// Submit delivers a moderator's response to the pending Approve call identified
// by id. It is the entry point the session calls when a CommandApprovalResponse
// arrives.
//
// Responses are accepted only from participants in moderator mode; any other
// mode is rejected server-side. Unknown or already-resolved IDs are ignored.
// The first valid response wins; later responses for the same ID are dropped.
func (h *HumanApprover) Submit(id string, approved bool, reason, user string, mode types.SessionParticipantMode) {
	// Defense in depth: never trust a client's claimed mode. Only moderators
	// may decide commands.
	if mode != types.SessionModeratorMode {
		return
	}

	h.mu.Lock()
	ch, ok := h.pending[id]
	h.mu.Unlock()
	if !ok {
		// Unknown or already-resolved/expired ID.
		return
	}

	dec := Decision{
		Approved: approved,
		Approver: user,
		Reason:   reason,
		Mode:     ModeHuman,
	}

	// The channel is buffered with capacity 1; a non-blocking send ensures the
	// first response wins and any later response is dropped without blocking the
	// caller.
	select {
	case ch <- dec:
	default:
	}
}
