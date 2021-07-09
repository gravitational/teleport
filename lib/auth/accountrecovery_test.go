/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"github.com/tstranex/u2f"
)

type testWithCloudModules struct {
	modules.Modules
}

func (m *testWithCloudModules) Features() modules.Features {
	return modules.Features{
		Cloud: true, // Enable cloud feature which is required for account recovery.
	}
}

// TestGenerateUpsertAndVerifyRecoveryCodes tests the following:
//  - generation of recovery codes are of correct format
//  - recovery codes are upserted
//  - recovery codes can be verified and marked used
//  - reusing a used or non-existing token returns error
func TestGenerateUpsertAndVerifyRecoveryCodes(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user := "fake@fake.com"
	rc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	require.Len(t, rc, 3)

	// Test each codes are of correct format and used.
	for _, token := range rc {
		s := strings.Split(token, "-")

		// 9 b/c 1 for prefix, 8 for words.
		require.Len(t, s, 9)
		require.Contains(t, token, "tele-")

		// Test codes match.
		err := srv.Auth().verifyRecoveryCode(ctx, user, []byte(token))
		require.NoError(t, err)
	}

	// Test used codes are marked used.
	recovery, err := srv.Auth().GetRecoveryCodes(ctx, user)
	require.NoError(t, err)
	for _, token := range recovery.GetCodes() {
		require.True(t, token.IsUsed)
	}

	// Test with a used code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))

	// Test with invalid recoery code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte("invalidcode"))
	require.True(t, trace.IsBadParameter(err))

	// Test with non-existing user returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, "doesnotexist", []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))
}

func TestRecoveryCodeEventsEmitted(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	user := "fake@fake.com"

	// Test generated recovery codes event.
	tc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	event := mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeGeneratedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodesGeneratedCode, event.GetCode())

	// Test used recovery code event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.NoError(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUsedCode, event.GetCode())

	// Re-using the same token emits failed event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.Error(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUsedFailureCode, event.GetCode())
}

// TestAddTOTPWithRecoveryTokenAndPassword tests a scenario where
// user has an accout with a password and otp but lost their device
// and wants access to add a new totp device.
func TestAddTOTPWithRecoveryTokenAndPassword(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	// User starts with an account with a password and otp.
	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Get access to begin recovery.
	startToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.RecoverType_RECOVER_TOTP,
	})
	require.NoError(t, err)
	require.Equal(t, ResetPasswordTokenRecoveryStart, startToken.GetSubKind())

	// Get access to add a new totp device.
	approvedToken, err := srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  startToken.GetName(),
		Username: startToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_Password{Password: u.password},
	})
	require.NoError(t, err)
	require.Equal(t, ResetPasswordTokenRecoveryApproved, approvedToken.GetSubKind())

	newOTP, err := getOTPCode(srv, approvedToken.GetName())
	require.NoError(t, err)

	// Add new totp device with existing device name.
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:           approvedToken.GetName(),
		SecondFactorToken: newOTP,
		DeviceName:        "otp",
	})
	require.True(t, trace.IsAlreadyExists(err))

	// Add new totp device with unique device name.
	res2, err := srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:           approvedToken.GetName(),
		SecondFactorToken: newOTP,
		DeviceName:        "new-otp",
	})
	require.NoError(t, err)
	require.Equal(t, res2.Username, u.username)
	require.Len(t, res2.RecoveryCodes, 3)

	// Test new recovery codes work.
	for _, token := range res2.RecoveryCodes {
		_, err = srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
			Username:     u.username,
			RecoveryCode: []byte(token),
		})
		require.NoError(t, err)
	}

	// Test there are 2 mfa devices.
	mfas, err := srv.Auth().GetMFADevices(ctx, u.username)
	require.NoError(t, err)

	deviceNames := make([]string, 0, len(mfas))
	for _, mfa := range mfas {
		deviceNames = append(deviceNames, mfa.GetName())
	}
	require.ElementsMatch(t, []string{"otp", "new-otp"}, deviceNames)

	// Try authenticating with first device.
	newOTP, err = totp.GenerateCode(mfas[0].GetTotp().Key, srv.Clock().Now().Add(30*time.Second))
	require.NoError(t, err)
	_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
		Username: u.username,
		OTP: &OTPCreds{
			Password: u.password,
			Token:    newOTP,
		},
	})
	require.NoError(t, err)

	// Try authenticating with second device.
	newOTP, err = totp.GenerateCode(mfas[1].GetTotp().Key, srv.Clock().Now())
	require.NoError(t, err)
	_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
		Username: u.username,
		OTP: &OTPCreds{
			Password: u.password,
			Token:    newOTP,
		},
	})
	require.NoError(t, err)
}

