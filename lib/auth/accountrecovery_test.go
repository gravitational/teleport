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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/u2f"
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
	require.Len(t, rc, numOfRecoveryCodes)

	// Test codes are not marked used.
	recovery, err := srv.Auth().GetRecoveryCodes(ctx, user, true /* withSecrets */)
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
	recovery, err = srv.Auth().GetRecoveryCodes(ctx, user, true /* withSecrets */)
	require.NoError(t, err)
	for _, token := range recovery.GetCodes() {
		require.True(t, token.IsUsed)
	}

	// Test with a used code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(rc[0]))
	require.True(t, trace.IsAccessDenied(err))

	// Test with invalid recovery code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte("invalidcode"))
	require.True(t, trace.IsAccessDenied(err))

	// Test with non-existing user returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, "doesnotexist", []byte(rc[0]))
	require.True(t, trace.IsAccessDenied(err))
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

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Test with no recover type.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
	})
	require.Error(t, err)

	cases := []struct {
		name         string
		recoverType  types.UserTokenUsage
		recoveryCode string
	}{
		{
			name:         "request StartAccountRecovery to recover a MFA",
			recoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
			recoveryCode: u.recoveryCodes[1],
		},
		{
			name:         "request StartAccountRecovery to recover password",
			recoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
			recoveryCode: u.recoveryCodes[2],
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			startToken, err := srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
				Username:     u.username,
				RecoveryCode: []byte(c.recoveryCode),
				RecoverType:  c.recoverType,
			})
			require.NoError(t, err)
			require.Equal(t, UserTokenTypeRecoveryStart, startToken.GetSubKind())
			require.Equal(t, c.recoverType, startToken.GetUsage())
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
		})
	}
}

func TestStartAccountRecovery_WithLock(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Trigger login lock.
	triggerLoginLock(t, srv.Auth(), u.username)

	// Test recovery is still allowed after login lock.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[0]),
		RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
	})
	require.NoError(t, err)

	// Trigger max failed recovery attempts.
	for i := 1; i <= defaults.MaxAccountRecoveryAttempts; i++ {
		_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
			Username: u.username,
		})
		require.True(t, trace.IsAccessDenied(err))

		if i == defaults.MaxAccountRecoveryAttempts {
			require.Equal(t, MaxFailedAttemptsFromStartRecoveryErrMsg, err.Error())
		}
	}

	// Test recovery is denied from attempt recovery lock.
	_, err = srv.Auth().StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
		Username:     u.username,
		RecoveryCode: []byte(u.recoveryCodes[1]),
		RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
	})
	require.True(t, trace.IsAccessDenied(err))
	require.Equal(t, startRecoveryMaxFailedAttemptsErrMsg, err.Error())

	// Test locks have been placed.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.False(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
	require.Equal(t, user.GetStatus().LockExpires, user.GetStatus().RecoveryAttemptLockExpires)
}

func TestStartAccountRecovery_UserErrors(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	cases := []struct {
		desc      string
		expErrMsg string
		req       *proto.StartAccountRecoveryRequest
	}{
		{
			desc:      "username not in valid email format",
			expErrMsg: startRecoveryGenericErrMsg,
			req: &proto.StartAccountRecoveryRequest{
				Username: "malformed-email",
			},
		},
		{
			desc:      "user does not exist",
			expErrMsg: startRecoveryBadAuthnErrMsg,
			req: &proto.StartAccountRecoveryRequest{
				Username: "dne@test.com",
			},
		},
		{
			desc:      "invalid recovery code",
			expErrMsg: startRecoveryBadAuthnErrMsg,
			req: &proto.StartAccountRecoveryRequest{
				Username:     u.username,
				RecoveryCode: []byte("invalid-code"),
			},
		},
		{
			desc:      "missing recover type in request",
			expErrMsg: startRecoveryGenericErrMsg,
			req: &proto.StartAccountRecoveryRequest{
				Username:     u.username,
				RecoveryCode: []byte(u.recoveryCodes[0]),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err = srv.Auth().StartAccountRecovery(ctx, c.req)
			require.True(t, trace.IsAccessDenied(err))
			require.Equal(t, c.expErrMsg, err.Error())
		})
	}
}

