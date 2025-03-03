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

package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/user"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	dbhelpers "github.com/gravitational/teleport/integration/db"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/apiserver/handler"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/tlsca"
	libutils "github.com/gravitational/teleport/lib/utils"
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

	t.Run("headless watcher", func(t *testing.T) {
		t.Parallel()

		testHeadlessWatcher(t, pack, creds)
	})

	t.Run("CreateConnectMyComputerRole", func(t *testing.T) {
		t.Parallel()
		testCreateConnectMyComputerRole(t, pack)
	})

	t.Run("CreateConnectMyComputerToken", func(t *testing.T) {
		t.Parallel()
		testCreateConnectMyComputerToken(t, pack, nil /* setupUserMFA */)
	})

	t.Run("WaitForConnectMyComputerNodeJoin", func(t *testing.T) {
		t.Parallel()
		testWaitForConnectMyComputerNodeJoin(t, pack, creds)
	})

	t.Run("DeleteConnectMyComputerNode", func(t *testing.T) {
		t.Parallel()
		testDeleteConnectMyComputerNode(t, pack)
	})

	t.Run("client cache", func(t *testing.T) {
		t.Parallel()

		testClientCache(t, pack, creds)
	})

	t.Run("ListDatabaseUsers", func(t *testing.T) {
		// ListDatabaseUsers cannot be run in parallel as it modifies the default roles of users set up
		// through the test pack.
		// TODO(ravicious): After some optimizations, those tests could run in parallel. Instead of
		// modifying existing roles, they could create new users with new roles and then update the role
		// mapping between the root the leaf cluster through authServer.UpdateUserCARoleMap.
		testListDatabaseUsers(t, pack)
	})

	t.Run("with MFA", func(t *testing.T) {
		authServer := pack.Root.Cluster.Process.GetAuthServer()
		rpID, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
		require.NoError(t, err)

		// Enforce MFA
		helpers.UpsertAuthPrefAndWaitForCache(t, context.Background(), authServer, &types.AuthPreferenceV2{
			Spec: types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				Webauthn: &types.Webauthn{
					RPID: rpID,
				},
			},
		})

		// Remove MFA enforcement on cleanup.
		t.Cleanup(func() {
			helpers.UpsertAuthPrefAndWaitForCache(t, context.Background(), authServer, &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOff,
				},
			})
			require.NoError(t, err)
		})

		setupUserMFA := func(t *testing.T, userName string, tshdEventsService *mockTSHDEventsService) client.WebauthnLoginFunc {
			// Configure user account with an MFA device.
			origin := fmt.Sprintf("https://%s", rpID)
			device, err := mocku2f.Create()
			require.NoError(t, err)
			device.SetPasswordless()

			token, err := authServer.CreateResetPasswordToken(context.Background(), authclient.CreateUserTokenRequest{
				Name: userName,
			})
			require.NoError(t, err)

			tokenID := token.GetName()
			res, err := authServer.CreateRegisterChallenge(context.Background(), &proto.CreateRegisterChallengeRequest{
				TokenID:     tokenID,
				DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
			})
			require.NoError(t, err)
			cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

			ccr, err := device.SignCredentialCreation(origin, cc)
			require.NoError(t, err)
			_, err = authServer.ChangeUserAuthentication(context.Background(), &proto.ChangeUserAuthenticationRequest{
				TokenID: tokenID,
				NewMFARegisterResponse: &proto.MFARegisterResponse{
					Response: &proto.MFARegisterResponse_Webauthn{
						Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
					},
				},
			})
			require.NoError(t, err)

			// Prepare a function which simulates key tap.
			var webauthLoginCallCount atomic.Uint32
			webauthnLogin := func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
				t.Helper()
				updatedWebauthnLoginCallCount := webauthLoginCallCount.Add(1)

				// When daemon.mfaPrompt prompts for MFA, it spawns two goroutines. One calls PromptMFA on
				// tshdEventService and expects OTP in response (if available). Another calls this function.
				// Whichever returns a non-error response first wins.
				//
				// Since in this test we use Webauthn, this function can return ASAP without giving a chance
				// to the other to call PromptMFA. This would cause race conditions, as we might want to
				// verify later in the test that PromptMFA has indeed been called.
				//
				// To ensure that, this function waits until PromptMFA has been called before proceeding.
				// This also simulates a flow where the user was notified about the need to tap the key
				// through the UI and then taps the key.
				assert.EventuallyWithT(t, func(t *assert.CollectT) {
					// Each call to webauthnLogin should have an equivalent call to PromptMFA and there should
					// be no multiple concurrent calls.
					assert.Equal(t, updatedWebauthnLoginCallCount, tshdEventsService.promptMFACallCount.Load(),
						"Expected each call to webauthnLogin to have an equivalent call to PromptMFA")
				}, 5*time.Second, 50*time.Millisecond)

				car, err := device.SignAssertion(origin, assertion)
				if err != nil {
					return nil, "", err
				}

				carProto := wantypes.CredentialAssertionResponseToProto(car)

				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: carProto,
					},
				}, "", nil
			}

			return webauthnLogin
		}

		t.Run("CreateConnectMyComputerToken", func(t *testing.T) {
			t.Parallel()

			testCreateConnectMyComputerToken(t, pack, setupUserMFA)
		})
	})
}

