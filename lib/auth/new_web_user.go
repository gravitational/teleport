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
	"github.com/gravitational/log"
	"github.com/gravitational/session"
	"github.com/gravitational/trace"
)

const (
	SignupTokenTTL            = time.Hour * 24
	SignupTokenUserActionsTTL = time.Hour
	HOTPFirstTokensRange      = 5
)

var TokenTTLAfterUse = time.Second * 10

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and hotp generator
func (s *AuthServer) CreateSignupToken(user string) (token string, e error) {
	_, err := s.GetPasswordHash(user)
	if err == nil {
		return "", trace.Errorf("can't add user %v, user already exists", user)
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

	otpMarshalled, err := hotp.Marshal(otp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otpFirstValues := make([]string, HOTPFirstTokensRange)
	for i := 0; i < HOTPFirstTokensRange; i++ {
		otpFirstValues[i] = otp.OTP()
	}

	tokenData := services.SignupToken{
		Token:           token,
		User:            user,
		Hotp:            otpMarshalled,
		HotpFirstValues: otpFirstValues,
		HotpQR:          otpQR,
	}

	err = s.UpsertSignupToken(token, tokenData, SignupTokenTTL+SignupTokenUserActionsTTL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

// GetSignupTokenData returns token data for a valid token
func (s *AuthServer) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {

	err := s.AcquireLock("signuptoken"+token, time.Hour)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}

	defer func() {
		err := s.ReleaseLock("signuptoken" + token)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	tokenData, ttl, err := s.GetSignupToken(token)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
	}

	if ttl < SignupTokenUserActionsTTL {
		// user should have time to fill the signup form
		return "", nil, nil, trace.Errorf("token was expired")
	}

	_, err = s.GetPasswordHash(tokenData.User)
	if err == nil {
		return "", nil, nil, trace.Errorf("can't add user %v, user already exists", tokenData.User)
	}

	return tokenData.User, tokenData.HotpQR, tokenData.HotpFirstValues, nil
}

// CreateUserWithToken creates account with provided token and password.
// Account username and hotp generator are taken from token data.
// Deletes token after account creation.
func (s *AuthServer) CreateUserWithToken(token, password, hotpToken string) error {
	err := s.AcquireLock("signuptoken"+token, time.Hour)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := s.ReleaseLock("signuptoken" + token)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	tokenData, _, err := s.GetSignupToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.GetPasswordHash(tokenData.User)
	if err == nil {
		// that means that the account was already created
		e := s.CheckPasswordWOToken(tokenData.User, []byte(password))
		if e != nil {
			// different users tries to create one account
			return trace.Errorf("can't add user %v, user already exists", tokenData.User)
		} else {
			// one user just quickly clicked "Confirm" twice
			return nil
		}
	}

	otp, err := hotp.Unmarshal(tokenData.Hotp)
	if err != nil {
		return trace.Wrap(err)
	}

	ok := otp.Scan(hotpToken, HOTPFirstTokensRange)
	if !ok {
		return trace.Wrap(err)
	}

	_, _, err = s.UpsertPassword(tokenData.User, []byte(password))
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertHOTP(tokenData.User, otp)
	if err != nil {
		return trace.Wrap(err)
	}

	go func(s *AuthServer, token string) {
		time.Sleep(TokenTTLAfterUse) // If user will quickly click "Confirm" twice
		err = s.DeleteSignupToken(token)
		if err != nil {
			log.Errorf(err.Error())
		}
	}(s, token)

	return nil
}
