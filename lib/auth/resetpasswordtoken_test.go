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
	"time"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

type ResetPasswordTokenTest struct {
	bk          backend.Backend
	a           *Server
	mockEmitter *events.MockEmitter
}

var _ = check.Suite(&ResetPasswordTokenTest{})

func (s *ResetPasswordTokenTest) SetUpTest(c *check.C) {
	var err error
	c.Assert(err, check.IsNil)
	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, check.IsNil)
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	c.Assert(err, check.IsNil)

	err = s.a.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	// Set services.ClusterConfig to disallow local auth.
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		LocalAuth: services.NewBool(true),
	})
	c.Assert(err, check.IsNil)

	err = s.a.SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	s.mockEmitter = &events.MockEmitter{}
	s.a.emitter = s.mockEmitter
}

func (s *ResetPasswordTokenTest) TestCreateResetPasswordToken(c *check.C) {
	username := "joe@example.com"
	pass := "pass123"
	_, _, err := CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, check.IsNil)

	req := CreateResetPasswordTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := s.a.CreateResetPasswordToken(context.TODO(), req)
	c.Assert(err, check.IsNil)
	c.Assert(token.GetUser(), check.Equals, username)
	c.Assert(token.GetURL(), check.Equals, "https://<proxyhost>:3080/web/reset/"+token.GetName())
	event := s.mockEmitter.LastEvent()

	c.Assert(event.GetType(), check.DeepEquals, events.ResetPasswordTokenCreateEvent)
	c.Assert(event.(*events.ResetPasswordTokenCreate).Name, check.Equals, "joe@example.com")
	c.Assert(event.(*events.ResetPasswordTokenCreate).User, check.Equals, teleport.UserSystem)

	// verify that password was reset
	err = s.a.CheckPasswordWOToken(username, []byte(pass))
	c.Assert(err, check.NotNil)

	// create another reset token for the same user
	token, err = s.a.CreateResetPasswordToken(context.TODO(), req)
	c.Assert(err, check.IsNil)

	// previous token must be deleted
	tokens, err := s.a.GetResetPasswordTokens(context.TODO())
	c.Assert(err, check.IsNil)
	c.Assert(len(tokens), check.Equals, 1)
	c.Assert(tokens[0].GetName(), check.Equals, token.GetName())
}

func (s *ResetPasswordTokenTest) TestCreateResetPasswordTokenErrors(c *check.C) {
	username := "joe@example.com"
	_, _, err := CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, check.IsNil)

	type testCase struct {
		desc string
		req  CreateResetPasswordTokenRequest
	}

	testCases := []testCase{
		{
			desc: "Reset Password: TTL < 0",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  -1,
			},
		},
		{
			desc: "Reset Password: TTL > max",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  defaults.MaxChangePasswordTokenTTL + time.Hour,
			},
		},
		{
			desc: "Reset Password: empty user name",
			req: CreateResetPasswordTokenRequest{
				TTL: time.Hour,
			},
		},
		{
			desc: "Reset Password: user does not exist",
			req: CreateResetPasswordTokenRequest{
				Name: "doesnotexist@example.com",
				TTL:  time.Hour,
			},
		},
		{
			desc: "Invite: TTL > max",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  defaults.MaxSignupTokenTTL + time.Hour,
				Type: ResetPasswordTokenTypeInvite,
			},
		},
	}

	for _, tc := range testCases {
		_, err := s.a.CreateResetPasswordToken(context.TODO(), tc.req)
		c.Assert(err, check.NotNil, check.Commentf("test case %q", tc.desc))
	}
}

// TestFormatAccountName makes sure that the OTP account name fallback values
// are correct. description
func (s *ResetPasswordTokenTest) TestFormatAccountName(c *check.C) {
	tests := []struct {
		description    string
		inDebugAuth    *debugAuth
		outAccountName string
		outError       bool
	}{
		{
			description: "failed to fetch proxies",
			inDebugAuth: &debugAuth{
				proxiesError: true,
			},
			outAccountName: "",
			outError:       true,
		},
		{
			description: "proxies with public address",
			inDebugAuth: &debugAuth{
				proxies: []services.Server{
					&services.ServerV2{
						Spec: services.ServerSpecV2{
							PublicAddr: "foo",
							Version:    "bar",
						},
					},
				},
			},
			outAccountName: "foo@foo",
			outError:       false,
		},
		{
			description: "proxies with no public address",
			inDebugAuth: &debugAuth{
				proxies: []services.Server{
					&services.ServerV2{
						Spec: services.ServerSpecV2{
							Hostname: "baz",
							Version:  "quxx",
						},
					},
				},
			},
			outAccountName: "foo@baz:3080",
			outError:       false,
		},
		{
			description: "no proxies, with domain name",
			inDebugAuth: &debugAuth{
				clusterName: "example.com",
			},
			outAccountName: "foo@example.com",
			outError:       false,
		},
		{
			description:    "no proxies, no domain name",
			inDebugAuth:    &debugAuth{},
			outAccountName: "foo@00000000-0000-0000-0000-000000000000",
			outError:       false,
		},
	}
	for _, tt := range tests {
		accountName, err := formatAccountName(tt.inDebugAuth, "foo", "00000000-0000-0000-0000-000000000000")
		c.Assert(err != nil, check.Equals, tt.outError, check.Commentf("Test case: %q.", tt.description))
		c.Assert(accountName, check.Equals, tt.outAccountName, check.Commentf("Test case: %q.", tt.description))
	}
}

type debugAuth struct {
	proxies      []services.Server
	proxiesError bool
	clusterName  string
}

func (s *debugAuth) GetProxies() ([]services.Server, error) {
	if s.proxiesError {
		return nil, trace.BadParameter("failed to fetch proxies")
	}
	return s.proxies, nil
}

func (s *debugAuth) GetDomainName() (string, error) {
	if s.clusterName == "" {
		return "", trace.NotFound("no cluster name set")
	}
	return s.clusterName, nil
}
