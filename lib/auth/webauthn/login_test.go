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

package webauthn_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestLoginFlow_BeginFinish(t *testing.T) {
	// Simulate a previously registered U2F device.
	u2fKey, err := mocku2f.Create()
	require.NoError(t, err)
	u2fKey.SetCounter(10)                          // Arbitrary
	devAddedAt := time.Now().Add(-5 * time.Minute) // Make sure devAddedAt is in the past.
	u2fDev, err := keyToMFADevice(u2fKey, devAddedAt /* addedAt */, devAddedAt /* lastUsed */)
	require.NoError(t, err)

	// U2F user has a legacy device and no webID.
	const u2fUser = "alpaca"
	u2fIdentity := newFakeIdentity(u2fUser, u2fDev)

	// webUser gets a newly registered device and a webID.
	const webUser = "llama"
	webIdentity := newFakeIdentity(webUser)

	u2fConfig := &types.U2F{AppID: "https://example.com:3080"}
	webConfig := &types.Webauthn{RPID: "example.com"}

	const u2fOrigin = "https://example.com:3080"
	const webOrigin = "https://example.com"
	ctx := context.Background()

	// Register a Webauthn device.
	// Last registration step creates the user webID and adds the new device to
	// identity.
	webKey, err := mocku2f.Create()
	require.NoError(t, err)
	webKey.PreferRPID = true // Webauthn-registered device
	webKey.SetCounter(20)    // Arbitrary, recorded during registration
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: webIdentity,
	}
	cc, err := webRegistration.Begin(ctx, webUser, false /* passwordless */)
	require.NoError(t, err)
	ccr, err := webKey.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
		User:             webUser,
		DeviceName:       "webauthn1",
		CreationResponse: ccr,
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		identity     *fakeIdentity
		user, origin string
		key          *mocku2f.Key
		wantWebID    bool
	}{
		{
			name:     "OK U2F device login",
			identity: u2fIdentity,
			user:     u2fUser,
			origin:   u2fOrigin,
			key:      u2fKey,
		},
		{
			name:      "OK Webauthn device login",
			identity:  webIdentity,
			user:      webUser,
			origin:    webOrigin,
			key:       webKey,
			wantWebID: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity := test.identity
			user := test.user

			webLogin := &wanlib.LoginFlow{
				U2F:      u2fConfig,
				Webauthn: webConfig,
				Identity: test.identity,
			}

			// 1st step of the login ceremony.
			assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
			require.NoError(t, err)
			// We care about a few specific settings, for everything else defaults are
			// OK.
			require.Equal(t, webConfig.RPID, assertion.Response.RelyingPartyID)
			require.Equal(t, u2fConfig.AppID, assertion.Response.Extensions["appid"])
			require.Equal(t, protocol.VerificationDiscouraged, assertion.Response.UserVerification)
			// Did we record the SessionData in storage?
			require.Len(t, identity.SessionData, 1)
			// Did we record the web ID in the SessionData?
			var sd *wantypes.SessionData
			for _, v := range identity.SessionData {
				sd = v // Retrieve without guessing the key
				break
			}
			if test.wantWebID {
				require.NotEmpty(t, sd.UserId)
			} else {
				require.Empty(t, sd.UserId)
			}

			// User interaction would happen here.
			wantCounter := test.key.Counter()
			assertionResp, err := test.key.SignAssertion(test.origin, assertion)
			require.NoError(t, err)

			// 2nd and last step of the login ceremony.
			beforeLastUsed := time.Now().Add(-1 * time.Second)
			loginData, err := webLogin.Finish(ctx, user, assertionResp, &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
			require.NoError(t, err)
			// Last used time and counter are updated.
			require.True(t, beforeLastUsed.Before(loginData.Device.LastUsed))
			require.Equal(t, wantCounter, getSignatureCounter(loginData.Device))
			// Did we update the device in storage?
			require.NotEmpty(t, identity.UpdatedDevices)
			got := identity.UpdatedDevices[len(identity.UpdatedDevices)-1]
			if diff := cmp.Diff(loginData.Device, got); diff != "" {
				t.Errorf("Updated device mismatch (-want +got):\n%s", diff)
			}
			// Did we delete the challenge?
			require.Empty(t, identity.SessionData)
		})
	}
}

