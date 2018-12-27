/*
Copyright 2015-2018 Gravitational, Inc.

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
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestServices(t *testing.T) { check.TestingT(t) }

type ServicesSuite struct {
	bk    backend.Backend
	suite *suite.ServicesTestSuite
}

var _ = check.Suite(&ServicesSuite{})

func (s *ServicesSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())
}

func (s *ServicesSuite) SetUpTest(c *check.C) {
	var err error

	s.bk, err = lite.NewWithConfig(context.TODO(), lite.Config{Path: c.MkDir(), PollStreamPeriod: 200 * time.Millisecond})
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{}
	suite.CAS = NewCAService(s.bk)
	suite.PresenceS = NewPresenceService(s.bk)
	suite.ProvisioningS = NewProvisioningService(s.bk)
	suite.WebS = NewIdentityService(s.bk)
	suite.Access = NewAccessService(s.bk)
	suite.EventsS = NewEventsService(s.bk)
	suite.ChangesC = make(chan interface{})
	s.suite = suite
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

func (s *ServicesSuite) TestReverseTunnelsCRUD(c *check.C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *ServicesSuite) TestUsersCRUD(c *check.C) {
	s.suite.UsersCRUD(c)
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
