/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

func TestEventHandler(t *testing.T) {
	authHelper := &integration.MinimalAuthHelper{}

	// starts a Teleport auth service and creates the event forwarder
	// user and role.
	// Start the Teleport Auth server and get the admin client.
	adminClient := authHelper.StartServer(t)
	t.Cleanup(func() { require.NoError(t, authHelper.Auth().Close()) })
	_, err := adminClient.Ping(t.Context())
	require.NoError(t, err)

	eventHandlerRole, err := types.NewRole("teleport-event-handler", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindEvent, types.KindSession},
					Verbs:     []string{types.VerbList, types.VerbRead},
				},
			},
		},
		Deny: types.RoleConditions{},
	})
	require.NoError(t, err)

	eventHandlerRole, err = adminClient.CreateRole(t.Context(), eventHandlerRole)
	require.NoError(t, err)

	eventHandlerUser, err := types.NewUser("teleport-event-handler")
	require.NoError(t, err)

	eventHandlerUser.SetRoles([]string{eventHandlerRole.GetName()})
	eventHandlerUser, err = adminClient.CreateUser(t.Context(), eventHandlerUser)
	require.NoError(t, err)

	// Starts a fake fluentd server.
	err = logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	fakeFluentd := NewFakeFluentd(t)
	fakeFluentd.Start()
	t.Cleanup(fakeFluentd.Close)

	startTime := time.Now().Add(-time.Minute)

	fluentdConfig := fakeFluentd.GetClientConfig()
	fluentdConfig.FluentdURL = fakeFluentd.GetURL()
	fluentdConfig.FluentdSessionURL = fluentdConfig.FluentdURL + "/session"

	appConfig := StartCmdConfig{
		TeleportConfig: TeleportConfig{
			TeleportAddr:         authHelper.ServerAddr(),
			TeleportIdentityFile: authHelper.SignIdentityForUser(t, t.Context(), eventHandlerUser),
		},
		FluentdConfig: fluentdConfig,
		IngestConfig: IngestConfig{
			StorageDir:       t.TempDir(),
			Timeout:          time.Second,
			BatchSize:        100,
			Concurrency:      5,
			StartTime:        &startTime,
			SkipSessionTypes: map[string]struct{}{"print": {}, "desktop.recording": {}},
			WindowSize:       time.Hour * 24,
		},
	}

	// Original TestEvent
	tests := []struct {
		name          string
		generateEvent func(*testing.T, *client.Client) any
		checkEvent    func(*testing.T, string, any) bool
	}{
		{
			name: "new role",
			generateEvent: func(t *testing.T, c *client.Client) any {
				roleName := uuid.New().String()
				role, err := types.NewRole(roleName, types.RoleSpecV6{
					Options: types.RoleOptions{},
					Allow:   types.RoleConditions{},
					Deny:    types.RoleConditions{},
				})
				require.NoError(t, err)
				role, err = c.CreateRole(t.Context(), role)
				require.NoError(t, err)
				return role.GetName()
			},
			checkEvent: func(t *testing.T, event string, n any) bool {
				roleName, ok := n.(string)
				require.True(t, ok)
				return strings.Contains(event, roleName)
			},
		},
		{
			name: "new token",
			generateEvent: func(t *testing.T, c *client.Client) any {
				tokenName := uuid.New().String()
				token, err := types.NewProvisionToken(tokenName, types.SystemRoles{types.RoleNode}, time.Time{})
				require.NoError(t, err)
				err = c.CreateToken(t.Context(), token)
				require.NoError(t, err)
				return nil
			},
			checkEvent: func(t *testing.T, event string, _ any) bool {
				return strings.Contains(event, "join_token.create")
			},
		},
	}

	// Start the event forwarder
	app, err := NewApp(&appConfig, slog.Default())
	require.NoError(t, err)

	t.Cleanup(app.Close)

	integration.RunAndWaitReady(t, app)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			any := tt.generateEvent(t, adminClient)

			waitCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			for eventFound := false; !eventFound; {
				event, err := fakeFluentd.GetMessage(waitCtx)
				require.NoError(t, err, "did not receive the event after 5 seconds")
				if tt.checkEvent(t, event, any) {
					t.Logf("Event matched: %s", event)
					eventFound = true
				} else {
					t.Logf("Event skipped: %s", event)
				}
			}
		})
	}
}