func keyToMFADevice(dev *mocku2f.Key, addedAt, lastUsed time.Time) (*types.MFADevice, error) {
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&dev.PrivateKey.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.MFADevice{
		AddedAt:  addedAt,
		LastUsed: lastUsed,
		Device: &types.MFADevice_U2F{
			U2F: &types.U2FDevice{
				KeyHandle: dev.KeyHandle,
				PubKey:    pubKeyDER,
				Counter:   dev.Counter(),
			},
		},
	}, nil
}

func getSignatureCounter(dev *types.MFADevice) uint32 {
	switch d := dev.Device.(type) {
	case *types.MFADevice_U2F:
		return d.U2F.Counter
	case *types.MFADevice_Webauthn:
		return d.Webauthn.SignatureCounter
	default:
		return 0
	}
}

func TestLoginFlow_Begin_errors(t *testing.T) {
	const user = "llama"
	webLogin := wanlib.LoginFlow{
		Webauthn: &types.Webauthn{RPID: "localhost"},
		Identity: newFakeIdentity(user),
	}

	ctx := context.Background()
	tests := []struct {
		name          string
		user          string
		assertErrType func(error) bool
		wantErr       string
	}{
		{
			name:          "NOK empty user",
			assertErrType: trace.IsBadParameter,
			wantErr:       "user required",
		},
		{
			name:          "NOK no registered devices",
			user:          user,
			assertErrType: trace.IsNotFound,
			wantErr:       "no credentials",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := webLogin.Begin(ctx, test.user, &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
			require.True(t, test.assertErrType(err), "got err = %v, want BadParameter", err)
			require.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestLoginFlow_Finish_errors(t *testing.T) {
	ctx := context.Background()
	const user = "llama"
	const webOrigin = "https://localhost"

	webConfig := &types.Webauthn{RPID: "localhost"}
	identity := newFakeIdentity(user)
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: identity,
	}

	key, err := mocku2f.Create()
	require.NoError(t, err)
	key.PreferRPID = true
	cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
	require.NoError(t, err)
	ccr, err := key.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
		User:             user,
		DeviceName:       "webauthn1",
		CreationResponse: ccr,
	})
	require.NoError(t, err)

	webLogin := wanlib.LoginFlow{
		U2F:      &types.U2F{AppID: "https://example.com"},
		Webauthn: webConfig,
		Identity: identity,
	}
	assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
		Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
	})
	require.NoError(t, err)
	okResp, err := key.SignAssertion(webOrigin, assertion)
	require.NoError(t, err)

	tests := []struct {
		name       string
		user       string
		createResp func() *wantypes.CredentialAssertionResponse
	}{
		{
			name:       "NOK empty user",
			user:       "",
			createResp: func() *wantypes.CredentialAssertionResponse { return okResp },
		},
		{
			name:       "NOK nil resp",
			user:       user,
			createResp: func() *wantypes.CredentialAssertionResponse { return nil },
		},
		{
			name:       "NOK empty resp",
			user:       user,
			createResp: func() *wantypes.CredentialAssertionResponse { return &wantypes.CredentialAssertionResponse{} },
		},
		{
			name: "NOK assertion with bad origin",
			user: user,
			createResp: func() *wantypes.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				resp, err := key.SignAssertion("https://badorigin.com", assertion)
				require.NoError(t, err)
				return resp
			},
		},
		{
			name: "NOK assertion with bad RPID",
			user: user,
			createResp: func() *wantypes.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				assertion.Response.RelyingPartyID = "badrpid.com"

				resp, err := key.SignAssertion(webOrigin, assertion)
				require.NoError(t, err)
				return resp
			},
		},
		{
			name: "NOK assertion signed by unknown device",
			user: user,
			createResp: func() *wantypes.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)

				unknownKey, err := mocku2f.Create()
				require.NoError(t, err)
				unknownKey.PreferRPID = true
				unknownKey.IgnoreAllowedCredentials = true

				resp, err := unknownKey.SignAssertion(webOrigin, assertion)
				require.NoError(t, err)
				return resp
			},
		},
		{
			name: "NOK assertion with invalid signature",
			user: user,
			createResp: func() *wantypes.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				// Flip a challenge bit, this should be enough to consistently fail
				// signature checking.
				assertion.Response.Challenge[0] = 1 ^ assertion.Response.Challenge[0]

				resp, err := key.SignAssertion(webOrigin, assertion)
				require.NoError(t, err)
				return resp
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := webLogin.Finish(ctx, test.user, test.createResp(), &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
			require.Error(t, err)
		})
	}
}

