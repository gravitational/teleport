// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package decision_test

import (
	"context"

	"google.golang.org/grpc"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

type fakeDecisionServiceClient struct {
	decisionpb.DecisionServiceClient

	sshResponse      *decisionpb.EvaluateSSHAccessResponse
	databaseResponse *decisionpb.EvaluateDatabaseAccessResponse
}

// EvaluateSSHAccess evaluates an SSH access attempt.
func (f fakeDecisionServiceClient) EvaluateSSHAccess(ctx context.Context, in *decisionpb.EvaluateSSHAccessRequest, opts ...grpc.CallOption) (*decisionpb.EvaluateSSHAccessResponse, error) {
	return f.sshResponse, nil
}

// EvaluateDatabaseAccess evaluate a database access attempt.
func (f fakeDecisionServiceClient) EvaluateDatabaseAccess(ctx context.Context, in *decisionpb.EvaluateDatabaseAccessRequest, opts ...grpc.CallOption) (*decisionpb.EvaluateDatabaseAccessResponse, error) {
	return f.databaseResponse, nil
}
