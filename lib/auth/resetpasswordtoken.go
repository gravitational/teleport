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
	"context"
	"fmt"
	"image/png"
	"net/url"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/pquerna/otp/totp"
)

const (
	// ResetPasswordTokenTypeInvite indicates invite UI flow
	ResetPasswordTokenTypeInvite = "invite"
	// ResetPasswordTokenTypePassword indicates set new password UI flow
	ResetPasswordTokenTypePassword = "password"
)

// CreateResetPasswordTokenRequest is a request to create a new reset password token
type CreateResetPasswordTokenRequest struct {
	// Name is the user name to reset.
	Name string `json:"name"`
	// TTL specifies how long the generated reset token is valid for.
	TTL time.Duration `json:"ttl"`
	// Type is a token type.
	Type string `json:"type"`
}

// CheckAndSetDefaults checks and sets the defaults
func (r *CreateResetPasswordTokenRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}
	if r.TTL < 0 {
		return trace.BadParameter("TTL can't be negative")
	}

	if r.Type == "" {
		r.Type = ResetPasswordTokenTypePassword
	}

	// We use the same mechanism to handle invites and password resets
	// as both allow setting up a new password based on auth preferences.
	// The only difference is default TTL values and URLs to web UI.
	switch r.Type {
	case ResetPasswordTokenTypeInvite:
		if r.TTL == 0 {
			r.TTL = defaults.SignupTokenTTL
		}

		if r.TTL > defaults.MaxSignupTokenTTL {
			return trace.BadParameter(
				"failed to create user invite token: maximum token TTL is %v hours",
				defaults.MaxSignupTokenTTL)
		}
	case ResetPasswordTokenTypePassword:
		if r.TTL == 0 {
			r.TTL = defaults.ChangePasswordTokenTTL
		}
		if r.TTL > defaults.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"failed to create reset password token: maximum token TTL is %v hours",
				defaults.MaxChangePasswordTokenTTL)
		}
	default:
		return trace.BadParameter("unknown reset password token request type(%v)", r.Type)
	}

	return nil
}

// CreateResetPasswordToken creates a reset password token
func (s *Server) CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.ResetPassword(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.newResetPasswordToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// remove any other existing tokens for this user
	err = s.deleteResetPasswordTokens(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.Identity.CreateResetPasswordToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.GetResetPasswordToken(ctx, token.GetName())
}

// RotateResetPasswordTokenSecrets rotates secrets for a given tokenID.
// It gets called every time a user fetches 2nd-factor secrets during registration attempt.
// This ensures that an attacker that gains the ResetPasswordToken link can not view it,
// extract the OTP key from the QR code, then allow the user to signup with
// the same OTP token.
func (s *Server) RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (services.ResetPasswordTokenSecrets, error) {
	token, err := s.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, qr, err := newTOTPKeys("Teleport", token.GetUser()+"@"+s.AuthServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, err := services.NewResetPasswordTokenSecrets(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets.Spec.OTPKey = key
	secrets.Spec.QRCode = string(qr)
	err = s.UpsertResetPasswordTokenSecrets(ctx, &secrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &secrets, nil
}

func (s *Server) newResetPasswordToken(req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	tokenID, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxies, err := s.GetProxies()
	if err != nil {
		log.Errorf("Unable to retrieve proxy list: %v", err)
	}

	proxyHost, _ := services.GuessProxyHostAndVersion(proxies)
	url, err := formatResetPasswordTokenURL(proxyHost, tokenID, req.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token := services.NewResetPasswordToken(tokenID)
	token.Metadata.SetExpiry(s.clock.Now().UTC().Add(req.TTL))
	token.Spec.User = req.Name
	token.Spec.Created = s.clock.Now().UTC()
	token.Spec.URL = url
	return &token, nil
}

func formatResetPasswordTokenURL(proxyHost string, tokenID string, reqType string) (string, error) {
	if proxyHost == "" {
		proxyHost = fmt.Sprintf("<proxyhost>:%v", defaults.HTTPListenPort)
	}

	u := &url.URL{
		Scheme: "https",
		Host:   proxyHost,
	}

	// We have 2 differen UI flows to process password reset tokens
	if reqType == ResetPasswordTokenTypeInvite {
		u.Path = fmt.Sprintf("/web/invite/%v", tokenID)
	} else if reqType == ResetPasswordTokenTypePassword {
		u.Path = fmt.Sprintf("/web/reset/%v", tokenID)
	}

	return u.String(), nil
}

func (s *Server) deleteResetPasswordTokens(ctx context.Context, username string) error {
	tokens, err := s.GetResetPasswordTokens(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range tokens {
		if token.GetUser() != username {
			continue
		}

		err = s.DeleteResetPasswordToken(ctx, token.GetName())
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
