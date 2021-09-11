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

const currentCounter = 10

func TestLoginFlow_BeginFinish_u2f(t *testing.T) {
	// Let's simulate a previously registered U2F device, without going through
	// the trouble of actually registering it.
	dev, err := mocku2f.Create()
	require.NoError(t, err)
	dev.SetCounter(currentCounter)
	devAddedAt := time.Now().Add(-5 * time.Minute) // Make sure devAddedAt is in the past.
	mfaDev, err := keyToMFADevice(dev, currentCounter-1, devAddedAt /* addedAt */, devAddedAt /* lastUsed */)
	require.NoError(t, err)

	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "llama",
		},
		Spec: types.UserSpecV2{
			LocalAuth: &types.LocalAuthSecrets{
				MFA: []*types.MFADevice{mfaDev},
			},
		},
	}
	identity := &fakeIdentity{
		User:        user,
		SessionData: make(map[string]*wantypes.SessionData),
	}

	u2fConfig := types.U2F{AppID: "https://example.com:3080"}
	webConfig := types.Webauthn{RPID: "example.com"}
	webLogin := wanlib.LoginFlow{
		U2F:      &u2fConfig,
		Webauthn: &webConfig,
		Identity: identity,
	}

	ctx := context.Background()

	// webLogin.Begin and webLogin.Finish are the actual methods under test, the
	// rest is setup/sanity checking.
	assertion, err := webLogin.Begin(ctx, user.GetName())
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
	assertionResp, err := dev.SignAssertion("https://example.com:3080" /* origin */, assertion)
	require.NoError(t, err)

	// webLogin.Finish is the other part of the test, completing the login flow.
	beforeLastUsed := time.Now().Add(-1 * time.Second)
	loginDevice, err := webLogin.Finish(ctx, user.GetName(), assertionResp)
	require.NoError(t, err)
	require.True(t, beforeLastUsed.Before(loginDevice.LastUsed))

	// Last used time and counter are be updated, everything else is equal.
	wantDev, _ := keyToMFADevice(dev, currentCounter, devAddedAt, loginDevice.LastUsed)
	if diff := cmp.Diff(wantDev, loginDevice); diff != "" {
		t.Errorf("Finish() mismatch (-want +got):\n%s", diff)
	}

	// Did we update the device in storage?
	require.Len(t, identity.UpdatedDevices, 1)
	got := identity.UpdatedDevices[0]
	if diff := cmp.Diff(wantDev, got); diff != "" {
		t.Errorf("Updated device mismatch (-want +got):\n%s", diff)
	}

	// Did we delete the challenge?
	require.Empty(t, identity.SessionData)
}

func TestLoginFlow_Begin_errors(t *testing.T) {
	webLogin := wanlib.LoginFlow{
		Webauthn: &types.Webauthn{RPID: "localhost"},
		Identity: &fakeIdentity{},
	}

	ctx := context.Background()
	_, err := webLogin.Begin(ctx, "")
	require.True(t, trace.IsBadParameter(err))
}

func TestLoginFlow_Finish_errors(t *testing.T) {
	key, err := mocku2f.Create()
	require.NoError(t, err)
	now := time.Now()
	device, err := keyToMFADevice(key, 0 /* counter */, now /* addedAt */, now /* lastUsed */)
	require.NoError(t, err)

	webLogin := wanlib.LoginFlow{
		U2F:      &types.U2F{AppID: "https://localhost"},
		Webauthn: &types.Webauthn{RPID: "localhost"},
		Identity: &fakeIdentity{
			User: &types.UserV2{
				Spec: types.UserSpecV2{
					LocalAuth: &types.LocalAuthSecrets{
						MFA: []*types.MFADevice{device},
					},
				},
			},
			SessionData: make(map[string]*wantypes.SessionData),
		},
	}

	ctx := context.Background()
	const user = "llama"
	assertion, err := webLogin.Begin(ctx, user)
	require.NoError(t, err)
	okResp, err := key.SignAssertion("https://localhost", assertion)
	require.NoError(t, err)

	tests := []struct {
		name string
		user string
		resp *wanlib.CredentialAssertionResponse
	}{
		{
			name: "NOK empty user",
			user: "",
			resp: okResp,
		},
		{
			name: "NOK nil resp",
			user: user,
			resp: nil,
		},
		{
			name: "NOK empty resp",
			user: user,
			resp: &wanlib.CredentialAssertionResponse{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := webLogin.Finish(ctx, test.user, test.resp)
			require.Error(t, err)
		})
	}
}

func keyToMFADevice(dev *mocku2f.Key, counter uint32, addedAt, lastUsed time.Time) (*types.MFADevice, error) {
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
				Counter:   counter,
			},
		},
	}, nil
}

type fakeIdentity struct {
	User           *types.UserV2
	UpdatedDevices []*types.MFADevice
	SessionData    map[string]*wantypes.SessionData
}

func (f *fakeIdentity) GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
	return f.User.GetLocalAuth().MFA, nil
}

func (f *fakeIdentity) UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	f.UpdatedDevices = append(f.UpdatedDevices, d)
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
