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
	"github.com/gravitational/teleport/lib/events"
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
func (s *AuthServer) CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
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

	if err := s.emitter.EmitAuditEvent(ctx, &events.ResetPasswordTokenCreate{
		Metadata: events.Metadata{
			Type: events.ResetPasswordTokenCreateEvent,
			Code: events.ResetPasswordTokenCreateCode,
		},
		UserMetadata: events.UserMetadata{
			User: clientUsername(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: s.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create reset password token event.")
	}

	return s.GetResetPasswordToken(ctx, token.GetName())
}

// proxyDomainGetter is a reduced subset of the Auth API for formatAccountName.
type proxyDomainGetter interface {
	GetProxies() ([]services.Server, error)
	GetDomainName() (string, error)
}

// formatAccountName builds the account name to display in OTP applications.
// Format for accountName is user@address. User is passed in, this function
// tries to find the best available address.
func formatAccountName(s proxyDomainGetter, username string, authHostname string) (string, error) {
	var err error
	var proxyHost string

	// Get a list of proxies.
	proxies, err := s.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	// If no proxies were found, try and set address to the name of the cluster.
	// If even the cluster name is not found (an unlikely) event, fallback to
	// hostname of the auth server.
	//
	// If a proxy was found, and any of the proxies has a public address set,
	// use that. If none of the proxies have a public address set, use the
	// hostname of the first proxy found.
	if len(proxies) == 0 {
		proxyHost, err = s.GetDomainName()
		if err != nil {
			log.Errorf("Failed to retrieve cluster name, falling back to hostname: %v.", err)
			proxyHost = authHostname
		}
	} else {
		proxyHost, _, err = services.GuessProxyHostAndVersion(proxies)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return fmt.Sprintf("%v@%v", username, proxyHost), nil
}

// RotateResetPasswordTokenSecrets rotates secrets for a given tokenID.
// It gets called every time a user fetches 2nd-factor secrets during registration attempt.
// This ensures that an attacker that gains the ResetPasswordToken link can not view it,
// extract the OTP key from the QR code, then allow the user to signup with
// the same OTP token.
func (s *AuthServer) RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (services.ResetPasswordTokenSecrets, error) {
	token, err := s.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch account name to display in OTP apps.
	accountName, err := formatAccountName(s, token.GetUser(), s.AuthServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, qr, err := newTOTPKeys("Teleport", accountName)
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

func (s *AuthServer) newResetPasswordToken(req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	var err error
	var proxyHost string

	tokenID, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the list of proxies and try and guess the address of the proxy. If
	// failed to guess public address, use "<proxyhost>:3080" as a fallback.
	proxies, err := s.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(proxies) == 0 {
		proxyHost = fmt.Sprintf("<proxyhost>:%v", defaults.HTTPListenPort)
	} else {
		proxyHost, _, err = services.GuessProxyHostAndVersion(proxies)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

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
	u := &url.URL{
		Scheme: "https",
		Host:   proxyHost,
	}

	// We have 2 different UI flows to process password reset tokens
	if reqType == ResetPasswordTokenTypeInvite {
		u.Path = fmt.Sprintf("/web/invite/%v", tokenID)
	} else if reqType == ResetPasswordTokenTypePassword {
		u.Path = fmt.Sprintf("/web/reset/%v", tokenID)
	}

	return u.String(), nil
}

func (s *AuthServer) deleteResetPasswordTokens(ctx context.Context, username string) error {
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
	if err := png.Encode(&otpQRBuf, otpImage); err != nil {
		return "", nil, trace.Wrap(err)
	}

	return otpKey.Secret(), otpQRBuf.Bytes(), nil
}
