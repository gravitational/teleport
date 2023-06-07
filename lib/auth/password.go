// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"crypto/subtle"
	"net/mail"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// This is bcrypt hash for password "barbaz".
var fakePasswordHash = []byte(`$2a$10$Yy.e6BmS2SrGbBDsyDLVkOANZmvjjMR890nUGSXFJHBXWzxe7T44m`)

// ChangeUserAuthentication implements AuthService.ChangeUserAuthentication.
func (s *Server) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	user, err := s.changeUserAuthentication(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if a user can receive new recovery codes.
	_, emailErr := mail.ParseAddress(user.GetName())
	hasEmail := emailErr == nil
	hasMFA := req.GetNewMFARegisterResponse() != nil
	recoveryAllowed := s.isAccountRecoveryAllowed(ctx) == nil
	createRecoveryCodes := hasEmail && hasMFA && recoveryAllowed

	var newRecovery *proto.RecoveryCodes
	if createRecoveryCodes {
		newRecovery, err = s.generateAndUpsertRecoveryCodes(ctx, user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	webSession, err := s.createUserWebSession(ctx, user, req.LoginIP)
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

	return &proto.ChangeUserAuthenticationResponse{
		WebSession: sess,
		Recovery:   newRecovery,
	}, nil
}

// ResetPassword securely generates a new random password and assigns it to user.
// This method is used to invalidate existing user password during password
// reset process.
func (s *Server) ResetPassword(username string) (string, error) {
	user, err := s.GetUser(username, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	password, err := utils.CryptoRandomHex(defaults.ResetPasswordLength)
	if err != nil {
		return "", trace.Wrap(err)
	}

	err = s.UpsertPassword(user.GetName(), []byte(password))
	if err != nil {
		return "", trace.Wrap(err)
	}

	return password, nil
}

// ChangePassword updates users password based on the old password.
func (s *Server) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) error {
	// validate new password
	if err := services.VerifyPassword(req.NewPassword); err != nil {
		return trace.Wrap(err)
	}

	// Authenticate.
	user := req.User
	authReq := AuthenticateUserRequest{
		Username: user,
		Webauthn: wanlib.CredentialAssertionResponseFromProto(req.Webauthn),
	}
	if len(req.OldPassword) > 0 {
		authReq.Pass = &PassCreds{
			Password: req.OldPassword,
		}
	}
	if req.SecondFactorToken != "" {
		authReq.OTP = &OTPCreds{
			Password: req.OldPassword,
			Token:    req.SecondFactorToken,
		}
	}
	if _, _, err := s.authenticateUser(ctx, authReq); err != nil {
		return trace.Wrap(err)
	}

	if err := s.UpsertPassword(user, req.NewPassword); err != nil {
		return trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.UserPasswordChange{
		Metadata: apievents.Metadata{
			Type: events.UserPasswordChangeEvent,
			Code: events.UserPasswordChangeCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, user),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit password change event.")
	}
	return nil
}

// checkPasswordWOToken checks just password without checking OTP tokens
// used in case of SSH authentication, when token has been validated.
func (s *Server) checkPasswordWOToken(user string, password []byte) error {
	const errMsg = "invalid username or password"

	err := services.VerifyPassword(password)
	if err != nil {
		return trace.BadParameter(errMsg)
	}

	hash, err := s.GetPasswordHash(user)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	userFound := true
	if trace.IsNotFound(err) {
		userFound = false
		log.Debugf("Username %q not found, using fake hash to mitigate timing attacks.", user)
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

	return nil
}

type checkPasswordResult struct {
	mfaDev *types.MFADevice
}

// checkPassword checks the password and OTP token. Called by tsh or lib/web/*.
func (s *Server) checkPassword(user string, password []byte, otpToken string) (*checkPasswordResult, error) {
	err := s.checkPasswordWOToken(user, password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaDev, err := s.checkOTP(user, otpToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &checkPasswordResult{mfaDev: mfaDev}, nil
}

// checkOTP checks if the OTP token is valid.
func (s *Server) checkOTP(user string, otpToken string) (*types.MFADevice, error) {
	// get the previously used token to mitigate token replay attacks
	usedToken, err := s.GetUsedTOTPToken(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we use a constant time compare function to mitigate timing attacks
	if subtle.ConstantTimeCompare([]byte(otpToken), []byte(usedToken)) == 1 {
		return nil, trace.BadParameter("previously used totp token")
	}

	ctx := context.TODO()
	devs, err := s.Services.GetMFADevices(ctx, user, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, dev := range devs {
		totpDev := dev.GetTotp()
		if totpDev == nil {
			continue
		}

		if err := s.checkTOTP(ctx, user, otpToken, dev); err != nil {
			log.WithError(err).Errorf("Using TOTP device %q", dev.GetName())
			continue
		}
		return dev, nil
	}
	return nil, trace.AccessDenied("invalid totp token")
}

// checkTOTP checks if the TOTP token is valid.
func (s *Server) checkTOTP(ctx context.Context, user, otpToken string, dev *types.MFADevice) error {
	if dev.GetTotp() == nil {
		return trace.BadParameter("checkTOTP called with non-TOTP MFADevice %T", dev.Device)
	}
	// we use totp.ValidateCustom over totp.Validate so we can use
	// a fake clock in tests to get reliable results
	valid, err := totp.ValidateCustom(otpToken, dev.GetTotp().Key, s.clock.Now(), totp.ValidateOpts{
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
	if err := s.UpsertUsedTOTPToken(user, otpToken); err != nil {
		return trace.Wrap(err)
	}

	// Update LastUsed timestamp on the device.
	dev.LastUsed = s.clock.Now()
	if err := s.UpsertMFADevice(ctx, user, dev); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) changeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (types.User, error) {
	// Get cluster configuration and check if local auth is allowed.
	authPref, err := s.GetAuthPreference(ctx)
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
	token, err := s.getResetPasswordToken(ctx, req.TokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.Expiry().Before(s.clock.Now().UTC()) {
		return nil, trace.BadParameter("expired token")
	}

	err = s.changeUserSecondFactor(ctx, req, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := token.GetUser()
	// Delete this token first to minimize the chances
	// of partially updated user with still valid token.
	err = s.deleteUserTokens(ctx, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !reqPasswordless {
		if err := s.UpsertPassword(username, req.GetNewPassword()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	user, err := s.GetUser(username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

func (s *Server) changeUserSecondFactor(ctx context.Context, req *proto.ChangeUserAuthenticationRequest, token types.UserToken) error {
	username := token.GetUser()
	cap, err := s.GetAuthPreference(ctx)
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

	_, err = s.verifyMFARespAndAddDevice(ctx, &newMFADeviceFields{
		username:      token.GetUser(),
		newDeviceName: deviceName,
		tokenID:       token.GetName(),
		deviceResp:    req.GetNewMFARegisterResponse(),
		deviceUsage:   deviceUsage,
	})
	return trace.Wrap(err)
}