// TestAddU2FWithRecoveryTokenAndPassword tests a scenario where
// user has an accout with a password and u2f but lost their u2f key
// and user wants access to add a new u2f device.
func TestAddU2FWithRecoveryTokenAndPassword(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	// User starts with an account with a password and u2f.
	u, err := createUserAuthCreds(srv, "u2f")
	require.NoError(t, err)

	// Preserve first u2f key handle.
	chal, err := srv.Auth().GetMFAAuthenticateChallenge(u.username, u.password)
	require.NoError(t, err)
	require.Len(t, chal.U2FChallenges, 1)
	firstChal := chal.U2FChallenges[0]

	// Get access to begin recovery.
	startToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.RecoverType_RECOVER_U2F,
	})
	require.NoError(t, err)

	// Get access to add a new device.
	approvedToken, err := srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  startToken.GetName(),
		Username: startToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_Password{Password: []byte("abc123")},
	})
	require.NoError(t, err)

	// Create a new u2f key.
	u2fRegResp, newU2FKey, err := getMockedU2FAndRegisterRes(srv, approvedToken.GetName())
	require.NoError(t, err)

	// Test adding new u2f device with an already existing device name.
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:             approvedToken.GetName(),
		U2FRegisterResponse: u2fRegResp,
		DeviceName:          "u2f",
	})
	require.True(t, trace.IsAlreadyExists(err))

	// Test adding new u2f device with unique device name
	res, err := srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:             approvedToken.GetName(),
		U2FRegisterResponse: u2fRegResp,
		DeviceName:          "new-u2f",
	})
	require.NoError(t, err)
	require.Len(t, res.RecoveryCodes, 3)

	// There should be 2 mfa devices.
	devices, err := srv.Auth().GetMFADevices(ctx, u.username)
	require.NoError(t, err)
	require.Len(t, devices, 2)

	// Try authenticating with the two u2f devices.
	chal, err = srv.Auth().GetMFAAuthenticateChallenge(u.username, u.password)
	require.NoError(t, err)
	require.Len(t, chal.U2FChallenges, 2)

	var secondChal *u2f.SignRequest
	for _, chal := range chal.U2FChallenges {
		if chal.KeyHandle != firstChal.KeyHandle {
			secondChal = &chal
		} else {
			// Update challenge
			firstChal = chal
		}
	}
	require.NotEmpty(t, secondChal)

	// Test first u2f key.
	signResponse, err := u.u2fKey.SignResponse(&u2f.SignRequest{
		Version:   firstChal.Version,
		Challenge: firstChal.Challenge,
		KeyHandle: firstChal.KeyHandle,
		AppID:     firstChal.AppID,
	})
	require.NoError(t, err)

	_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
		Username: u.username,
		U2F: &U2FSignResponseCreds{
			SignResponse: *signResponse,
		},
	})
	require.NoError(t, err)

	// Test second u2f key.
	signResponse, err = newU2FKey.SignResponse(&u2f.SignRequest{
		Version:   secondChal.Version,
		Challenge: secondChal.Challenge,
		KeyHandle: secondChal.KeyHandle,
		AppID:     secondChal.AppID,
	})
	require.NoError(t, err)

	_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
		Username: u.username,
		U2F: &U2FSignResponseCreds{
			SignResponse: *signResponse,
		},
	})
	require.NoError(t, err)
}

