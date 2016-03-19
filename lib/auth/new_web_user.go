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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gokyle/hotp"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

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
		return "", trace.Wrap(
			teleport.BadParameter(
				"user", fmt.Sprintf("user '%v' already exists", user)))
	}

	token, err := utils.CryptoRandomHex(WebSessionTokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otp, err := hotp.GenerateHOTP(defaults.HOTPTokenDigits, false)
	if err != nil {
		log.Errorf("[AUTH API] failed to generate HOTP: %v", err)
		return "", trace.Wrap(err)
	}
	otpQR, err := otp.QR("Teleport: " + user + "@" + s.AuthServiceName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otpMarshalled, err := hotp.Marshal(otp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otpFirstValues := make([]string, defaults.HOTPFirstTokensRange)
	for i := 0; i < defaults.HOTPFirstTokensRange; i++ {
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

	err = s.UpsertSignupToken(token, tokenData, defaults.MaxSignupTokenTTL)
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

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return "", nil, nil, trace.Wrap(err)
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
func (s *AuthServer) CreateUserWithToken(token, password, hotpToken string) (*Session, error) {
	err := s.AcquireLock("signuptoken"+token, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		err := s.ReleaseLock("signuptoken" + token)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otp, err := hotp.Unmarshal(tokenData.Hotp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ok := otp.Scan(hotpToken, defaults.HOTPFirstTokensRange)
	if !ok {
		return nil, trace.Wrap(teleport.BadParameter("hotp", "wrong HOTP token"))
	}

	_, _, err = s.UpsertPassword(tokenData.User, []byte(password))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apply user allowed logins
	if err = s.UpsertUser(services.User{Name: tokenData.User, AllowedLogins: tokenData.AllowedLogins}); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertHOTP(tokenData.User, otp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("[AUTH] created new user: %v as %v", tokenData.User, tokenData.AllowedLogins)

	if err = s.DeleteSignupToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.NewWebSession(tokenData.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertWebSession(tokenData.User, sess, WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess.WS.Priv = nil
	return sess, nil
}