func TestPasswordlessFlow_BeginAndFinish(t *testing.T) {
	// Prepare identity and configs.
	const user = "llama"
	identity := newFakeIdentity(user)
	webConfig := &types.Webauthn{RPID: "example.com"}

	const webOrigin = "https://example.com"
	ctx := context.Background()

	// Register a Webauthn device.
	// Last registration step adds the created device to identity.
	webKey, err := mocku2f.Create()
	require.NoError(t, err)
	webKey.IgnoreAllowedCredentials = true // Allowed credentials will be empty
	webKey.SetUV = true                    // Required for passwordless
	webKey.AllowResidentKey = true         // Required for passwordless
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: identity,
	}
	cc, err := webRegistration.Begin(ctx, user, true /* passwordless */)
	require.NoError(t, err)
	ccr, err := webKey.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
		User:             user,
		DeviceName:       "webauthn1",
		CreationResponse: ccr,
		Passwordless:     true,
	})
	require.NoError(t, err)

	webLogin := &wanlib.PasswordlessFlow{
		Webauthn: webConfig,
		Identity: identity,
	}

	tests := []struct {
		name   string
		origin string
		key    *mocku2f.Key
		user   string
	}{
		{
			name:   "OK",
			origin: webOrigin,
			key:    webKey,
			user:   user,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 1st step of the login ceremony.
			assertion, err := webLogin.Begin(ctx)
			require.NoError(t, err)

			// Verify that passwordless settings are correct.
			require.Empty(t, assertion.Response.AllowedCredentials)
			require.Equal(t, protocol.VerificationRequired, assertion.Response.UserVerification)

			// Verify that we recorded user verification requirements in storage.
			require.Len(t, identity.SessionData, 1)
			var sd *wantypes.SessionData
			for _, v := range identity.SessionData {
				sd = v // Get SessionData without guessing the key.
				break
			}
			wantSD := &wantypes.SessionData{
				Challenge:        sd.Challenge,
				UserId:           nil,         // aka unset
				AllowCredentials: [][]uint8{}, // aka unset
				ResidentKey:      false,       // irrelevant for login
				UserVerification: string(protocol.VerificationRequired),
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				},
			}
			if diff := cmp.Diff(wantSD, sd); diff != "" {
				t.Fatalf("SessionData mismatch (-want +got):\n%s", diff)
			}
			// User interaction would happen here.
			assertionResp, err := test.key.SignAssertion(test.origin, assertion)
			require.NoError(t, err)

			// 2nd and last step of the login ceremony.
			loginData, err := webLogin.Finish(ctx, assertionResp)
			require.NoError(t, err)
			require.NotNil(t, loginData.Device)
			require.Equal(t, test.user, loginData.User)
		})
	}
}

func TestPasswordlessFlow_Finish_errors(t *testing.T) {
	const user = "llama"
	const webOrigin = "https://example.com"
	identity := newFakeIdentity(user)
	webConfig := &types.Webauthn{RPID: "example.com"}

	// webKey is an unregistered device.
	webKey, err := mocku2f.Create()
	require.NoError(t, err)
	webKey.IgnoreAllowedCredentials = true // Allowed credentials will be empty
	webKey.SetUV = true                    // Required for passwordless

	ctx := context.Background()
	webLogin := &wanlib.PasswordlessFlow{
		Webauthn: webConfig,
		Identity: identity,
	}

	// Prepare a signed assertion response. The response would be accepted if
	// webKey was previously registered.
	assertion, err := webLogin.Begin(ctx)
	require.NoError(t, err)
	assertionResp, err := webKey.SignAssertion(webOrigin, assertion)
	require.NoError(t, err)

	tests := []struct {
		name          string
		createResp    func() *wantypes.CredentialAssertionResponse
		assertErrType func(error) bool
		wantErrMsg    string
	}{
		{
			name: "NOK response without UserID",
			createResp: func() *wantypes.CredentialAssertionResponse {
				// UserHandle is already nil on assertionResp
				return assertionResp
			},
			assertErrType: trace.IsBadParameter,
			wantErrMsg:    "user handle required",
		},
		{
			name: "NOK unknown user handle",
			createResp: func() *wantypes.CredentialAssertionResponse {
				unknownHandle := make([]byte, 10 /* arbitrary */)
				cp := *assertionResp
				cp.AssertionResponse.UserHandle = unknownHandle
				return &cp
			},
			assertErrType: trace.IsNotFound,
			wantErrMsg:    "not found",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := webLogin.Finish(ctx, test.createResp())
			require.True(t, test.assertErrType(err), "assertErrType failed, err = %v", err)
			require.Contains(t, err.Error(), test.wantErrMsg)
		})
	}
}

