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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// Testenv is a test environment for decisionv1.Service.
type Testenv struct {
	TestServer *auth.TestServer

	// AuthAdminClient is an admin Auth client.
	AuthAdminClient *authclient.Client

	// DecisionClient is an admin decision client.
	// Created from AuthAdminClient.
	DecisionClient decisionpb.DecisionServiceClient
}

func NewTestenv(t *testing.T) *Testenv {
	t.Helper()

	testServer, err := auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			Dir: t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOTP, // Required.
			},
		},
	})
	require.NoError(t, err, "NewTestServer failed")
	t.Cleanup(func() {
		assert.NoError(t,
			testServer.Shutdown(context.Background()),
			"testServer.Shutdown failed")
	})

	adminClient, err := testServer.NewClient(auth.TestAdmin())
	require.NoError(t, err, "NewClient failed")
	t.Cleanup(func() {
		assert.NoError(t, adminClient.Close(), "adminClient.Close() failed")
	})

	return &Testenv{
		TestServer:      testServer,
		AuthAdminClient: adminClient,
		DecisionClient:  adminClient.DecisionClient(),
	}
}
