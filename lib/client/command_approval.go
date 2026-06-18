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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/srv/approval"
)

// renderApprovalPrompt formats a moderator-facing approval prompt for a pending
// command. It uses \r\n line endings because the moderator terminal is in raw
// mode while attached to a session.
//
// TODO(approval): Web UI renderer — the Web UI moderator experience is a
// fast-follow and will render this prompt in the browser instead of the
// terminal.
func renderApprovalPrompt(req approval.CommandApprovalRequest) string {
	var b strings.Builder
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "Command approval requested by %q (login %q):\r\n", req.Participant, req.Login)
	fmt.Fprintf(&b, "  %s\r\n", req.Command)
	if !req.ExpiresAt.IsZero() {
		fmt.Fprintf(&b, "(expires at %s)\r\n", req.ExpiresAt.Format(time.Kitchen))
	}
	b.WriteString("[a] approve   [d] deny\r\n")
	return b.String()
}

// pendingApproval is a concurrency-safe holder for the single command approval
// request that is currently awaiting a moderator decision. The request arrives
// on the global-request goroutine (handleGlobalRequests) while the moderator's
// keypress is read on a separate goroutine (handleNonPeerControls), so access
// must be synchronized. Only one command is pending per session at a time.
type pendingApproval struct {
	mu  sync.Mutex
	req *approval.CommandApprovalRequest // nil when none pending
}

// set stores req as the current pending approval, replacing any existing one.
func (p *pendingApproval) set(req approval.CommandApprovalRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.req = &req
}

// take returns the current pending approval and clears it. The boolean is false
// when nothing is pending.
func (p *pendingApproval) take() (approval.CommandApprovalRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.req == nil {
		return approval.CommandApprovalRequest{}, false
	}
	req := *p.req
	p.req = nil
	return req, true
}

// clear removes any current pending approval.
func (p *pendingApproval) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.req = nil
}