func TestApproveAccountRecovery_WithAuthnErrors(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	fakeClock := srv.Clock().(clockwork.FakeClock)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	cases := []struct {
		name           string
		recoverType    types.UserTokenUsage
		invalidReq     *proto.ApproveAccountRecoveryRequest
		createValidReq func(*proto.MFAAuthenticateChallenge) *proto.ApproveAccountRecoveryRequest
	}{
		{
			name:        "authenticate with invalid/valid totp code",
			recoverType: types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
			invalidReq: &proto.ApproveAccountRecoveryRequest{
				AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{Code: "invalid-totp-code"}},
				}},
			},
			createValidReq: func(c *proto.MFAAuthenticateChallenge) *proto.ApproveAccountRecoveryRequest {
				mfaResp, err := u.totpDev.SolveAuthn(c)
				require.NoError(t, err)
				return &proto.ApproveAccountRecoveryRequest{
					AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{
						MFAAuthenticateResponse: mfaResp,
					},
				}
			},
		},
		{
			name:        "authenticate with invalid/valid webauthn response",
			recoverType: types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
			invalidReq: &proto.ApproveAccountRecoveryRequest{
				AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{
					MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: &wantypes.CredentialAssertionResponse{}, // invalid response
						},
					},
				},
			},
			createValidReq: func(c *proto.MFAAuthenticateChallenge) *proto.ApproveAccountRecoveryRequest {
				mfaResp, err := u.webDev.SolveAuthn(c)
				require.NoError(t, err)
				return &proto.ApproveAccountRecoveryRequest{
					AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{
						MFAAuthenticateResponse: mfaResp,
					},
				}
			},
		},
		{
			name:        "authenticate with invalid/valid password",
			recoverType: types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
			invalidReq: &proto.ApproveAccountRecoveryRequest{
				AuthnCred: &proto.ApproveAccountRecoveryRequest_Password{Password: []byte("invalid-password")},
			},
			createValidReq: func(c *proto.MFAAuthenticateChallenge) *proto.ApproveAccountRecoveryRequest {
				return &proto.ApproveAccountRecoveryRequest{
					AuthnCred: &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Acquire a start token.
			startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, c.recoverType)
			require.NoError(t, err)

			// Try a failed attempt, to test it gets cleared later.
			c.invalidReq.Username = u.username
			c.invalidReq.RecoveryStartTokenID = startToken.GetName()
			_, err = srv.Auth().ApproveAccountRecovery(ctx, c.invalidReq)
			require.True(t, trace.IsAccessDenied(err))
			require.Equal(t, approveRecoveryBadAuthnErrMsg, err.Error())

			attempts, err := srv.Auth().GetUserRecoveryAttempts(ctx, u.username)
			require.NoError(t, err)
			require.Len(t, attempts, 1)

			// Get request with authn.
			mfaChallenge, err := srv.Auth().CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
					Username: u.username,
					Password: u.password,
				}},
			})
			require.NoError(t, err)
			req := c.createValidReq(mfaChallenge)
			req.Username = u.username
			req.RecoveryStartTokenID = startToken.GetName()

			// Acquire an approval token with the start token.
			approvedToken, err := srv.Auth().ApproveAccountRecovery(ctx, req)
			require.NoError(t, err)
			require.Equal(t, UserTokenTypeRecoveryApproved, approvedToken.GetSubKind())
			require.Equal(t, c.recoverType.String(), approvedToken.GetUsage().String())

			// Test events emitted.
			event := mockEmitter.LastEvent()
			require.Equal(t, event.GetType(), events.RecoveryTokenCreateEvent)
			require.Equal(t, event.GetCode(), events.RecoveryTokenCreateCode)
			require.Equal(t, event.(*apievents.UserTokenCreate).Name, u.username)

			// Test start token got deleted.
			_, err = srv.Auth().GetUserToken(ctx, startToken.GetName())
			require.True(t, trace.IsNotFound(err))

			// Test expired token.
			fakeClock.Advance(defaults.RecoveryApprovedTokenTTL)
			_, err = srv.Auth().GetUserToken(ctx, approvedToken.GetName())
			require.True(t, trace.IsNotFound(err))

			// Test recovery attempts are deleted.
			attempts, err = srv.Auth().GetUserRecoveryAttempts(ctx, u.username)
			require.NoError(t, err)
			require.Len(t, attempts, 0)
		})
	}
}