// TestChangePasswordWithRecoveryTokenAndOTP tests a scenario where
// user has an accout with a password and totp but forgot their password and user
// goes through the flow to reset password.
func TestChangePasswordWithRecoveryTokenAndOTP(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	// User starts with an account with a password and totp.
	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Get access to begin recovery.
	startToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.RecoverType_RECOVER_PASSWORD,
	})
	require.NoError(t, err)
	require.Equal(t, ResetPasswordTokenRecoveryStart, startToken.GetSubKind())

	// Get new otp code
	mfas, err := srv.Auth().GetMFADevices(ctx, u.username)
	require.NoError(t, err)

	newOTP, err := totp.GenerateCode(mfas[0].GetTotp().Key, srv.Clock().Now().Add(30*time.Second))
	require.NoError(t, err)

	// Get access to change password.
	approvedToken, err := srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  startToken.GetName(),
		Username: startToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_SecondFactorToken{SecondFactorToken: newOTP},
	})
	require.NoError(t, err)
	require.Equal(t, ResetPasswordTokenRecoveryApproved, approvedToken.GetSubKind())

	// Change password.
	newPassword := []byte("some-new-password")
	res2, err := srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:  approvedToken.GetName(),
		Password: newPassword,
	})
	require.NoError(t, err)
	require.Len(t, res2.RecoveryCodes, 3)

	// Test old password doesn't work.
	err = srv.Auth().checkPasswordWOToken(u.username, u.password)
	require.Error(t, err)

	// Test new password.
	err = srv.Auth().checkPasswordWOToken(u.username, newPassword)
	require.NoError(t, err)
}

// TestChangePasswordWithRecoveryTokenAndU2F tests a scenario where
// user has an accout with a password and u2f key but forgot their password and user
// goes through the flow to reset password.
func TestChangePasswordWithRecoveryTokenAndU2F(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	// User starts with an account with a password and u2f.
	u, err := createUserAuthCreds(srv, "u2f")
	require.NoError(t, err)

	// Get access to start recovery.
	startToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.RecoverType_RECOVER_PASSWORD,
	})
	require.NoError(t, err)

	// Get u2f challenge and sign.
	chal, err := srv.Auth().GetMFAAuthenticateChallengeWithToken(ctx, &proto.GetMFAAuthenticateChallengeWithTokenRequest{
		TokenID: startToken.GetName(),
	})
	require.NoError(t, err)

	u2f, err := u.u2fKey.SignResponse(&u2f.SignRequest{
		Version:   chal.GetU2F()[0].Version,
		Challenge: chal.GetU2F()[0].Challenge,
		KeyHandle: chal.GetU2F()[0].KeyHandle,
		AppID:     chal.GetU2F()[0].AppID,
	})
	require.NoError(t, err)

	// Get access to change password.
	approvedToken, err := srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  startToken.GetName(),
		Username: startToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_U2FSignResponse{U2FSignResponse: &proto.U2FResponse{
			KeyHandle:  u2f.KeyHandle,
			ClientData: u2f.ClientData,
			Signature:  u2f.SignatureData,
		}},
	})
	require.NoError(t, err)

	// Change password.
	newPassword := []byte("some-new-password")
	res2, err := srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:  approvedToken.GetName(),
		Password: newPassword,
	})
	require.NoError(t, err)
	require.Len(t, res2.RecoveryCodes, 3)

	// Test old password doesn't work.
	err = srv.Auth().checkPasswordWOToken(u.username, u.password)
	require.Error(t, err)

	// Test new password.
	err = srv.Auth().checkPasswordWOToken(u.username, newPassword)
	require.NoError(t, err)
}

// TestLockWhenMaxFailedVerifyingRecoveryCode tests that user gets locked from login
// and from further recovery attempts when reaching max recovery attempt from providing
// invalid recovery codes.
func TestLockWhenMaxFailedVerifyingRecoveryCode(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	fakeClock := srv.Clock().(clockwork.FakeClock)

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Trigger max failed recovery attempts.
	for i := 1; i <= defaults.MaxRecoveryAttempts; i++ {
		_, err = srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
			Username:     u.username,
			RecoveryCode: []byte("invalid-code"),
		})
		require.Error(t, err)

		// The third failed attempt should return error.
		if i == defaults.MaxRecoveryAttempts {
			require.Contains(t, err.Error(), AccountRecoveryEmailMarker)
			require.True(t, trace.IsAccessDenied(err))
		}
	}

	// Test user account is locked and recovery attempt is locked.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.False(t, user.GetStatus().LockExpires.IsZero())
	require.False(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())

	// Advance time and make sure we can try recovery again with a valid code this time.
	fakeClock.Advance(defaults.AccountLockInterval)
	_, err = srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
	})
	require.NoError(t, err)

}