// TestCredentialRPID tests the recording of CredentialRpId and scenarios
// related to RPID mismatch.
func TestCredentialRPID(t *testing.T) {
	const origin = "https://example.com"
	const originOther = "https://notexample.com"
	const rpID = "example.com"
	const user = "llama"

	ctx := context.Background()
	identity := newFakeIdentity(user)
	webConfig := &types.Webauthn{RPID: rpID}
	webOtherRP := &types.Webauthn{RPID: "notexample.com"}

	dev1Key, err := mocku2f.Create()
	require.NoError(t, err)

	register := func(config *types.Webauthn, user, origin, deviceName string, key *mocku2f.Key) (*types.MFADevice, error) {
		webRegistration := &wanlib.RegistrationFlow{
			Webauthn: config,
			Identity: identity,
		}

		const passwordless = false
		cc, err := webRegistration.Begin(ctx, user, passwordless)
		if err != nil {
			return nil, err
		}

		ccr, err := key.SignCredentialCreation(origin, cc)
		if err != nil {
			return nil, err
		}

		return webRegistration.Finish(ctx, wanlib.RegisterResponse{
			User:             user,
			DeviceName:       deviceName,
			CreationResponse: ccr,
			Passwordless:     passwordless,
		})
	}

	t.Run("register writes credential RPID", func(t *testing.T) {
		mfaDev, err := register(webConfig, user, origin, "dev1" /* deviceName */, dev1Key)
		require.NoError(t, err, "Registration failed")
		assert.Equal(t, rpID, mfaDev.GetWebauthn().CredentialRpId, "CredentialRpId mismatch")
	})

	// "Reset" all stored CredentialRpIds to simulate devices created before the
	// field existed.
	assert.Len(t, identity.User.GetLocalAuth().MFA, 1, "MFA device count mismatch")
	for _, dev := range identity.User.GetLocalAuth().MFA {
		dev.GetWebauthn().CredentialRpId = ""
	}

	t.Run("login issues challenges for unknown credential RPID", func(t *testing.T) {
		webLogin := &wanlib.LoginFlow{
			Webauthn: webOtherRP, // Wrong RPID!
			Identity: identity,
		}

		_, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		})
		assert.NoError(t, err, "Begin failed, expected assertion for `dev1`")
	})

	t.Run("login writes credential RPID", func(t *testing.T) {
		webLogin := &wanlib.LoginFlow{
			Webauthn: webConfig,
			Identity: identity,
		}

		assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		})
		require.NoError(t, err, "Begin failed")

		car, err := dev1Key.SignAssertion(origin, assertion)
		require.NoError(t, err, "SignAssertion failed")

		loginData, err := webLogin.Finish(ctx, user, car, &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		})
		require.NoError(t, err, "Finish failed")
		assert.Equal(t, rpID, loginData.Device.GetWebauthn().CredentialRpId, "CredentialRpId mismatch")
	})

	t.Run("login doesn't issue challenges for the wrong RPIDs", func(t *testing.T) {
		webLogin := &wanlib.LoginFlow{
			Webauthn: webOtherRP, // Wrong RPID!
			Identity: identity,
		}

		_, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		})
		assert.ErrorIs(t, err, wanlib.ErrInvalidCredentials, "Begin error mismatch")
	})

	t.Run("login issues challenges if at least one device matches", func(t *testing.T) {
		other1Key, err := mocku2f.Create()
		require.NoError(t, err)

		// Register a device for the wrong/new RPID.
		// Storage is now a mix of devices for both RPs.
		_, err = register(webOtherRP, user, originOther, "other1" /* deviceName */, other1Key)
		require.NoError(t, err, "Registration failed")

		webLogin := &wanlib.LoginFlow{
			Webauthn: webOtherRP,
			Identity: identity,
		}
		assertion, err := webLogin.Begin(ctx, user, &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		})
		require.NoError(t, err, "Begin failed, expected assertion for device `other1`")

		// Verify that we got the correct device.
		assert.Len(t, assertion.Response.AllowedCredentials, 1, "AllowedCredentials")
		assert.Equal(t,
			other1Key.KeyHandle,
			assertion.Response.AllowedCredentials[0].CredentialID,
			"Expected key handle for device `other1`")
	})
}

