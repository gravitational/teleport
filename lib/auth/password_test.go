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
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/stretchr/testify/require"

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
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
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
	c.Assert(s.mockEmitter.LastEvent().(*apievents.UserPasswordChange).User, Equals, "user1")

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

func TestChangeUserAuthentication(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	tests := []struct {
		name              string
		setAuthPreference func()
		getReq            func(string) *proto.ChangeUserAuthenticationRequest
		getInvalidReq     func(string) *proto.ChangeUserAuthenticationRequest
	}{
		{
			name: "with second factor off and password only",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOff,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password1"),
				}
			},
		},
		{
			name: "with second factor otp",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOTP,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				secrets, err := srv.Auth().RotateUserTokenSecrets(ctx, resetTokenID)
				require.NoError(t, err)

				otpToken, err := totp.GenerateCode(secrets.GetOTPKey(), srv.Clock().Now())
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password2"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
						TOTP: &proto.TOTPRegisterResponse{Code: otpToken},
					}},
				}
			},
			// Invalid u2f fields when auth settings set to only otp.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewPassword:            []byte("password2"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{}},
				}
			},
		},
		{
			name: "with second factor u2f",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorU2F,
					U2F: &types.U2F{
						AppID:  "teleport",
						Facets: []string{"teleport"},
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				u2fRegResp, _, err := getMockedU2FAndRegisterRes(srv.Auth(), resetTokenID)
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password3"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
						U2F: u2fRegResp,
					}},
				}
			},
			// Invalid totp fields when auth settings set to only u2f.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewPassword:            []byte("password3"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{}},
				}
			},
		},
		{
			name: "with second factor on",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOn,
					U2F: &types.U2F{
						AppID:  "teleport",
						Facets: []string{"teleport"},
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				u2fRegResp, _, err := getMockedU2FAndRegisterRes(srv.Auth(), resetTokenID)
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password4"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
						U2F: u2fRegResp,
					}},
				}
			},
			// Empty register response, when auth settings requires second factors.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password4"),
				}
			},
		},
		{
			name: "with second factor optional and no second factor",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					U2F: &types.U2F{
						AppID:  "teleport",
						Facets: []string{"teleport"},
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password5"),
				}
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			username := fmt.Sprintf("llama%v@goteleport.com", rand.Int())
			_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
			require.NoError(t, err)

			c.setAuthPreference()

			token, err := srv.Auth().CreateResetPasswordToken(context.TODO(), CreateUserTokenRequest{
				Name: username,
			})
			require.NoError(t, err)

			if c.getInvalidReq != nil {
				invalidReq := c.getInvalidReq(token.GetName())
				_, err = srv.Auth().changeUserAuthentication(ctx, invalidReq)
				require.True(t, trace.IsBadParameter(err))
			}

			validReq := c.getReq(token.GetName())
			_, err = srv.Auth().changeUserAuthentication(ctx, validReq)
			require.NoError(t, err)

			// Test password is updated.
			err = srv.Auth().checkPasswordWOToken(username, validReq.NewPassword)
			require.NoError(t, err)
		})
	}
}

func (s *PasswordSuite) TestChangeUserAuthenticationWithErrors(c *C) {
	ctx := context.Background()
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	c.Assert(err, IsNil)

	username := "joe@example.com"
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(ctx, CreateUserTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	validPassword := []byte("qweQWE1")
	validTokenID := token.GetName()

	type testCase struct {
		desc         string
		secondFactor constants.SecondFactorType
		req          *proto.ChangeUserAuthenticationRequest
	}

	testCases := []testCase{
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid tokenID value",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     "what_token",
				NewPassword: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid password",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: []byte("short"),
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "missing second factor",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "invalid OTP value",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: validPassword,
				NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
					TOTP: &proto.TOTPRegisterResponse{Code: "invalid"},
				}},
			},
		},
	}

	for _, tc := range testCases {
		// set new auth preference settings
		authPreference.SetSecondFactor(tc.secondFactor)
		err = s.a.SetAuthPreference(ctx, authPreference)
		c.Assert(err, IsNil)

		_, err = s.a.changeUserAuthentication(ctx, tc.req)
		c.Assert(err, NotNil, Commentf("test case %q", tc.desc))
	}

	authPreference.SetSecondFactor(constants.SecondFactorOff)
	err = s.a.SetAuthPreference(ctx, authPreference)
	c.Assert(err, IsNil)

	_, err = s.a.changeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     validTokenID,
		NewPassword: validPassword,
	})
	c.Assert(err, IsNil)

	// invite token cannot be reused
	_, err = s.a.changeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     validTokenID,
		NewPassword: validPassword,
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
	ctx := context.Background()
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
		Type:         constants.Local,
		SecondFactor: secondFactorType,
	})
	if err != nil {
		return req, err
	}

	err = s.a.SetAuthPreference(ctx, ap)
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
