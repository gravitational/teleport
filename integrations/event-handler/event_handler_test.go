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
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

type EventHandlerSuite struct {
	suite.Suite
	AuthHelper  integration.AuthHelper
	appConfig   StartCmdConfig
	fakeFluentd *FakeFluentd

	client         *client.Client
	teleportConfig lib.TeleportConfig
}

func TestEventHandler(t *testing.T) {
	suite.Run(t, &EventHandlerSuite{
		AuthHelper: &integration.MinimalAuthHelper{},
	})
}

// SetupSuite starts a Teleport auth service and creates the event forwarder
// user and role. This runs a once for the whole suite.
func (s *EventHandlerSuite) SetupSuite() {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

	// Start the Teleport Auth server and get the admin client.
	s.client = s.AuthHelper.StartServer(s.T())
	_, err = s.client.Ping(ctx)
	require.NoError(s.T(), err)

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
	require.NoError(s.T(), err)

	eventHandlerRole, err = s.client.CreateRole(ctx, eventHandlerRole)
	require.NoError(s.T(), err)

	eventHandlerUser, err := types.NewUser("teleport-event-handler")
	require.NoError(s.T(), err)

	eventHandlerUser.SetRoles([]string{eventHandlerRole.GetName()})
	eventHandlerUser, err = s.client.CreateUser(ctx, eventHandlerUser)
	require.NoError(s.T(), err)

	s.teleportConfig.Addr = s.AuthHelper.ServerAddr()
	s.teleportConfig.Identity = s.AuthHelper.SignIdentityForUser(s.T(), ctx, eventHandlerUser)
}

// SetupTest starts a fake fluentd server.
// This runs before every test from the suite.
func (s *EventHandlerSuite) SetupTest() {
	t := s.T()

	// Start fake fluentd
	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)

	s.fakeFluentd = NewFakeFluentd(t)
	s.fakeFluentd.Start()
	t.Cleanup(s.fakeFluentd.Close)

	startTime := time.Now().Add(-time.Minute)

	conf := StartCmdConfig{
		TeleportConfig: TeleportConfig{
			TeleportAddr:         s.teleportConfig.Addr,
			TeleportIdentityFile: s.teleportConfig.Identity,
		},
		FluentdConfig: s.fakeFluentd.GetClientConfig(),
		IngestConfig: IngestConfig{
			StorageDir:       t.TempDir(),
			Timeout:          time.Second,
			BatchSize:        100,
			Concurrency:      5,
			StartTime:        &startTime,
			SkipSessionTypes: map[string]struct{}{"print": {}},
			WindowSize:       time.Hour * 24,
		},
	}

	conf.FluentdURL = s.fakeFluentd.GetURL()
	conf.FluentdSessionURL = conf.FluentdURL + "/session"

	s.appConfig = conf
}

func (s *EventHandlerSuite) startApp() {
	s.T().Helper()
	t := s.T()
	t.Helper()

	app, err := NewApp(&s.appConfig, slog.Default())
	require.NoError(t, err)

	integration.RunAndWaitReady(s.T(), app)
}

// nonce is data produced to uniquely identify an event.
// The nonce is propagated from the event generator to the event checker.
// All events not matching the nonce are skipped.
type nonce any

func (s *EventHandlerSuite) TestEvent() {
	ctx, cancel := context.WithCancel(context.Background())
	s.T().Cleanup(cancel)

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
	s.startApp()

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			nonce := tt.generateEvent(t, s.client)

			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			s.T().Cleanup(cancel)

			eventFound := false
			for !eventFound {
				event, err := s.fakeFluentd.GetMessage(waitCtx)
				require.NoError(s.T(), err, "did not receive the event after 5 seconds")
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