func TestApproveAccountRecovery_WithLock(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Acquire a start token.
	startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
	require.NoError(t, err)

	// Trigger login lock.
	triggerLoginLock(t, srv.Auth(), u.username)

	// Test recovery is still allowed after login lock.
	_, err = srv.Auth().ApproveAccountRecovery(ctx, &proto.ApproveAccountRecoveryRequest{
		Username:             u.username,
		RecoveryStartTokenID: startToken.GetName(),
		AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
	})
	require.NoError(t, err)

	// Acquire another start token, as last success would have deleted it.
	startToken, err = srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
	require.NoError(t, err)

	// Trigger max failed recovery attempts.
	for i := 1; i <= defaults.MaxAccountRecoveryAttempts; i++ {
		_, err = srv.Auth().ApproveAccountRecovery(ctx, &proto.ApproveAccountRecoveryRequest{
			RecoveryStartTokenID: startToken.GetName(),
			Username:             u.username,
			AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: []byte("wrong-password")},
		})
		require.True(t, trace.IsAccessDenied(err))

		if i == defaults.MaxAccountRecoveryAttempts {
			require.Equal(t, MaxFailedAttemptsFromApproveRecoveryErrMsg, err.Error())
		}
	}

	// Test start token is deleted from max failed attempts.
	_, err = srv.Auth().ApproveAccountRecovery(ctx, &proto.ApproveAccountRecoveryRequest{
		Username:             u.username,
		RecoveryStartTokenID: startToken.GetName(),
		AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
	})
	require.True(t, trace.IsAccessDenied(err))

	// Test only login is locked.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.True(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
	require.False(t, user.GetStatus().LockExpires.IsZero())

	// Test recovery attempts got reset.
	attempts, err := srv.Auth().GetUserRecoveryAttempts(ctx, u.username)
	require.NoError(t, err)
	require.Len(t, attempts, 0)
}

func TestApproveAccountRecovery_WithErrors(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	cases := []struct {
		name       string
		expErrMsg  string
		getRequest func() *proto.ApproveAccountRecoveryRequest
	}{
		{
			name: "invalid token type",
			getRequest: func() *proto.ApproveAccountRecoveryRequest {
				// Generate an incorrect token type.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: approvedToken.GetName(),
				}
			},
		},
		{
			name:      "token not found",
			expErrMsg: approveRecoveryGenericErrMsg,
			getRequest: func() *proto.ApproveAccountRecoveryRequest {
				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: "non-existent-token-id",
				}
			},
		},
		{
			name:      "username does not match",
			expErrMsg: approveRecoveryBadAuthnErrMsg,
			getRequest: func() *proto.ApproveAccountRecoveryRequest {
				// Acquire a start token.
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startToken.GetName(),
					Username:             "invalid-username",
				}
			},
		},
		{
			name:      "provide password when it expects MFA authn response",
			expErrMsg: approveRecoveryBadAuthnErrMsg,
			getRequest: func() *proto.ApproveAccountRecoveryRequest {
				// Acquire a start token for recovering second factor.
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startToken.GetName(),
					AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: []byte("some-password")},
				}
			},
		},
		{
			name:      "provide MFA authn response when it expects password",
			expErrMsg: approveRecoveryBadAuthnErrMsg,
			getRequest: func() *proto.ApproveAccountRecoveryRequest {
				// Acquire a start token for recovering password.
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startToken.GetName(),
					AuthnCred:            &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{}},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err = srv.Auth().ApproveAccountRecovery(ctx, c.getRequest())
			switch {
			case c.expErrMsg != "":
				require.True(t, trace.IsAccessDenied(err))
				require.Equal(t, c.expErrMsg, err.Error())
			default:
				require.True(t, trace.IsAccessDenied(err))
			}
		})
	}
}

