/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"bytes"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestIntegrationCollection_WriteText(t *testing.T) {
	ig1, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "aws-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)

	ig2, err := types.NewIntegrationGitHub(
		types.Metadata{Name: "github-my-org"},
		&types.GitHubIntegrationSpecV1{
			Organization: "my-org",
		},
	)
	require.NoError(t, err)

	collection := &integrationCollection{
		integrations: []types.Integration{ig1, ig2},
	}

	var buf bytes.Buffer
	require.NoError(t, collection.WriteText(&buf, false))

	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

func TestGitHubIntegrationHandler(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
	})

	process, err := testenv.NewTeleportProcess(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

	handler := Handlers()[types.KindIntegration]
	require.NotNil(t, handler)

	// For accessing backend directly.
	credsSvc, err := local.NewPluginStaticCredentialsService(process.GetBackend())
	require.NoError(t, err)
	igSvc, err := local.NewIntegrationsService(process.GetBackend())
	require.NoError(t, err)

	makeGitHubIntegration := func(name, clientID, clientSecret string) types.Resource {
		ig, err := types.NewIntegrationGitHub(
			types.Metadata{Name: name},
			&types.GitHubIntegrationSpecV1{
				Organization: name,
			},
		)
		require.NoError(t, err)
		require.NoError(t, ig.SetCredentials(&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_IdSecret{
				IdSecret: &types.PluginIdSecretCredential{
					Id:     clientID,
					Secret: clientSecret,
				},
			},
		}))
		return ig
	}

	mustFindOAuthClientID := func(t *testing.T, igName, wantClientID string) {
		t.Helper()
		ig, err := igSvc.GetIntegration(t.Context(), igName)
		require.NoError(t, err)
		ref := ig.GetCredentials().GetStaticCredentialsRef()
		require.NotNil(t, ref)
		cred, err := credentials.GetByPurpose(t.Context(), ref, credentials.PurposeGitHubOAuth, credsSvc)
		require.NoError(t, err)
		require.Equal(t, wantClientID, cred.GetOAuthClientID())
	}

	t.Run("Create", func(t *testing.T) {
		ig := makeGitHubIntegration("github-test-org", "test-id", "test-secret")
		raw := mustMakeUnknownResource(t, ig)
		require.NoError(t, handler.Create(t.Context(), clt, raw, CreateOpts{}))

		mustFindOAuthClientID(t, "github-test-org", "test-id")
	})

	t.Run("Get", func(t *testing.T) {
		collection, err := handler.Get(t.Context(), clt, services.Ref{Name: "github-test-org"}, GetOpts{})
		require.NoError(t, err)
		resources := collection.Resources()
		require.Len(t, resources, 1)
		require.Equal(t, "github-test-org", resources[0].GetName())
	})

	t.Run("Update", func(t *testing.T) {
		collection, err := handler.Get(t.Context(), clt, services.Ref{Name: "github-test-org"}, GetOpts{})
		require.NoError(t, err)
		resources := collection.Resources()
		require.Len(t, resources, 1)

		ig, ok := resources[0].(types.Integration)
		require.True(t, ok)
		ig.SetGitHubIntegrationSpec(&types.GitHubIntegrationSpecV1{
			Organization: "updated-org",
		})
		require.NoError(t, ig.SetCredentials(&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_IdSecret{
				IdSecret: &types.PluginIdSecretCredential{
					Id:     "new-id",
					Secret: "new-secret",
				},
			},
		}))
		raw := mustMakeUnknownResource(t, ig)
		require.NoError(t, handler.Update(t.Context(), clt, raw, CreateOpts{}))

		collection, err = handler.Get(t.Context(), clt, services.Ref{Name: "github-test-org"}, GetOpts{})
		require.NoError(t, err)
		updated := collection.Resources()[0].(types.Integration)
		require.Equal(t, "updated-org", updated.GetGitHubIntegrationSpec().Organization)
		// TODO(greedy52) look for new-id once new credentials is passed to backend.
		mustFindOAuthClientID(t, "github-test-org", "test-id")
	})

	t.Run("CreateForce", func(t *testing.T) {
		ig := makeGitHubIntegration("github-test-org", "test-id", "test-secret")
		raw := mustMakeUnknownResource(t, ig)
		require.NoError(t, handler.Create(t.Context(), clt, raw, CreateOpts{Force: true}))
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, handler.Delete(t.Context(), clt, services.Ref{Name: "github-test-org"}))

		_, err := handler.Get(t.Context(), clt, services.Ref{Name: "github-test-org"}, GetOpts{})
		require.True(t, trace.IsNotFound(err), "expected not found, got: %v", err)
	})
}
