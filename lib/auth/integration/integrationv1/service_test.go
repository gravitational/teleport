/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"cmp"
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestIntegrationCRUD(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	clusterName := "test-cluster"
	proxyPublicAddr := "127.0.0.1.nip.io"

	ca := newCertAuthority(t, types.HostCA, clusterName)
	ctx, localClient, resourceSvc := initSvc(t, ca, clusterName, proxyPublicAddr)

	noError := func(err error) bool {
		return err == nil
	}

	sampleIntegrationFn := func(t *testing.T, name string) types.Integration {
		ig, err := types.NewIntegrationAWSOIDC(
			types.Metadata{Name: name},
			&types.AWSOIDCIntegrationSpecV1{
				RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
			},
		)
		require.NoError(t, err)
		return ig
	}

	tt := []struct {
		Name            string
		Role            types.RoleSpecV6
		IntegrationName string
		Setup           func(t *testing.T, igName string)
		Test            func(ctx context.Context, resourceSvc *Service, igName string) error
		Validate        func(t *testing.T, igName string)
		Cleanup         func(t *testing.T, igName string)
		ErrAssertion    func(error) bool
	}{
		// Read
		{
			Name: "allowed read access to integrations",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
					Name: igName,
				})
				return err
			},
			ErrAssertion: noError,
		},
		{
			Name: "no access to read integrations",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
					Name: igName,
				})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},
		{
			Name: "denied access to read integrations",
			Role: types.RoleSpecV6{
				Deny: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.GetIntegration(ctx, &integrationpb.GetIntegrationRequest{
					Name: igName,
				})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},

		// List
		{
			Name: "allowed list access to integrations",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbList, types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, _ string) {
				for i := 0; i < 10; i++ {
					_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, uuid.NewString()))
					require.NoError(t, err)
				}
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.ListIntegrations(ctx, &integrationpb.ListIntegrationsRequest{
					Limit:   0,
					NextKey: "",
				})
				return err
			},
			ErrAssertion: noError,
		},
		{
			Name: "no list access to integrations",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.ListIntegrations(ctx, &integrationpb.ListIntegrationsRequest{
					Limit:   0,
					NextKey: "",
				})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},

		// Create
		{
			Name: "no access to create integrations",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig := sampleIntegrationFn(t, igName)
				_, err := resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: ig.(*types.IntegrationV1)})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},
		{
			Name: "access to create integrations",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			IntegrationName: "integration-allow-create-access",
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig := sampleIntegrationFn(t, igName)
				_, err := resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: ig.(*types.IntegrationV1)})
				return err
			},
			ErrAssertion: noError,
		},
		{
			Name: "access to create integrations but name is invalid",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			IntegrationName: "integration-awsoidc-invalid.name",
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig := sampleIntegrationFn(t, igName)
				_, err := resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: ig.(*types.IntegrationV1)})
				return err
			},
			ErrAssertion: trace.IsBadParameter,
		},
		{
			Name: "create github integrations",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig, err := newGitHubIntegration(igName, "id", "secret")
				if err != nil {
					return trace.Wrap(err)
				}

				_, err = resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: ig})
				return trace.Wrap(err)
			},
			Validate: func(t *testing.T, igName string) {
				mustFindGitHubCredentials(t, localClient, igName, "id", "secret")
			},
			ErrAssertion: noError,
		},

		// Update
		{
			Name: "no access to update integration",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig := sampleIntegrationFn(t, igName)
				_, err := resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: ig.(*types.IntegrationV1)})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},
		{
			Name: "access to update integration",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbUpdate},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				ig := sampleIntegrationFn(t, igName)
				_, err := resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: ig.(*types.IntegrationV1)})
				return err
			},
			ErrAssertion: noError,
		},
		{
			Name: "credentials updated",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				oldIg, err := newGitHubIntegration(igName, "id", "secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: oldIg})
				if err != nil {
					return trace.Wrap(err)
				}

				newIg, err := newGitHubIntegration(igName, "new-id", "new-secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: newIg})
				return err
			},
			Validate: func(t *testing.T, igName string) {
				mustFindGitHubCredentials(t, localClient, igName, "new-id", "new-secret")
			},
			ErrAssertion: noError,
		},
		{
			Name: "credentials preserved",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				oldIg, err := newGitHubIntegration(igName, "id", "secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: oldIg})
				if err != nil {
					return trace.Wrap(err)
				}

				newIg, err := newGitHubIntegration(igName, "", "")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: newIg})
				return err
			},
			Validate: func(t *testing.T, igName string) {
				mustFindGitHubCredentials(t, localClient, igName, "id", "secret")
			},
			ErrAssertion: noError,
		},
		{
			Name: "OAuth secret only update",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				oldIg, err := newGitHubIntegration(igName, "id", "secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: oldIg})
				if err != nil {
					return trace.Wrap(err)
				}

				newIg, err := newGitHubIntegration(igName, "", "new-secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: newIg})
				return err
			},
			Validate: func(t *testing.T, igName string) {
				mustFindGitHubCredentials(t, localClient, igName, "id", "new-secret")
			},
			ErrAssertion: noError,
		},
		{
			Name: "OAuth update fails when secret is missing",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				oldIg, err := newGitHubIntegration(igName, "id", "secret")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.CreateIntegration(ctx, &integrationpb.CreateIntegrationRequest{Integration: oldIg})
				if err != nil {
					return trace.Wrap(err)
				}

				newIg, err := newGitHubIntegration(igName, "new-id", "")
				if err != nil {
					return trace.Wrap(err)
				}
				_, err = resourceSvc.UpdateIntegration(ctx, &integrationpb.UpdateIntegrationRequest{Integration: newIg})
				return err
			},
			ErrAssertion: trace.IsBadParameter,
		},

		// Delete
		{
			Name: "no access to delete integration",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: "x"})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},
		{
			Name: "cant delete integration referenced by draft external audit storage",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
				_, err = localClient.GenerateDraftExternalAuditStorage(ctx, igName, "us-west-2")
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: igName})
				return err

			},
			Cleanup: func(t *testing.T, igName string) {
				err := localClient.DeleteDraftExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			ErrAssertion: trace.IsBadParameter,
		},
		{
			Name: "cant delete integration referenced by cluster external audit storage",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
				_, err = localClient.GenerateDraftExternalAuditStorage(ctx, igName, "us-west-2")
				require.NoError(t, err)
				err = localClient.PromoteToClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: igName})
				return err
			},
			Cleanup: func(t *testing.T, igName string) {
				err := localClient.DisableClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			ErrAssertion: trace.IsBadParameter,
		},
		{
			Name: "can't delete integration referenced by AWS Identity Center plugin",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
				// other existing plugin should not affect identity center plugin referenced integration.
				require.NoError(t, localClient.CreatePlugin(ctx, NewMattermostPlugin()))
				require.NoError(t, localClient.CreatePlugin(ctx, NewIdentityCenterPlugin(igName, igName)))
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: igName})
				return err
			},
			Cleanup: func(t *testing.T, igName string) {
				require.NoError(t, localClient.DeletePlugin(ctx, types.PluginTypeMattermost))
				require.NoError(t, localClient.DeletePlugin(ctx, types.PluginTypeAWSIdentityCenter))
			},
			ErrAssertion: trace.IsBadParameter,
		},
		{
			Name: "access to delete integration",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				_, err := localClient.CreateIntegration(ctx, sampleIntegrationFn(t, igName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: igName})
				return err
			},
			ErrAssertion: noError,
		},
		{
			Name: "credentials are also deleted",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				t.Helper()

				ig := sampleIntegrationFn(t, igName)
				refUUID := uuid.NewString()
				ig.SetCredentials(&types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: credentials.NewRefWithUUID(refUUID),
					},
				})

				_, err := localClient.CreateIntegration(ctx, ig)
				require.NoError(t, err)

				// Save a fake credentials that can be found using igName.
				cred := &types.PluginStaticCredentialsV1{
					ResourceHeader: types.ResourceHeader{
						Metadata: types.Metadata{
							Name: igName,
							Labels: map[string]string{
								credentials.LabelStaticCredentialsIntegration: refUUID,
								credentials.LabelStaticCredentialsPurpose:     "test",
							},
						},
					},
					Spec: &types.PluginStaticCredentialsSpecV1{
						Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
							OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
								ClientId:     "id",
								ClientSecret: "secret",
							},
						},
					},
				}
				err = localClient.CreatePluginStaticCredentials(ctx, cred)
				require.NoError(t, err)

				// double-check
				staticCreds, err := localClient.GetPluginStaticCredentials(context.Background(), igName)
				require.NoError(t, err)
				require.NotNil(t, staticCreds)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{Name: igName})
				return err
			},
			Validate: func(t *testing.T, igName string) {
				t.Helper()
				_, err := localClient.GetPluginStaticCredentials(context.Background(), igName)
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
			ErrAssertion: noError,
		},
		{
			Name: "cannot delete github integration with existing git server",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				t.Helper()

				ig, err := newGitHubIntegration(igName, "", "")
				require.NoError(t, err)
				_, err = localClient.CreateIntegration(ctx, ig)
				require.NoError(t, err)
				_, err = localClient.CreateGitServer(ctx, mustMakeGitHubServer(t, ig))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{
					Name: igName,
				})
				return err
			},
			ErrAssertion: trace.IsBadParameter,
		},
		{
			Name: "no access to delete github integration with associated resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, igName string) {
				t.Helper()

				ig, err := newGitHubIntegration(igName, "", "")
				require.NoError(t, err)
				_, err = localClient.CreateIntegration(ctx, ig)
				require.NoError(t, err)
				_, err = localClient.CreateGitServer(ctx, mustMakeGitHubServer(t, ig))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{
					Name:                      igName,
					DeleteAssociatedResources: true,
				})
				return err
			},
			ErrAssertion: trace.IsAccessDenied,
		},
		{
			Name: "delete github integration with associated resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{
					{
						Resources: []string{types.KindIntegration},
						Verbs:     []string{types.VerbDelete},
					},
					{
						Resources: []string{types.KindGitServer},
						Verbs:     []string{types.VerbDelete, types.VerbList},
					},
				}},
			},
			Setup: func(t *testing.T, igName string) {
				t.Helper()

				ig, err := newGitHubIntegration(igName, "", "")
				require.NoError(t, err)
				_, err = localClient.CreateIntegration(ctx, ig)
				require.NoError(t, err)
				_, err = localClient.CreateGitServer(ctx, mustMakeGitHubServer(t, ig))
				require.NoError(t, err)

				anotherIg, err := newGitHubIntegration(igName+igName, "", "")
				require.NoError(t, err)
				_, err = localClient.CreateIntegration(ctx, anotherIg)
				require.NoError(t, err)
				_, err = localClient.CreateGitServer(ctx, mustMakeGitHubServer(t, anotherIg))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteIntegration(ctx, &integrationpb.DeleteIntegrationRequest{
					Name:                      igName,
					DeleteAssociatedResources: true,
				})
				return err
			},
			Validate: func(t *testing.T, igName string) {
				t.Helper()
				// Validate git server associated with the integration is
				// removed and other git server is intact.
				_, err := localClient.GetGitServer(context.Background(), igName)
				require.True(t, trace.IsNotFound(err))
				_, err = localClient.GetGitServer(context.Background(), igName+igName)
				require.NoError(t, err)
			},
			ErrAssertion: noError,
		},

		// Delete all
		{
			Name: "delete all integrations fails",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindIntegration},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, igName string) error {
				_, err := resourceSvc.DeleteAllIntegrations(ctx, &integrationpb.DeleteAllIntegrationsRequest{})
				return err
			},
			// Deleting all integrations via gRPC is not supported.
			ErrAssertion: trace.IsBadParameter,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			localCtx := authorizerForDummyUser(t, ctx, tc.Role, localClient)
			igName := cmp.Or(tc.IntegrationName, uuid.NewString())
			if tc.Setup != nil {
				tc.Setup(t, igName)
			}

			if tc.Cleanup != nil {
				t.Cleanup(func() { tc.Cleanup(t, igName) })
			}

			err := tc.Test(localCtx, resourceSvc, igName)
			require.True(t, tc.ErrAssertion(err), trace.DebugReport(err))

			// Extra validation
			if tc.Validate != nil {
				tc.Validate(t, igName)
			}
		})
	}
}

