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
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
)

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and hotp generator
//
// allowedLogins are linux user logins allowed for the new user to use
func (s *AuthServer) CreateSignupToken(user services.User) (string, error) {
	if err := user.Check(); err != nil {
		return "", trace.Wrap(err)
	}
	// make sure that connectors actually exist
	for _, id := range user.GetIdentities() {
		if err := id.Check(); err != nil {
			return "", trace.Wrap(err)
		}
		if _, err := s.GetOIDCConnector(id.ConnectorID, false); err != nil {
			return "", trace.Wrap(err)
		}
	}
	// check existing
	_, err := s.GetPasswordHash(user.GetName())
	if err == nil {
		return "", trace.BadParameter("user '%v' already exists", user)
	}

	token, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otp, err := hotp.GenerateHOTP(defaults.HOTPTokenDigits, false)
	if err != nil {
		log.Errorf("[AUTH API] failed to generate HOTP: %v", err)
		return "", trace.Wrap(err)
	}
	otpQR, err := otp.QR("Teleport: " + user.GetName() + "@" + s.AuthServiceName)
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
		Token: token,
		User: services.TeleportUser{
			Name:           user.GetName(),
			AllowedLogins:  user.GetAllowedLogins(),
			OIDCIdentities: user.GetIdentities()},
		Hotp:            otpMarshalled,
		HotpFirstValues: otpFirstValues,
		HotpQR:          otpQR,
	}

	err = s.UpsertSignupToken(token, tokenData, defaults.MaxSignupTokenTTL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	log.Infof("[AUTH API] created the signup token for %v as %v", user)
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

	_, err = s.GetPasswordHash(tokenData.User.GetName())
	if err == nil {
		return "", nil, nil, trace.Errorf("can't add user %v, user already exists", tokenData.User)
	}

	return tokenData.User.GetName(), tokenData.HotpQR, tokenData.HotpFirstValues, nil
}

func (s *AuthServer) CreateSignupU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	err := s.CheckU2FEnabled()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lock := "signuptoken"+token

	err = s.AcquireLock(lock, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		err := s.ReleaseLock(lock)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.GetPasswordHash(tokenData.User.GetName())
	if err == nil {
		return nil, trace.AlreadyExists("can't add user %v, user already exists", tokenData.User)
	}

	c, err := u2f.NewChallenge(s.U2F.AppID, s.U2F.Facets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := c.RegisterRequest()

	err = s.UpsertU2FRegisterChallenge(token, c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return request, nil
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
		return nil, trace.BadParameter("wrong HOTP token")
	}

	_, _, err = s.UpsertPassword(tokenData.User.GetName(), []byte(password))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apply user allowed logins
	if err = s.UpsertUser(&tokenData.User); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertHOTP(tokenData.User.GetName(), otp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("[AUTH] created new user: %v", &tokenData.User)

	if err = s.DeleteSignupToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.NewWebSession(tokenData.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertWebSession(tokenData.User.GetName(), sess, WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess.WS.Priv = nil
	return sess, nil
}

func (s *AuthServer) CreateUserWithU2FToken(token string, password string, response u2f.RegisterResponse) (*Session, error) {
	err := s.CheckU2FEnabled()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lock := "signuptoken"+token

	err = s.AcquireLock(lock, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		err := s.ReleaseLock(lock)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	tokenData, err := s.GetSignupToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge, err := s.GetU2FRegisterChallenge(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reg, err := u2f.Register(response, *challenge, &u2f.Config{SkipAttestationVerify: true})
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	err = s.UpsertU2FRegistration(tokenData.User.GetName(), reg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = s.UpsertU2FRegistrationCounter(tokenData.User.GetName(), 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, _, err = s.UpsertPassword(tokenData.User.GetName(), []byte(password))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apply user allowed logins
	if err = s.UpsertUser(&tokenData.User); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("[AUTH] created new user: %v", &tokenData.User)

	if err = s.DeleteSignupToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.NewWebSession(tokenData.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertWebSession(tokenData.User.GetName(), sess, WebSessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess.WS.Priv = nil
	return sess, nil
}
