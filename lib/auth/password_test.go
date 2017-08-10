/*
Copyright 2017 Gravitational, Inc.

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
package auth

import (
	"fmt"
	"time"

	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type PasswordSuite struct {
	bk backend.Backend
	a  *AuthServer
}

var _ = check.Suite(&PasswordSuite{})
var _ = fmt.Printf

func (s *PasswordSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *PasswordSuite) TearDownSuite(c *check.C) {
}

func (s *PasswordSuite) SetUpTest(c *check.C) {
	var err error
	s.bk, err = boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, check.IsNil)
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, check.IsNil)

	authConfig := &InitConfig{
		Backend:      s.bk,
		Authority:    authority.New(),
		ClusterName:  clusterName,
		StaticTokens: staticTokens,
	}
	s.a = NewAuthServer(authConfig)
}

func (s *PasswordSuite) TearDownTest(c *check.C) {
}

func (s *PasswordSuite) TestTiming(c *check.C) {
	username := "foo"
	password := "barbaz"

	err := s.a.UpsertPassword(username, []byte(password))
	c.Assert(err, check.IsNil)

	var elapsedExists time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		s.a.CheckPasswordWOToken(username, []byte(password))
		elapsed := time.Since(start)
		elapsedExists = elapsedExists + elapsed
	}

	var elapsedNotExists time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		s.a.CheckPasswordWOToken("blah", []byte(password))
		elapsed := time.Since(start)
		elapsedNotExists = elapsedNotExists + elapsed
	}

	// elapsedDifference must be less than 20 ms
	elapsedDifference := elapsedExists/10 - elapsedNotExists/10
	comment := check.Commentf("elapsed difference (%v) greater than 30 ms", elapsedDifference)
	c.Assert(elapsedDifference.Seconds() < 0.030, check.Equals, true, comment)
}