func authorizerForDummyUser(t *testing.T, ctx context.Context, roleSpec types.RoleSpecV6, localClient localClient) context.Context {
	// Create role
	roleName := "role-" + uuid.NewString()
	role, err := types.NewRole(roleName, roleSpec)
	require.NoError(t, err)

	role, err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	user, err = localClient.CreateUser(ctx, user)
	require.NoError(t, err)

	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})
}

type localClient interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	GenerateDraftExternalAuditStorage(ctx context.Context, integrationName, region string) (*externalauditstorage.ExternalAuditStorage, error)
	DeleteDraftExternalAuditStorage(ctx context.Context) error
	PromoteToClusterExternalAuditStorage(ctx context.Context) error
	DisableClusterExternalAuditStorage(ctx context.Context) error
	CreatePlugin(ctx context.Context, plugin types.Plugin) error
	DeletePlugin(ctx context.Context, name string) error
	services.PluginStaticCredentials
	services.Integrations
	services.GitServers
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

// NewIdentityCenterPlugin returns a new types.PluginV1 with PluginSpecV1_Mattermost settings.
func NewMattermostPlugin() *types.PluginV1 {
	return &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				"teleport.dev/hosted-plugin": "true",
			},
			Name: types.PluginTypeMattermost,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Mattermost{
				Mattermost: &types.PluginMattermostSettings{
					ServerUrl:     "https://example.com",
					Channel:       "test_channel",
					Team:          "test_team",
					ReportToEmail: "test@example.com",
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": "mattermost",
					},
				},
			},
		},
	}
}

