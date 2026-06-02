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

package embeddedtbot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/oidc/fakeissuer"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedjoining "github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestBotJoinAuth(t *testing.T) {
	// Configure and start Teleport server
	clusterName := "root.example.com"
	ctx := t.Context()
	logger := logtest.NewLogger()
	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logger,
	})

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Version = "v3"

	require.NoError(t, teleportServer.CreateEx(t, nil, rcConf))
	auth := teleportServer.Process.GetAuthServer()

	require.NoError(t, teleportServer.Start())
	t.Cleanup(func() { _ = teleportServer.StopAll() })

	// Create operator role

	unrestricted := []string{"list", "create", "read", "update", "delete"}
	operatorRole, err := types.NewRole(
		testlib.ValidRandomResourceName("role-"),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindRole, unrestricted),
					types.NewRule(types.KindUser, unrestricted),
					types.NewRule(types.KindAuthConnector, unrestricted),
					types.NewRule(types.KindLoginRule, unrestricted),
					types.NewRule(types.KindToken, unrestricted),
					types.NewRule(types.KindOktaImportRule, unrestricted),
				},
			},
		})
	require.NoError(t, err)
	_, err = auth.CreateRole(ctx, operatorRole)
	require.NoError(t, err)

	// Create bot token

	operatorName := "operator"
	botName := "bot-" + operatorName
	tokenName := operatorName + "-token"
	token, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Time{},
		types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			JoinMethod: types.JoinMethodToken,
			BotName:    operatorName,
		})
	require.NoError(t, err)
	require.NoError(t, auth.CreateToken(ctx, token))

	// Create bot that can impersonate operator role and join with token
	// A bot is not a real resource, it is composed of two sub-resources:
	// - a bot role (that grants the ability to impersonate the desired role)
	// - a bot user

	botRole, err := createBotRole(operatorName, botName, []string{operatorRole.GetName()})
	require.NoError(t, err)
	_, err = auth.CreateRole(ctx, botRole)
	require.NoError(t, err)

	botUser, err := createBotUser(operatorName, botName, map[string][]string{})
	require.NoError(t, err)
	_, err = auth.Services.CreateUser(ctx, botUser)
	require.NoError(t, err)

	// Configure the bot to join the auth server
	authAddr, err := teleportServer.Process.AuthAddr()
	require.NoError(t, err)
	botConfig := &BotConfig{
		Onboarding: onboarding.Config{
			TokenValue: tokenName,
			JoinMethod: types.JoinMethodToken,
		},
		AuthServer: authAddr.Addr,
		CredentialLifetime: bot.CredentialLifetime{
			TTL:             defaultCertificateTTL,
			RenewalInterval: defaultRenewalInterval,
		},
	}
	bot, err := New(botConfig, logger)
	require.NoError(t, err)
	pong, err := bot.Preflight(ctx)
	require.NoError(t, err)
	require.Equal(t, clusterName, pong.ClusterName)

	botClient, err := bot.StartAndWaitForClient(ctx, 10*time.Second)
	require.NoError(t, err)
	botPong, err := botClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, clusterName, botPong.ClusterName)
}

func createBotRole(botName string, resourceName string, roleRequests []string) (types.Role, error) {
	role, err := types.NewRole(resourceName, types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxSessionTTL: types.Duration(12 * time.Hour),
		},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				// Bots read certificate authorities to watch for CA rotations
				types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
			},
			Impersonate: &types.ImpersonateConditions{
				Roles: roleRequests,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	meta := role.GetMetadata()
	meta.Description = fmt.Sprintf("Role for bot %s", botName)
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels[types.BotLabel] = botName
	role.SetMetadata(meta)
	return role, nil
}