// TestLockWhenMaxFailedAuthenticatingWithToken tests if token is deleted and
// user is login locked if users reach max recovery attempt from providing invalid password
// or a second factor.
func TestLockWhenMaxFailedAuthenticatingWithToken(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	resetToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
	})
	require.NoError(t, err)

	// Trigger max failed recovery attempts.
	for i := 1; i <= defaults.MaxRecoveryAttempts; i++ {
		_, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
			TokenID:  resetToken.GetName(),
			Username: resetToken.GetUser(),
			AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_SecondFactorToken{SecondFactorToken: "invalid-token"},
		})
		require.Error(t, err)

		// The third failed attempt should return error.
		if i == defaults.MaxRecoveryAttempts {
			require.Contains(t, err.Error(), AccountRecoveryEmailMarker)
		}
	}

	// Test after lock, token is deleted.
	_, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  resetToken.GetName(),
		Username: resetToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_SecondFactorToken{SecondFactorToken: "invalid-token"},
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Test login is actually locked.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.True(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
	require.False(t, user.GetStatus().LockExpires.IsZero())
}

// TestRecoveryAllowedWithLoginLocked tests a user can still recover if they first
// locked themselves from max failed login attempts. After user successfully changes
// their auth cred, the locks should be reset so user can login immediately after.
func TestRecoveryAllowedWithLoginLocked(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Purposely get login locked.
	for i := 1; i <= defaults.MaxLoginAttempts; i++ {
		_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
			Username: u.username,
			OTP: &OTPCreds{
				Password: u.password,
				Token:    "invalid-token",
			},
		})
		require.Error(t, err)

		if i == defaults.MaxLoginAttempts {
			require.True(t, trace.IsAccessDenied(err))
		}
	}

	// Test login is locked.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.True(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
	require.False(t, user.GetStatus().LockExpires.IsZero())

	// Still allow recovery.
	resetToken, err := srv.Auth().VerifyRecoveryCode(ctx, &proto.VerifyRecoveryCodeRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.RecoverType_RECOVER_PASSWORD,
	})
	require.NoError(t, err)

	// Set up new totp.
	mfas, err := srv.Auth().GetMFADevices(ctx, u.username)
	require.NoError(t, err)

	newOTP, err := totp.GenerateCode(mfas[0].GetTotp().Key, srv.Clock().Now().Add(30*time.Second))
	require.NoError(t, err)

	resetToken, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  resetToken.GetName(),
		Username: resetToken.GetUser(),
		AuthCred: &proto.AuthenticateUserWithRecoveryTokenRequest_SecondFactorToken{SecondFactorToken: newOTP},
	})
	require.NoError(t, err)

	// Recover password to trigger unlock.
	newPassword := []byte("some-new-password")
	res2, err := srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID:  resetToken.GetName(),
		Password: newPassword,
	})
	require.NoError(t, err)
	require.Len(t, res2.RecoveryCodes, 3)

	// Test login locks are removed after successful recovering of password.
	user, err = srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.False(t, user.GetStatus().IsLocked)
	require.True(t, user.GetStatus().LockExpires.IsZero())
	require.True(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
}

// TestInvalidRecoveryTokenTypes test checks are placed to ensure the
// correct type of token is being used.
func TestInvalidRecoveryTokenTypes(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Create a non recovery token (wrong token).
	wrongToken, err := srv.Auth().CreateResetPasswordToken(ctx, CreateResetPasswordTokenRequest{
		Name: u.username,
		Type: ResetPasswordTokenInvite,
	})
	require.NoError(t, err)

	// Test wrong token type for authenticating user.
	_, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  wrongToken.GetName(),
		Username: wrongToken.GetUser(),
	})
	require.Contains(t, err.Error(), "invalid token")

	// Test wrong token type for changing a user auth cred.
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID: wrongToken.GetName(),
	})
	require.Contains(t, err.Error(), "invalid token")

	// Test giving recovery start token to last step in recovery returns error (expects an recovery approved token).
	wrongToken, err = srv.Auth().createRecoveryToken(ctx, u.username, ResetPasswordTokenRecoveryStart, 0)
	require.NoError(t, err)
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID: wrongToken.GetName(),
	})
	require.Contains(t, err.Error(), "invalid token")

	// Test giving recovery approved token to authentication step returns error (expects a recovery start token).
	wrongToken, err = srv.Auth().createRecoveryToken(ctx, u.username, ResetPasswordTokenRecoveryApproved, 0)
	require.NoError(t, err)
	_, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  wrongToken.GetName(),
		Username: wrongToken.GetUser(),
	})
	require.Contains(t, err.Error(), "invalid token")
}