// NewIdentityCenterPlugin returns a new types.PluginV1 with PluginSpecV1_AwsIc settings.
func NewIdentityCenterPlugin(serviceProviderName, integrationName string) *types.PluginV1 {
	return &types.PluginV1{
		Metadata: types.Metadata{
			Name: types.PluginTypeAWSIdentityCenter,
			Labels: map[string]string{
				types.HostedPluginLabel: "true",
			},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_AwsIc{
				AwsIc: &types.PluginAWSICSettings{
					IntegrationName:         integrationName,
					Region:                  "test-region",
					Arn:                     "test-arn",
					AccessListDefaultOwners: []string{"user1", "user2"},
					ProvisioningSpec: &types.AWSICProvisioningSpec{
						BaseUrl: "https://example.com",
					},
					SamlIdpServiceProviderName: serviceProviderName,
				},
			},
		},
	}
}

func initSvc(t *testing.T, ca types.CertAuthority, clusterName string, proxyPublicAddr string) (context.Context, localClient, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)
	easSvc := local.NewExternalAuditStorageService(backend)
	pluginSvc := local.NewPluginsService(backend)

	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)
	gitServerService, err := local.NewGitServerService(backend)
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	localResourceService, err := local.NewIntegrationsService(backend)
	require.NoError(t, err)
	localCredService, err := local.NewPluginStaticCredentialsService(backend)
	require.NoError(t, err)

	backendSvc := struct {
		services.Integrations
		services.PluginStaticCredentials
		services.GitServers
	}{
		Integrations:            localResourceService,
		PluginStaticCredentials: localCredService,
		GitServers:              gitServerService,
	}

	cacheResourceService, err := local.NewIntegrationsService(backend, local.WithIntegrationsServiceCacheMode(true))
	require.NoError(t, err)

	cache := &mockCache{
		domainName: clusterName,
		ca:         ca,
		proxies: []types.Server{
			&types.ServerV2{Spec: types.ServerSpecV2{
				PublicAddrs: []string{proxyPublicAddr},
			}},
		},
		IntegrationsService:            *cacheResourceService,
		PluginStaticCredentialsService: localCredService,
	}

	resourceSvc, err := NewService(&ServiceConfig{
		Backend:         backendSvc,
		Authorizer:      authorizer,
		Cache:           cache,
		KeyStoreManager: keystore.NewSoftwareKeystoreForTests(t),
		Emitter:         events.NewDiscardEmitter(),
	})
	require.NoError(t, err)

	return ctx, struct {
		*local.AccessService
		*local.IdentityService
		*local.ExternalAuditStorageService
		*local.IntegrationsService
		*local.PluginsService
		*local.PluginStaticCredentialsService
		*local.GitServerService
	}{
		AccessService:                  roleSvc,
		IdentityService:                userSvc,
		ExternalAuditStorageService:    easSvc,
		IntegrationsService:            localResourceService,
		PluginsService:                 pluginSvc,
		PluginStaticCredentialsService: localCredService,
		GitServerService:               gitServerService,
	}, resourceSvc
}