func testAddingRootCluster(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	t.Helper()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                t.TempDir(),
		InsecureSkipVerify: true,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
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
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
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

	require.Len(t, response.Clusters, 1)
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
	_, err = authServer.UpsertRole(context.Background(), requestableRole)
	require.NoError(t, err)

	// add role that allows to request "requestableRole"
	_, err = authServer.UpsertRole(context.Background(), userRole)
	require.NoError(t, err)

	user, err := types.NewUser(userName)
	user.AddRole(userRole.GetName())
	require.NoError(t, err)

	_, err = authServer.UpsertUser(context.Background(), user)
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
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	clusterIDCache := clusteridcache.Cache{}

	daemonService, err := daemon.New(daemon.Config{
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
		ClusterIDCache: &clusterIDCache,
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
	clusterURI := uri.NewClusterURI(rootClusterName)

	response, err := handler.GetCluster(context.Background(), &api.GetClusterRequest{
		ClusterUri: clusterURI.String(),
	})
	require.NoError(t, err)

	require.Equal(t, userName, response.LoggedInUser.Name)
	require.ElementsMatch(t, []string{requestableRoleName}, response.LoggedInUser.RequestableRoles)
	require.ElementsMatch(t, []string{suggestedReviewer}, response.LoggedInUser.SuggestedReviewers)

	// Verify that cluster ID cache gets updated.
	clusterIDFromCache, ok := clusterIDCache.Load(clusterURI)
	require.True(t, ok, "ID for cluster %q was not found in the cache", clusterURI)
	require.NotEmpty(t, clusterIDFromCache)
	require.Equal(t, response.AuthClusterId, clusterIDFromCache)
}

func testHeadlessWatcher(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	t.Helper()
	ctx := context.Background()

	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	cluster, _, err := storage.Add(ctx, tc.WebProxyAddr)
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	expires := pack.Root.Cluster.Config.Clock.Now().Add(time.Minute)
	ha, err := types.NewHeadlessAuthentication(pack.Root.User.GetName(), "uuid", expires)
	require.NoError(t, err)
	ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING

	// Start the tshd event service and connect the daemon to it.
	tshdEventsService, addr := newMockTSHDEventsServiceServer(t)
	err = daemonService.UpdateAndDialTshdEventsServerAddress(addr)
	require.NoError(t, err)

	err = daemonService.StartHeadlessWatcher(cluster.URI.String(), false /* waitInit */)
	require.NoError(t, err)

	// Stop and restart the watcher twice to simulate logout + login + relogin. Ensure the watcher catches events.

	err = daemonService.StopHeadlessWatcher(cluster.URI.String())
	require.NoError(t, err)
	err = daemonService.StartHeadlessWatcher(cluster.URI.String(), false /* waitInit */)
	require.NoError(t, err)
	err = daemonService.StartHeadlessWatcher(cluster.URI.String(), true /* waitInit */)
	require.NoError(t, err)

	// Ensure the watcher catches events and sends them to the Electron App.

	err = pack.Root.Cluster.Process.GetAuthServer().UpsertHeadlessAuthentication(ctx, ha)
	assert.NoError(t, err)

	assert.Eventually(t,
		func() bool {
			return tshdEventsService.sendPendingHeadlessAuthenticationCount.Load() == 1
		},
		10*time.Second,
		500*time.Millisecond,
		"Expected tshdEventService to receive 1 SendPendingHeadlessAuthentication message but got %v",
		tshdEventsService.sendPendingHeadlessAuthenticationCount.Load(),
	)
}

func testClientCache(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	ctx := context.Background()

	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storageFakeClock := clockwork.NewFakeClockAt(time.Now())

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		Clock:              storageFakeClock,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	cluster, _, err := storage.Add(ctx, tc.WebProxyAddr)
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		daemonService.Stop()
	})

	// Check if parallel calls trying to get a client will return the same one.
	eg, egCtx := errgroup.WithContext(ctx)
	blocker := make(chan struct{})
	const concurrentCalls = 5
	concurrentCallsForClient := make([]*client.ClusterClient, concurrentCalls)
	for i := range concurrentCallsForClient {
		client := &concurrentCallsForClient[i]
		eg.Go(func() error {
			<-blocker
			c, err := daemonService.GetCachedClient(egCtx, cluster.URI)
			*client = c
			return err
		})
	}
	// unblock the operation which is still in progress
	close(blocker)
	require.NoError(t, eg.Wait())
	require.Subset(t, concurrentCallsForClient[:1], concurrentCallsForClient[1:])

	// Since we have a client in the cache, it should be returned.
	secondCallForClient, err := daemonService.GetCachedClient(ctx, cluster.URI)
	require.NoError(t, err)
	require.Equal(t, concurrentCallsForClient[0], secondCallForClient)

	// Let's remove the client from the cache.
	// The call to GetCachedClient will
	// connect to proxy and return a new client.
	err = daemonService.ClearCachedClientsForRoot(cluster.URI)
	require.NoError(t, err)
	thirdCallForClient, err := daemonService.GetCachedClient(ctx, cluster.URI)
	require.NoError(t, err)
	require.NotEqual(t, secondCallForClient, thirdCallForClient)
}

