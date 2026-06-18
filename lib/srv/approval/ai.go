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

	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
)

// AIApproverName is the Approver value on decisions produced by the autonomous
// AI moderator.
const AIApproverName = "ai-moderator"

// CommandEvaluatorClient calls the auth EvaluateCommand RPC. The node's auth
// client (authclient.ClientI, which embeds proto.AuthServiceClient) satisfies
// this interface. It is kept narrow so this package does not import the wider
// auth client surface.
type CommandEvaluatorClient interface {
	EvaluateCommand(ctx context.Context, req *proto.EvaluateCommandRequest, opts ...grpc.CallOption) (*proto.EvaluateCommandResponse, error)
}

// AIApprover is a CommandApprover that delegates each command decision to the
// auth server's EvaluateCommand RPC, which judges the command against a
// natural-language policy using a configured model. It is fail-closed: any RPC
// error yields a denying Decision (the WithTimeout wrapper additionally guards
// against a hung RPC).
type AIApprover struct {
	client CommandEvaluatorClient
	policy string
	model  string
}

// NewAIApprover returns an AIApprover that evaluates commands via client,
// judging them against policy using the named model.
func NewAIApprover(client CommandEvaluatorClient, policy, model string) *AIApprover {
	return &AIApprover{
		client: client,
		policy: policy,
		model:  model,
	}
}

// Approve evaluates req via the EvaluateCommand RPC. On any RPC error it denies
// the command (fail-closed). On success it returns the model's decision and
// reasoning.
func (a *AIApprover) Approve(ctx context.Context, req CommandRequest) Decision {
	resp, err := a.client.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{
		SessionID:   req.SessionID,
		Command:     req.Command,
		Policy:      a.policy,
		Model:       a.model,
		Participant: req.Participant,
		Login:       req.Login,
		ServerID:    req.ServerID,
		SessionKind: string(req.Kind),
	})
	if err != nil {
		return Decision{
			Approved: false,
			Approver: AIApproverName,
			Mode:     ModeAI,
			Reason:   "AI evaluation failed: " + err.Error(),
		}
	}

	return Decision{
		Approved: resp.Approved,
		Approver: AIApproverName,
		Mode:     ModeAI,
		Reason:   resp.Reasoning,
	}
}