func TestCompleteAccountRecovery(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Test new password with lock that should not affect changing authn.
	triggerLoginLock(t, srv.Auth(), u.username)

	// Acquire an approved token for recovering password.
	approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
	require.NoError(t, err)

	err = srv.Auth().CompleteAccountRecovery(ctx, &proto.CompleteAccountRecoveryRequest{
		RecoveryApprovedTokenID: approvedToken.GetName(),
		NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewPassword{NewPassword: []byte("llamas-are-cool")},
	})
	require.NoError(t, err)

	// Test locks are removed.
	user, err := srv.Auth().GetUser(u.username, false)
	require.NoError(t, err)
	require.False(t, user.GetStatus().IsLocked)
	require.True(t, user.GetStatus().RecoveryAttemptLockExpires.IsZero())
	require.True(t, user.GetStatus().LockExpires.IsZero())

	// Test login attempts are removed.
	attempts, err := srv.Auth().GetUserLoginAttempts(u.username)
	require.NoError(t, err)
	require.Len(t, attempts, 0)

	// Test adding MFA devices.
	approvedToken, err = srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
	require.NoError(t, err)

	cases := []struct {
		name       string
		getRequest func() *proto.CompleteAccountRecoveryRequest
	}{
		{
			name: "add new TOTP device",
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				res, err := srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
					TokenID:    approvedToken.GetName(),
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				})
				require.NoError(t, err)

				otpCode, err := totp.GenerateCode(res.GetTOTP().GetSecret(), srv.Clock().Now())
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-otp",
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{Code: otpCode}},
					}},
				}
			},
		},
		{
			name: "add new U2F device",
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				u2fRegResp, _, err := getLegacyMockedU2FAndRegisterRes(srv.Auth(), approvedToken.GetName())
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-u2f",
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_U2F{U2F: u2fRegResp},
					}},
				}
			},
		},
		{
			name: "add new WEBAUTHN device",
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				_, webauthnRegRes, err := getMockedWebauthnAndRegisterRes(srv.Auth(), approvedToken.GetName())
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-webauthn",
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: webauthnRegRes},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := c.getRequest()

			// Change authentication.
			err = srv.Auth().CompleteAccountRecovery(ctx, req)
			require.NoError(t, err)

			// Test events emitted.
			event := mockEmitter.LastEvent()
			require.Equal(t, event.GetType(), events.MFADeviceAddEvent)
			require.Equal(t, event.GetCode(), events.MFADeviceAddEventCode)
			require.Equal(t, event.(*apievents.MFADeviceAdd).UserMetadata.User, u.username)

			// Test new device has been added.
			mfas, err := srv.Auth().Identity.GetMFADevices(ctx, u.username, false)
			require.NoError(t, err)

			found := false
			for _, mfa := range mfas {
				if mfa.GetName() == req.NewDeviceName {
					found = true
					break
				}
			}
			require.True(t, found, "MFA device %q not found", req.NewDeviceName)
		})
	}
}

