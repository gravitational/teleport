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
// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"time"

	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/session"
	"github.com/gravitational/trace"
)

const (
	ADD_USER_TOKEN_TTL              = time.Hour * 24
	ADD_USER_TOKEN_USER_ACTIONS_TTL = time.Hour
	AUTH_TARGET_ADD_USER_FORM       = "AuthTargetAddUserForm"
	AUTH_TARGET_ADD_USER_FINISH     = "AuthTargetAddUserFinish"
)

func (s *AuthServer) CreateAddUserToken(user string) (token string, e error) {
	s.AddUserMutex.Lock()
	defer s.AddUserMutex.Unlock()

	_, err := s.GetPasswordHash(user)
	if err == nil {
		return "", trace.Errorf("Can't add user %v, user already exists", user)
	}

	t, err := session.NewID(s.scrt)
	if err != nil {
		return "", trace.Wrap(err)
	}
	token = string(t.PID)

	otp, err := hotp.GenerateHOTP(6, false)
	if err != nil {
		return "", trace.Wrap(err)
	}
	otpQR, err := otp.QR(user)
	if err != nil {
		return "", trace.Wrap(err)
	}
	otp.Increment()
	otpFirstValue := otp.OTP()

	otpMarshalled, err := hotp.Marshal(otp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	tokenData := services.AddUserToken{
		Token: token,
		User:  user,
		AuthTargets: map[string]int{
			AUTH_TARGET_ADD_USER_FORM:   1,
			AUTH_TARGET_ADD_USER_FINISH: 1,
		},
		Hotp:           otpMarshalled,
		HotpFirstValue: otpFirstValue,
		HotpQR:         otpQR,
	}

	err = s.UpsertAddUserToken(token, tokenData, ADD_USER_TOKEN_TTL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

func (s *AuthServer) AuthWithAddUserToken(token string, target string) error {
	s.AddUserMutex.Lock()
	defer s.AddUserMutex.Unlock()

	tokenData, err := s.GetAddUserToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	_, exist := tokenData.AuthTargets[target]
	if !exist {
		return trace.Errorf("Token was already used")
	}

	delete(tokenData.AuthTargets, target)

	err = s.UpsertAddUserToken(token, tokenData, ADD_USER_TOKEN_USER_ACTIONS_TTL)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *AuthServer) GetAddUserTokenData(token string) (user string,
	QRImg []byte, hotpFirstValue string, e error) {

	s.AddUserMutex.Lock()
	defer s.AddUserMutex.Unlock()

	tokenData, err := s.GetAddUserToken(token)
	if err != nil {
		return "", nil, "", trace.Wrap(err)
	}

	if len(tokenData.HotpFirstValue) == 0 {
		return "", nil, "", trace.Errorf("Token was already used")
	}

	hotpFirstValue = tokenData.HotpFirstValue
	user = tokenData.User
	QRImg = tokenData.HotpQR

	tokenData.HotpFirstValue = ""

	err = s.UpsertAddUserToken(token, tokenData, ADD_USER_TOKEN_USER_ACTIONS_TTL)
	if err != nil {
		return "", nil, "", trace.Wrap(err)
	}

	return user, QRImg, hotpFirstValue, nil
}

func (s *AuthServer) CreateUserWithToken(token string, password string) error {
	s.AddUserMutex.Lock()
	defer s.AddUserMutex.Unlock()

	tokenData, err := s.GetAddUserToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	_, _, err = s.UpsertPassword(tokenData.User, []byte(password))
	if err != nil {
		return trace.Wrap(err)
	}

	otp, err := hotp.Unmarshal(tokenData.Hotp)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertHOTP(tokenData.User, otp)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.DeleteAddUserToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
