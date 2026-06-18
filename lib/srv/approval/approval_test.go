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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestDecisionDenied(t *testing.T) {
	d := Decision{Approved: false, Approver: "ai-moderator", Reason: "blocked", Mode: ModeAI}
	require.False(t, d.Approved)
	require.Equal(t, "ai-moderator", d.Approver)
	require.Equal(t, ModeAI, d.Mode)
}

func TestCommandRequestKind(t *testing.T) {
	r := CommandRequest{Command: "ls", Kind: types.SSHSessionKind}
	require.Equal(t, types.SSHSessionKind, r.Kind)
}

func TestWithTimeoutDeniesOnSlowApprover(t *testing.T) {
	slow := approverFunc(func(ctx context.Context, _ CommandRequest) Decision {
		<-ctx.Done()                    // never decides on its own
		return Decision{Approved: true} // would-be approve, must be overridden
	})
	a := WithTimeout(slow, 20*time.Millisecond)
	d := a.Approve(context.Background(), CommandRequest{Command: "rm -rf /"})
	require.False(t, d.Approved, "timeout must deny")
	require.Contains(t, d.Reason, "fail-closed")
}

func TestWithTimeoutDeniesOnParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // parent already cancelled
	blocked := approverFunc(func(ctx context.Context, _ CommandRequest) Decision {
		<-ctx.Done()
		return Decision{Approved: true}
	})
	a := WithTimeout(blocked, time.Minute)
	d := a.Approve(ctx, CommandRequest{Command: "ls"})
	require.False(t, d.Approved)
	require.Contains(t, d.Reason, "fail-closed")
	require.Equal(t, ApproverSystem, d.Approver)
}

func TestWithTimeoutDeniesOnPanic(t *testing.T) {
	panicker := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		var nilClient *struct{ x int }
		_ = nilClient.x // typed-nil deref: panics
		return Decision{Approved: true}
	})
	a := WithTimeout(panicker, time.Second)
	d := a.Approve(context.Background(), CommandRequest{Command: "rm -rf /"})
	require.False(t, d.Approved, "a panicking approver must fail closed (deny)")
	require.Contains(t, d.Reason, "fail-closed")
	require.Equal(t, ApproverSystem, d.Approver)
}

func TestWithTimeoutPassesThroughFastDecision(t *testing.T) {
	fast := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		return Decision{Approved: true, Approver: "bob", Mode: ModeHuman}
	})
	a := WithTimeout(fast, time.Second)
	d := a.Approve(context.Background(), CommandRequest{Command: "ls"})
	require.True(t, d.Approved)
	require.Equal(t, "bob", d.Approver)
}
