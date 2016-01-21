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
	SignupTokenTTL            = time.Hour * 24
	SignupTokenUserActionsTTL = time.Hour
	AuthTargetSignupForm      = "AuthTargetSignupForm"
	AuthTargetSignupFinish    = "AuthTargetSignupFinish"
)

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates and username, hotp generator
func (s *AuthServer) CreateSignupToken(user string) (token string, e error) {
	s.SignupMutex.Lock()
	defer s.SignupMutex.Unlock()

	_, err := s.GetPasswordHash(user)
	if err == nil {
		return "", trace.Errorf("Can't add user %v, user already exists", user)
	}

	t, err := session.NewID(s.scrt)
	if err != nil {
		return "", trace.Wrap(err)
	}
	token = string(t.PID)

	otp, err := hotp.GenerateHOTP(services.HOTPTokenDigits, false)
	if err != nil {
		return "", trace.Wrap(err)
	}
	otpQR, err := otp.QR(user + "@Teleport")
	if err != nil {
		return "", trace.Wrap(err)
	}
	otp.Increment()
	otpFirstValue := otp.OTP()

	otpMarshalled, err := hotp.Marshal(otp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	tokenData := services.SignupToken{
		Token: token,
		User:  user,
		AuthTargets: map[string]int{
			AuthTargetSignupForm:   1,
			AuthTargetSignupFinish: 1,
		},
		Hotp:           otpMarshalled,
		HotpFirstValue: otpFirstValue,
		HotpQR:         otpQR,
	}

	err = s.UpsertSignupToken(token, tokenData, SignupTokenTTL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

// AuthWithSignupToken returns nil once for each valid (token, target) string
// Possible targets: AuthTargetSignupForm, AuthTargetSignupFinish
func (s *AuthServer) AuthWithSignupToken(token string, target string) error {
	s.SignupMutex.Lock()
	defer s.SignupMutex.Unlock()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	_, exist := tokenData.AuthTargets[target]
	if !exist {
		return trace.Errorf("Token was already used")
	}

	delete(tokenData.AuthTargets, target)

	err = s.UpsertSignupToken(token, tokenData, SignupTokenUserActionsTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Returns token data once for each valid token
func (s *AuthServer) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValue string, e error) {

	s.SignupMutex.Lock()
	defer s.SignupMutex.Unlock()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return "", nil, "", trace.Wrap(err)
	}

	if len(tokenData.HotpFirstValue) == 0 {
		return "", nil, "", trace.Errorf("Token was already used")
	}

	_, err = s.GetPasswordHash(tokenData.User)
	if err == nil {
		return "", nil, "", trace.Errorf("Can't add user %v, user already exists", tokenData.User)
	}

	hotpFirstValue = tokenData.HotpFirstValue
	user = tokenData.User
	QRImg = tokenData.HotpQR

	tokenData.HotpFirstValue = ""

	err = s.UpsertSignupToken(token, tokenData, SignupTokenUserActionsTTL)
	if err != nil {
		return "", nil, "", trace.Wrap(err)
	}

	return user, QRImg, hotpFirstValue, nil
}

// CreateUserWithToken creates account with provided token and password.
// account username and hotp generator are taken from token data.
// Deletes token after account creation.
func (s *AuthServer) CreateUserWithToken(token string, password string) error {
	s.SignupMutex.Lock()
	defer s.SignupMutex.Unlock()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.GetPasswordHash(tokenData.User)
	if err == nil {
		return trace.Errorf("Can't add user %v, user already exists", tokenData.User)
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

	err = s.DeleteSignupToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