// TestInvalidUserAuthCred tests that checks are placed to ensure the correct
// authentication cred is being changed (password) or added (a new second factor).
func TestInvalidUserAuthCred(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserAuthCreds(srv, "otp")
	require.NoError(t, err)

	// Create a non recovery token (wrong token).
	wrongToken, err := srv.Auth().CreateResetPasswordToken(ctx, CreateResetPasswordTokenRequest{
		Name: u.username,
		Type: ResetPasswordTokenInvite,
	})
	require.NoError(t, err)

	// Test wrong token type for authenticating user.
	_, err = srv.Auth().AuthenticateUserWithRecoveryToken(ctx, &proto.AuthenticateUserWithRecoveryTokenRequest{
		TokenID:  wrongToken.GetName(),
		Username: wrongToken.GetUser(),
	})
	require.Contains(t, err.Error(), "invalid token")

	// Test wrong token type for changing a user auth cred.
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
		TokenID: wrongToken.GetName(),
	})
	require.Contains(t, err.Error(), "invalid token")

	// Test providing password, when token expects totp
	token, err := srv.Auth().createRecoveryToken(ctx, u.username, ResetPasswordTokenRecoveryApproved, types.RecoverType_RECOVER_TOTP)
	require.NoError(t, err)
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{TokenID: token.GetName()})
	require.Contains(t, err.Error(), "second factor token")

	// Test providing u2f, when token expects totp
	token, err = srv.Auth().createRecoveryToken(ctx, u.username, ResetPasswordTokenRecoveryApproved, types.RecoverType_RECOVER_U2F)
	require.NoError(t, err)
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{TokenID: token.GetName()})
	require.Contains(t, err.Error(), "u2f")

	// Test providing otp, when token expects u2f
	token, err = srv.Auth().createRecoveryToken(ctx, u.username, ResetPasswordTokenRecoveryApproved, types.RecoverType_RECOVER_PASSWORD)
	require.NoError(t, err)
	_, err = srv.Auth().RecoverAccountWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{TokenID: token.GetName()})
	require.Contains(t, err.Error(), "password")
}

type userAuthCreds struct {
	recoveryCodes []string
	username      string
	password      []byte
	u2fKey        *mocku2f.Key
}

func createUserAuthCreds(srv *TestTLSServer, secondFactor string) (*userAuthCreds, error) {
	ctx := context.Background()
	username := "fake@fake.com"
	password := []byte("abc123")

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := srv.Auth().SetAuthPreference(ctx, ap); err != nil {
		return nil, trace.Wrap(err)
	}

	_, _, err = CreateUserAndRole(srv.Auth(), username, []string{username})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resetToken, err := srv.Auth().CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res *proto.ChangePasswordWithTokenResponse
	if secondFactor == "otp" {
		otp, err := getOTPCode(srv, resetToken.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err = srv.Auth().ChangePasswordWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
			TokenID:           resetToken.GetName(),
			Password:          password,
			SecondFactorToken: otp,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var u2fKey *mocku2f.Key
	if secondFactor == "u2f" {
		var u2fRegResp *proto.U2FRegisterResponse
		u2fRegResp, u2fKey, err = getMockedU2FAndRegisterRes(srv, resetToken.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err = srv.Auth().ChangePasswordWithToken(ctx, &proto.NewUserAuthCredWithTokenRequest{
			TokenID:             resetToken.GetName(),
			Password:            password,
			U2FRegisterResponse: u2fRegResp,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &userAuthCreds{
		recoveryCodes: res.RecoveryCodes,
		username:      username,
		password:      []byte("abc123"),
		u2fKey:        u2fKey,
	}, nil
}

func getOTPCode(srv *TestTLSServer, tokenID string) (string, error) {
	secrets, err := srv.Auth().RotateResetPasswordTokenSecrets(context.TODO(), tokenID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	otp, err := totp.GenerateCode(secrets.GetOTPKey(), srv.Clock().Now())
	if err != nil {
		return "", trace.Wrap(err)
	}

	return otp, nil
}

func getMockedU2FAndRegisterRes(srv *TestTLSServer, tokenID string) (*proto.U2FRegisterResponse, *mocku2f.Key, error) {
	res, err := srv.Auth().CreateSignupU2FRegisterRequest(tokenID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	u2fKey, err := mocku2f.Create()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	u2fRegResp, err := u2fKey.RegisterResponse(res)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &proto.U2FRegisterResponse{
		RegistrationData: u2fRegResp.RegistrationData,
		ClientData:       u2fRegResp.ClientData,
	}, u2fKey, nil
}
