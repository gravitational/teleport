/*
Copyright 2017-2020 Gravitational, Inc.

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
	"bytes"
	"fmt"
	"image/png"
	"net/url"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/pquerna/otp/totp"

	"github.com/gravitational/trace"
)

const (
	// UserTokenTypeInvite indicates invite UI flow
	UserTokenTypeInvite = "invite"
	// UserTokenTypePasswordChange indicates change password UI flow
	UserTokenTypePasswordChange = "reset-password"
)

// CreateUserTokenRequest is a request to create a new user token
type CreateUserTokenRequest struct {
	// Name is the user name to reset.
	Name string `json:"name"`
	// TTL specifies how long the generated reset token is valid for.
	TTL time.Duration `json:"ttl"`
	// Type is user token type.
	Type string `json:"type"`
}

// CheckAndSetDefaults checks and sets the defaults
func (r *CreateUserTokenRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}
	if r.TTL < 0 {
		return trace.BadParameter("ttl can't be negative")
	}

	if r.Type == "" {
		r.Type = UserTokenTypePasswordChange
	}

	// We use the same mechanism to handle invites and password resets
	// as both allow setting up a new password based on auth preferences.
	// The only difference is default TTL values and URLs to web UI.
	switch r.Type {
	case UserTokenTypeInvite:
		if r.TTL == 0 {
			r.TTL = defaults.SignupTokenTTL
		}

		if r.TTL > defaults.MaxSignupTokenTTL {
			return trace.BadParameter(
				"failed to create user invite token: maximum token TTL is %v hours",
				defaults.MaxSignupTokenTTL)
		}
	case UserTokenTypePasswordChange:
		if r.TTL == 0 {
			r.TTL = defaults.ChangePasswordTokenTTL
		}
		if r.TTL > defaults.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"failed to create reset password token: maximum token TTL is %v hours",
				defaults.MaxChangePasswordTokenTTL)
		}
	default:
		return trace.BadParameter("unknown user token request type(%v)", r.Type)
	}

	return nil
}

// CreateUserToken creates user token
func (s *AuthServer) CreateUserToken(req CreateUserTokenRequest) (services.UserToken, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: check if some users cannot be reset
	_, err = s.ResetPassword(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// remove any other invite tokens for this user
	err = s.deleteUserTokens(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.Identity.CreateUserToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.GetUserToken(token.GetName())
}

// RotateUserTokenSecrets rotates user token secrets
func (s *AuthServer) RotateUserTokenSecrets(tokenID string) (services.UserTokenSecrets, error) {
	userToken, err := s.GetUserToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, qr, err := newTOTPKeys("Teleport", userToken.GetUser()+"@"+s.AuthServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, err := services.NewUserTokenSecrets(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets.Spec.OTPKey = key
	secrets.Spec.QRCode = string(qr)
	err = s.UpsertUserTokenSecrets(&secrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &secrets, nil
}

func (s *AuthServer) newUserToken(req CreateUserTokenRequest) (services.UserToken, error) {
	tokenID, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	url, err := formatUserTokenURL(s.publicURL(), tokenID, req.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken := services.NewUserToken(tokenID)
	userToken.Metadata.SetExpiry(s.clock.Now().UTC().Add(req.TTL))
	userToken.Spec.User = req.Name
	userToken.Spec.Created = s.clock.Now().UTC()
	userToken.Spec.URL = url
	return &userToken, nil
}

func (s *AuthServer) publicURL() string {
	proxyHost := "<proxyhost>:3080"
	proxies, err := s.GetProxies()
	if err != nil {
		log.Errorf("Unable to retrieve proxy list: %v", err)
	}

	if len(proxies) > 0 {
		proxyHost = proxies[0].GetPublicAddr()
		if proxyHost == "" {
			proxyHost = fmt.Sprintf("%v:%v", proxies[0].GetHostname(), defaults.HTTPListenPort)
			log.Debugf("public_address not set for proxy, returning proxyHost: %q", proxyHost)
		}
	}

	return fmt.Sprintf("https://" + proxyHost)
}

func formatUserTokenURL(advertiseURL string, tokenID string, reqType string) (string, error) {
	u, err := url.Parse(advertiseURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	u.RawQuery = ""
	// We have 2 differen UI flows to process user tokens
	if reqType == UserTokenTypeInvite {
		u.Path = fmt.Sprintf("/web/invite/%v", tokenID)
	} else if reqType == UserTokenTypePasswordChange {
		u.Path = fmt.Sprintf("/web/reset/%v", tokenID)
	}

	return u.String(), nil
}

func (s *AuthServer) deleteUserTokens(username string) error {
	userTokens, err := s.GetUserTokens()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range userTokens {
		if token.GetUser() != username {
			continue
		}

		err = s.DeleteUserToken(token.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func newTOTPKeys(issuer string, accountName string) (key string, qr []byte, err error) {
	// create totp key
	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	// create QR code
	var otpQRBuf bytes.Buffer
	otpImage, err := otpKey.Image(456, 456)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	png.Encode(&otpQRBuf, otpImage)

	return otpKey.Secret(), otpQRBuf.Bytes(), nil
}