func testCreateConnectMyComputerRole(t *testing.T, pack *dbhelpers.DatabasePack) {
	systemUser, err := user.Current()
	require.NoError(t, err)

	tests := []struct {
		name                     string
		assertCertsReloaded      require.BoolAssertionFunc
		existingRole             func(userName string) types.RoleV6
		assignExistingRoleToUser bool
	}{
		{
			name:                "role does not exist",
			assertCertsReloaded: require.True,
		},
		{
			name:                "role exists and includes current system username",
			assertCertsReloaded: require.True,
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
			name:                "role exists and does not include current system username",
			assertCertsReloaded: require.True,
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
			name:                "role exists and has no logins",
			assertCertsReloaded: require.True,
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
			name:                "role exists and owner node label was changed",
			assertCertsReloaded: require.True,
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
			name:                     "user already has existing role that includes current system username",
			assignExistingRoleToUser: true,
			assertCertsReloaded:      require.False,
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
			name:                     "user already has existing role that does not include current system username",
			assignExistingRoleToUser: true,
			assertCertsReloaded:      require.True,
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
			name:                     "user already has existing role with modified owner node label",
			assignExistingRoleToUser: true,
			assertCertsReloaded:      require.False,
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
			name:                     "user already has existing role that does not include current system username and has modified owner node label",
			assignExistingRoleToUser: true,
			assertCertsReloaded:      require.True,
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
				_, err := authServer.UpsertRole(ctx, &role)
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
			if test.assignExistingRoleToUser {
				if existingRole == nil {
					t.Log("assignExistingRoleToUser must be used together with existingRole")
					t.Fail()
					return
				}
				userRoles = append(userRoles, existingRole)
			}
			_, err = auth.CreateUser(ctx, authServer, userName, userRoles...)
			require.NoError(t, err)

			userPassword := uuid
			require.NoError(t, authServer.UpsertPassword(userName, []byte(userPassword)))

			// Prepare daemon.Service.
			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                t.TempDir(),
				InsecureSkipVerify: true,
				HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
					return nil
				},
			})
			require.NoError(t, err)

			daemonService, err := daemon.New(daemon.Config{
				Storage:        storage,
				KubeconfigsDir: t.TempDir(),
				AgentsDir:      t.TempDir(),
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
			rootClusterURI := uri.NewClusterURI(rootClusterName).String()

			// Log in as the new user.
			// It's important to use the actual login handler rather than mustLogin. mustLogin completely
			// skips the actual login flow and saves valid certs to disk. We already had a regression that
			// was not caught by this test because the test did not trigger certain code paths because it
			// was using mustLogin as a shortcut.
			_, err = handler.AddCluster(ctx, &api.AddClusterRequest{Name: pack.Root.Cluster.Web})
			require.NoError(t, err)
			_, err = handler.Login(ctx, &api.LoginRequest{
				ClusterUri: rootClusterURI,
				Params: &api.LoginRequest_Local{
					Local: &api.LoginRequest_LocalParams{User: userName, Password: userPassword},
				},
			})
			require.NoError(t, err)

			// Call CreateConnectMyComputerRole.
			response, err := handler.CreateConnectMyComputerRole(ctx, &api.CreateConnectMyComputerRoleRequest{
				RootClusterUri: rootClusterURI,
			})
			require.NoError(t, err)

			test.assertCertsReloaded(t, response.CertsReloaded, "CertsReloaded is the opposite of the expected value")

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

func testCreateConnectMyComputerToken(t *testing.T, pack *dbhelpers.DatabasePack, setupUserMFA setupUserMFAFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	authServer := pack.Root.Cluster.Process.GetAuthServer()
	uuid := uuid.NewString()
	userName := fmt.Sprintf("user-cmc-%s", uuid)

	// Prepare a role with rules required to call CreateConnectMyComputerNodeToken.
	ruleWithAllowRules, err := types.NewRole(fmt.Sprintf("cmc-allow-rules-%v", uuid),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindToken, services.RW()),
				},
			},
		})
	require.NoError(t, err)
	userRoles := []types.Role{ruleWithAllowRules}

	_, err = auth.CreateUser(ctx, authServer, userName, userRoles...)
	require.NoError(t, err)

	tshdEventsService, addr := newMockTSHDEventsServiceServer(t)
	var webauthnLogin client.WebauthnLoginFunc
	if setupUserMFA != nil {
		webauthnLogin = setupUserMFA(t, userName, tshdEventsService)
	}

	// Log in as the new user.
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: userName,
	})
	require.NoError(t, err)
	tc := mustLogin(t, userName, pack, creds)

	fakeClock := clockwork.NewFakeClock()

	// Prepare daemon.Service.
	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		Clock:              fakeClock,
		WebauthnLogin:      webauthnLogin,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	daemonService, err := daemon.New(daemon.Config{
		Clock:          fakeClock,
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
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

	err = daemonService.UpdateAndDialTshdEventsServerAddress(addr)
	require.NoError(t, err)

	// Call CreateConnectMyComputerNodeToken.
	rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)
	rootClusterURI := uri.NewClusterURI(rootClusterName).String()
	requestCreatedAt := fakeClock.Now()
	createdTokenResponse, err := handler.CreateConnectMyComputerNodeToken(ctx, &api.CreateConnectMyComputerNodeTokenRequest{
		RootClusterUri: rootClusterURI,
	})
	require.NoError(t, err)

	// Verify that token exists
	tokenFromAuthServer, err := authServer.GetToken(ctx, createdTokenResponse.GetToken())
	require.NoError(t, err)

	// Verify that the token can be used to join nodes...
	require.Equal(t, types.SystemRoles{types.RoleNode}, tokenFromAuthServer.GetRoles())
	// ...and is valid for no longer than 5 minutes.
	require.LessOrEqual(t, tokenFromAuthServer.Expiry(), requestCreatedAt.Add(5*time.Minute))

	if setupUserMFA != nil {
		require.Equal(t, uint32(1), tshdEventsService.promptMFACallCount.Load(),
			"Unexpected number of calls to TSHDEventsClient.PromptMFA")
	}
}

