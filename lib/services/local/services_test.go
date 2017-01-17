/*
Copyright 2015 Gravitational, Inc.

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
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestServices(t *testing.T) { TestingT(t) }

type ServicesSuite struct {
	bk    backend.Backend
	suite *suite.ServicesTestSuite
	dir   string
}

var _ = Suite(&ServicesSuite{})

func (s *ServicesSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *ServicesSuite) SetUpTest(c *C) {
	var err error

	s.dir = c.MkDir()
	s.bk, err = boltbk.New(backend.Params{"path": s.dir})
	c.Assert(err, IsNil)

	suite := &suite.ServicesTestSuite{}
	suite.CAS = NewCAService(s.bk)
	suite.PresenceS = NewPresenceService(s.bk)
	suite.ProvisioningS = NewProvisioningService(s.bk)
	suite.WebS = NewIdentityService(s.bk)
	suite.Access = NewAccessService(s.bk)
	suite.ChangesC = make(chan interface{})
	s.suite = suite
}

func (s *ServicesSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *ServicesSuite) TestUserCACRUD(c *C) {
	s.suite.CertAuthCRUD(c)
}

func (s *ServicesSuite) TestServerCRUD(c *C) {
	s.suite.ServerCRUD(c)
}

func (s *ServicesSuite) TestReverseTunnelsCRUD(c *C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *ServicesSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *ServicesSuite) TestLoginAttempts(c *C) {
	s.suite.LoginAttempts(c)
}

func (s *ServicesSuite) TestPasswordHashCRUD(c *C) {
	s.suite.PasswordHashCRUD(c)
}

func (s *ServicesSuite) TestPasswordAndHotpCRUD(c *C) {
	s.suite.PasswordCRUD(c)
}

func (s *ServicesSuite) TestPasswordGarbage(c *C) {
	s.suite.PasswordGarbage(c)
}

func (s *ServicesSuite) TestWebSessionCRUD(c *C) {
	s.suite.WebSessionCRUD(c)
}

func (s *ServicesSuite) TestToken(c *C) {
	s.suite.TokenCRUD(c)
}

func (s *ServicesSuite) TestRoles(c *C) {
	s.suite.RolesCRUD(c)
}

func (s *ServicesSuite) TestU2FCRUD(c *C) {
	s.suite.U2FCRUD(c)
}
