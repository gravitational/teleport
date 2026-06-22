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

	"github.com/stretchr/testify/require"
)

func TestEscalatingAIApprover(t *testing.T) {
	t.Parallel()

	const sessionID = "session-1"
	req := CommandRequest{SessionID: sessionID, Command: "rm -rf /"}

	aiApprove := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		return Decision{Approved: true, Approver: AIApproverName, Mode: ModeAI}
	})
	aiDeny := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		return Decision{Approved: false, Approver: AIApproverName, Reason: "destructive command", Mode: ModeAI}
	})
	aiFail := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		return Decision{Approved: false, Approver: ApproverSystem, Reason: "AI evaluation failed (fail-closed): boom"}
	})

	t.Run("AI approves: human not consulted, no notify", func(t *testing.T) {
		t.Parallel()
		humanCalled := false
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			humanCalled = true
			return Decision{Approved: false}
		})
		var notified []string
		e := NewEscalatingAIApprover(aiApprove, human, func() bool { return true }, func(s string) { notified = append(notified, s) })

		dec := e.Approve(context.Background(), req)
		require.True(t, dec.Approved)
		require.Equal(t, AIApproverName, dec.Approver)
		require.Equal(t, ModeAI, dec.Mode)
		require.False(t, humanCalled, "AI approvals must never be escalated")
		require.Empty(t, notified, "notify must not be called on AI approval")
	})

	t.Run("AI denies, no moderator: AI denial stands", func(t *testing.T) {
		t.Parallel()
		humanCalled := false
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			humanCalled = true
			return Decision{Approved: true}
		})
		var notified []string
		e := NewEscalatingAIApprover(aiDeny, human, func() bool { return false }, func(s string) { notified = append(notified, s) })

		dec := e.Approve(context.Background(), req)
		require.False(t, dec.Approved)
		require.Equal(t, AIApproverName, dec.Approver)
		require.Equal(t, ModeAI, dec.Mode)
		require.Equal(t, "destructive command", dec.Reason)
		require.False(t, humanCalled, "no moderator: must not escalate")
		require.Empty(t, notified)
	})

	t.Run("nil hasModerator: AI denial stands", func(t *testing.T) {
		t.Parallel()
		humanCalled := false
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			humanCalled = true
			return Decision{Approved: true}
		})
		e := NewEscalatingAIApprover(aiDeny, human, nil, nil)
		dec := e.Approve(context.Background(), req)
		require.False(t, dec.Approved)
		require.Equal(t, AIApproverName, dec.Approver)
		require.False(t, humanCalled)
	})

	t.Run("AI denies, moderator present, human approves", func(t *testing.T) {
		t.Parallel()
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			return Decision{Approved: true, Approver: "moderator-1", Reason: "ok", Mode: ModeHuman}
		})
		var notified []string
		e := NewEscalatingAIApprover(aiDeny, human, func() bool { return true }, func(s string) { notified = append(notified, s) })

		dec := e.Approve(context.Background(), req)
		require.True(t, dec.Approved)
		require.Equal(t, "moderator-1", dec.Approver)
		require.Equal(t, ModeHuman, dec.Mode)
		require.Equal(t, []string{"destructive command"}, notified, "notify called once with the AI reason")
	})

	t.Run("AI denies, moderator present, human denies", func(t *testing.T) {
		t.Parallel()
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			return Decision{Approved: false, Approver: "moderator-1", Reason: "still no", Mode: ModeHuman}
		})
		var notified []string
		e := NewEscalatingAIApprover(aiDeny, human, func() bool { return true }, func(s string) { notified = append(notified, s) })

		dec := e.Approve(context.Background(), req)
		require.False(t, dec.Approved)
		require.Equal(t, "moderator-1", dec.Approver)
		require.Equal(t, ModeHuman, dec.Mode)
		require.Equal(t, "still no", dec.Reason)
		require.Len(t, notified, 1)
	})

	t.Run("AI fails (system), moderator present: escalates", func(t *testing.T) {
		t.Parallel()
		humanCalled := false
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			humanCalled = true
			return Decision{Approved: true, Approver: "moderator-1", Mode: ModeHuman}
		})
		var notified []string
		e := NewEscalatingAIApprover(aiFail, human, func() bool { return true }, func(s string) { notified = append(notified, s) })

		dec := e.Approve(context.Background(), req)
		require.True(t, dec.Approved, "an AI failure with a moderator present must escalate to the human")
		require.True(t, humanCalled)
		require.Equal(t, "moderator-1", dec.Approver)
		require.Len(t, notified, 1)
	})

	t.Run("AI fails (system), no moderator: failure stands", func(t *testing.T) {
		t.Parallel()
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			t.Fatal("human must not be consulted with no moderator present")
			return Decision{}
		})
		e := NewEscalatingAIApprover(aiFail, human, func() bool { return false }, nil)
		dec := e.Approve(context.Background(), req)
		require.False(t, dec.Approved)
		require.Equal(t, ApproverSystem, dec.Approver)
	})

	t.Run("nil notify is safe", func(t *testing.T) {
		t.Parallel()
		human := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
			return Decision{Approved: true, Approver: "moderator-1", Mode: ModeHuman}
		})
		e := NewEscalatingAIApprover(aiDeny, human, func() bool { return true }, nil)
		dec := e.Approve(context.Background(), req)
		require.True(t, dec.Approved)
	})
}