func testWaitForConnectMyComputerNodeJoin(t *testing.T, pack *dbhelpers.DatabasePack, creds *helpers.UserCreds) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	t.Cleanup(cancel)

	tc := mustLogin(t, pack.Root.User.GetName(), pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	agentsDir := t.TempDir()
	daemonService, err := daemon.New(daemon.Config{
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      agentsDir,
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

	profileName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)

	waitForNodeJoinErr := make(chan error)

	go func() {
		_, err := handler.WaitForConnectMyComputerNodeJoin(ctx, &api.WaitForConnectMyComputerNodeJoinRequest{
			RootClusterUri: uri.NewClusterURI(profileName).String(),
		})
		waitForNodeJoinErr <- err
	}()

	// Start the new node.
	nodeConfig := newNodeConfig(t, "token", types.JoinMethodToken)
	nodeConfig.SetAuthServerAddress(pack.Root.Cluster.Config.Auth.ListenAddr)
	nodeConfig.DataDir = filepath.Join(agentsDir, profileName, "data")
	nodeConfig.Logger = libutils.NewSlogLoggerForTests()
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	_, err = nodeSvc.WaitForEventTimeout(10*time.Second, service.TeleportReadyEvent)
	require.NoError(t, err, "timeout waiting for node readiness")

	// Verify that WaitForConnectMyComputerNodeJoin returned with no errors.
	require.NoError(t, <-waitForNodeJoinErr)
}

func testDeleteConnectMyComputerNode(t *testing.T, pack *dbhelpers.DatabasePack) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	t.Cleanup(cancel)

	authServer := pack.Root.Cluster.Process.GetAuthServer()
	uuid := uuid.NewString()
	userName := fmt.Sprintf("user-cmc-%s", uuid)

	// Prepare a role with rules required to call DeleteConnectMyComputerNode.
	ruleWithAllowRules, err := types.NewRole(fmt.Sprintf("cmc-allow-rules-%v", uuid),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindNode, services.RW()),
				},
			},
		})
	require.NoError(t, err)
	userRoles := []types.Role{ruleWithAllowRules}

	_, err = auth.CreateUser(ctx, authServer, userName, userRoles...)
	require.NoError(t, err)

	// Log in as the new user.
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  pack.Root.Cluster.Process,
		Username: userName,
	})
	require.NoError(t, err)
	tc := mustLogin(t, userName, pack, creds)

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                tc.KeysDir,
		InsecureSkipVerify: tc.InsecureSkipVerify,
		HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
			return nil
		},
	})
	require.NoError(t, err)

	agentsDir := t.TempDir()
	daemonService, err := daemon.New(daemon.Config{
		Storage:        storage,
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      agentsDir,
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

	profileName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)

	// Start the new node.
	nodeConfig := newNodeConfig(t, "token", types.JoinMethodToken)
	nodeConfig.SetAuthServerAddress(pack.Root.Cluster.Config.Auth.ListenAddr)
	nodeConfig.DataDir = filepath.Join(agentsDir, profileName, "data")
	nodeConfig.Logger = libutils.NewSlogLoggerForTests()
	nodeSvc, err := service.NewTeleport(nodeConfig)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Start())
	t.Cleanup(func() { require.NoError(t, nodeSvc.Close()) })

	// waits for the node to be added
	require.Eventually(t, func() bool {
		_, err := authServer.GetNode(ctx, defaults.Namespace, nodeConfig.HostUUID)
		return err == nil
	}, time.Minute, time.Second, "waiting for node to join cluster")

	//  stop the node before attempting to remove it, to more closely resemble what's going to happen in production
	err = nodeSvc.Close()
	require.NoError(t, err)

	// test
	_, err = handler.DeleteConnectMyComputerNode(ctx, &api.DeleteConnectMyComputerNodeRequest{
		RootClusterUri: uri.NewClusterURI(profileName).String(),
	})
	require.NoError(t, err)

	// waits for the node to be deleted
	require.Eventually(t, func() bool {
		_, err := authServer.GetNode(ctx, defaults.Namespace, nodeConfig.HostUUID)
		return trace.IsNotFound(err)
	}, time.Minute, time.Second, "waiting for node to be deleted")
}

