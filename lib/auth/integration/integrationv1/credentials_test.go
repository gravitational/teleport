/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integrationv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

func TestExportIntegrationCertAuthorities(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ca := newCertAuthority(t, types.HostCA, "test-cluster")
	ctx, localClient, resourceSvc := initSvc(t, ca, ca.GetClusterName(), "127.0.0.1")

	githubIntegration, err := newGitHubIntegration("github-my-org", "id", "secret")
	require.NoError(t, err)

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "aws-oidc"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)

	adminCtx := authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	})

	_, err = resourceSvc.CreateIntegration(adminCtx, &integrationpb.CreateIntegrationRequest{Integration: githubIntegration})
	require.NoError(t, err)
	_, err = resourceSvc.CreateIntegration(adminCtx, &integrationpb.CreateIntegrationRequest{Integration: oidcIntegration})
	require.NoError(t, err)

	tests := []struct {
		name        string
		integration string
		identity    context.Context
		check       func(*testing.T, *integrationpb.ExportIntegrationCertAuthoritiesResponse, error)
	}{
		{
			name:        "success",
			integration: githubIntegration.GetName(),
			identity:    adminCtx,
			check: func(t *testing.T, resp *integrationpb.ExportIntegrationCertAuthoritiesResponse, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.NotNil(t, resp.CertAuthorities)
				require.Len(t, resp.CertAuthorities.SSH, 1)
				require.NotNil(t, resp.CertAuthorities.SSH[0])
				assert.NotEmpty(t, resp.CertAuthorities.SSH[0].PublicKey)
				assert.Empty(t, resp.CertAuthorities.SSH[0].PrivateKey)
			},
		},
		{
			name:        "not found",
			integration: "not-found",
			identity:    adminCtx,
			check: func(t *testing.T, resp *integrationpb.ExportIntegrationCertAuthoritiesResponse, err error) {
				t.Helper()
				require.Nil(t, resp)
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:        "not allowed",
			integration: githubIntegration.GetName(),
			identity:    authorizerForDummyUser(t, ctx, types.RoleSpecV6{}, localClient),
			check: func(t *testing.T, resp *integrationpb.ExportIntegrationCertAuthoritiesResponse, err error) {
				t.Helper()
				require.Nil(t, resp)
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:        "not supported",
			integration: oidcIntegration.GetName(),
			identity:    adminCtx,
			check: func(t *testing.T, resp *integrationpb.ExportIntegrationCertAuthoritiesResponse, err error) {
				t.Helper()
				require.Nil(t, resp)
				require.Error(t, err)
				require.True(t, trace.IsNotImplemented(err))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := resourceSvc.ExportIntegrationCertAuthorities(test.identity, &integrationpb.ExportIntegrationCertAuthoritiesRequest{
				Integration: test.integration,
			})
			test.check(t, resp, err)
		})
	}
}
