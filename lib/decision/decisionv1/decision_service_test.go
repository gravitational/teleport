// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package decisionv1_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/auth"
)

func TestDecisionServiceRequiresLocalAdmin(t *testing.T) {
	t.Parallel()

	env := NewTestenv(t)
	ctx := context.Background()

	_, _, err := auth.CreateUserAndRoleWithoutRoles(env.AuthAdminClient, "alice", []string{"alice"})
	require.NoError(t, err, "Creating use alice failed")

	aliceClient, err := env.TestServer.NewClient(auth.TestUser("alice"))
	require.NoError(t, err, "NewClient failed")
	t.Cleanup(func() {
		assert.NoError(t, aliceClient.Close(), "aliceClient.Close() failed")
	})

	tests := []struct {
		name          string
		client        decisionpb.DecisionServiceClient
		expectedError any
	}{
		{
			name:          "admin",
			client:        env.DecisionClient,
			expectedError: &trace.NotImplementedError{},
		},
		{
			name:          "alice",
			client:        aliceClient.DecisionClient(),
			expectedError: &trace.AccessDeniedError{},
		},
	}

	t.Run("EvaluateSSHAccess", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := test.client.EvaluateSSHAccess(ctx, &decisionpb.EvaluateSSHAccessRequest{})
				assert.ErrorAs(t, err, &test.expectedError, "EvaluateSSHAccess error mismatch")
			})
		}
	})

	t.Run("EvaluateSSHJoin", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := test.client.EvaluateSSHJoin(ctx, &decisionpb.EvaluateSSHJoinRequest{})
				assert.ErrorAs(t, err, &test.expectedError, "EvaluateSSHJoin error mismatch")
			})
		}
	})

	t.Run("EvaluateDatabaseAccess", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := test.client.EvaluateDatabaseAccess(ctx, &decisionpb.EvaluateDatabaseAccessRequest{})
				assert.ErrorAs(t, err, &test.expectedError, "EvaluateDatabaseAccess error mismatch")
			})
		}
	})
}