// testListDatabaseUsers adds a unique string under spec.allow.db_users of the role automatically
// given to a user by [dbhelpers.DatabasePack] and then checks if that string is returned when
// calling [handler.Handler.ListDatabaseUsers].
func testListDatabaseUsers(t *testing.T, pack *dbhelpers.DatabasePack) {
	ctx := context.Background()

	mustAddDBUserToUserRole := func(ctx context.Context, t *testing.T, cluster *helpers.TeleInstance, user, dbUser string) {
		t.Helper()
		authServer := cluster.Process.GetAuthServer()
		roleName := services.RoleNameForUser(user)
		role, err := authServer.GetRole(ctx, roleName)
		require.NoError(t, err)

		dbUsers := role.GetDatabaseUsers(types.Allow)
		dbUsers = append(dbUsers, dbUser)
		role.SetDatabaseUsers(types.Allow, dbUsers)
		_, err = authServer.UpdateRole(ctx, role)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			role, err := authServer.GetRole(ctx, roleName)
			if assert.NoError(collect, err) {
				assert.Equal(collect, dbUsers, role.GetDatabaseUsers(types.Allow))
			}
		}, 10*time.Second, 100*time.Millisecond)
	}

	mustUpdateUserRoles := func(ctx context.Context, t *testing.T, cluster *helpers.TeleInstance, userName string, roles []string) {
		t.Helper()
		authServer := cluster.Process.GetAuthServer()
		user, err := authServer.GetUser(ctx, userName, false /* withSecrets */)
		require.NoError(t, err)

		user.SetRoles(roles)
		_, err = authServer.UpdateUser(ctx, user)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			user, err := authServer.GetUser(ctx, userName, false /* withSecrets */)
			if assert.NoError(collect, err) {
				assert.Equal(collect, roles, user.GetRoles())
			}
		}, 10*time.Second, 100*time.Millisecond)
	}

	// Allow resource access requests to be created.
	currentModules := modules.GetModules()
	t.Cleanup(func() { modules.SetModules(currentModules) })
	modules.SetModules(&modules.TestModules{TestBuildType: modules.BuildEnterprise})

	rootClusterName, _, err := net.SplitHostPort(pack.Root.Cluster.Web)
	require.NoError(t, err)
	rootDatabaseURI := uri.NewClusterURI(rootClusterName).AppendDB(pack.Root.PostgresService.Name)
	leafDatabaseURI := uri.NewClusterURI(rootClusterName).AppendLeafCluster(pack.Leaf.Cluster.Secrets.SiteName).AppendDB(pack.Leaf.PostgresService.Name)

	rootDBUser := fmt.Sprintf("root-db-user-%s", uuid.NewString())
	leafDBUser := fmt.Sprintf("leaf-db-user-%s", uuid.NewString())
	leafDBUserWithAccessRequest := fmt.Sprintf("leaf-db-user-with-access-request-%s", uuid.NewString())

	rootUserName := pack.Root.User.GetName()
	leafUserName := pack.Leaf.User.GetName()
	rootRoleName := services.RoleNameForUser(rootUserName)

	tests := []struct {
		name                string
		dbURI               uri.ResourceURI
		wantDBUser          string
		prepareRole         func(ctx context.Context, t *testing.T)
		createAccessRequest func(ctx context.Context, t *testing.T) string
	}{
		{
			name:       "root cluster",
			dbURI:      rootDatabaseURI,
			wantDBUser: rootDBUser,
			prepareRole: func(ctx context.Context, t *testing.T) {
				mustAddDBUserToUserRole(ctx, t, pack.Root.Cluster, rootUserName, rootDBUser)
			},
		},
		{
			name:       "leaf cluster",
			dbURI:      leafDatabaseURI,
			wantDBUser: leafDBUser,
			prepareRole: func(ctx context.Context, t *testing.T) {
				mustAddDBUserToUserRole(ctx, t, pack.Leaf.Cluster, leafUserName, leafDBUser)
			},
		},
		{
			name:       "leaf cluster with resource access request",
			dbURI:      leafDatabaseURI,
			wantDBUser: leafDBUserWithAccessRequest,
			// Remove role from root-user and move it to search_as_roles.
			//
			// root-user has access to leafDatabaseURI through the user:root-user role which gets mapped
			// to a corresponding leaf cluster role.
			// We want to create a resource access request for that database. To do this, we need to
			// create a new role which lets root-user request the database.
			prepareRole: func(ctx context.Context, t *testing.T) {
				mustAddDBUserToUserRole(ctx, t, pack.Leaf.Cluster, leafUserName, leafDBUserWithAccessRequest)

				authServer := pack.Root.Cluster.Process.GetAuthServer()

				// Create new role that lets root-user request the database.
				requesterRole, err := types.NewRole(fmt.Sprintf("requester-%s", uuid.NewString()), types.RoleSpecV6{
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							SearchAsRoles: []string{rootRoleName},
						},
					},
				})
				require.NoError(t, err)
				requesterRole, err = authServer.CreateRole(ctx, requesterRole)
				require.NoError(t, err)

				user, err := authServer.GetUser(ctx, rootUserName, false /* withSecrets */)
				require.NoError(t, err)

				// Delete rootRoleName from roles, add requester role. Restore original role set after test
				// is done.
				currentRoles := user.GetRoles()
				t.Cleanup(func() { mustUpdateUserRoles(ctx, t, pack.Root.Cluster, rootUserName, currentRoles) })
				mustUpdateUserRoles(ctx, t, pack.Root.Cluster, rootUserName, []string{requesterRole.GetName()})
			},
			createAccessRequest: func(ctx context.Context, t *testing.T) string {
				req, err := services.NewAccessRequestWithResources(rootUserName, []string{rootRoleName}, []types.ResourceID{
					types.ResourceID{
						ClusterName: pack.Leaf.Cluster.Secrets.SiteName,
						Kind:        types.KindDatabase,
						Name:        pack.Leaf.PostgresService.Name,
					},
				})
				require.NoError(t, err)

				authServer := pack.Root.Cluster.Process.GetAuthServer()
				req, err = authServer.CreateAccessRequestV2(ctx, req, tlsca.Identity{})
				require.NoError(t, err)

				err = authServer.SetAccessRequestState(ctx, types.AccessRequestUpdate{
					RequestID: req.GetName(),
					State:     types.RequestState_APPROVED,
				})
				require.NoError(t, err)

				return req.GetName()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.prepareRole(ctx, t)
			var accessRequestID string
			if test.createAccessRequest != nil {
				accessRequestID = test.createAccessRequest(ctx, t)

				if accessRequestID == "" {
					require.FailNow(t, "createAccessRequest returned empty access request ID")
				}
			}

			creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
				Process:  pack.Root.Cluster.Process,
				Username: rootUserName,
			})
			require.NoError(t, err)

			tc := mustLogin(t, rootUserName, pack, creds)

			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                tc.KeysDir,
				InsecureSkipVerify: tc.InsecureSkipVerify,
				HardwareKeyPromptConstructor: func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
					return nil
				},
			})
			require.NoError(t, err)

			daemonService, err := daemon.New(daemon.Config{
				Storage:        storage,
				KubeconfigsDir: t.TempDir(),
				AgentsDir:      t.TempDir(),
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

			if accessRequestID != "" {
				_, err := handler.AssumeRole(ctx, &api.AssumeRoleRequest{
					RootClusterUri:   test.dbURI.GetRootClusterURI().String(),
					AccessRequestIds: []string{accessRequestID},
				})
				require.NoError(t, err)
			}

			res, err := handler.ListDatabaseUsers(ctx, &api.ListDatabaseUsersRequest{
				DbUri: test.dbURI.String(),
			})
			require.NoError(t, err)
			require.Contains(t, res.Users, test.wantDBUser)
		})
	}

}

