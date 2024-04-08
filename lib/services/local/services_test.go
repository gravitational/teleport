/*
Copyright 2015-2019 Gravitational, Inc.

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

package local

import (
	"context"
	"os"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type servicesContext struct {
	bk    backend.Backend
	suite *suite.ServicesTestSuite
}

func setupServicesContext(ctx context.Context, t *testing.T) *servicesContext {
	var tt servicesContext
	t.Cleanup(func() { tt.Close() })

	clock := clockwork.NewFakeClock()

	var err error
	tt.bk, err = memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	configService, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	eventsService := NewEventsService(tt.bk)
	presenceService := NewPresenceService(tt.bk)

	tt.suite = &suite.ServicesTestSuite{
		CAS:           NewCAService(tt.bk),
		PresenceS:     presenceService,
		ProvisioningS: NewProvisioningService(tt.bk),
		WebS:          NewTestIdentityService(tt.bk),
		Access:        NewAccessService(tt.bk),
		EventsS:       eventsService,
		ChangesC:      make(chan interface{}),
		ConfigS:       configService,
		RestrictionsS: NewRestrictionsService(tt.bk),
		Clock:         clock,
	}

	return &tt
}

func (tt *servicesContext) Close() error {
	return tt.bk.Close()
}

func TestCRUD(t *testing.T) {
	tt := setupServicesContext(context.Background(), t)

	t.Run("TestUserCACRUD", tt.suite.CertAuthCRUD)
	t.Run("TestServerCRUD", tt.suite.ServerCRUD)
	t.Run("TestAppServerCRUD", tt.suite.AppServerCRUD)
	t.Run("TestReverseTunnelsCRUD", tt.suite.ReverseTunnelsCRUD)
	t.Run("TestUsersCRUD", tt.suite.UsersCRUD)
	t.Run("TestUsersExpiry", tt.suite.UsersExpiry)
	t.Run("TestLoginAttempts", tt.suite.LoginAttempts)
	t.Run("TestPasswordHashCRUD", tt.suite.PasswordHashCRUD)
	t.Run("TestWebSessionCRUD", tt.suite.WebSessionCRUD)
	t.Run("TestToken", tt.suite.TokenCRUD)
	t.Run("TestRoles", tt.suite.RolesCRUD)
	t.Run("TestSAMLCRUD", tt.suite.SAMLCRUD)
	t.Run("TestTunnelConnectionsCRUD", tt.suite.TunnelConnectionsCRUD)
	t.Run("TestGithubConnectorCRUD", tt.suite.GithubConnectorCRUD)
	t.Run("TestRemoteClustersCRUD", tt.suite.RemoteClustersCRUD)
	t.Run("TestEvents", tt.suite.Events)
	t.Run("TestEventsClusterConfig", tt.suite.EventsClusterConfig)
	t.Run("TestNetworkRestrictions", func(t *testing.T) { tt.suite.NetworkRestrictions(t) })
}

func TestSemaphore(t *testing.T) {
	tt := setupServicesContext(context.Background(), t)

	t.Run("TestSemaphoreLock", tt.suite.SemaphoreLock)
	t.Run("TestSemaphoreConcurrency", tt.suite.SemaphoreConcurrency)
	t.Run("TestSemaphoreContention", tt.suite.SemaphoreContention)
	t.Run("TestSemaphoreFlakiness", tt.suite.SemaphoreFlakiness)
}
