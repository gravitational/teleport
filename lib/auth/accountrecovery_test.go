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
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"

	"github.com/stretchr/testify/require"
)

type testWithCloudModules struct {
	modules.Modules
}

func (m *testWithCloudModules) Features() modules.Features {
	return modules.Features{
		Cloud: true, // Enable cloud feature which is required for account recovery.
	}
}

// TestGenerateAndUpsertRecoveryCodes tests the following:
//  - generation of recovery codes are of correct format
//  - recovery codes are upserted
//  - recovery codes can be verified and marked used
//  - reusing a used or non-existing token returns error
func TestGenerateAndUpsertRecoveryCodes(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user := "fake@fake.com"
	rc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	require.Len(t, rc, 3)

	// Test codes are not marked used.
	recovery, err := srv.Auth().GetRecoveryCodes(ctx, user)
	require.NoError(t, err)
	for _, token := range recovery.GetCodes() {
		require.False(t, token.IsUsed)
	}

	// Test each codes are of correct format and used.
	for _, code := range rc {
		s := strings.Split(code, "-")

		// 9 b/c 1 for prefix, 8 for words.
		require.Len(t, s, 9)
		require.True(t, strings.HasPrefix(code, "tele-"))

		// Test codes match.
		err := srv.Auth().verifyRecoveryCode(ctx, user, []byte(code))
		require.NoError(t, err)
	}

	// Test used codes are marked used.
	recovery, err = srv.Auth().GetRecoveryCodes(ctx, user)
	require.NoError(t, err)
	for _, token := range recovery.GetCodes() {
		require.True(t, token.IsUsed)
	}

	// Test with a used code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))

	// Test with invalid recovery code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte("invalidcode"))
	require.True(t, trace.IsBadParameter(err))

	// Test with non-existing user returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, "doesnotexist", []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))
}

func TestRecoveryCodeEventsEmitted(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	user := "fake@fake.com"

	// Test generated recovery codes event.
	tc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	event := mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeGeneratedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodesGenerateCode, event.GetCode())

	// Test used recovery code event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.NoError(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUseSuccessCode, event.GetCode())

	// Re-using the same token emits failed event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.Error(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUseFailureCode, event.GetCode())
}

func TestStartAccountRecovery(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	fakeClock := srv.Clock().(clockwork.FakeClock)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactorAndRecoveryCodes(srv, "otp")
	require.NoError(t, err)

	// Test with recover type 2FA.
	startToken, err := srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.UserTokenUsage_RECOVER_2FA,
	})
	require.NoError(t, err)
	require.Equal(t, UserTokenTypeRecoveryStart, startToken.GetSubKind())
	require.Equal(t, types.UserTokenUsage_RECOVER_2FA, startToken.GetUsage())
	require.Equal(t, startToken.GetURL(), fmt.Sprintf("https://<proxyhost>:3080/web/recovery/steps/%s/verify", startToken.GetName()))

	// Test token returned correct byte length.
	bytes, err := hex.DecodeString(startToken.GetName())
	require.NoError(t, err)
	require.Len(t, bytes, RecoveryTokenLenBytes)

	// Test expired token.
	fakeClock.Advance(defaults.RecoveryStartTokenTTL)
	_, err = srv.Auth().GetUserToken(ctx, startToken.GetName())
	require.True(t, trace.IsNotFound(err))

	// Test events emitted.
	event := mockEmitter.LastEvent()
	require.Equal(t, event.GetType(), events.RecoveryTokenCreateEvent)
	require.Equal(t, event.GetCode(), events.RecoveryTokenCreateCode)
	require.Equal(t, event.(*apievents.UserTokenCreate).Name, u.username)

	// Test with recover type PWD.
	startToken, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[1]),
		RecoverType:  types.UserTokenUsage_RECOVER_PWD,
	})
	require.NoError(t, err)
	require.Equal(t, types.UserTokenUsage_RECOVER_PWD, startToken.GetUsage())

	// Test with no recover type.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[2]),
	})
	require.Error(t, err)
}

func TestStartAccountRecovery_WithLock(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	fakeClock := srv.Clock().(clockwork.FakeClock)

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactorAndRecoveryCodes(srv, "otp")
	require.NoError(t, err)

	// Test max failed recovery attempt locks both login and further recovery attempt.
	for i := 1; i <= defaults.MaxAccountRecoveryAttempts; i++ {
		_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
			Username: u.username,
		})
		require.Error(t, err)
	}

	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.False(t, user.GetStatus().LockExpires.IsZero())
	require.False(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())

	// Advance time to remove lock and attempts.
	fakeClock.Advance(defaults.AttemptTTL)

	// Trigger login lock.
	for i := 1; i <= defaults.MaxLoginAttempts; i++ {
		_, err = srv.Auth().authenticateUser(ctx, AuthenticateUserRequest{
			Username: u.username,
			OTP:      &OTPCreds{},
		})
		require.Error(t, err)

		if i == defaults.MaxLoginAttempts {
			require.True(t, trace.IsAccessDenied(err))
		}
	}

	// Test recovery is still allowed after login lock.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.UserTokenUsage_RECOVER_2FA,
	})
	require.NoError(t, err)

	// Trigger max failed recovery attempts.
	for i := 1; i <= defaults.MaxAccountRecoveryAttempts; i++ {
		_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
			Username: u.username,
		})
		require.Error(t, err)

		// The third failed attempt should return error.
		if i == defaults.MaxAccountRecoveryAttempts {
			require.EqualValues(t, ErrMaxFailedRecoveryAttempts, err)
		}
	}

	// Test recovery is denied from attempt recovery lock.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[1]),
		RecoverType:  types.UserTokenUsage_RECOVER_2FA,
	})
	require.True(t, trace.IsAccessDenied(err))
}

type userAuthCreds struct {
	recoveryCodes []string
	username      string
	password      []byte
	u2fKey        *mocku2f.Key
}

func createUserWithSecondFactorAndRecoveryCodes(srv *TestTLSServer, secondFactor string) (*userAuthCreds, error) {
	ctx := context.Background()
	username := "fake@fake.com"
	password := []byte("abc123")

	// Enable second factors.
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

	// Insert recovery code for user.
	recoveryCodes, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resetToken, err := srv.Auth().CreateResetPasswordToken(context.TODO(), CreateUserTokenRequest{
		Name: username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add second factor.
	if secondFactor == "otp" {
		otp, err := getOTPCode(srv, resetToken.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = srv.Auth().changeUserSecondFactor(&proto.ChangeUserAuthenticationRequest{
			TokenID: resetToken.GetName(),
			NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
				TOTP: &proto.TOTPRegisterResponse{Code: otp},
			}},
		}, resetToken)
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

		err = srv.Auth().changeUserSecondFactor(&proto.ChangeUserAuthenticationRequest{
			TokenID: resetToken.GetName(),
			NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
				U2F: &proto.U2FRegisterResponse{
					RegistrationData: u2fRegResp.RegistrationData,
					ClientData:       u2fRegResp.ClientData,
				},
			}},
		}, resetToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &userAuthCreds{
		recoveryCodes: recoveryCodes,
		username:      username,
		password:      password,
		u2fKey:        u2fKey,
	}, nil
}

func getOTPCode(srv *TestTLSServer, tokenID string) (string, error) {
	secrets, err := srv.Auth().RotateUserTokenSecrets(context.TODO(), tokenID)
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
