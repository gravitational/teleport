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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/gravitational/trace"
)

const (
	// UserTokenTypeResetPasswordInvite is a token type used for the UI invite flow that
	// allows users to change their password and set second factor (if enabled).
	UserTokenTypeResetPasswordInvite = "invite"
	// UserTokenTypeResetPassword is a token type used for the UI flow where user
	// re-sets their password and second factor (if enabled).
	UserTokenTypeResetPassword = "password"
	// UserTokenTypeRecoveryStart describes a recovery token issued to users who
	// successfully verified their recovery code.
	UserTokenTypeRecoveryStart = "recovery_start"
	// UserTokenTypeRecoveryApproved describes a recovery token issued to users who
	// successfully verified their second auth credential (either password or a second factor) and
	// can now start changing their password or add a new second factor device.
	// This token is also used to allow users to delete exisiting second factor devices
	// and retrieve their new set of recovery codes as part of the recovery flow.
	UserTokenTypeRecoveryApproved = "recovery_approved"
	// UserTokenTypePrivilege describes a token type that grants access to a privileged action
	// that requires users to re-authenticate with their second factor while looged in. This
	// token is issued to users who has successfully re-authenticated.
	UserTokenTypePrivilege = "privilege"
	// UserTokenTypePrivilegeException describes a token type that allowed a user to bypass
	// second factor re-authentication which in other cases would be required eg:
	// allowing user to add a mfa device if they don't have any registered.
	UserTokenTypePrivilegeException = "privilege_exception"
)

// CreateUserTokenRequest is a request to create a new user token.
type CreateUserTokenRequest struct {
	// Name is the user name for token.
	Name string `json:"name"`
	// TTL specifies how long the generated token is valid for.
	TTL time.Duration `json:"ttl"`
	// Type is the token type.
	Type string `json:"type"`
}

// CheckAndSetDefaults checks and sets the defaults.
func (r *CreateUserTokenRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}

	if r.TTL < 0 {
		return trace.BadParameter("TTL can't be negative")
	}

	if r.Type == "" {
		r.Type = UserTokenTypeResetPassword
	}

	switch r.Type {
	case UserTokenTypeResetPasswordInvite:
		if r.TTL == 0 {
			r.TTL = defaults.SignupTokenTTL
		}

		if r.TTL > defaults.MaxSignupTokenTTL {
			return trace.BadParameter(
				"failed to create user token for reset password invite: maximum token TTL is %v hours",
				defaults.MaxSignupTokenTTL)
		}

	case UserTokenTypeResetPassword:
		if r.TTL == 0 {
			r.TTL = defaults.ChangePasswordTokenTTL
		}

		if r.TTL > defaults.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"failed to create user token for reset password: maximum token TTL is %v hours",
				defaults.MaxChangePasswordTokenTTL)
		}

	case UserTokenTypeRecoveryStart:
		r.TTL = defaults.RecoveryStartTokenTTL

	case UserTokenTypeRecoveryApproved:
		r.TTL = defaults.RecoveryApprovedTokenTTL

	case UserTokenTypePrivilege, UserTokenTypePrivilegeException:
		r.TTL = defaults.PrivilegeTokenTTL

	default:
		return trace.BadParameter("unknown user token request type(%v)", r.Type)
	}

	return nil
}