type mockCache struct {
	domainName string
	ca         types.CertAuthority

	proxies   []types.Server
	returnErr error

	local.IntegrationsService
	*local.PluginStaticCredentialsService
}

func (m *mockCache) GetProxies() ([]types.Server, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return m.proxies, nil
}

func (m *mockCache) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	return nil, nil
}

func (m *mockCache) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	return nil
}

// GetClusterName returns local auth domain of the current auth server
func (m *mockCache) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return &types.ClusterNameV2{
		Spec: types.ClusterNameSpecV2{
			ClusterName: m.domainName,
		},
	}, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (m *mockCache) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	return m.ca, nil
}

func newCertAuthority(t *testing.T, caType types.CertAuthType, domain string) types.CertAuthority {
	t.Helper()

	ta := testauthority.New()
	pub, priv, err := ta.GenerateJWT()
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: domain,
		ActiveKeys: types.CAKeySet{
			JWT: []*types.JWTKeyPair{{
				PublicKey:      pub,
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
			}},
		},
	})
	require.NoError(t, err)

	return ca
}

func newGitHubIntegration(name, id, secret string) (*types.IntegrationV1, error) {
	ig, err := types.NewIntegrationGitHub(
		types.Metadata{
			Name: name,
		},
		&types.GitHubIntegrationSpecV1{
			Organization: "my-org",
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if id != "" || secret != "" {
		ig.SetCredentials(&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_IdSecret{
				IdSecret: &types.PluginIdSecretCredential{
					Id:     id,
					Secret: secret,
				},
			},
		})
	}
	return ig, nil
}

func mustFindGitHubCredentials(t *testing.T, localClient Backend, igName, wantId, wantSecret string) {
	t.Helper()

	ig, err := localClient.GetIntegration(context.Background(), igName)
	require.NoError(t, err)

	creds := ig.GetCredentials()
	require.NotNil(t, creds)
	require.NotNil(t, creds.GetStaticCredentialsRef())

	staticCreds, err := localClient.GetPluginStaticCredentialsByLabels(context.Background(), creds.GetStaticCredentialsRef().Labels)
	require.NoError(t, err)
	require.Len(t, staticCreds, 2)

	var seenSSHCA, seenOAuth bool
	for _, cred := range staticCreds {
		if len(cred.GetSSHCertAuthorities()) != 0 {
			seenSSHCA = true
			continue
		}
		if id, secret := cred.GetOAuthClientSecret(); id != "" {
			assert.Equal(t, wantId, id)
			assert.Equal(t, wantSecret, secret)
			seenOAuth = true
		}
	}
	assert.True(t, seenSSHCA)
	assert.True(t, seenOAuth)
}

func mustMakeGitHubServer(t *testing.T, ig types.Integration) types.Server {
	t.Helper()
	require.NotNil(t, ig.GetGitHubIntegrationSpec())
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Organization: ig.GetGitHubIntegrationSpec().Organization,
		Integration:  ig.GetName(),
	})
	require.NoError(t, err)
	server.SetName(ig.GetName())
	return server
}
