/*
Copyright 2021 Gravitational, Inc.

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

package webauthn_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestWebauthnGlobalDisable(t *testing.T) {
	ctx := context.Background()

	const user = "llama"
	cfg := &types.Webauthn{
		RPID:     "localhost",
		Disabled: true,
	}
	identity := newFakeIdentity(user)
	loginFlow := &wanlib.LoginFlow{
		Webauthn: cfg,
		Identity: identity,
	}
	registrationFlow := &wanlib.RegistrationFlow{
		Webauthn: cfg,
		Identity: identity,
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "LoginFlow.Begin",
			fn: func() error {
				_, err := loginFlow.Begin(ctx, user)
				return err
			},
		},
		{
			name: "LoginFlow.Finish",
			fn: func() error {
				_, err := loginFlow.Finish(ctx, user, &wanlib.CredentialAssertionResponse{})
				return err
			},
		},
		{
			name: "RegistrationFlow.Begin",
			fn: func() error {
				_, err := registrationFlow.Begin(ctx, user)
				return err
			},
		},
		{
			name: "RegistrationFlow.Finish",
			fn: func() error {
				_, err := registrationFlow.Finish(ctx, user, "devName", &wanlib.CredentialCreationResponse{})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.fn()
			require.Error(t, err)
			require.Contains(t, err.Error(), "webauthn disabled")
		})
	}
}

func TestLoginFlow_BeginFinish(t *testing.T) {
	// Simulate a previously registered U2F device.
	u2fKey, err := mocku2f.Create()
	require.NoError(t, err)
	u2fKey.SetCounter(10)                          // Arbitrary
	devAddedAt := time.Now().Add(-5 * time.Minute) // Make sure devAddedAt is in the past.
	u2fDev, err := keyToMFADevice(u2fKey, devAddedAt /* addedAt */, devAddedAt /* lastUsed */)
	require.NoError(t, err)

	// Prepare identity and configs
	const user = "llama"
	identity := newFakeIdentity(user, u2fDev)
	u2fConfig := &types.U2F{AppID: "https://example.com:3080"}
	webConfig := &types.Webauthn{RPID: "example.com"}

	const u2fOrigin = "https://example.com:3080"
	const webOrigin = "https://example.com"
	ctx := context.Background()

	// Register a Webauthn device.
	// Last registration step adds the created device to identity.
	webKey, err := mocku2f.Create()
	require.NoError(t, err)
	webKey.PreferRPID = true // Webauthn-registered device
	webKey.SetCounter(20)    // Arbitrary, recorded during registration
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: identity,
	}
	cc, err := webRegistration.Begin(ctx, user)
	require.NoError(t, err)
	ccr, err := webKey.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	_, err = webRegistration.Finish(ctx, user, "webauthn1" /* deviceName */, ccr)
	require.NoError(t, err)

	webLogin := &wanlib.LoginFlow{
		U2F:      u2fConfig,
		Webauthn: webConfig,
		Identity: identity,
	}

	tests := []struct {
		name         string
		user, origin string
		key          *mocku2f.Key
	}{
		{
			name:   "OK U2F device login",
			user:   user,
			origin: u2fOrigin,
			key:    u2fKey,
		},
		{
			name:   "OK Webauthn device login",
			user:   user,
			origin: webOrigin,
			key:    webKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 1st step of the login ceremony.
			assertion, err := webLogin.Begin(ctx, user)
			require.NoError(t, err)
			// We care about RPID and AppID, for everything else defaults are OK.
			require.Equal(t, webConfig.RPID, assertion.Response.RelyingPartyID)
			require.Equal(t, u2fConfig.AppID, assertion.Response.Extensions["appid"])
			// Did we record the SessionData in storage?
			require.Len(t, identity.SessionData, 1)
			// Retrieve session data without guessing the "sessionID" component of the
			// key.
			var sd *wantypes.SessionData
			for _, v := range identity.SessionData {
				sd = v
				break
			}
			// Did we create a new web user ID? Was it used?
			webID := identity.User.GetLocalAuth().Webauthn.UserID
			require.NotEmpty(t, webID)
			require.Equal(t, webID, sd.UserId)

			// User interaction would happen here.
			wantCounter := test.key.Counter()
			assertionResp, err := test.key.SignAssertion(test.origin, assertion)
			require.NoError(t, err)

			// 2nd and last step of the login ceremony.
			beforeLastUsed := time.Now().Add(-1 * time.Second)
			loginDevice, err := webLogin.Finish(ctx, user, assertionResp)
			require.NoError(t, err)
			// Last used time and counter are be updated.
			require.True(t, beforeLastUsed.Before(loginDevice.LastUsed))
			require.Equal(t, wantCounter, getSignatureCounter(loginDevice))
			// Did we update the device in storage?
			require.NotEmpty(t, identity.UpdatedDevices)
			got := identity.UpdatedDevices[len(identity.UpdatedDevices)-1]
			if diff := cmp.Diff(loginDevice, got); diff != "" {
				t.Errorf("Updated device mismatch (-want +got):\n%s", diff)
			}
			// Did we delete the challenge?
			require.Empty(t, identity.SessionData)
		})
	}
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
	webLogin := wanlib.LoginFlow{
		Webauthn: &types.Webauthn{RPID: "localhost"},
		Identity: newFakeIdentity("llama" /* user */),
	}

	ctx := context.Background()
	_, err := webLogin.Begin(ctx, "")
	require.True(t, trace.IsBadParameter(err))
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
	cc, err := webRegistration.Begin(ctx, user)
	require.NoError(t, err)
	ccr, err := key.SignCredentialCreation(webOrigin, cc)
	require.NoError(t, err)
	_, err = webRegistration.Finish(ctx, user, "webauthn1" /* deviceName */, ccr)
	require.NoError(t, err)

	webLogin := wanlib.LoginFlow{
		U2F:      &types.U2F{AppID: "https://example.com"},
		Webauthn: webConfig,
		Identity: identity,
	}
	assertion, err := webLogin.Begin(ctx, user)
	require.NoError(t, err)
	okResp, err := key.SignAssertion(webOrigin, assertion)
	require.NoError(t, err)

	tests := []struct {
		name       string
		user       string
		createResp func() *wanlib.CredentialAssertionResponse
	}{
		{
			name:       "NOK empty user",
			user:       "",
			createResp: func() *wanlib.CredentialAssertionResponse { return okResp },
		},
		{
			name:       "NOK nil resp",
			user:       user,
			createResp: func() *wanlib.CredentialAssertionResponse { return nil },
		},
		{
			name:       "NOK empty resp",
			user:       user,
			createResp: func() *wanlib.CredentialAssertionResponse { return &wanlib.CredentialAssertionResponse{} },
		},
		{
			name: "NOK assertion with bad origin",
			user: user,
			createResp: func() *wanlib.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user)
				require.NoError(t, err)
				resp, err := key.SignAssertion("https://badorigin.com", assertion)
				require.NoError(t, err)
				return resp
			},
		},
		{
			name: "NOK assertion with bad RPID",
			user: user,
			createResp: func() *wanlib.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user)
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
			createResp: func() *wanlib.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user)
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
			createResp: func() *wanlib.CredentialAssertionResponse {
				assertion, err := webLogin.Begin(ctx, user)
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
			_, err := webLogin.Finish(ctx, test.user, test.createResp())
			require.Error(t, err)
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

type fakeIdentity struct {
	User           *types.UserV2
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
	return f.User.GetLocalAuth().MFA, nil
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
	return nil
}

func (f *fakeIdentity) GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
	wla := f.User.GetLocalAuth().Webauthn
	if wla == nil {
		return nil, trace.NotFound("not found")
	}
	return wla, nil
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
	return fmt.Sprintf("%v/%v", user, sessionID)
}
