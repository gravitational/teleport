/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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

	// userTokenTypePrivilegeOTP is used to hold OTP data during (otherwise)
	// token-less registrations.
	// This kind of token is an internal artifact of Teleport and should only be
	// allowed for OTP device registrations.
	userTokenTypePrivilegeOTP = "privilege_otp"
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

	case UserTokenTypePrivilege, UserTokenTypePrivilegeException, userTokenTypePrivilegeOTP:
		r.TTL = defaults.PrivilegeTokenTTL

	default:
		return trace.BadParameter("unknown user token request type(%v)", r.Type)
	}

	return nil
}

// CreateResetPasswordToken creates a reset password token
func (a *Server) CreateResetPasswordToken(ctx context.Context, req CreateUserTokenRequest) (types.UserToken, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Type != UserTokenTypeResetPassword && req.Type != UserTokenTypeResetPasswordInvite {
		return nil, trace.BadParameter("invalid reset password token request type")
	}

	switch user, err := a.GetUser(ctx, req.Name, false /* withSecrets */); {
	case err != nil:
		return nil, trace.Wrap(err)
	case user.GetUserType() != types.UserTypeLocal:
		return nil, trace.AccessDenied("only local users may be reset")
	}

	if err := a.resetPassword(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.resetMFA(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := a.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// remove any other existing tokens for this user
	err = a.deleteUserTokens(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = a.CreateUserToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.ResetPasswordTokenCreateEvent,
			Code: events.ResetPasswordTokenCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: a.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create reset password token event.")
	}

	return a.GetUserToken(ctx, token.GetName())
}

func (a *Server) resetMFA(ctx context.Context, user string) error {
	devs, err := a.Services.GetMFADevices(ctx, user, false)
	if err != nil {
		return trace.Wrap(err)
	}
	var errs []error
	for _, d := range devs {
		errs = append(errs, a.DeleteMFADevice(ctx, user, d.Id))
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

// createTOTPUserTokenSecrets creates new UserTokenSecrets resource for the given token.
func (a *Server) createTOTPUserTokenSecrets(ctx context.Context, token types.UserToken, otpKey *otp.Key) (types.UserTokenSecrets, error) {
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
	err = a.UpsertUserTokenSecrets(ctx, secrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

func (a *Server) newTOTPKey(user string) (*otp.Key, *totp.GenerateOpts, error) {
	// Fetch account name to display in OTP apps.
	accountName, err := formatAccountName(a, user, a.AuthServiceName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
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

func (a *Server) newUserToken(req CreateUserTokenRequest) (types.UserToken, error) {
	var err error
	var proxyHost string

	tokenLenBytes := defaults.TokenLenBytes
	if req.Type == UserTokenTypeRecoveryStart {
		tokenLenBytes = defaults.RecoveryTokenLenBytes
	}

	tokenID, err := utils.CryptoRandomHex(tokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the list of proxies and try and guess the address of the proxy. If
	// failed to guess public address, use "<proxyhost>:3080" as a fallback.
	proxies, err := a.GetProxies()
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
	token.SetExpiry(a.clock.Now().UTC().Add(req.TTL))
	token.SetUser(req.Name)
	token.SetCreated(a.clock.Now().UTC())
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
func (a *Server) deleteUserTokens(ctx context.Context, username string) error {
	tokens, err := a.GetUserTokens(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range tokens {
		if token.GetUser() != username {
			continue
		}

		err = a.DeleteUserToken(ctx, token.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// getResetPasswordToken returns user token with subkind set to reset or invite, both
// types which allows users to change their password and set new second factors (if enabled).
func (a *Server) getResetPasswordToken(ctx context.Context, tokenID string) (types.UserToken, error) {
	token, err := a.GetUserToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.GetSubKind() != UserTokenTypeResetPassword && token.GetSubKind() != UserTokenTypeResetPasswordInvite {
		return nil, trace.BadParameter("invalid token")
	}

	return token, nil
}

// createRecoveryToken creates a user token for account recovery.
func (a *Server) createRecoveryToken(ctx context.Context, username, tokenType string, usage types.UserTokenUsage) (types.UserToken, error) {
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

	newToken, err := a.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Mark what recover type user requested.
	newToken.SetUsage(usage)

	if _, err := a.CreateUserToken(ctx, newToken); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.RecoveryTokenCreateEvent,
			Code: events.RecoveryTokenCreateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: a.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create recovery token event.")
	}

	return newToken, nil
}

// CreatePrivilegeToken implements AuthService.CreatePrivilegeToken.
func (a *Server) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For a user to add a device, second factor must be enabled.
	// A nil request will be interpreted as a user who has second factor enabled
	// but does not have any MFA registered, as can be the case with second factor optional.
	if authPref.GetSecondFactor() == constants.SecondFactorOff {
		return nil, trace.AccessDenied("second factor must be enabled")
	}

	tokenKind := UserTokenTypePrivilege
	requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES}
	switch hasDevices, err := a.validateMFAAuthResponseForRegister(ctx, req.GetExistingMFAResponse(), username, requiredExt); {
	case err != nil:
		return nil, trace.Wrap(err)
	case !hasDevices:
		tokenKind = UserTokenTypePrivilegeException
	}

	// Delete any existing user tokens for user before creating.
	if err := a.deleteUserTokens(ctx, username); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := a.createPrivilegeToken(ctx, username, tokenKind)
	return token, trace.Wrap(err)
}

func (a *Server) createPrivilegeToken(ctx context.Context, username, tokenKind string) (*types.UserTokenV3, error) {
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

	newToken, err := a.newUserToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := a.CreateUserToken(ctx, newToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.PrivilegeTokenCreateEvent,
			Code: events.PrivilegeTokenCreateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: a.GetClock().Now().UTC().Add(req.TTL),
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
func (a *Server) verifyUserToken(token types.UserToken, allowedKinds ...string) error {
	if token.Expiry().Before(a.clock.Now().UTC()) {
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
