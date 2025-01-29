/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/web"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	helpers.TestMainImplementation(m)
}

func TestMFAAuthenticateChallenge_IsMFARequired(t *testing.T) {
	ctx := context.Background()

	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	rootServer := testserver.MakeTestServer(t,
		testserver.WithBootstrap(alice),
		testserver.WithClusterName(t, "root"),
	)
	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)
	rootAuth := rootServer.GetAuthServer()

	leafServer := testserver.MakeTestServer(t,
		testserver.WithClusterName(t, "leaf"),
	)
	leafAuth := leafServer.GetAuthServer()

	// Require Session MFA in the leaf only.
	leafAccess, err := leafAuth.GetRole(ctx, "access")
	require.NoError(t, err)
	o := leafAccess.GetOptions()
	o.RequireMFAType = types.RequireMFAType_SESSION
	leafAccess.SetOptions(o)
	_, err = leafAuth.UpsertRole(ctx, leafAccess)
	require.NoError(t, err)

	testserver.SetupTrustedCluster(ctx, t, rootServer, leafServer)

	password := uuid.NewString()
	require.NoError(t, rootAuth.UpsertPassword("alice", []byte(password)))

	webPack := helpers.LoginWebClient(t, proxyAddr.String(), "alice", password)
	for _, tt := range []struct {
		name string
		req  web.CreateAuthenticateChallengeRequest
	}{
		{
			name: "",
			req: web.CreateAuthenticateChallengeRequest{
				ChallengeScope: int(mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION),
				IsMFARequiredRequest: &web.IsMFARequiredRequest{
					App: &web.IsMFARequiredApp{
						ResolveAppParams: web.ResolveAppParams{
							AppName:     "",
							ClusterName: "",
						},
					},
				},
			},
		}, {
			name: "",
			req: web.CreateAuthenticateChallengeRequest{
				ChallengeScope: int(mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION),
				IsMFARequiredRequest: &web.IsMFARequiredRequest{
					ClusterID: "",
					App: &web.IsMFARequiredApp{
						ResolveAppParams: web.ResolveAppParams{
							AppName: "",
						},
					},
				},
			},
		}, {
			name: "",
			req: web.CreateAuthenticateChallengeRequest{
				ChallengeScope: int(mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION),
				IsMFARequiredRequest: &web.IsMFARequiredRequest{
					App: &web.IsMFARequiredApp{
						ResolveAppParams: web.ResolveAppParams{
							FQDNHint: "",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			webPack.DoRequest(t, http.MethodPost, "/webapi/mfa/authenticationchallenge", tt.req)
		})
	}
}