// CreateResetPasswordToken creates a reset password token
func (s *Server) CreateResetPasswordToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Type != UserTokenTypeResetPassword && req.Type != UserTokenTypeResetPasswordInvite {
		return nil, trace.BadParameter("invalid reset password token request type")
	}

	_, err = s.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.ResetPassword(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.resetMFA(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// remove any other existing tokens for this user
	err = s.deleteUserTokens(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.Identity.CreateUserToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.ResetPasswordTokenCreateEvent,
			Code: events.ResetPasswordTokenCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: s.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create reset password token event.")
	}

	return s.GetUserToken(ctx, token.GetName())
}

func (s *Server) resetMFA(ctx context.Context, user string) error {
	devs, err := s.Identity.GetMFADevices(ctx, user, false)
	if err != nil {
		return trace.Wrap(err)
	}
	var errs []error
	for _, d := range devs {
		errs = append(errs, s.DeleteMFADevice(ctx, user, d.Id))
	}
	return trace.NewAggregate(errs...)
}

// proxyDomainGetter is a reduced subset of the Auth API for formatAccountName.
type proxyDomainGetter interface {
	GetProxies() ([]types.Server, error)
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

// DELETE IN 9.0.0 replaced by CreateRegisterChallenge.
//
// RotateUserTokenSecrets rotates secrets for a given tokenID.
// It gets called every time a user fetches 2nd-factor secrets during registration attempt.
// This ensures that an attacker that gains the user token link can not view it,
// extract the OTP key from the QR code, then allow the user to signup with
// the same OTP token.
func (s *Server) RotateUserTokenSecrets(ctx context.Context, tokenID string) (types.UserTokenSecrets, error) {
	token, err := s.GetUserToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otpKey, _, err := s.newTOTPKey(token.GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, err := s.createTOTPUserTokenSecrets(ctx, token, otpKey)
	return secrets, trace.Wrap(err)
}

// createTOTPUserTokenSecrets creates new UserTokenSecretes resource for the given token.
func (s *Server) createTOTPUserTokenSecrets(ctx context.Context, token types.UserToken, otpKey *otp.Key) (types.UserTokenSecrets, error) {
	// Create QR code.
	var otpQRBuf bytes.Buffer
	otpImage, err := otpKey.Image(456, 456)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := png.Encode(&otpQRBuf, otpImage); err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, err := types.NewUserTokenSecrets(token.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	secrets.SetOTPKey(otpKey.Secret())
	secrets.SetQRCode(otpQRBuf.Bytes())
	secrets.SetExpiry(token.Expiry())
	err = s.UpsertUserTokenSecrets(ctx, secrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

func (s *Server) newTOTPKey(user string) (*otp.Key, *totp.GenerateOpts, error) {
	// Fetch account name to display in OTP apps.
	accountName, err := formatAccountName(s, user, s.AuthServiceName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	opts := totp.GenerateOpts{
		Issuer:      clusterName.GetClusterName(),
		AccountName: accountName,
		Period:      30, // seconds
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	}
	key, err := totp.Generate(opts)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return key, &opts, nil
}

func (s *Server) newUserToken(req CreateUserTokenRequest) (types.UserToken, error) {
	var err error
	var proxyHost string

	tokenLenBytes := TokenLenBytes
	if req.Type == UserTokenTypeRecoveryStart {
		tokenLenBytes = RecoveryTokenLenBytes
	}

	tokenID, err := utils.CryptoRandomHex(tokenLenBytes)
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

	url, err := formatUserTokenURL(proxyHost, tokenID, req.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := types.NewUserToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token.SetSubKind(req.Type)
	token.SetExpiry(s.clock.Now().UTC().Add(req.TTL))
	token.SetUser(req.Name)
	token.SetCreated(s.clock.Now().UTC())
	token.SetURL(url)
	return token, nil
}

func formatUserTokenURL(proxyHost string, tokenID string, reqType string) (string, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   proxyHost,
	}

	// Defines different UI flows that process user tokens.
	switch reqType {
	case UserTokenTypeResetPasswordInvite:
		u.Path = fmt.Sprintf("/web/invite/%v", tokenID)

	case UserTokenTypeResetPassword:
		u.Path = fmt.Sprintf("/web/reset/%v", tokenID)

	case UserTokenTypeRecoveryStart:
		u.Path = fmt.Sprintf("/web/recovery/steps/%v/verify", tokenID)
	}

	return u.String(), nil
}

// deleteUserTokens deletes all user tokens for the specified user.
func (s *Server) deleteUserTokens(ctx context.Context, username string) error {
	tokens, err := s.GetUserTokens(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range tokens {
		if token.GetUser() != username {
			continue
		}

		err = s.DeleteUserToken(ctx, token.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// getResetPasswordToken returns user token with subkind set to reset or invite, both
// types which allows users to change their password and set new second factors (if enabled).
func (s *Server) getResetPasswordToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	token, err := s.GetUserToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN 9.0.0: remove checking for empty string.
	if token.GetSubKind() != "" && token.GetSubKind() != UserTokenTypeResetPassword && token.GetSubKind() != UserTokenTypeResetPasswordInvite {
		return nil, trace.BadParameter("invalid token")
	}

	return token, nil
}

// createRecoveryToken creates a user token for account recovery.
func (s *Server) createRecoveryToken(ctx context.Context, username, tokenType string, usage types.UserTokenUsage) (types.UserToken, error) {
	if tokenType != UserTokenTypeRecoveryStart && tokenType != UserTokenTypeRecoveryApproved {
		return nil, trace.BadParameter("invalid recovery token type: %s", tokenType)
	}

	if usage != types.UserTokenUsage_USER_TOKEN_RECOVER_MFA && usage != types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD {
		return nil, trace.BadParameter("invalid recovery token usage type %s", usage.String())
	}

	req := CreateUserTokenRequest{
		Name: username,
		Type: tokenType,
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	newToken, err := s.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Mark what recover type user requested.
	newToken.SetUsage(usage)

	if _, err := s.Identity.CreateUserToken(ctx, newToken); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.RecoveryTokenCreateEvent,
			Code: events.RecoveryTokenCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: username,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: s.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create recovery token event.")
	}

	return newToken, nil
}

// CreatePrivilegeToken implements AuthService.CreatePrivilegeToken.
func (s *Server) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	username, err := GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authPref.GetAllowLocalAuth() {
		return nil, trace.AccessDenied("local auth needs to be enabled")
	}

	// For a user to add a device, second factor must be enabled.
	// A nil request will be interpreted as a user who has second factor enabled
	// but does not have any MFA registered, as can be the case with second factor optional.
	secondFactor := authPref.GetSecondFactor()
	if secondFactor == constants.SecondFactorOff {
		return nil, trace.AccessDenied("second factor must be enabled")
	}

	tokenKind := UserTokenTypePrivilege

	switch {
	case req.GetExistingMFAResponse() == nil:
		// Allows users with no devices to bypass second factor re-auth.
		devices, err := s.Identity.GetMFADevices(ctx, username, false)
		switch {
		case err != nil:
			return nil, trace.Wrap(err)
		case len(devices) > 0:
			return nil, trace.BadParameter("second factor authentication required")
		}

		tokenKind = UserTokenTypePrivilegeException

	default:
		if err := s.WithUserLock(username, func() error {
			_, err := s.validateMFAAuthResponse(ctx, username, req.GetExistingMFAResponse(), s.Identity)
			return err
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Delete any existing user tokens for user before creating.
	if err := s.deleteUserTokens(ctx, username); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.createPrivilegeToken(ctx, username, tokenKind)
	return token, trace.Wrap(err)
}

func (s *Server) createPrivilegeToken(ctx context.Context, username, tokenKind string) (*types.UserTokenV3, error) {
	if tokenKind != UserTokenTypePrivilege && tokenKind != UserTokenTypePrivilegeException {
		return nil, trace.BadParameter("invalid privilege token type")
	}

	req := CreateUserTokenRequest{
		Name: username,
		Type: tokenKind,
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	newToken, err := s.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.Identity.CreateUserToken(ctx, newToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.PrivilegeTokenCreateEvent,
			Code: events.PrivilegeTokenCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: username,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: s.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create privilege token event.")
	}

	convertedToken, ok := token.(*types.UserTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected UserToken type %T", token)
	}

	return convertedToken, nil
}

// verifyUserToken verifies that the token is not expired and is of the allowed kinds.
func (s *Server) verifyUserToken(token types.UserToken, allowedKinds ...string) error {
	if token.Expiry().Before(s.clock.Now().UTC()) {
		// Provide obscure message on purpose, while logging the real error server side.
		log.Debugf("Expired token(%s) type(%s)", token.GetName(), token.GetSubKind())
		return trace.AccessDenied("invalid token")
	}

	for _, kind := range allowedKinds {
		if token.GetSubKind() == kind {
			return nil
		}
	}

	log.Debugf("Invalid token(%s) type(%s), expected type: %v", token.GetName(), token.GetSubKind(), allowedKinds)
	return trace.AccessDenied("invalid token")
}