func TestCompleteAccountRecovery_WithErrors(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	cases := []struct {
		name           string
		expErrMsg      string
		isDuplicate    bool
		isBadParameter bool
		getRequest     func() *proto.CompleteAccountRecoveryRequest
	}{
		{
			name: "invalid token type",
			// expectErrMsg not supplied on purpose, there is no const err message for this error.
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Generate an incorrect token type.
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: startToken.GetName(),
				}
			},
		},
		{
			name:      "token not found",
			expErrMsg: completeRecoveryGenericErrMsg,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: "non-existent-token-id",
				}
			},
		},
		{
			name:      "provide new password when it expects new MFA register response",
			expErrMsg: completeRecoveryGenericErrMsg,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Acquire an approved token for recovering second factor.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewPassword{NewPassword: []byte("some-new-password")},
				}
			},
		},
		{
			name:      "provide new MFA register response when it expects new password",
			expErrMsg: completeRecoveryGenericErrMsg,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Acquire an approved token for recovering password.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{}},
				}
			},
		},
		{
			name:        "duplicate device name",
			isDuplicate: true,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Acquire an approved token for recovering second factor.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				// Retrieve list of devices to get the name of an existing device.
				devs, err := srv.Auth().Identity.GetMFADevices(ctx, u.username, false)
				require.NoError(t, err)
				require.NotEmpty(t, devs)

				// New u2f register response.
				u2fRegResp, _, err := getLegacyMockedU2FAndRegisterRes(srv.Auth(), approvedToken.GetName())
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewDeviceName:           devs[0].GetName(),
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_U2F{U2F: u2fRegResp},
					}},
				}
			},
		},
		{
			name:           "providing TOTP fields when TOTP is not enabled by auth settings",
			isBadParameter: true,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Acquire an approved token for recovering second factor.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, ap)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{},
					}},
				}
			},
		},
		{
			name:           "providing Webauthn fields when Webauthn is not enabled by auth settings",
			isBadParameter: true,
			getRequest: func() *proto.CompleteAccountRecoveryRequest {
				// Acquire an approved token for recovering second factor.
				approvedToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_MFA)
				require.NoError(t, err)

				ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOTP,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, ap)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedToken.GetName(),
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{
						NewMFAResponse: &proto.MFARegisterResponse{
							Response: &proto.MFARegisterResponse_Webauthn{
								Webauthn: &wantypes.CredentialCreationResponse{},
							},
						},
					},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err = srv.Auth().CompleteAccountRecovery(ctx, c.getRequest())
			switch {
			case c.isDuplicate:
				require.True(t, trace.IsAlreadyExists(err))
			case c.isBadParameter:
				require.True(t, trace.IsBadParameter(err))
			case c.expErrMsg != "":
				require.True(t, trace.IsAccessDenied(err))
				require.Equal(t, c.expErrMsg, err.Error())
			default:
				require.True(t, trace.IsAccessDenied(err))
			}
		})
	}
}

