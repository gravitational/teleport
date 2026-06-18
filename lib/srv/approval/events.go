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
	"fmt"
	"sync"
	"time"
)

// CommandApprovalRequest is broadcast from the session host to moderator
// clients to request a decision on a single command. The ID correlates the
// request with the moderator's CommandApprovalResponse.
type CommandApprovalRequest struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Command     string    `json:"command"`
	Participant string    `json:"participant"`
	Login       string    `json:"login"`
	ServerID    string    `json:"server_id"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CommandApprovalResponse is sent by a moderator client back to the session
// host in reply to a CommandApprovalRequest with the matching ID.
type CommandApprovalResponse struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// approvalIDGen generates unique correlation IDs for approval requests within a
// session. It is safe for concurrent use.
type approvalIDGen struct {
	mu      sync.Mutex
	counter uint64
}

// next returns a monotonically increasing ID prefixed with sessionID.
// Uniqueness is guaranteed only per generator instance: callers use one
// generator per session namespace, so IDs do not collide across the requests a
// single session generates.
func (g *approvalIDGen) next(sessionID string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("%s-%d", sessionID, g.counter)
}