func TestLoginFlow_scopeAndReuse(t *testing.T) {
	// webUser gets a newly registered device and a webID.
	const webUser = "llama"
	webIdentity := newFakeIdentity(webUser)
	webConfig := &types.Webauthn{RPID: "example.com"}

	const webOrigin = "https://example.com"
	ctx := context.Background()

	// Register a Webauthn device.
	// Last registration step creates the user webID and adds the new device to
	// identity.
	webKey, err := mocku2f.Create()
	require.NoError(t, err)
	webKey.PreferRPID = true // Webauthn-registered device
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: webIdentity,
	}
	cc, err := webRegistration.Begin(ctx, webUser, false /* passwordless */)
	require.NoError(t, err)
	ccr, err := webKey.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	device, err := webRegistration.Finish(ctx, wanlib.RegisterResponse{
		User:             webUser,
		DeviceName:       "webauthn1",
		CreationResponse: ccr,
	})
	require.NoError(t, err)

	t.Run("Begin", func(t *testing.T) {
		tests := []struct {
			name         string
			challengeExt *mfav1.ChallengeExtensions
			assertErr    require.ErrorAssertionFunc
		}{
			{
				name:         "NOK challenge extensions not provided",
				challengeExt: nil,
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsBadParameter(err), "expected bad parameter err but got %T", err)
					require.ErrorContains(t, err, "extensions must be supplied")
				},
			},
			{
				name: "NOK reuse not allowed for scope",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsBadParameter(err), "expected bad parameter err but got %T", err)
					require.ErrorContains(t, err, "cannot allow reuse")
				},
			},
			{
				name: "NOK scope PASSWORDLESS_LOGIN not allowed",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
				},
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsBadParameter(err), "expected bad parameter err but got %T", err)
					require.ErrorContains(t, err, "passwordless challenge scope")
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				user := webUser

				webLogin := &wanlib.LoginFlow{
					Webauthn: webConfig,
					Identity: webIdentity,
				}

				_, err := webLogin.Begin(ctx, user, test.challengeExt)
				if test.assertErr != nil {
					test.assertErr(t, err)
					return
				}
				require.NoError(t, err)
			})
		}
	})

	t.Run("Finish", func(t *testing.T) {
		tests := []struct {
			name         string
			challengeExt *mfav1.ChallengeExtensions
			requiredExt  *mfav1.ChallengeExtensions
			assertErr    require.ErrorAssertionFunc
		}{
			{
				name: "NOK required challenge extensions not provided",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				requiredExt: nil,
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsBadParameter(err), "expected bad parameter err but got %T", err)
					require.ErrorContains(t, err, "extensions must be supplied")
				},
			}, {
				name: "NOK scope not satisfied",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsAccessDenied(err), "expected access denied err but got %T", err)
					require.ErrorContains(t, err, "is not satisfied")
				},
			}, {
				// Old clients do not yet provide a scope, so we only enforce scope
				// opportunistically during login finish.
				// TODO(Joerger): DELETE IN v16.0.0 - change to NOK
				name: "OK scope not specified",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
			}, {
				name: "OK scope not required",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED,
				},
			}, {
				name: "OK required scope satisfied",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
			}, {
				name: "NOK reuse requested but not allowed",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				},
				assertErr: func(t require.TestingT, err error, i ...interface{}) {
					require.True(t, trace.IsAccessDenied(err), "expected access denied err but got %T", err)
					require.ErrorContains(t, err, "reuse is not permitted")
				},
			}, {
				name: "OK reuse not requested but allowed",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
			}, {
				name: "OK reuse requested and allowed",
				challengeExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				requiredExt: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				identity := webIdentity
				user := webUser

				webLogin := &wanlib.LoginFlow{
					Webauthn: webConfig,
					Identity: webIdentity,
				}

				assertion, err := webLogin.Begin(ctx, user, test.challengeExt)
				require.NoError(t, err)

				assertionResp, err := webKey.SignAssertion(webOrigin, assertion)
				require.NoError(t, err)

				loginData, err := webLogin.Finish(ctx, user, assertionResp, test.requiredExt)
				if test.assertErr != nil {
					test.assertErr(t, err)
					return
				}

				require.NoError(t, err)
				require.Equal(t, &wanlib.LoginData{
					Device:     device,
					User:       user,
					AllowReuse: loginData.AllowReuse,
				}, loginData)

				// Session data should only be deleted if reuse was not requested on begin.
				if test.challengeExt.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES {
					require.NotEmpty(t, identity.SessionData)
				} else {
					require.Empty(t, identity.SessionData)
				}
			})
		}
	})
}

