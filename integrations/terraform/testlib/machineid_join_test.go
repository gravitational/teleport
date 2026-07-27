/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/wait"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/oidc/fakeissuer"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedjoining "github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"

	"github.com/gravitational/teleport/integrations/terraform/provider"
)

func TestTerraformJoin(t *testing.T) {
	require.NoError(t, os.Setenv("TF_ACC", "true"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Test setup: start a Telpeort auth server
	authHelper := &integration.MinimalAuthHelper{}
	clt := authHelper.StartServer(t)

	var err error
	// Test setup: create the terraform role
	tfRole := services.NewPresetTerraformProviderRole()
	tfRole, err = clt.CreateRole(ctx, tfRole)
	require.NoError(t, err)

	// Test setup: create a fake Kubernetes signer that will allow us to use the kubernetes/jwks join method
	clock := clockwork.NewRealClock()
	signer, err := fakeissuer.NewKubernetesSigner(clock)
	require.NoError(t, err)

	jwks, err := signer.GetMarshaledJWKS()
	require.NoError(t, err)

	// Test setup: create a token and a bot that can join the cluster with JWT signed by our fake Kubernetes signer
	testBotName := "testBot"
	testTokenName := "testToken"
	fakeNamespace := "test-namespace"
	fakeServiceAccount := "test-service-account"
	token, err := types.NewProvisionTokenFromSpec(
		testTokenName,
		clock.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodKubernetes,
			BotName:    testBotName,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: fmt.Sprintf("%s:%s", fakeNamespace, fakeServiceAccount),
					},
				},
				Type: types.KubernetesJoinTypeStaticJWKS,
				StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
					JWKS: jwks,
				},
			},
		})
	require.NoError(t, err)
	err = clt.CreateToken(ctx, token)
	require.NoError(t, err)

	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: testBotName,
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{tfRole.GetName()},
		},
	}
	_, err = clt.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: bot})
	require.NoError(t, err)

	// Test setup: sign a Kube JWT for our bot to join the cluster
	// We sign the token, write it to a temporary file, and point the embedded tbot to it
	// with an environment variable.
	pong, err := clt.Ping(ctx)
	require.NoError(t, err)
	clusterName := pong.ClusterName
	jwt, err := signer.SignServiceAccountJWT("pod-name-doesnt-matter", fakeNamespace, fakeServiceAccount, clusterName)
	require.NoError(t, err)

	tempDir := t.TempDir()
	jwtPath := filepath.Join(tempDir, "token")
	require.NoError(t, os.WriteFile(jwtPath, []byte(jwt), 0600))

	// Test setup: craft a Terraform provider configuration
	terraformConfig := fmt.Sprintf(`
		provider "teleport" {
			addr = %q
			join_token = %q
			join_method = %q
			retry_base_duration = "900ms"
			retry_cap_duration = "4s"
			retry_max_tries = "12"
			kubernetes_token_path = %q
		}
	`, authHelper.ServerAddr(), testTokenName, types.JoinMethodKubernetes, jwtPath)

	terraformProvider := provider.New()
	terraformProviders := make(map[string]func() (tfprotov6.ProviderServer, error))
	terraformProviders["teleport"] = func() (tfprotov6.ProviderServer, error) {
		// Terraform configures provider on every test step, but does not clean up previous one, which produces
		// to "too many open files" at some point.
		//
		// With this statement we try to forcefully close previously opened client, which stays cached in
		// the provider variable.
		p, ok := terraformProvider.(*provider.Provider)
		require.True(t, ok)
		require.NoError(t, p.Close())
		return providerserver.NewProtocol6(terraformProvider)(), nil
	}

	// Test execution: apply a TF resource with the provider joining via MachineID
	dummyResource, err := fixtures.ReadFile(filepath.Join("fixtures", "app_0_create.tf"))
	require.NoError(t, err)
	testConfig := terraformConfig + "\n" + string(dummyResource)
	name := "teleport_app.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: testConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func TestTerraformJoinViaProxy(t *testing.T) {
	require.NoError(t, os.Setenv("TF_ACC", "true"))

	// Test setup: start a full Teleport process including a proxy.
	process, err := testenv.NewTeleportProcess(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

	// Test setup: get the terraform role
	tfRole, err := clt.GetRole(t.Context(), teleport.PresetTerraformProviderRoleName)
	require.NoError(t, err)

	// Test setup: create a fake Kubernetes signer that will allow us to use the kubernetes/jwks join method
	clock := clockwork.NewRealClock()
	signer, err := fakeissuer.NewKubernetesSigner(clock)
	require.NoError(t, err)

	jwks, err := signer.GetMarshaledJWKS()
	require.NoError(t, err)

	// Test setup: create a token and a bot that can join the cluster with JWT signed by our fake Kubernetes signer
	testBotName := "testBot"
	testTokenName := "testToken"
	fakeNamespace := "test-namespace"
	fakeServiceAccount := "test-service-account"
	token, err := types.NewProvisionTokenFromSpec(
		testTokenName,
		clock.Now().Add(time.Hour),
		types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodKubernetes,
			BotName:    testBotName,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: fmt.Sprintf("%s:%s", fakeNamespace, fakeServiceAccount),
					},
				},
				Type: types.KubernetesJoinTypeStaticJWKS,
				StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
					JWKS: jwks,
				},
			},
		})
	require.NoError(t, err)
	err = clt.CreateToken(t.Context(), token)
	require.NoError(t, err)

	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: testBotName,
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{tfRole.GetName()},
		},
	}
	_, err = clt.BotServiceClient().CreateBot(t.Context(), &machineidv1.CreateBotRequest{Bot: bot})
	require.NoError(t, err)

	// Test setup: sign a Kube JWT for our bot to join the cluster
	// We sign the token, write it to a temporary file, and point the embedded tbot to it
	// with an environment variable.
	pong, err := clt.Ping(t.Context())
	require.NoError(t, err)
	clusterName := pong.ClusterName
	jwt, err := signer.SignServiceAccountJWT("pod-name-doesnt-matter", fakeNamespace, fakeServiceAccount, clusterName)
	require.NoError(t, err)

	tempDir := t.TempDir()
	jwtPath := filepath.Join(tempDir, "token")
	require.NoError(t, os.WriteFile(jwtPath, []byte(jwt), 0600))

	// Test setup: craft a Terraform provider configuration
	proxyAddr, err := process.ProxyTunnelAddr()
	require.NoError(t, err)

	terraformConfig := fmt.Sprintf(`
		provider "teleport" {
			addr = %q
			join_token = %q
			join_method = %q
			insecure = true
			retry_base_duration = "900ms"
			retry_cap_duration = "4s"
			retry_max_tries = "12"
			kubernetes_token_path = %q
		}
	`, proxyAddr, testTokenName, types.JoinMethodKubernetes, jwtPath)

	terraformProvider := provider.New()
	terraformProviders := make(map[string]func() (tfprotov6.ProviderServer, error))
	terraformProviders["teleport"] = func() (tfprotov6.ProviderServer, error) {
		// Terraform configures provider on every test step, but does not clean up previous one, which produces
		// to "too many open files" at some point.
		//
		// With this statement we try to forcefully close previously opened client, which stays cached in
		// the provider variable.
		p, ok := terraformProvider.(*provider.Provider)
		require.True(t, ok)
		require.NoError(t, p.Close())
		return providerserver.NewProtocol6(terraformProvider)(), nil
	}

	// Test execution: apply a TF resource with the provider joining via MachineID
	dummyResource, err := fixtures.ReadFile(filepath.Join("fixtures", "app_0_create.tf"))
	require.NoError(t, err)
	testConfig := terraformConfig + "\n" + string(dummyResource)
	name := "teleport_app.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: terraformProviders,
		Steps: []resource.TestStep{
			{
				Config: testConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(name, "kind", "app"),
					resource.TestCheckResourceAttr(name, "spec.uri", "localhost:3000"),
				),
			},
		},
	})
}

