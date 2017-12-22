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
	"encoding/base32"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	. "gopkg.in/check.v1"
)

type PasswordSuite struct {
	bk backend.Backend
	a  *AuthServer
}

var _ = Suite(&PasswordSuite{})
var _ = fmt.Printf

func (s *PasswordSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *PasswordSuite) TearDownSuite(c *C) {
}

func (s *PasswordSuite) SetUpTest(c *C) {
	var err error
	c.Assert(err, IsNil)
	s.bk, err = boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	authConfig := &InitConfig{
		Backend:   s.bk,
		Authority: authority.New(),
	}
	s.a = NewAuthServer(authConfig)

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TearDownTest(c *C) {
}

func (s *PasswordSuite) TestTiming(c *C) {
	username := "foo"
	password := "barbaz"

	err := s.a.UpsertPassword(username, []byte(password))
	c.Assert(err, IsNil)

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
	comment := Commentf("elapsed difference (%v) greater than 30 ms", elapsedDifference)
	c.Assert(elapsedDifference.Seconds() < 0.030, Equals, true, comment)
}

func (s *PasswordSuite) TestChangePassword(c *C) {
	req, err := s.prepareForPasswordChange("user1", []byte("abc123"), teleport.OFF)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.clock = fakeClock
	req.NewPassword = []byte("abce456")

	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithOTP(c *C) {
	req, err := s.prepareForPasswordChange("user2", []byte("abc123"), teleport.OTP)
	c.Assert(err, IsNil)

	otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
	err = s.a.UpsertTOTP(req.User, otpSecret)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.clock = fakeClock

	validToken, err := totp.GenerateCode(otpSecret, s.a.clock.Now())
	c.Assert(err, IsNil)

	// change password
	req.NewPassword = []byte("abce456")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	validToken, _ = totp.GenerateCode(otpSecret, s.a.clock.Now())
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) shouldLockAfterFailedAttempts(c *C, req services.ChangePasswordReq) {
	loginAttempts, _ := s.a.GetUserLoginAttempts(req.User)
	c.Assert(len(loginAttempts), Equals, 0)
	for i := 0; i < defaults.MaxLoginAttempts; i++ {
		err := s.a.ChangePassword(req)
		c.Assert(err, NotNil)
		loginAttempts, _ = s.a.GetUserLoginAttempts(req.User)
		c.Assert(len(loginAttempts), Equals, i+1)
	}

	err := s.a.ChangePassword(req)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *PasswordSuite) prepareForPasswordChange(user string, pass []byte, secondFactorType string) (services.ChangePasswordReq, error) {
	req := services.ChangePasswordReq{
		User:        user,
		OldPassword: pass,
	}

	err := s.a.UpsertCertAuthority(suite.NewTestCA(services.UserCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	err = s.a.UpsertCertAuthority(suite.NewTestCA(services.HostCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: secondFactorType,
	})
	if err != nil {
		return req, err
	}

	err = s.a.SetAuthPreference(ap)
	if err != nil {
		return req, err
	}

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	if err != nil {
		return req, err
	}
	err = s.a.UpsertPassword(user, pass)
	if err != nil {
		return req, err
	}

	_, err = s.a.SignIn(user, pass)
	if err != nil {
		return req, err
	}

	return req, nil
}
