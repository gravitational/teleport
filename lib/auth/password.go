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
	"context"
	"crypto/subtle"
	"net/mail"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// This is bcrypt hash for password "barbaz".
var fakePasswordHash = []byte(`$2a$10$Yy.e6BmS2SrGbBDsyDLVkOANZmvjjMR890nUGSXFJHBXWzxe7T44m`)

// ChangeUserAuthentication implements AuthService.ChangeUserAuthentication.
func (a *Server) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	user, err := a.changeUserAuthentication(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if a user can receive new recovery codes.
	_, emailErr := mail.ParseAddress(user.GetName())
	hasEmail := emailErr == nil
	hasMFA := req.GetNewMFARegisterResponse() != nil
	recoveryAllowed := a.isAccountRecoveryAllowed(ctx) == nil
	createRecoveryCodes := hasEmail && hasMFA && recoveryAllowed

	var newRecovery *proto.RecoveryCodes
	if createRecoveryCodes {
		newRecovery, err = a.generateAndUpsertRecoveryCodes(ctx, user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	webSession, err := a.createUserWebSession(ctx, user, req.LoginIP)
	if err != nil {
		if keys.IsPrivateKeyPolicyError(err) {
			// Do not return an error, otherwise
			// the user won't be able to receive
			// recovery codes. Even with no recovery codes
			// this positive response indicates the user
			// has successfully reset/registered their account.
			return &proto.ChangeUserAuthenticationResponse{
				Recovery:                newRecovery,
				PrivateKeyPolicyEnabled: true,
			}, nil
		}
		return nil, trace.Wrap(err)
	}

	sess, ok := webSession.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected WebSessionV2 type %T", sess)
	}

	// TODO(codingllama): Issue device web token here?
	//  This could enable the initial transition, after the user sets password and
	//  MFA, to trigger device web login.
	//  At the moment it's highly unlikely the user has an enrolled device at this
	//  stage, so there's little reason to do it.

	return &proto.ChangeUserAuthenticationResponse{
		WebSession: sess,
		Recovery:   newRecovery,
	}, nil
}

// resetPassword deletes the user's password. Used to invalidate existing user
// password during password reset process.
//
// It does not fail if the user doesn't exist or doesn't have a password.
func (a *Server) resetPassword(ctx context.Context, username string) error {
	if err := a.DeletePassword(ctx, username); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// ChangePassword updates users password based on the old password.
func (a *Server) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) error {
	// validate new password
	if err := services.VerifyPassword(req.NewPassword); err != nil {
		return trace.Wrap(err)
	}

	// Authenticate.
	user := req.User
	authReq := AuthenticateUserRequest{
		Username: user,
		Webauthn: wantypes.CredentialAssertionResponseFromProto(req.Webauthn),
	}
	requiredExt := mfav1.ChallengeExtensions{
		Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_CHANGE_PASSWORD,
	}
	if len(req.OldPassword) > 0 {
		authReq.Pass = &PassCreds{
			Password: req.OldPassword,
		}
	} else {
		// If the user didn't provide their old password, we need to require
		// identity verification (i.e. make sure that a resident token used for
		// MFA).
		requiredExt.UserVerificationRequirement = string(protocol.VerificationRequired)
	}
	if req.SecondFactorToken != "" {
		authReq.OTP = &OTPCreds{
			Password: req.OldPassword,
			Token:    req.SecondFactorToken,
		}
	}
	verifyMFALocks, _, _, err := a.authenticateUser(ctx, authReq, requiredExt)
	if err != nil {
		return trace.Wrap(err)
	}
	// Verify if the MFA device used is locked.
	if err := verifyMFALocks(verifyMFADeviceLocksParams{}); err != nil {
		return trace.Wrap(err)
	}

	if err := a.UpsertPassword(user, req.NewPassword); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.UserPasswordChange{
		Metadata: apievents.Metadata{
			Type: events.UserPasswordChangeEvent,
			Code: events.UserPasswordChangeCode,
		},
		UserMetadata:       authz.ClientUserMetadataWithUser(ctx, user),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit password change event.")
	}
	return nil
}

// checkPasswordWOToken checks just password without checking OTP tokens. Marks
// user's password state as SET if necessary.  Used in case of SSH or Web
// authentication, when token has been validated.
func (a *Server) checkPasswordWOToken(ctx context.Context, user string, password []byte) error {
	const errMsg = "invalid username or password"

	hash, err := a.GetPasswordHash(user)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	userFound := true
	if trace.IsNotFound(err) {
		userFound = false
		log.Debugf("Password for username %q not found, using fake hash to mitigate timing attacks.", user)
		hash = fakePasswordHash
	}

	if err = bcrypt.CompareHashAndPassword(hash, password); err != nil {
		log.Debugf("Password for %q does not match", user)
		return trace.BadParameter(errMsg)
	}

	// Careful! The bcrypt check above may succeed for an unknown user when the
	// provided password is "barbaz", which is what fakePasswordHash hashes to.
	if !userFound {
		return trace.BadParameter(errMsg)
	}

	// At this point, we know that the user provided a correct password, so we may
	// stop worrying about timing attacks against the user existence or password.
	// Now mark the password state as SET to gradually transition those users for
	// whom it's not known.
	_, err = a.UpdateAndSwapUser(ctx, user, true /* withSecrets */, func(u types.User) (bool, error) {
		if u.GetPasswordState() == types.PasswordState_PASSWORD_STATE_SET {
			return false, nil
		}
		u.SetPasswordState(types.PasswordState_PASSWORD_STATE_SET)
		return true, nil
	})
	if err != nil {
		// Don't let the password state flag change fail the entire operation.
		log.
			WithError(err).
			WithField("user", user).
			Warn("Failed to set password state")
	}

	return nil
}

type checkPasswordResult struct {
	mfaDev *types.MFADevice
}

// checkPassword checks the password and OTP token. Called by tsh or lib/web/*.
func (a *Server) checkPassword(ctx context.Context, user string, password []byte, otpToken string) (*checkPasswordResult, error) {
	err := a.checkPasswordWOToken(ctx, user, password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaDev, err := a.checkOTP(user, otpToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &checkPasswordResult{mfaDev: mfaDev}, nil
}

// checkOTP checks if the OTP token is valid.
func (a *Server) checkOTP(user string, otpToken string) (*types.MFADevice, error) {
	// get the previously used token to mitigate token replay attacks
	usedToken, err := a.GetUsedTOTPToken(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we use a constant time compare function to mitigate timing attacks
	if subtle.ConstantTimeCompare([]byte(otpToken), []byte(usedToken)) == 1 {
		return nil, trace.BadParameter("previously used totp token")
	}

	ctx := context.TODO()
	devs, err := a.Services.GetMFADevices(ctx, user, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, dev := range devs {
		totpDev := dev.GetTotp()
		if totpDev == nil {
			continue
		}

		if err := a.checkTOTP(ctx, user, otpToken, dev); err != nil {
			log.WithError(err).Errorf("Using TOTP device %q", dev.GetName())
			continue
		}
		return dev, nil
	}
	// This message is relied upon by the Web UI in
	// web/packages/teleport/src/Account/ManageDevices/AddAuthDeviceWizard/AddAuthDeviceWizard.tsx/RequthenticateStep().
	// Please keep these in sync.
	return nil, trace.AccessDenied("invalid totp token")
}

// checkTOTP checks if the TOTP token is valid.
func (a *Server) checkTOTP(ctx context.Context, user, otpToken string, dev *types.MFADevice) error {
	if dev.GetTotp() == nil {
		return trace.BadParameter("checkTOTP called with non-TOTP MFADevice %T", dev.Device)
	}
	// we use totp.ValidateCustom over totp.Validate so we can use
	// a fake clock in tests to get reliable results
	valid, err := totp.ValidateCustom(otpToken, dev.GetTotp().Key, a.clock.Now(), totp.ValidateOpts{
		Period:    teleport.TOTPValidityPeriod,
		Skew:      teleport.TOTPSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return trace.AccessDenied("failed to validate TOTP code: %v", err)
	}
	if !valid {
		return trace.AccessDenied("invalid one time token, please check if the token has expired and try again")
	}
	// if we have a valid token, update the previously used token
	if err := a.UpsertUsedTOTPToken(user, otpToken); err != nil {
		return trace.Wrap(err)
	}

	// Update LastUsed timestamp on the device.
	dev.LastUsed = a.clock.Now()
	if err := a.UpsertMFADevice(ctx, user, dev); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *Server) changeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (types.User, error) {
	// Get cluster configuration and check if local auth is allowed.
	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authPref.GetAllowLocalAuth() {
		return nil, trace.AccessDenied(noLocalAuth)
	}

	reqPasswordless := len(req.GetNewPassword()) == 0 && authPref.GetAllowPasswordless()
	switch {
	case reqPasswordless:
		if req.GetNewMFARegisterResponse() == nil || req.NewMFARegisterResponse.GetWebauthn() == nil {
			return nil, trace.BadParameter("passwordless: missing webauthn credentials")
		}
	default:
		if err := services.VerifyPassword(req.GetNewPassword()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Check if token exists.
	token, err := a.getResetPasswordToken(ctx, req.TokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.Expiry().Before(a.clock.Now().UTC()) {
		return nil, trace.BadParameter("expired token")
	}

	// Check if the user still exists before potentially recreating the user
	// below. If the user was deleted, do NOT honor the request and delete any
	// other tokens associated with the user.
	if _, err := a.GetUser(ctx, token.GetUser(), false); err != nil {
		if trace.IsNotFound(err) {
			// Delete any remaining tokens for users that no longer exist.
			if err := a.deleteUserTokens(ctx, token.GetUser()); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return nil, trace.Wrap(err)
	}

	if err := a.changeUserSecondFactor(ctx, req, token); err != nil {
		return nil, trace.Wrap(err)
	}

	username := token.GetUser()
	// Delete this token first to minimize the chances
	// of partially updated user with still valid token.
	if err := a.deleteUserTokens(ctx, username); err != nil {
		return nil, trace.Wrap(err)
	}

	if !reqPasswordless {
		if err := a.UpsertPassword(username, req.GetNewPassword()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	user, err := a.GetUser(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

func (a *Server) changeUserSecondFactor(ctx context.Context, req *proto.ChangeUserAuthenticationRequest, token types.UserToken) error {
	username := token.GetUser()
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	switch sf := cap.GetSecondFactor(); {
	case sf == constants.SecondFactorOff:
		return nil
	case req.GetNewMFARegisterResponse() == nil && sf == constants.SecondFactorOptional:
		// Optional second factor does not enforce users to add a MFA device.
		// No need to check if a user already has registered devices since we expect
		// users to have no devices at this point.
		//
		// The ChangeUserAuthenticationRequest is made with a reset or invite token
		// where a reset token would've reset the users' MFA devices, and an invite
		// token is a new user with no devices.
		return nil
	case req.GetNewMFARegisterResponse() == nil:
		return trace.BadParameter("no second factor sent during user %q password reset", username)
	}

	deviceName := req.GetNewDeviceName()
	// Using default values here is safe since we don't expect users to have
	// any devices at this point.
	if deviceName == "" {
		switch {
		case req.GetNewMFARegisterResponse().GetTOTP() != nil:
			deviceName = "otp"
		case req.GetNewMFARegisterResponse().GetWebauthn() != nil:
			deviceName = "webauthn"
		default:
			// Fallback to something reasonable while letting verifyMFARespAndAddDevice
			// worry about the "unknown" response type.
			deviceName = "mfa"
			log.Warnf("Unexpected MFA register response type, setting device name to %q: %T", deviceName, req.GetNewMFARegisterResponse().Response)
		}
	}

	deviceUsage := proto.DeviceUsage_DEVICE_USAGE_MFA
	if len(req.GetNewPassword()) == 0 {
		deviceUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	}

	_, err = a.verifyMFARespAndAddDevice(ctx, &newMFADeviceFields{
		username:      token.GetUser(),
		newDeviceName: deviceName,
		tokenID:       token.GetName(),
		deviceResp:    req.GetNewMFARegisterResponse(),
		deviceUsage:   deviceUsage,
	})
	return trace.Wrap(err)
}