// TestAccountRecoveryFlow tests the recovery flow from start to finish.
func TestAccountRecoveryFlow(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	cases := []struct {
		name               string
		getStartRequest    func(*userAuthCreds) *proto.StartAccountRecoveryRequest
		getApproveRequest  func(*userAuthCreds, *proto.MFAAuthenticateChallenge, string) *proto.ApproveAccountRecoveryRequest
		getCompleteRequest func(*userAuthCreds, string) *proto.CompleteAccountRecoveryRequest
	}{
		{
			name: "recover password with otp",
			getStartRequest: func(u *userAuthCreds) *proto.StartAccountRecoveryRequest {
				return &proto.StartAccountRecoveryRequest{
					Username:     u.username,
					RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
					RecoveryCode: []byte(u.recoveryCodes[0]),
				}
			},
			getApproveRequest: func(u *userAuthCreds, c *proto.MFAAuthenticateChallenge, startTokenID string) *proto.ApproveAccountRecoveryRequest {
				mfaResp, err := u.totpDev.SolveAuthn(c)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startTokenID,
					Username:             u.username,
					AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{
						MFAAuthenticateResponse: mfaResp,
					},
				}
			},
			getCompleteRequest: func(u *userAuthCreds, approvedTokenID string) *proto.CompleteAccountRecoveryRequest {
				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedTokenID,
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewPassword{NewPassword: []byte("new-password-1")},
				}
			},
		},
		{
			name: "recover password with webauthn",
			getStartRequest: func(u *userAuthCreds) *proto.StartAccountRecoveryRequest {
				return &proto.StartAccountRecoveryRequest{
					Username:     u.username,
					RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
					RecoveryCode: []byte(u.recoveryCodes[0]),
				}
			},
			getApproveRequest: func(u *userAuthCreds, c *proto.MFAAuthenticateChallenge, startTokenID string) *proto.ApproveAccountRecoveryRequest {
				mfaResp, err := u.webDev.SolveAuthn(c)
				require.NoError(t, err)

				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startTokenID,
					Username:             u.username,
					AuthnCred: &proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse{
						MFAAuthenticateResponse: mfaResp,
					},
				}
			},
			getCompleteRequest: func(u *userAuthCreds, approvedTokenID string) *proto.CompleteAccountRecoveryRequest {
				return &proto.CompleteAccountRecoveryRequest{
					RecoveryApprovedTokenID: approvedTokenID,
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewPassword{NewPassword: []byte("new-password-2")},
				}
			},
		},
		{
			name: "recover webauthn with password",
			getStartRequest: func(u *userAuthCreds) *proto.StartAccountRecoveryRequest {
				return &proto.StartAccountRecoveryRequest{
					Username:     u.username,
					RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
					RecoveryCode: []byte(u.recoveryCodes[0]),
				}
			},
			getApproveRequest: func(u *userAuthCreds, c *proto.MFAAuthenticateChallenge, startTokenID string) *proto.ApproveAccountRecoveryRequest {
				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startTokenID,
					Username:             u.username,
					AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
				}
			},
			getCompleteRequest: func(u *userAuthCreds, approvedTokenID string) *proto.CompleteAccountRecoveryRequest {
				_, webauthnRegRes, err := getMockedWebauthnAndRegisterRes(srv.Auth(), approvedTokenID)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-webauthn",
					RecoveryApprovedTokenID: approvedTokenID,
					NewAuthnCred:            &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: webauthnRegRes},
				}
			},
		},
		{
			name: "recover u2f with password",
			getStartRequest: func(u *userAuthCreds) *proto.StartAccountRecoveryRequest {
				return &proto.StartAccountRecoveryRequest{
					Username:     u.username,
					RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
					RecoveryCode: []byte(u.recoveryCodes[0]),
				}
			},
			getApproveRequest: func(u *userAuthCreds, c *proto.MFAAuthenticateChallenge, startTokenID string) *proto.ApproveAccountRecoveryRequest {
				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startTokenID,
					Username:             u.username,
					AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
				}
			},
			getCompleteRequest: func(u *userAuthCreds, approvedTokenID string) *proto.CompleteAccountRecoveryRequest {
				u2fRegResp, _, err := getLegacyMockedU2FAndRegisterRes(srv.Auth(), approvedTokenID)
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-u2f",
					RecoveryApprovedTokenID: approvedTokenID,
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_U2F{U2F: u2fRegResp},
					}},
				}
			},
		},
		{
			name: "recover otp with password",
			getStartRequest: func(u *userAuthCreds) *proto.StartAccountRecoveryRequest {
				return &proto.StartAccountRecoveryRequest{
					Username:     u.username,
					RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_MFA,
					RecoveryCode: []byte(u.recoveryCodes[0]),
				}
			},
			getApproveRequest: func(u *userAuthCreds, c *proto.MFAAuthenticateChallenge, startTokenID string) *proto.ApproveAccountRecoveryRequest {
				return &proto.ApproveAccountRecoveryRequest{
					RecoveryStartTokenID: startTokenID,
					Username:             u.username,
					AuthnCred:            &proto.ApproveAccountRecoveryRequest_Password{Password: u.password},
				}
			},
			getCompleteRequest: func(u *userAuthCreds, approvedTokenID string) *proto.CompleteAccountRecoveryRequest {
				res, err := srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
					TokenID:    approvedTokenID,
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				})
				require.NoError(t, err)

				otpCode, err := totp.GenerateCode(res.GetTOTP().GetSecret(), srv.Clock().Now())
				require.NoError(t, err)

				return &proto.CompleteAccountRecoveryRequest{
					NewDeviceName:           "new-otp",
					RecoveryApprovedTokenID: approvedTokenID,
					NewAuthnCred: &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{Code: otpCode}},
					}},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user, err := createUserWithSecondFactors(srv)
			require.NoError(t, err)

			// Step 1: Obtain a start token.
			startToken, err := srv.Auth().StartAccountRecovery(ctx, c.getStartRequest(user))
			require.NoError(t, err)

			// Step 2: Obtain an approval token using the start token.
			mfaChallenge, err := srv.Auth().CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
					Username: user.username,
					Password: user.password,
				}},
			})
			require.NoError(t, err)
			approvedToken, err := srv.Auth().ApproveAccountRecovery(ctx, c.getApproveRequest(user, mfaChallenge, startToken.GetName()))
			require.NoError(t, err)

			// Step 3: Complete recovery with the obtained approved token.
			err = srv.Auth().CompleteAccountRecovery(ctx, c.getCompleteRequest(user, approvedToken.GetName()))
			require.NoError(t, err)
		})
	}
}

func TestGetAccountRecoveryToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	cases := []struct {
		name    string
		wantErr bool
		req     CreateUserTokenRequest
	}{
		{
			name:    "invalid recovery token type",
			wantErr: true,
			req: CreateUserTokenRequest{
				TTL:  5 * time.Minute,
				Type: UserTokenTypeResetPassword,
			},
		},
		{
			name:    "token not found",
			wantErr: true,
			req: CreateUserTokenRequest{
				TTL:  5 * time.Minute,
				Type: "unknown-token-type",
			},
		},
		{
			name: "recovery start token type",
			req: CreateUserTokenRequest{
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryStart,
			},
		},
		{
			name: "recovery approve token type",
			req: CreateUserTokenRequest{
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Create and insert a token.
			token, err := srv.Auth().newUserToken(c.req)
			require.NoError(t, err)
			_, err = srv.Auth().Identity.CreateUserToken(context.Background(), token)
			require.NoError(t, err)

			retToken, err := srv.Auth().GetAccountRecoveryToken(context.Background(), &proto.GetAccountRecoveryTokenRequest{
				RecoveryTokenID: token.GetName(),
			})

			switch {
			case c.wantErr:
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				require.Equal(t, c.req.Type, retToken.GetSubKind())
			}
		})
	}
}

