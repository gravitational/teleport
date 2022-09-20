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
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type ServicesSuite struct {
	bk    backend.Backend
	suite *suite.ServicesTestSuite
}

var _ = check.Suite(&ServicesSuite{})

func (s *ServicesSuite) SetUpTest(c *check.C) {
	var err error
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	s.bk, err = lite.NewWithConfig(ctx, lite.Config{
		Path:             c.MkDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	c.Assert(err, check.IsNil)

	configService, err := NewClusterConfigurationService(s.bk)
	c.Assert(err, check.IsNil)

	eventsService := NewEventsService(s.bk)
	presenceService := NewPresenceService(s.bk)

	s.suite = &suite.ServicesTestSuite{
		CAS:           NewCAService(s.bk),
		PresenceS:     presenceService,
		ProvisioningS: NewProvisioningService(s.bk),
		WebS:          NewIdentityService(s.bk),
		Access:        NewAccessService(s.bk),
		EventsS:       eventsService,
		ChangesC:      make(chan interface{}),
		ConfigS:       configService,
		RestrictionsS: NewRestrictionsService(s.bk),
		Clock:         clock,
	}
}

func (s *ServicesSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *ServicesSuite) TestUserCACRUD(c *check.C) {
	s.suite.CertAuthCRUD(c)
}

func (s *ServicesSuite) TestServerCRUD(c *check.C) {
	s.suite.ServerCRUD(c)
}

// TestAppServerCRUD tests CRUD functionality for services.App.
func (s *ServicesSuite) TestAppServerCRUD(c *check.C) {
	s.suite.AppServerCRUD(c)
}

func (s *ServicesSuite) TestReverseTunnelsCRUD(c *check.C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *ServicesSuite) TestUsersCRUD(c *check.C) {
	s.suite.UsersCRUD(c)
}

func (s *ServicesSuite) TestUsersExpiry(c *check.C) {
	s.suite.UsersExpiry(c)
}

func (s *ServicesSuite) TestLoginAttempts(c *check.C) {
	s.suite.LoginAttempts(c)
}

func (s *ServicesSuite) TestPasswordHashCRUD(c *check.C) {
	s.suite.PasswordHashCRUD(c)
}

func (s *ServicesSuite) TestWebSessionCRUD(c *check.C) {
	s.suite.WebSessionCRUD(c)
}

func (s *ServicesSuite) TestToken(c *check.C) {
	s.suite.TokenCRUD(c)
}

func (s *ServicesSuite) TestRoles(c *check.C) {
	s.suite.RolesCRUD(c)
}

func (s *ServicesSuite) TestU2FCRUD(c *check.C) {
	s.suite.U2FCRUD(c)
}

func (s *ServicesSuite) TestSAMLCRUD(c *check.C) {
	s.suite.SAMLCRUD(c)
}

func (s *ServicesSuite) TestTunnelConnectionsCRUD(c *check.C) {
	s.suite.TunnelConnectionsCRUD(c)
}

func (s *ServicesSuite) TestGithubConnectorCRUD(c *check.C) {
	s.suite.GithubConnectorCRUD(c)
}

func (s *ServicesSuite) TestRemoteClustersCRUD(c *check.C) {
	s.suite.RemoteClustersCRUD(c)
}

func (s *ServicesSuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *ServicesSuite) TestEventsClusterConfig(c *check.C) {
	s.suite.EventsClusterConfig(c)
}

func (s *ServicesSuite) TestSemaphoreLock(c *check.C) {
	s.suite.SemaphoreLock(c)
}

func (s *ServicesSuite) TestSemaphoreConcurrency(c *check.C) {
	s.suite.SemaphoreConcurrency(c)
}

func (s *ServicesSuite) TestSemaphoreContention(c *check.C) {
	s.suite.SemaphoreContention(c)
}

func (s *ServicesSuite) TestSemaphoreFlakiness(c *check.C) {
	s.suite.SemaphoreFlakiness(c)
}

func (s *ServicesSuite) TestNetworkRestrictions(c *check.C) {
	s.suite.NetworkRestrictions(c)
}