// mustLogin logs in as the given user by completely skipping the actual login flow and saving valid
// certs to disk. clusters.Storage can then be pointed to tc.KeysDir and daemon.Service can act as
// if the user was successfully logged in.
//
// This is faster than going through the actual process, but keep in mind that it might skip some
// vital steps. It should be used only for tests which don't depend on complex user setup and do not
// reissue certs or modify them in some other way.
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

type setupUserMFAFunc func(t *testing.T, userName string, tshdEventsService *mockTSHDEventsService) client.WebauthnLoginFunc

type mockTSHDEventsService struct {
	api.UnimplementedTshdEventsServiceServer
	sendPendingHeadlessAuthenticationCount atomic.Uint32
	promptMFACallCount                     atomic.Uint32
}

func newMockTSHDEventsServiceServer(t *testing.T) (service *mockTSHDEventsService, addr string) {
	t.Helper()
	tshdEventsService := &mockTSHDEventsService{}

	ls, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	api.RegisterTshdEventsServiceServer(grpcServer, tshdEventsService)

	serveErr := make(chan error)
	go func() {
		serveErr <- grpcServer.Serve(ls)
	}()

	t.Cleanup(func() {
		grpcServer.GracefulStop()

		// For test cases that did not send any grpc calls, test may finish
		// before grpcServer.Serve is called and grpcServer.Serve will return
		// grpc.ErrServerStopped.
		err := <-serveErr
		if !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err)
		}
	})

	return tshdEventsService, ls.Addr().String()
}

func (c *mockTSHDEventsService) SendPendingHeadlessAuthentication(context.Context, *api.SendPendingHeadlessAuthenticationRequest) (*api.SendPendingHeadlessAuthenticationResponse, error) {
	c.sendPendingHeadlessAuthenticationCount.Add(1)
	return &api.SendPendingHeadlessAuthenticationResponse{}, nil
}

func (c *mockTSHDEventsService) PromptMFA(context.Context, *api.PromptMFARequest) (*api.PromptMFAResponse, error) {
	c.promptMFACallCount.Add(1)

	// PromptMFAResponse returns the TOTP code, so PromptMFA itself
	// needs to be implemented only once we need to test TOTP MFA.
	return nil, trace.NotImplemented("mockTSHDEventsService does not implement PromptMFA")
}
