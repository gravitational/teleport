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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

func TestBotJoinAuth(t *testing.T) {
	// Configure and start Teleport server
	clusterName := "root.example.com"
	ctx := context.Background()
	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Log:         utils.NewLoggerForTests(),
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
	_, err = auth.Services.Identity.CreateUser(ctx, botUser)
	require.NoError(t, err)

	// Configure the bot to join the auth server
	authAddr, err := teleportServer.Process.AuthAddr()
	require.NoError(t, err)
	botConfig := &BotConfig{
		Onboarding: config.OnboardingConfig{
			TokenValue: tokenName,
			JoinMethod: types.JoinMethodToken,
		},
		AuthServer: authAddr.Addr,
		CertificateLifetime: config.CertificateLifetime{
			TTL:             defaultCertificateTTL,
			RenewalInterval: defaultRenewalInterval,
		},
		Oneshot: true,
		Debug:   true,
	}
	bot, err := New(botConfig)
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
	meta.Description = fmt.Sprintf("Automatically generated role for bot %s", botName)
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
