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

// Package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"fmt"
	"time"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"

	log "github.com/Sirupsen/logrus"
	"github.com/gokyle/hotp"
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
//
// allowedLogins are linux user logins allowed for the new user to use
func (s *AuthServer) CreateSignupToken(user string, allowedLogins []string) (string, error) {
	if !cstrings.IsValidUnixUser(user) {
		return "", trace.Wrap(
			teleport.BadParameter("user", fmt.Sprintf("'%v' is not a valid user name", user)))
	}
	for _, login := range allowedLogins {
		if !cstrings.IsValidUnixUser(login) {
			return "", trace.Wrap(teleport.BadParameter(
				"allowedLogins", fmt.Sprintf("'%v' is not a valid user name", login)))
		}
	}
	// check existing
	_, err := s.GetPasswordHash(user)
	if err == nil {
		return "", trace.Errorf("login '%v' already exists", user)
	}

	token, err := CryptoRandomHex(WebSessionTokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otp, err := hotp.GenerateHOTP(services.HOTPTokenDigits, false)
	if err != nil {
		log.Errorf("[AUTH API] failed to generate HOTP: %v", err)
		return "", trace.Wrap(err)
	}
	otpQR, err := otp.QR("teleport: " + user + "@" + s.Hostname)
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
		AllowedLogins:   allowedLogins,
	}

	err = s.UpsertSignupToken(token, tokenData, SignupTokenTTL+SignupTokenUserActionsTTL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	log.Infof("[AUTH API] created the signup token for %v as %v", user, allowedLogins)
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
		log.Errorf("[AUTH] error reading token (%v): %v", token, err)
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
		return trace.Errorf("Wrong HOTP token")
	}

	_, _, err = s.UpsertPassword(tokenData.User, []byte(password))
	if err != nil {
		log.Errorf("[AUTH] error saving new user (%v) to DB: %v", tokenData.User, err)
		return trace.Wrap(err)
	}

	// apply user allowed logins
	if err = s.UpsertUser(services.User{Name: tokenData.User, AllowedLogins: tokenData.AllowedLogins}); err != nil {
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

	log.Infof("[AUTH] created new user: %v as %v",
		tokenData.User, tokenData.AllowedLogins)
	return nil
}