func TestLoginFlow_userVerification(t *testing.T) {
	// Prepare a user and a pair of registered devices.
	mfaDev, err := mocku2f.Create()
	require.NoError(t, err)

	pwdlessDev, err := mocku2f.Create()
	require.NoError(t, err)
	pwdlessDev.IgnoreAllowedCredentials = true // passwordless settings
	pwdlessDev.SetUV = true
	pwdlessDev.AllowResidentKey = true

	const user = "llama"
	const origin = "https://example.com"
	webIdentity := newFakeIdentity(user)
	webConfig := &types.Webauthn{RPID: "example.com"}

	ctx := context.Background()
	register := func(t *testing.T, dev *mocku2f.Key, rr wanlib.RegisterResponse) {
		webRegistration := &wanlib.RegistrationFlow{
			Webauthn: webConfig,
			Identity: webIdentity,
		}

		cc, err := webRegistration.Begin(ctx, rr.User, rr.Passwordless)
		require.NoError(t, err)

		ccr, err := dev.SignCredentialCreation(origin, cc)
		require.NoError(t, err)
		rr.CreationResponse = ccr

		_, err = webRegistration.Finish(ctx, rr)
		require.NoError(t, err)
	}

	// Register devices. They are "persisted" in the fake identify.
	register(t, mfaDev, wanlib.RegisterResponse{
		User:       user,
		DeviceName: "mfa",
	})
	register(t, pwdlessDev, wanlib.RegisterResponse{
		User:         user,
		DeviceName:   "pwdless",
		Passwordless: true,
	})

	tests := []struct {
		name                      string
		exts, requiredExts        *mfav1.ChallengeExtensions
		dev                       *mocku2f.Key
		wantAssertionVerification string
		wantErr                   string
	}{
		{
			name: "mfaDev fails mismatched UserVerification",
			exts: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				// User verification wrongly not required here!
			},
			requiredExts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationRequired),
			},
			dev:     mfaDev,
			wantErr: "authenticator response",
		},
		{
			name: "pwdlessDev succeeds mismatched UserVerification",
			exts: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				// User verification wrongly not required here!
			},
			requiredExts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationRequired),
			},
			dev: pwdlessDev, // Returns UV=1 regardless of requests.
		},
		{
			name: "verification preferred",
			exts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationPreferred),
			},
			requiredExts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationPreferred),
			},
			dev:                       mfaDev, // Not capable of UV, but still allowed by settings.
			wantAssertionVerification: string(protocol.VerificationPreferred),
		},
		{
			name: "verification required",
			exts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationRequired),
			},
			requiredExts: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				UserVerificationRequirement: string(protocol.VerificationRequired),
			},
			dev:                       pwdlessDev, // Capable of UV.
			wantAssertionVerification: string(protocol.VerificationRequired),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset before test.
			webIdentity.SessionData = make(map[string]*wantypes.SessionData)

			lf := &wanlib.LoginFlow{
				Webauthn: webConfig,
				Identity: webIdentity,
			}

			assertion, err := lf.Begin(ctx, user, test.exts)
			require.NoError(t, err, "lf.Begin")

			if test.wantAssertionVerification != "" {
				// Verify assertion.
				assert.Equal(t,
					test.wantAssertionVerification,
					string(assertion.Response.UserVerification),
					"assertion.Response.UserVerification mismatch")

				// Verify stored session data.
				if assert.Len(t, webIdentity.SessionData, 1, "stored SessionData mismatch") {
					// Verify our single SD instance.
					// We don't care about the key, just the value.
					for _, sd := range webIdentity.SessionData {
						assert.Equal(t,
							test.wantAssertionVerification,
							sd.UserVerification,
							"stored SessionData.UserVerification mismatch")
						break // Only one key anyway.
					}
				}
			}

			assertionResp, err := test.dev.SignAssertion(origin, assertion)
			require.NoError(t, err, "dev.SignAssertion")

			_, err = lf.Finish(ctx, user, assertionResp, test.requiredExts)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "lf.Finish error mismatch")
			} else {
				assert.NoError(t, err, "lf.Finish")
			}
		})
	}
}

