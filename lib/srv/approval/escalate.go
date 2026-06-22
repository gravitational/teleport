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

import "context"

// EscalatingAIApprover runs an AI approver first; if it does not approve and a
// human moderator is connected, it escalates the command to a human approver.
// If no moderator is connected, the AI decision stands. AI approvals are never
// escalated.
//
// Escalation only ADDS a human approve path when a moderator is present: it
// never turns an AI deny or failure into an approval without a human's explicit
// approval. With no moderator present it is fail-closed (the AI decision, deny
// or system failure, stands).
type EscalatingAIApprover struct {
	ai           CommandApprover
	human        CommandApprover
	hasModerator func() bool
	// notify, if set, is called with a short status message when escalation to a
	// human begins (so participants see why the command is still pending).
	notify func(string)
}

// NewEscalatingAIApprover returns an EscalatingAIApprover. hasModerator reports
// whether a moderator is currently connected (it may be nil, treated as "no
// moderator"); notify, if non-nil, is invoked with the AI reason when an
// escalation to a human begins.
func NewEscalatingAIApprover(ai, human CommandApprover, hasModerator func() bool, notify func(string)) *EscalatingAIApprover {
	return &EscalatingAIApprover{
		ai:           ai,
		human:        human,
		hasModerator: hasModerator,
		notify:       notify,
	}
}

// Approve evaluates req with the AI approver. An AI approval is returned as-is.
// Otherwise, if a moderator is connected, the command is escalated to the human
// approver (whose decision is returned); if not, the AI decision stands.
func (e *EscalatingAIApprover) Approve(ctx context.Context, req CommandRequest) Decision {
	aiDec := e.ai.Approve(ctx, req)
	if aiDec.Approved {
		return aiDec
	}
	if e.hasModerator == nil || !e.hasModerator() {
		// No moderator to escalate to; the AI decision (deny or system failure)
		// stands. Fail-closed.
		return aiDec
	}
	if e.notify != nil {
		// The caller formats the participant-facing message from this reason.
		e.notify(aiDec.Reason)
	}
	return e.human.Approve(ctx, req)
}
