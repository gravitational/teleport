// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"fmt"
	"net"
	"os/user"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/apiserver/handler"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
)

func TestTeleterm(t *testing.T) {
	pack := dbhelpers.SetupDatabaseTest(t,
		dbhelpers.WithListenerSetupDatabaseTest(helpers.SingleProxyPortSetup),
		dbhelpers.WithLeafConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		dbhelpers.WithRootConfig(func(config *servicecfg.Config) {
			config.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)
	pack.WaitForLeaf(t)

	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: pack.Root.User.GetName(),
	})
	require.NoError(t, err)

	t.Run("adding root cluster", func(t *testing.T) {
		t.Parallel()

		testAddingRootCluster(t, pack, creds)
	})

	t.Run("ListRootClusters returns logged in user", func(t *testing.T) {
		t.Parallel()

		testListRootClustersReturnsLoggedInUser(t, pack, creds)
	})

	t.Run("GetCluster returns properties from auth server", func(t *testing.T) {
		t.Parallel()

		testGetClusterReturnsPropertiesFromAuthServer(t, pack)
	})

	t.Run("CreateConnectMyComputerRole", func(t *testing.T) {
		t.Parallel()
		testCreateConnectMyComputerRole(t, pack)
	})
}

func testAddingRootCluster(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	t.Helper()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                t.TempDir(),
		InsecureSkipVerify: true,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	addedCluster, err := daemonService.AddCluster(context.Background(), pack.Root.Cluster.Web)
	require.NoError(t, err)

	clusters, err := daemonService.ListRootClusters(context.Background())
	require.NoError(t, err)

	clusterURIs := make([]uri.ResourceURI, 0, len(clusters))
	for _, cluster := range clusters {
		clusterURIs = append(clusterURIs, cluster.URI)
	}
	require.ElementsMatch(t, clusterURIs, []uri.ResourceURI{addedCluster.URI})
}

func testListRootClustersReturnsLoggedInUser(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	handler, err := handler.New(
		handler.Config{
			DaemonService: daemonService,
		},
	)
	require.NoError(t, err)

	response, err := handler.ListRootClusters(context.Background(), &api.ListClustersRequest{})
	require.NoError(t, err)

	require.Equal(t, 1, len(response.Clusters))
	require.Equal(t, pack.Root.User.GetName(), response.Clusters[0].LoggedInUser.Name)
}

func testGetClusterReturnsPropertiesFromAuthServer(t *testing.T, pack *dbhelpers.DatabasePack) {
	authServer := pack.Root.Cluster.Process.GetAuthServer()

	// Use random names to not collide with other tests.
	uuid := uuid.NewString()
	suggestedReviewer := "suggested-reviewer"
	requestableRoleName := fmt.Sprintf("%s-%s", "requested-role", uuid)
	userName := fmt.Sprintf("%s-%s", "user", uuid)
	roleName := fmt.Sprintf("%s-%s", "get-cluster-role", uuid)

	requestableRole, err := types.NewRole(requestableRoleName, types.RoleSpecV6{})
	require.NoError(t, err)

	// Create user role with ability to request role
	userRole, err := types.NewRole(roleName, types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Logins: []string{
				userName,
			},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Request: &types.AccessRequestConditions{
				Roles:              []string{requestableRoleName},
				SuggestedReviewers: []string{suggestedReviewer},
			},
		},
	})
	require.NoError(t, err)

	// add role that user can request
	err = authServer.UpsertRole(context.Background(), requestableRole)
	require.NoError(t, err)

	// add role that allows to request "requestableRole"
	err = authServer.UpsertRole(context.Background(), userRole)
	require.NoError(t, err)

	user, err := types.NewUser(userName)
	user.AddRole(userRole.GetName())
	require.NoError(t, err)

	err = authServer.UpsertUser(user)
	require.NoError(t, err)

	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: userName,
	})
	require.NoError(t, err)

	tc := mustLogin(t, userName, pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	handler, err := handler.New(
		handler.Config{
			DaemonService: daemonService,
		},
	)
	require.NoError(t, err)

	rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)

	response, err := handler.GetCluster(context.Background(), &api.GetClusterRequest{
		ClusterUri: uri.NewClusterURI(rootClusterName).String(),
	})
	require.NoError(t, err)

	require.Equal(t, userName, response.LoggedInUser.Name)
	require.ElementsMatch(t, []string{requestableRoleName}, response.LoggedInUser.RequestableRoles)
	require.ElementsMatch(t, []string{suggestedReviewer}, response.LoggedInUser.SuggestedReviewers)
}