func TestTerraformJoinScoped(t *testing.T) {
	require.NoError(t, os.Setenv("TF_ACC", "true"))

	t.Parallel()
	// Test setup: Configure and start Teleport server
	clusterName := "root.example.com"
	logger := logtest.NewLogger()
	ctx := t.Context()

	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logger,
	})

	// Test setup: building an admin user and role that will be used to create fixtures.
	const adminUserName = "admin"
	adminRole, err := types.NewRole("admin", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUser, types.KindRole, scopedaccess.KindScopedToken, scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment, types.KindBot},
					Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbDelete, types.VerbList, types.VerbUpdate},
				},
			},
		},
		Deny: types.RoleConditions{},
	})
	require.NoError(t, err)
	teleportServer.AddUserWithRole(adminUserName, adminRole)

	// Test setup: starting the Teleport instance
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.ScopesFeatures = scopes.Features{Enabled: true}
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Version = "v3"

	require.NoError(t, teleportServer.CreateEx(t, nil, rcConf))

	require.NoError(t, teleportServer.Start())
	t.Cleanup(func() { _ = teleportServer.StopAll() })

	// Test setup: Building an admin client.
	// Note: this is very convoluted but the bot service is not stored in the auth server struct and cannot be accessed
	// without a rpc client.
	adminCreds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  teleportServer.Process,
		Username: "admin",
	})
	require.NoError(t, err)
	clt, err := teleportServer.NewClientWithCreds(
		helpers.ClientConfig{
			TeleportUser: adminUserName,
			Login:        adminUserName,
			Cluster:      clusterName,
		},
		*adminCreds,
	)
	require.NoError(t, err)

	clusterClient, err := clt.ConnectToCluster(ctx)
	require.NoError(t, err)
	adminClient, err := clusterClient.ConnectToCluster(ctx, clusterName)
	require.NoError(t, err)
	// Validate that the admin client works.
	adminPong, err := adminClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, clusterName, adminPong.ClusterName)

	// Test setup: Create all the scoped resources describing a scoped bot, and its RBAC.
	const (
		// must match the `scoped_token_0_create.tf` fixture's scope
		testRootScope = "/staging"
		testBotRole   = "scoped-role"
		testBotName   = "test-bot"
		testTokenName = "test-token"
	)

	// Role impersonated by the bot.
	botRole := scopedaccessv1.ScopedRole_builder{
		Kind:    scopedaccess.KindScopedRole,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: testBotRole,
		}.Build(),
		Scope: testRootScope,
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{testRootScope},
			Rules: []*scopedaccessv1.ScopedRule{
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{scopedaccess.KindScopedToken},
					Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbUpdate, types.VerbCreate, types.VerbDelete, types.VerbRead},
				}.Build(),
			},
		}.Build(),
	}.Build()
	require.NoError(t, scopedaccess.StrongValidateRole(botRole), "malformed role, this is a bug in the test fixture")
	_, err = adminClient.ScopedAccessServiceClient().CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: botRole,
	}.Build())
	require.NoError(t, err)

	// Create the bot.
	botResource := machineidv1.Bot_builder{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: testBotName,
		}.Build(),
		Spec:  &machineidv1.BotSpec{},
		Scope: testRootScope,
	}.Build()
	_, err = adminClient.BotServiceClient().CreateBot(ctx, machineidv1.CreateBotRequest_builder{
		Bot: botResource,
	}.Build())
	require.NoError(t, err)

	// Assign role to bot
	roleAssignment := scopedaccessv1.ScopedRoleAssignment_builder{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: testBotRole,
		}.Build(),
		Scope: testRootScope,
		Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
			Assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  scopes.QualifiedName{Name: testBotRole, Scope: testRootScope}.String(),
					Scope: testRootScope,
				}.Build(),
			},
			Bot: scopes.QualifiedName{Scope: testRootScope, Name: testBotName}.String(),
		}.Build(),
	}.Build()
	_, err = adminClient.ScopedAccessServiceClient().CreateScopedRoleAssignment(
		ctx,
		scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: roleAssignment,
		}.Build())
	require.NoError(t, err)

	// Test setup: Wait for the role assignment to enter the auth server cache
	// else the bot will race against the cache, joining will fail and the test wil be flaky.
	resp, err := wait.UntilFound(ctx, func(ctx context.Context) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
		return adminClient.ScopedAccessServiceClient().GetScopedRoleAssignment(
			ctx,
			scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
				Name:    testBotRole,
				Scope:   testRootScope,
				SubKind: scopedaccess.SubKindDynamic,
			}.Build(),
		)
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.GetAssignment())

	// Test setup: Create a fake Kubernetes JWKS signer that will be used by the bot to join the cluster.
	const (
		testPod            = "my-pod"
		testNamespace      = "my-namespace"
		testServiceAccount = "my-service-account"
	)
	signer, err := fakeissuer.NewKubernetesSigner(clockwork.NewRealClock())
	require.NoError(t, err)
	jwks, err := signer.GetMarshaledJWKS()
	require.NoError(t, err)
	jwt, err := signer.SignServiceAccountJWT(testPod, testNamespace, testServiceAccount, clusterName)
	require.NoError(t, err)

	tokenDir := t.TempDir()
	tokenPath := filepath.Join(tokenDir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte(jwt), 0600))

	// Test setup: Create the scoped token allowing the Bot to join with a JWT emitted by the Kube signer.
	token := scopedjoiningv1.ScopedToken_builder{
		Kind:     scopedaccess.KindScopedToken,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: testTokenName}.Build(),
		Scope:    testRootScope,
		Spec: scopedjoiningv1.ScopedTokenSpec_builder{
			JoinMethod: string(types.JoinMethodKubernetes),
			UsageMode:  scopedjoining.TokenUsageModeBot,
			Kubernetes: scopedjoiningv1.Kubernetes_builder{
				Allow: []*scopedjoiningv1.Kubernetes_Rule{
					scopedjoiningv1.Kubernetes_Rule_builder{
						ServiceAccount: testNamespace + ":" + testServiceAccount,
					}.Build(),
				},
				Type:       string(types.KubernetesJoinTypeStaticJWKS),
				StaticJwks: scopedjoiningv1.Kubernetes_StaticJWKSConfig_builder{Jwks: jwks}.Build(),
			}.Build(),
			Bot:   scopes.QualifiedName{Scope: testRootScope, Name: testBotName}.String(),
			Roles: []string{string(types.RoleBot)},
		}.Build(),
	}.Build()
	// Somehow the admin client interface doesn't expose CreateScopedToken ¯\_(ツ)_/¯
	// We use the auth server directly to create the scoped token.
	_, err = teleportServer.Process.GetAuthServer().CreateScopedToken(ctx, scopedjoiningv1.CreateScopedTokenRequest_builder{
		Token: token,
	}.Build())
	require.NoError(t, err)

	authAddr, err := teleportServer.Process.AuthAddr()
	require.NoError(t, err)
	proxyAddr, err := teleportServer.Process.ProxyWebAddr()
	require.NoError(t, err)

	tests := []struct {
		name string
		addr *utils.NetAddr
	}{
		{
			name: "join via auth",
			addr: authAddr,
		},
		{
			name: "join via proxy",
			addr: proxyAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terraformConfig := fmt.Sprintf(`
		provider "teleport" {
			addr = %q
			join_token = %q
			join_method = %q
			insecure = true
			retry_base_duration = "900ms"
			retry_cap_duration = "4s"
			retry_max_tries = "12"
			kubernetes_token_path = %q
			scoped = true
		}
	`, tt.addr, testTokenName, types.JoinMethodKubernetes, tokenPath)

			terraformProvider := provider.New()
			terraformProviders := make(map[string]func() (tfprotov6.ProviderServer, error))
			terraformProviders["teleport"] = func() (tfprotov6.ProviderServer, error) {
				// Terraform configures provider on every test step, but does not clean up previous one, which produces
				// to "too many open files" at some point.
				//
				// With this statement we try to forcefully close previously opened client, which stays cached in
				// the provider variable.
				p, ok := terraformProvider.(*provider.Provider)
				require.True(t, ok)
				require.NoError(t, p.Close())
				return providerserver.NewProtocol6(terraformProvider)(), nil
			}

			// Test execution: apply a TF resource with the provider joining via MachineID
			dummyResource, err := fixtures.ReadFile(filepath.Join("fixtures", "scoped_token_0_create.tf"))
			require.NoError(t, err)
			testConfig := terraformConfig + "\n" + string(dummyResource)
			name := "teleport_scoped_token.test"

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: terraformProviders,
				Steps: []resource.TestStep{
					{
						Config: testConfig,
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(name, "kind", "scoped_token"),
							resource.TestCheckResourceAttr(name, "spec.join_method", "token"),
						),
					},
				},
			})
		})
	}
}
