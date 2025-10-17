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
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

func startApp(t *testing.T, appConfig *StartCmdConfig) {
	t.Helper()
	app, err := NewApp(appConfig, slog.Default())
	require.NoError(t, err)

	t.Cleanup(func() {
		app.Close()
	})

	integration.RunAndWaitReady(t, app)
}

// nonce is data produced to uniquely identify an event.
// The nonce is propagated from the event generator to the event checker.
// All events not matching the nonce are skipped.
type nonce any

// TestEventHandler is the refactored test function that no longer uses
// testify/suite.
func TestEventHandler(t *testing.T) {
	AuthHelper := &integration.MinimalAuthHelper{}
	var teleportConfig lib.TeleportConfig

	var err error
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// starts a Teleport auth service and creates the event forwarder
	// user and role.
	// Start the Teleport Auth server and get the admin client.
	adminClient := AuthHelper.StartServer(t)
	_, err = adminClient.Ping(ctx)
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

	eventHandlerRole, err = adminClient.CreateRole(ctx, eventHandlerRole)
	require.NoError(t, err)

	eventHandlerUser, err := types.NewUser("teleport-event-handler")
	require.NoError(t, err)

	eventHandlerUser.SetRoles([]string{eventHandlerRole.GetName()})
	eventHandlerUser, err = adminClient.CreateUser(ctx, eventHandlerUser)
	require.NoError(t, err)

	teleportConfig.Addr = AuthHelper.ServerAddr()
	teleportConfig.Identity = AuthHelper.SignIdentityForUser(t, ctx, eventHandlerUser)

	// Starts a fake fluentd server.
	err = logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	fakeFluentd := NewFakeFluentd(t)
	fakeFluentd.Start()
	t.Cleanup(fakeFluentd.Close)

	startTime := time.Now().Add(-time.Minute)

	appConfig := StartCmdConfig{
		TeleportConfig: TeleportConfig{
			TeleportAddr:         teleportConfig.Addr,
			TeleportIdentityFile: teleportConfig.Identity,
		},
		FluentdConfig: fakeFluentd.GetClientConfig(),
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

	appConfig.FluentdURL = fakeFluentd.GetURL()
	appConfig.FluentdSessionURL = appConfig.FluentdURL + "/session"

	t.Cleanup(func() {
		AuthHelper.Auth().Close()
	})

	// Original TestEvent
	tests := []struct {
		name          string
		generateEvent func(*testing.T, *client.Client) nonce
		checkEvent    func(*testing.T, string, nonce) bool
	}{
		{
			name: "new role",
			generateEvent: func(t *testing.T, c *client.Client) nonce {
				roleName := uuid.New().String()
				role, err := types.NewRole(roleName, types.RoleSpecV6{
					Options: types.RoleOptions{},
					Allow:   types.RoleConditions{},
					Deny:    types.RoleConditions{},
				})
				require.NoError(t, err)
				role, err = c.CreateRole(ctx, role)
				require.NoError(t, err)
				return role.GetName()
			},
			checkEvent: func(t *testing.T, event string, n nonce) bool {
				roleName, ok := n.(string)
				require.True(t, ok)
				return strings.Contains(event, roleName)
			},
		},
		{
			name: "new token",
			generateEvent: func(t *testing.T, c *client.Client) nonce {
				tokenName := uuid.New().String()
				token, err := types.NewProvisionToken(tokenName, types.SystemRoles{types.RoleNode}, time.Time{})
				require.NoError(t, err)
				err = c.CreateToken(ctx, token)
				require.NoError(t, err)
				return nil
			},
			checkEvent: func(t *testing.T, event string, _ nonce) bool {
				return strings.Contains(event, "join_token.create")
			},
		},
	}

	// Start the event forwarder
	startApp(t, &appConfig)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce := tt.generateEvent(t, adminClient)

			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			t.Cleanup(cancel)

			eventFound := false
			for !eventFound {
				event, err := fakeFluentd.GetMessage(waitCtx)
				require.NoError(t, err, "did not receive the event after 5 seconds")
				if tt.checkEvent(t, event, nonce) {
					t.Logf("Event matched: %s", event)
					eventFound = true
				} else {
					t.Logf("Event skipped: %s", event)
				}
			}
		})
	}
}
