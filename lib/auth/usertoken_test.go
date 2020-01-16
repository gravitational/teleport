/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"fmt"
	"time"

	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type UserTokenTest struct {
	bk backend.Backend
	a  *AuthServer
}

var _ = fmt.Printf
var _ = Suite(&UserTokenTest{})

func (s *UserTokenTest) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *UserTokenTest) TearDownSuite(c *C) {
}

func (s *UserTokenTest) SetUpTest(c *C) {
	var err error
	c.Assert(err, IsNil)
	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewAuthServer(authConfig)
	c.Assert(err, IsNil)

	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// Set services.ClusterConfig to disallow local auth.
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		LocalAuth: services.NewBool(true),
	})
	c.Assert(err, IsNil)

	err = s.a.SetClusterConfig(clusterConfig)
	c.Assert(err, IsNil)
}

func (s *UserTokenTest) TearDownTest(c *C) {
}

func (s *UserTokenTest) TestCreateUserToken(c *C) {
	username := "joe@example.com"
	pass := "pass123"
	_, _, err := CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	req := CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := s.a.CreateUserToken(req)
	c.Assert(err, IsNil)
	c.Assert(token.GetUser(), Equals, username)
	c.Assert(token.GetURL(), Equals, "https://<proxyhost>:3080/web/reset/"+token.GetName())

	// verify that password was reset
	err = s.a.CheckPasswordWOToken(username, []byte(pass))
	c.Assert(err, NotNil)

	// create another reset token for the same user
	token, err = s.a.CreateUserToken(req)
	c.Assert(err, IsNil)

	// previous token must be deleted
	tokens, err := s.a.GetUserTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].GetName(), Equals, token.GetName())
}

func (s *UserTokenTest) TestCreateUserTokenErrors(c *C) {
	username := "joe@example.com"
	_, _, err := CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	type testCase struct {
		desc string
		req  CreateUserTokenRequest
	}

	testCases := []testCase{
		{
			desc: "Reset Password: TTL < 0",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  -1,
			},
		},
		{
			desc: "Reset Password: TTL > max",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxChangePasswordTokenTTL + time.Hour,
			},
		},
		{
			desc: "Reset Password: empty user name",
			req: CreateUserTokenRequest{
				TTL: time.Hour,
			},
		},
		{
			desc: "Reset Password: user does not exist",
			req: CreateUserTokenRequest{
				Name: "doesnotexist@example.com",
				TTL:  time.Hour,
			},
		},
		{
			desc: "Invite: TTL > max",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxSignupTokenTTL + time.Hour,
				Type: UserTokenTypeInvite,
			},
		},
	}

	for _, tc := range testCases {
		_, err := s.a.CreateUserToken(tc.req)
		c.Assert(err, NotNil, Commentf("test case %q", tc.desc))
	}
}
