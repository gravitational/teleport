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
		LocalConfigS:  configService,
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
	t.Run("TestPasswordCRUD", tt.suite.PasswordCRUD)
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
	t.Run("TestOIDCCRUD", tt.suite.OIDCCRUD)
}

func TestSemaphore(t *testing.T) {
	tt := setupServicesContext(context.Background(), t)

	t.Run("TestSemaphoreLock", tt.suite.SemaphoreLock)
	t.Run("TestSemaphoreConcurrency", tt.suite.SemaphoreConcurrency)
	t.Run("TestSemaphoreContention", tt.suite.SemaphoreContention)
	t.Run("TestSemaphoreFlakiness", tt.suite.SemaphoreFlakiness)
}
