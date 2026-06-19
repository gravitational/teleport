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

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// fakeEvaluatorClient is a test double for CommandEvaluatorClient that records
// the last request and returns a canned response or error.
type fakeEvaluatorClient struct {
	resp    *proto.EvaluateCommandResponse
	err     error
	gotReq  *proto.EvaluateCommandRequest
	callCnt int
}

func (f *fakeEvaluatorClient) EvaluateCommand(ctx context.Context, req *proto.EvaluateCommandRequest, opts ...grpc.CallOption) (*proto.EvaluateCommandResponse, error) {
	f.callCnt++
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestAIApprover_Approve(t *testing.T) {
	req := CommandRequest{
		SessionID:   "session-123",
		Command:     "rm -rf /tmp/foo",
		Participant: "alice",
		Login:       "root",
		ServerID:    "node-1",
		Kind:        types.SSHSessionKind,
	}

	t.Run("approves with reasoning", func(t *testing.T) {
		fake := &fakeEvaluatorClient{
			resp: &proto.EvaluateCommandResponse{Approved: true, Reasoning: "looks safe"},
		}
		a := NewAIApprover(fake, "deny dangerous commands", "claude")

		dec := a.Approve(context.Background(), req)

		require.True(t, dec.Approved)
		require.Equal(t, "ai-moderator", dec.Approver)
		require.Equal(t, ModeAI, dec.Mode)
		require.Equal(t, "looks safe", dec.Reason)

		// Verify the request was populated from req + policy + model.
		require.Equal(t, 1, fake.callCnt)
		require.NotNil(t, fake.gotReq)
		require.Equal(t, "session-123", fake.gotReq.SessionID)
		require.Equal(t, "rm -rf /tmp/foo", fake.gotReq.Command)
		require.Equal(t, "deny dangerous commands", fake.gotReq.Policy)
		require.Equal(t, "claude", fake.gotReq.Model)
		require.Equal(t, "alice", fake.gotReq.Participant)
		require.Equal(t, "root", fake.gotReq.Login)
		require.Equal(t, "node-1", fake.gotReq.ServerID)
		require.Equal(t, string(types.SSHSessionKind), fake.gotReq.SessionKind)
	})

	t.Run("denies with reasoning", func(t *testing.T) {
		fake := &fakeEvaluatorClient{
			resp: &proto.EvaluateCommandResponse{Approved: false, Reasoning: "destructive command"},
		}
		a := NewAIApprover(fake, "deny dangerous commands", "claude")

		dec := a.Approve(context.Background(), req)

		require.False(t, dec.Approved)
		require.Equal(t, "ai-moderator", dec.Approver)
		require.Equal(t, ModeAI, dec.Mode)
		require.Equal(t, "destructive command", dec.Reason)
	})

	t.Run("fails closed on client error", func(t *testing.T) {
		fake := &fakeEvaluatorClient{err: errors.New("rpc unavailable")}
		a := NewAIApprover(fake, "policy", "model")

		dec := a.Approve(context.Background(), req)

		require.False(t, dec.Approved, "AI approver must fail closed on RPC error")
		// An RPC/evaluation error is an infrastructure failure, not a deliberate
		// AI denial: attribute it to the system so it is audited as "failed"
		// (fail-closed) rather than "denied by ai-moderator". The AI mode is kept
		// for context.
		require.Equal(t, ApproverSystem, dec.Approver)
		require.Equal(t, ModeAI, dec.Mode)
		require.Contains(t, dec.Reason, "AI evaluation failed")
		require.Contains(t, dec.Reason, "rpc unavailable")
	})
}
