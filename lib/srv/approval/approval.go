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

// Package approval gates individual commands in moderated sessions on an
// approval decision from a human moderator or an autonomous AI moderator.
package approval

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// Mode identifies which kind of approver produced a decision.
type Mode string

const (
	ModeHuman Mode = "human"
	ModeAI    Mode = "ai"
)

// CommandRequest is a single command awaiting an approval decision.
type CommandRequest struct {
	SessionID   string
	Command     string
	Participant string // who typed it
	Login       string // target OS user
	ServerID    string
	Kind        types.SessionKind
}

// Decision is the outcome of evaluating a CommandRequest.
type Decision struct {
	Approved bool
	Approver string // moderator username or "ai-moderator"
	Reason   string
	Mode     Mode
}

// CommandApprover decides whether a command may run. Implementations MUST be
// fail-closed: any error or timeout yields a denying Decision.
type CommandApprover interface {
	Approve(ctx context.Context, req CommandRequest) Decision
}