type fakeIdentity struct {
	User *types.UserV2
	// MappedUser is used as the reply to GetTeleportUserByWebauthnID.
	// It's automatically assigned when UpsertWebauthnLocalAuth is called.
	MappedUser     string
	UpdatedDevices []*types.MFADevice
	SessionData    map[string]*wantypes.SessionData
}

func newFakeIdentity(user string, devices ...*types.MFADevice) *fakeIdentity {
	return &fakeIdentity{
		User: &types.UserV2{
			Metadata: types.Metadata{
				Name: user,
			},
			Spec: types.UserSpecV2{
				LocalAuth: &types.LocalAuthSecrets{
					MFA: devices,
				},
			},
		},
		SessionData: make(map[string]*wantypes.SessionData),
	}
}

func (f *fakeIdentity) GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
	// Return a defensive copy of the slice, the caller might modify it.
	return slices.Clone(f.User.GetLocalAuth().MFA), nil
}

func (f *fakeIdentity) UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	f.UpdatedDevices = append(f.UpdatedDevices, d)

	// Is this an update?
	for i, dev := range f.User.GetLocalAuth().MFA {
		if dev.Id == d.Id {
			f.User.GetLocalAuth().MFA[i] = dev
			return nil
		}
	}

	// Insert new device.
	f.User.GetLocalAuth().MFA = append(f.User.GetLocalAuth().MFA, d)
	return nil
}

func (f *fakeIdentity) UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error {
	f.User.GetLocalAuth().Webauthn = wla
	f.MappedUser = user
	return nil
}

func (f *fakeIdentity) GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
	wla := f.User.GetLocalAuth().Webauthn
	if wla == nil {
		return nil, trace.NotFound("not found")
	}
	return wla, nil
}

func (f *fakeIdentity) GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error) {
	if f.MappedUser == "" {
		return "", trace.NotFound("not found")
	}
	return f.MappedUser, nil
}

func (f *fakeIdentity) UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error {
	f.SessionData[sessionDataKey(user, sessionID)] = sd
	return nil
}

func (f *fakeIdentity) GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error) {
	sd, ok := f.SessionData[sessionDataKey(user, sessionID)]
	if !ok {
		return nil, trace.NotFound("not found")
	}
	return sd, nil
}

func (f *fakeIdentity) DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error {
	delete(f.SessionData, sessionDataKey(user, sessionID))
	return nil
}

func sessionDataKey(user string, sessionID string) string {
	return fmt.Sprintf("user/%v/%v", user, sessionID)
}

func (f *fakeIdentity) UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wantypes.SessionData) error {
	f.SessionData[globalSessionDataKey(scope, id)] = sd
	return nil
}

func (f *fakeIdentity) GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wantypes.SessionData, error) {
	sd, ok := f.SessionData[globalSessionDataKey(scope, id)]
	if !ok {
		return nil, trace.NotFound("not found")
	}
	return sd, nil
}

func (f *fakeIdentity) DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error {
	delete(f.SessionData, globalSessionDataKey(scope, id))
	return nil
}

func globalSessionDataKey(scope string, id string) string {
	return fmt.Sprintf("global/%v/%v", scope, id)
}
