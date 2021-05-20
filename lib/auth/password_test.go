/*
Copyright 2017-2018 Gravitational, Inc.

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
	"encoding/base32"
	"math"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"gopkg.in/check.v1"
	. "gopkg.in/check.v1"
)

type PasswordSuite struct {
	bk          backend.Backend
	a           *Server
	mockEmitter *events.MockEmitter
}

var _ = Suite(&PasswordSuite{})

func (s *PasswordSuite) SetUpTest(c *C) {
	var err error
	c.Assert(err, IsNil)
	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	// set cluster name
	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	c.Assert(err, IsNil)

	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	err = s.a.SetClusterNetworkingConfig(context.TODO(), types.DefaultClusterNetworkingConfig())
	c.Assert(err, IsNil)

	err = s.a.SetSessionRecordingConfig(context.TODO(), types.DefaultSessionRecordingConfig())
	c.Assert(err, IsNil)

	clusterConfig, err := types.NewClusterConfig(types.ClusterConfigSpecV3{
		LocalAuth: types.NewBool(true),
	})
	c.Assert(err, IsNil)

	err = s.a.SetClusterConfig(clusterConfig)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	s.mockEmitter = &events.MockEmitter{}
	s.a.emitter = s.mockEmitter
}

func (s *PasswordSuite) TearDownTest(c *C) {
}

func (s *PasswordSuite) TestTiming(c *C) {
	username := "foo"
	password := "barbaz"

	err := s.a.UpsertPassword(username, []byte(password))
	c.Assert(err, IsNil)

	type res struct {
		exists  bool
		elapsed time.Duration
		err     error
	}

	// Run multiple password checks in parallel, for both existing and
	// non-existing user. This should ensure that there's always contention and
	// that both checking paths are subject to it together.
	//
	// This should result in timing results being more similar to each other
	// and reduce test flakiness.
	wg := sync.WaitGroup{}
	resCh := make(chan res)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			err := s.a.checkPasswordWOToken(username, []byte(password))
			resCh <- res{
				exists:  true,
				elapsed: time.Since(start),
				err:     err,
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			err := s.a.checkPasswordWOToken("blah", []byte(password))
			resCh <- res{
				exists:  false,
				elapsed: time.Since(start),
				err:     err,
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resCh)
	}()

	var elapsedExists, elapsedNotExists time.Duration
	for r := range resCh {
		if r.exists {
			c.Assert(r.err, IsNil)
			elapsedExists += r.elapsed
		} else {
			c.Assert(r.err, NotNil)
			elapsedNotExists += r.elapsed
		}
	}

	// Get the relative percentage difference in runtimes of password check
	// with real and non-existent users. It should be <10%.
	diffFraction := math.Abs(1.0 - (float64(elapsedExists) / float64(elapsedNotExists)))
	comment := Commentf("elapsed difference (%v%%) greater than 10%%", 100*diffFraction)
	c.Assert(diffFraction < 0.1, Equals, true, comment)
}

func (s *PasswordSuite) TestUserNotFound(c *C) {
	username := "unknown-user"
	password := "barbaz"

	err := s.a.checkPasswordWOToken(username, []byte(password))
	c.Assert(err, NotNil)
	// Make sure the error is not a NotFound. That would be a username oracle.
	c.Assert(trace.IsBadParameter(err), Equals, true)
}

func (s *PasswordSuite) TestChangePassword(c *C) {
	req, err := s.prepareForPasswordChange("user1", []byte("abc123"), constants.SecondFactorOff)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)
	req.NewPassword = []byte("abce456")

	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
	c.Assert(s.mockEmitter.LastEvent().GetType(), DeepEquals, events.UserPasswordChangeEvent)
	c.Assert(s.mockEmitter.LastEvent().(*events.UserPasswordChange).User, Equals, "user1")

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithOTP(c *C) {
	req, err := s.prepareForPasswordChange("user2", []byte("abc123"), constants.SecondFactorOTP)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
	dev, err := services.NewTOTPDevice("otp", otpSecret, fakeClock.Now())
	c.Assert(err, check.IsNil)
	ctx := context.Background()
	err = s.a.UpsertMFADevice(ctx, req.User, dev)
	c.Assert(err, check.IsNil)

	validToken, err := totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	c.Assert(err, IsNil)

	// change password
	req.NewPassword = []byte("abce456")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	validToken, _ = totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithToken(c *C) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	c.Assert(err, IsNil)

	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	username := "joe@example.com"
	password := []byte("qweqweqwe")
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  token.GetName(),
		Password: password,
	})
	c.Assert(err, IsNil)

	// password should be updated
	err = s.a.checkPasswordWOToken(username, password)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithTokenOTP(c *C) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, IsNil)

	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	username := "joe@example.com"
	password := []byte("qweqweqwe")
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	secrets, err := s.a.RotateResetPasswordTokenSecrets(context.TODO(), token.GetName())
	c.Assert(err, IsNil)

	otpToken, err := totp.GenerateCode(secrets.GetOTPKey(), s.bk.Clock().Now())
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:           token.GetName(),
		Password:          password,
		SecondFactorToken: otpToken,
	})
	c.Assert(err, IsNil)

	err = s.a.checkPasswordWOToken(username, password)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithTokenErrors(c *C) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, IsNil)

	username := "joe@example.com"
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	validPassword := []byte("qweQWE1")
	validTokenID := token.GetName()

	type testCase struct {
		desc         string
		secondFactor constants.SecondFactorType
		req          ChangePasswordWithTokenRequest
	}

	testCases := []testCase{
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid tokenID value",
			req: ChangePasswordWithTokenRequest{
				TokenID:  "what_token",
				Password: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid password",
			req: ChangePasswordWithTokenRequest{
				TokenID:  validTokenID,
				Password: []byte("short"),
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "missing second factor",
			req: ChangePasswordWithTokenRequest{
				TokenID:  validTokenID,
				Password: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "invalid OTP value",
			req: ChangePasswordWithTokenRequest{
				TokenID:           validTokenID,
				Password:          validPassword,
				SecondFactorToken: "invalid",
			},
		},
	}

	for _, tc := range testCases {
		// set new auth preference settings
		authPreference.SetSecondFactor(tc.secondFactor)
		err = s.a.SetAuthPreference(authPreference)
		c.Assert(err, IsNil)

		_, err = s.a.changePasswordWithToken(context.TODO(), tc.req)
		c.Assert(err, NotNil, Commentf("test case %q", tc.desc))
	}

	authPreference.SetSecondFactor(constants.SecondFactorOff)
	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  validTokenID,
		Password: validPassword,
	})
	c.Assert(err, IsNil)

	// invite token cannot be reused
	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  validTokenID,
		Password: validPassword,
	})
	c.Assert(err, NotNil)
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

func (s *PasswordSuite) prepareForPasswordChange(user string, pass []byte, secondFactorType constants.SecondFactorType) (services.ChangePasswordReq, error) {
	req := services.ChangePasswordReq{
		User:        user,
		OldPassword: pass,
	}

	err := s.a.UpsertCertAuthority(suite.NewTestCA(types.UserCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	err = s.a.UpsertCertAuthority(suite.NewTestCA(types.HostCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
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

	return req, nil
}