// createBotUser creates a new backing User for bot use. A role with a
// matching name must already exist (see createBotRole).
func createBotUser(
	botName string,
	resourceName string,
	traits wrappers.Traits,
) (types.User, error) {
	user, err := types.NewUser(resourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles([]string{resourceName})

	metadata := user.GetMetadata()
	metadata.Labels = map[string]string{
		types.BotLabel:           botName,
		types.BotGenerationLabel: "0",
	}
	user.SetMetadata(metadata)
	user.SetTraits(traits)
	return user, nil
}

func TestScopedBotJoinAuth(t *testing.T) {
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
		testRootScope = "/my-scope"
		testBotRole   = "scoped-role"
		testBotName   = "test-bot"
		testTokenName = "test-token"
	)

	// Role impersonated by the bot.
	botRole := &scopedaccessv1.ScopedRole{
		Kind:    scopedaccess.KindScopedRole,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: testBotRole,
		},
		Scope: testRootScope,
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{testRootScope},
			Rules: []*scopedaccessv1.ScopedRule{
				// The role can read scoped roles. We will use this to validate its permissions later.
				{
					Resources: []string{scopedaccess.KindScopedRole},
					Verbs:     []string{types.VerbReadNoSecrets},
				},
			},
		},
	}
	require.NoError(t, scopedaccess.StrongValidateRole(botRole), "malformed role, this is a bug in the test fixture")
	_, err = adminClient.ScopedAccessServiceClient().CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: botRole,
	})
	require.NoError(t, err)

	// Create the bot.
	botResource := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: testBotName,
		},
		Spec:  &machineidv1.BotSpec{},
		Scope: testRootScope,
	}
	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
		Bot: botResource,
	})
	require.NoError(t, err)

	// Assign role to bot
	roleAssignment := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindDynamic,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: testBotRole,
		},
		Scope: testRootScope,
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  testBotRole,
					Scope: testRootScope,
				},
			},
			BotName:  testBotName,
			BotScope: testRootScope,
		},
	}
	_, err = adminClient.ScopedAccessServiceClient().CreateScopedRoleAssignment(
		ctx,
		&scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: roleAssignment,
		})
	require.NoError(t, err)

	// Test setup: Wait for the role assignment to enter the auth server cache
	// else the bot will race against the cache, joining will fail and the test wil be flaky.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := adminClient.ScopedAccessServiceClient().GetScopedRoleAssignment(
			ctx,
			&scopedaccessv1.GetScopedRoleAssignmentRequest{
				Name:    testBotRole,
				SubKind: scopedaccess.SubKindDynamic,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Assignment)
	}, 5*time.Second, 100*time.Millisecond, "expected role assignment to enter the auth cache")

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
	token := &scopedjoiningv1.ScopedToken{
		Kind:     scopedaccess.KindScopedToken,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: testTokenName},
		Scope:    testRootScope,
		Spec: &scopedjoiningv1.ScopedTokenSpec{
			JoinMethod: string(types.JoinMethodKubernetes),
			UsageMode:  scopedjoining.TokenUsageModeBot,
			Kubernetes: &scopedjoiningv1.Kubernetes{
				Allow: []*scopedjoiningv1.Kubernetes_Rule{
					{
						ServiceAccount: testNamespace + ":" + testServiceAccount,
					},
				},
				Type:       string(types.KubernetesJoinTypeStaticJWKS),
				StaticJwks: &scopedjoiningv1.Kubernetes_StaticJWKSConfig{Jwks: jwks},
			},
			BotName:  testBotName,
			BotScope: testRootScope,
			Roles:    []string{string(types.RoleBot)},
		},
	}
	// Somehow the admin client interface doesn't expose CreateScopedToken ¯\_(ツ)_/¯
	// We use the auth server directly to create the scoped token.
	_, err = teleportServer.Process.GetAuthServer().CreateScopedToken(ctx, &scopedjoiningv1.CreateScopedTokenRequest{
		Token: token,
	})
	require.NoError(t, err)

	// Test setup: Configure the bot to join the auth using the scoped join token and the JWT.
	authAddr, err := teleportServer.Process.AuthAddr()
	require.NoError(t, err)
	botConfig := &BotConfig{
		AuthServer: authAddr.Addr,
		Onboarding: onboarding.Config{
			TokenValue: testTokenName,
			JoinMethod: types.JoinMethodKubernetes,
			Kubernetes: onboarding.KubernetesOnboardingConfig{
				TokenPath: tokenPath,
			},
		},
		CredentialLifetime: bot.CredentialLifetime{
			TTL:             defaultCertificateTTL,
			RenewalInterval: defaultRenewalInterval,
		},
		Scoped: true,
	}

	// Test execution: Run the bot, make it join the cluster and yield credentials.
	embeddedBot, err := New(botConfig, logger)
	require.NoError(t, err)
	pong, err := embeddedBot.Preflight(ctx)
	require.NoError(t, err)
	require.Equal(t, clusterName, pong.ClusterName)

	botClient, err := embeddedBot.StartAndWaitForClient(ctx, 10*time.Second)
	require.NoError(t, err)

	// Test validation: check if the bot produced a valid client.
	botPong, err := botClient.Ping(ctx)
	require.NoError(t, err)
	require.Equal(t, clusterName, botPong.ClusterName)

	_, err = botClient.ScopedAccessServiceClient().GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: testBotRole,
	})
	require.NoError(t, err)
}
