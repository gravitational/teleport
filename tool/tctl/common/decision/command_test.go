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
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/tool/tctl/common/decision"
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

func TestCommands(t *testing.T) {
	var output bytes.Buffer
	cmd := decision.Command{Output: &output}

	cmd.Initialize(kingpin.New("tctl", "test"), nil, nil)

	match, err := cmd.TryRun(context.Background(), "decision evaluate-ssh-access", func(ctx context.Context) (client *authclient.Client, close func(context.Context), err error) {
		return nil, nil, errors.New("fail")
	})
	assert.True(t, match, "evaluate SSH command did not match")
	assert.Error(t, err, "expected failure from init function")

	match, err = cmd.TryRun(context.Background(), "decision evaluate-db-access", func(ctx context.Context) (client *authclient.Client, close func(context.Context), err error) {
		return nil, nil, errors.New("fail")
	})
	assert.True(t, match, "evaluate database command did not match")
	assert.Error(t, err, "expected failure from init function")

	match, err = cmd.TryRun(context.Background(), "decision evaluate-foo", func(ctx context.Context) (client *authclient.Client, close func(context.Context), err error) {
		return nil, nil, errors.New("fail")
	})
	assert.False(t, match, "evaluate foo command matched")
	assert.NoError(t, err, "error received when no command matched")
}