func TestCreateAccountRecoveryCodes(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	// Enable second factors.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)

	err = srv.Auth().SetAuthPreference(ctx, ap)
	require.NoError(t, err)

	cases := []struct {
		name    string
		wantErr bool
		req     CreateUserTokenRequest
	}{
		{
			name:    "invalid token type",
			wantErr: true,
			req: CreateUserTokenRequest{
				Name: "llama@email.com",
				TTL:  5 * time.Minute,
				Type: UserTokenTypeResetPassword,
			},
		},
		{
			name:    "token not found",
			wantErr: true,
			req: CreateUserTokenRequest{
				TTL:  5 * time.Minute,
				Type: "unknown-token-type",
			},
		},
		{
			name:    "invalid user name",
			wantErr: true,
			req: CreateUserTokenRequest{
				Name: "invalid-email",
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
		},
		{
			name: "recovery approved token",
			req: CreateUserTokenRequest{
				Name: "llama@email.com",
				TTL:  5 * time.Minute,
				Type: UserTokenTypeRecoveryApproved,
			},
		},
		{
			name: "privilege token",
			req: CreateUserTokenRequest{
				Name: "llama@email.com",
				TTL:  5 * time.Minute,
				Type: UserTokenTypePrivilege,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token, err := srv.Auth().newUserToken(c.req)
			require.NoError(t, err)

			_, err = srv.Auth().Identity.CreateUserToken(context.Background(), token)
			require.NoError(t, err)

			res, err := srv.Auth().CreateAccountRecoveryCodes(ctx, &proto.CreateAccountRecoveryCodesRequest{
				TokenID: token.GetName(),
			})

			switch {
			case c.wantErr:
				require.True(t, trace.IsAccessDenied(err))

			default:
				require.NoError(t, err)
				require.Len(t, res.GetRecoveryCodes(), numOfRecoveryCodes)

				// Check token is deleted after success.
				_, err = srv.Auth().Identity.GetUserToken(ctx, token.GetName())
				switch {
				case c.req.Type == UserTokenTypeRecoveryApproved:
					require.True(t, trace.IsNotFound(err))
				default:
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestGetAccountRecoveryCodes(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()

	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetModules(&testWithCloudModules{})

	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(u.username))
	require.NoError(t, err)

	rc, err := clt.GetAccountRecoveryCodes(ctx, &proto.GetAccountRecoveryCodesRequest{})
	require.NoError(t, err)
	require.Empty(t, rc.Spec.Codes)
	require.NotEmpty(t, rc.Spec.Created)
}

func triggerLoginLock(t *testing.T, srv *Server, username string) {
	for i := 1; i <= defaults.MaxLoginAttempts; i++ {
		_, err := srv.authenticateUser(context.Background(), AuthenticateUserRequest{
			Username: username,
			OTP:      &OTPCreds{},
		})
		require.True(t, trace.IsAccessDenied(err))
	}
}

type userAuthCreds struct {
	recoveryCodes []string
	username      string
	password      []byte

	totpDev, webDev *TestDevice
}

func createUserWithSecondFactors(srv *TestTLSServer) (*userAuthCreds, error) {
	ctx := context.Background()
	username := fmt.Sprintf("llama%v@goteleport.com", rand.Int())
	password := []byte("abc123")

	// Enable second factors.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "https://localhost",
			Facets: []string{"https://localhost"},
		},
		// Use default Webauthn config.
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

	resetToken, err := srv.Auth().CreateResetPasswordToken(ctx, CreateUserTokenRequest{
		Name: username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Insert a password, u2f device, and recovery codes.
	u2fRegResp, u2fKey, err := getLegacyMockedU2FAndRegisterRes(srv.Auth(), resetToken.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := srv.Auth().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     resetToken.GetName(),
		NewPassword: password,
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_U2F{U2F: u2fRegResp}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := srv.NewClient(TestUser(username))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u2fDev := &TestDevice{Key: u2fKey}
	totpDev, err := RegisterTestDevice(ctx, clt, "otp-1", proto.DeviceType_DEVICE_TYPE_TOTP, u2fDev, WithTestDeviceClock(srv.Clock()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webDev, err := RegisterTestDevice(ctx, clt, "web-1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, u2fDev)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &userAuthCreds{
		username:      username,
		password:      password,
		recoveryCodes: res.GetRecoveryCodes(),
		totpDev:       totpDev,
		webDev:        webDev,
	}, nil
}

// DELETE IN 9.0.0 in favor of getMockedWebauthnAndRegisterRes.
func getLegacyMockedU2FAndRegisterRes(authSrv *Server, tokenID string) (*proto.U2FRegisterResponse, *mocku2f.Key, error) {
	res, err := authSrv.CreateRegisterChallenge(context.Background(), &proto.CreateRegisterChallengeRequest{
		TokenID:    tokenID,
		DeviceType: proto.DeviceType_DEVICE_TYPE_U2F,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	u2fKey, err := mocku2f.Create()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	u2fRegResp, err := u2fKey.RegisterResponse(&u2f.RegisterChallenge{
		Version:   res.GetU2F().GetVersion(),
		Challenge: res.GetU2F().GetChallenge(),
		AppID:     res.GetU2F().GetAppID(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &proto.U2FRegisterResponse{
		RegistrationData: u2fRegResp.RegistrationData,
		ClientData:       u2fRegResp.ClientData,
	}, u2fKey, nil
}

func getMockedWebauthnAndRegisterRes(authSrv *Server, tokenID string) (*TestDevice, *proto.MFARegisterResponse, error) {
	res, err := authSrv.CreateRegisterChallenge(context.Background(), &proto.CreateRegisterChallengeRequest{
		TokenID:    tokenID,
		DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	dev, regRes, err := NewTestDeviceFromChallenge(res)
	return dev, regRes, trace.Wrap(err)
}