func testCreateConnectMyComputerRole(t *testing.T, pack *dbhelpers.DatabasePack) {
	systemUser, err := user.Current()
	require.NoError(t, err)

	tests := []struct {
		name               string
		userAlreadyHasRole bool
		existingRole       func(userName string) types.RoleV6
	}{
		{
			name: "role does not exist",
		},
		{
			name: "role exists and includes current system username",
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{userName},
							},
							Logins: []string{systemUser.Username},
						},
					},
				}
			},
		},
		{
			name: "role exists and does not include current system username",
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{userName},
							},
							Logins: []string{fmt.Sprintf("bogus-login-%v", uuid.NewString())},
						},
					},
				}
			},
		},
		{
			name: "role exists and has no logins",
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{userName},
							},
							Logins: []string{},
						},
					},
				}
			},
		},
		{
			name: "role exists and owner node label was changed",
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{"bogus-username"},
							},
							Logins: []string{systemUser.Username},
						},
					},
				}
			},
		},
		{
			name:               "user already has existing role that includes current system username",
			userAlreadyHasRole: true,
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{userName},
							},
							Logins: []string{systemUser.Username},
						},
					},
				}
			},
		},
		{
			name:               "user already has existing role that does not include current system username",
			userAlreadyHasRole: true,
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{userName},
							},
							Logins: []string{fmt.Sprintf("bogus-login-%v", uuid.NewString())},
						},
					},
				}
			},
		},
		{
			name:               "user already has existing role with modified owner node label",
			userAlreadyHasRole: true,
			existingRole: func(userName string) types.RoleV6 {
				return types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							NodeLabels: types.Labels{
								types.ConnectMyComputerNodeOwnerLabel: []string{"bogus-username"},
							},
							Logins: []string{fmt.Sprintf("bogus-login-%v", uuid.NewString())},
						},
					},
				}
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			authServer := pack.Root.Cluster.Process.GetAuthServer()
			uuid := uuid.NewString()
			userName := fmt.Sprintf("user-cmc-%s", uuid)
			roleName := fmt.Sprintf("connect-my-computer-%v", userName)

			var existingRole *types.RoleV6

			// Prepare an existing role if present.
			if test.existingRole != nil {
				role := test.existingRole(userName)
				role.SetMetadata(types.Metadata{
					Name: roleName,
				})
				existingRole = &role
				err = authServer.UpsertRole(ctx, &role)
				require.NoError(t, err)
			}

			// Prepare a role with rules required to call CreateConnectMyComputerRole.
			ruleWithAllowRules, err := types.NewRole(fmt.Sprintf("cmc-allow-rules-%v", uuid),
				types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							types.NewRule(types.KindUser, services.RW()),
							types.NewRule(types.KindRole, services.RW()),
						},
					},
				})
			require.NoError(t, err)
			userRoles := []types.Role{ruleWithAllowRules}

			// Create a new user to avoid colliding with other tests.
			// Assign to the user the role with allow rules and the existing role if present.
			if test.userAlreadyHasRole {
				if existingRole == nil {
					t.Log("userAlreadyHasRole must be used together with existingRole")
					t.Fail()
					return
				}
				userRoles = append(userRoles, existingRole)
			}
			_, err = auth.CreateUser(authServer, userName, userRoles...)
			require.NoError(t, err)

			// Log in as the new user.
			creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
				Process:  pack.Root.Cluster.Process,
				Username: userName,
			})
			require.NoError(t, err)
			tc := mustLogin(t, userName, pack, creds)

			// Prepare daemon.Service.
			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                tc.KeysDir,
				InsecureSkipVerify: tc.InsecureSkipVerify,
			})
			require.NoError(t, err)

			daemonService, err := daemon.New(daemon.Config{
				Storage: storage,
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				daemonService.Stop()
			})
			handler, err := handler.New(
				handler.Config{
					DaemonService: daemonService,
				},
			)
			require.NoError(t, err)

			// Call CreateConnectMyComputerRole.
			rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
			require.NoError(t, err)
			rootClusterURI := uri.NewClusterURI(rootClusterName).String()
			response, err := handler.CreateConnectMyComputerRole(ctx, &api.CreateConnectMyComputerRoleRequest{
				RootClusterUri: rootClusterURI,
			})
			require.NoError(t, err)

			if test.userAlreadyHasRole {
				require.False(t, response.CertsReloaded,
					"expected the handler to signal that the certs were not reloaded since the user was already assigned the role")
			} else {
				require.True(t, response.CertsReloaded,
					"expected the handler to signal that the certs were reloaded since the user was just assigned a new role")
			}

			// Verify that the role exists.
			role, err := authServer.GetRole(ctx, roleName)
			require.NoError(t, err)

			// Verify that the role grants expected privileges.
			require.Contains(t, role.GetNodeLabels(types.Allow), types.ConnectMyComputerNodeOwnerLabel)
			expectedNodeLabelValue := utils.Strings{userName}
			actualNodeLabelValue := role.GetNodeLabels(types.Allow)[types.ConnectMyComputerNodeOwnerLabel]
			require.Equal(t, expectedNodeLabelValue, actualNodeLabelValue)
			require.Contains(t, role.GetLogins(types.Allow), systemUser.Username)

			// Verify that the certs have been reloaded and that the user is assigned the role.
			//
			// GetCluster reads data from the cert. If the certs were not reloaded properly, GetCluster
			// will not return the role that's just been assigned to the user.
			clusterDetails, err := handler.GetCluster(ctx, &api.GetClusterRequest{
				ClusterUri: rootClusterURI,
			})
			require.NoError(t, err)
			require.Contains(t, clusterDetails.LoggedInUser.Roles, roleName,
				"the user certs don't include the freshly added role; the certs might have not been reloaded properly")
		})
	}
}

func mustLogin(t *testing.T, userName string, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) *client.TeleportClient {
	tc, err := pack.Root.Cluster.NewClientWithCreds(helpers.ClientConfig{
		Login:   userName,
		Cluster: pack.Root.Cluster.Secrets.SiteName,
	}, *creds)
	require.NoError(t, err)
	// Save the profile yaml file to disk as NewClientWithCreds doesn't do that by itself.
	err = tc.SaveProfile(false /* makeCurrent */)
	require.NoError(t, err)
	return tc
}
