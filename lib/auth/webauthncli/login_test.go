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

package webauthncli_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/flynn/hid"
	"github.com/flynn/u2f/u2fhid"
	"github.com/flynn/u2f/u2ftoken"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

func TestLogin(t *testing.T) {
	// Restore package defaults after test.
	oldDevices, oldOpen, oldNewToken := *wancli.U2FDevices, *wancli.U2FOpen, *wancli.U2FNewToken
	oldPollInterval := wancli.DevicePollInterval
	t.Cleanup(func() {
		*wancli.U2FDevices = oldDevices
		*wancli.U2FOpen = oldOpen
		*wancli.U2FNewToken = oldNewToken
		wancli.DevicePollInterval = oldPollInterval
	})
	wancli.DevicePollInterval = 1 // as tight as possible.

	const appID = "https://example.com"
	const rpID = "example.com"
	const username = "llama"
	const origin = appID // URL including protocol

	devUnknown, err := newFakeDevice("unknown" /* name */, "unknown" /* appID */)
	require.NoError(t, err)
	devRPID, err := newFakeDevice("rpid" /* name */, rpID /* appID */)
	require.NoError(t, err)
	devAppID, err := newFakeDevice("appid" /* name */, appID /* appID */)
	require.NoError(t, err)

	// Use a LoginFlow to create and check assertions.
	identity := &fakeIdentity{
		Devices: []*types.MFADevice{
			devUnknown.mfaDevice,
			devRPID.mfaDevice,
			devAppID.mfaDevice,
		},
	}
	loginFlow := &wanlib.LoginFlow{
		U2F: &types.U2F{
			AppID:  appID,
			Facets: []string{appID, rpID},
		},
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
		Identity: identity,
	}

	tests := []struct {
		name            string
		devs            []*fakeDevice
		setUserPresence *fakeDevice
		removeAppID     bool
		wantErr         bool
		wantRawID       []byte
	}{
		{
			name:            "OK U2F login with App ID",
			devs:            []*fakeDevice{devUnknown, devAppID},
			setUserPresence: devAppID,
			wantRawID:       devAppID.key.KeyHandle,
		},
		{
			name:            "OK U2F login with RPID",
			devs:            []*fakeDevice{devUnknown, devRPID},
			setUserPresence: devRPID,
			wantRawID:       devRPID.key.KeyHandle,
		},
		{
			name:            "OK U2F login with both App ID and RPID",
			devs:            []*fakeDevice{devUnknown, devRPID, devAppID},
			setUserPresence: devAppID, // user presence decides the device
			wantRawID:       devAppID.key.KeyHandle,
		},
		{
			name:            "NOK U2F login with unknown App ID",
			devs:            []*fakeDevice{devUnknown},
			setUserPresence: devUnknown, // doesn't matter, App ID won't match.
			wantErr:         true,
		},
		{
			name: "NOK U2F login without user presence",
			devs: []*fakeDevice{devUnknown, devAppID, devRPID},
			// setUserPresent unset, no devices are "tapped".
			wantErr: true,
		},
	}
	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			// Reset/set user presence flags.
			for _, dev := range test.devs {
				dev.SetUserPresence(false)
			}
			test.setUserPresence.SetUserPresence(true)

			assertion, err := loginFlow.Begin(ctx, username)
			require.NoError(t, err)
			if test.removeAppID {
				assertion.Response.Extensions = nil
			}

			fakeDevs := &fakeDevices{devs: test.devs}
			*wancli.U2FDevices = fakeDevs.devices
			*wancli.U2FOpen = fakeDevs.open
			*wancli.U2FNewToken = fakeDevs.newToken

			mfaResp, err := wancli.Login(ctx, origin, assertion)
			switch hasErr := err != nil; {
			case hasErr != test.wantErr:
				t.Fatalf("Login returned err = %v, wantErr = %v", err, test.wantErr)
			case hasErr:
				return // OK, error expected
			}
			require.NotNil(t, mfaResp.GetWebauthn())
			require.Equal(t, test.wantRawID, mfaResp.GetWebauthn().RawId)

			_, err = loginFlow.Finish(ctx, username, wanlib.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()))
			require.NoError(t, err)
		})
	}
}

func TestLogin_errors(t *testing.T) {
	device, err := newFakeDevice("appid" /* name */, "localhost" /* appID */)
	require.NoError(t, err)
	loginFlow := &wanlib.LoginFlow{
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		Identity: &fakeIdentity{
			Devices: []*types.MFADevice{
				device.mfaDevice,
			},
		},
	}

	const user = "llama"
	const origin = "https://localhost"
	ctx := context.Background()
	okAssertion, err := loginFlow.Begin(ctx, user)
	require.NoError(t, err)

	tests := []struct {
		name         string
		origin       string
		getAssertion func() *wanlib.CredentialAssertion
	}{
		{
			name:   "NOK origin empty",
			origin: "",
			getAssertion: func() *wanlib.CredentialAssertion {
				return okAssertion
			},
		},
		{
			name:   "NOK assertion nil",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				return nil
			},
		},
		{
			name:   "NOK assertion empty",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				return &wanlib.CredentialAssertion{}
			},
		},
		{
			name:   "NOK assertion missing challenge",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user)
				require.NoError(t, err)
				assertion.Response.Challenge = nil
				return assertion
			},
		},
		{
			name:   "NOK assertion missing RPID",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user)
				require.NoError(t, err)
				assertion.Response.RelyingPartyID = ""
				return assertion
			},
		},
		{
			name:   "NOK assertion missing credentials",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user)
				require.NoError(t, err)
				assertion.Response.AllowedCredentials = nil
				return assertion
			},
		},
		{
			name:   "NOK assertion invalid user verification requirement",
			origin: origin,
			getAssertion: func() *wanlib.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user)
				require.NoError(t, err)
				assertion.Response.UserVerification = protocol.VerificationRequired
				return assertion
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			_, err := wancli.Login(ctx, test.origin, test.getAssertion())
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

type fakeDevices struct {
	devs []*fakeDevice
}

func (f *fakeDevices) devices() ([]*hid.DeviceInfo, error) {
	infos := make([]*hid.DeviceInfo, len(f.devs))
	for i, dev := range f.devs {
		infos[i] = dev.info
	}
	return infos, nil
}

func (f *fakeDevices) open(info *hid.DeviceInfo) (*u2fhid.Device, error) {
	for _, dev := range f.devs {
		if dev.info == info {
			return dev.dev, nil
		}
	}
	return nil, trace.NotFound("device not found")
}

func (f *fakeDevices) newToken(d u2ftoken.Device) wancli.Token {
	innerDev := d.(*u2fhid.Device)
	for _, dev := range f.devs {
		if dev.dev == innerDev {
			return dev
		}
	}
	panic("device not found")
}

type fakeDevice struct {
	name      string
	appIDHash []byte
	key       *mocku2f.Key

	mfaDevice *types.MFADevice
	info      *hid.DeviceInfo
	dev       *u2fhid.Device

	// presenceCounter is used to simulate u2ftoken.ErrPresenceRequired errors.
	presenceCounter int
	// userPresent true means that the fakeDevice is ready to Authenticate or
	// Register.
	userPresent bool
}

func newFakeDevice(name, appID string) (*fakeDevice, error) {
	key, err := mocku2f.Create()
	if err != nil {
		return nil, err
	}
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&key.PrivateKey.PublicKey)
	if err != nil {
		panic(err)
	}

	mfaDevice := types.NewMFADevice(
		name /* name */, fmt.Sprintf("%X", key.KeyHandle) /* ID */, time.Now() /* addedAt */)
	mfaDevice.Device = &types.MFADevice_U2F{
		U2F: &types.U2FDevice{
			KeyHandle: key.KeyHandle,
			PubKey:    pubKeyDER,
			Counter:   0, // always zeroed for simplicity
		},
	}

	appIDHash := sha256.Sum256([]byte(appID))
	return &fakeDevice{
		name:      name,
		appIDHash: appIDHash[:],
		key:       key,
		mfaDevice: mfaDevice,
		info: &hid.DeviceInfo{
			Path: fmt.Sprintf("%v-%X", appID, key.KeyHandle),
		},
		dev: &u2fhid.Device{},
	}, nil
}

func (f *fakeDevice) SetUserPresence(present bool) {
	if f == nil {
		return
	}
	f.presenceCounter = 0
	f.userPresent = present
}

func (f *fakeDevice) CheckAuthenticate(req u2ftoken.AuthenticateRequest) error {
	// Is this the correct app and key handle?
	if !bytes.Equal(req.Application, f.appIDHash) || !bytes.Equal(req.KeyHandle, f.key.KeyHandle) {
		return u2ftoken.ErrUnknownKeyHandle
	}
	return nil
}

func (f *fakeDevice) Authenticate(req u2ftoken.AuthenticateRequest) (*u2ftoken.AuthenticateResponse, error) {
	// Fail presence tests a few times.
	const minPresenceCounter = 2
	if !f.userPresent || f.presenceCounter < minPresenceCounter {
		f.presenceCounter++
		return nil, u2ftoken.ErrPresenceRequired
	}
	f.presenceCounter = 0

	// Authenticate runs in a lower abstraction level than mocku2f.Key, so let's
	// assemble the data and do the signing ourselves.
	// See
	// https://fidoalliance.org/specs/fido-u2f-v1.2-ps-20170411/fido-u2f-raw-message-formats-v1.2-ps-20170411.html#authentication-response-message-success.
	const userPresenceFlag = 1
	counter := []byte{0, 0, 0, 0} // always zeroed, makes things simpler
	dataToSign := &bytes.Buffer{}
	dataToSign.Write(req.Application)
	dataToSign.WriteByte(userPresenceFlag)
	dataToSign.Write(counter)
	dataToSign.Write(req.Challenge)
	dataHash := sha256.Sum256(dataToSign.Bytes())

	signature, err := f.key.PrivateKey.Sign(rand.Reader, dataHash[:], crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawResponse := &bytes.Buffer{}
	rawResponse.WriteByte(userPresenceFlag)
	rawResponse.Write(counter)
	rawResponse.Write(signature)

	return &u2ftoken.AuthenticateResponse{
		Counter:     0,
		Signature:   signature,
		RawResponse: rawResponse.Bytes(),
	}, nil
}

func (f *fakeDevice) Register(req u2ftoken.RegisterRequest) ([]byte, error) {
	// Unused by tests.
	panic("unimplemented")
}

type fakeIdentity struct {
	Devices     []*types.MFADevice
	LocalAuth   *types.WebauthnLocalAuth
	SessionData *wantypes.SessionData
}

func (f *fakeIdentity) UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error {
	f.LocalAuth = wla
	return nil
}

func (f *fakeIdentity) GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
	if f.LocalAuth == nil {
		return nil, trace.NotFound("not found") // code relies on not found to work properly
	}
	return f.LocalAuth, nil
}

func (f *fakeIdentity) GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
	return f.Devices, nil
}

func (f *fakeIdentity) UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	// Unimportant for the tests here.
	return nil
}

func (f *fakeIdentity) UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error {
	f.SessionData = sd
	return nil
}

func (f *fakeIdentity) GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error) {
	return f.SessionData, nil
}

func (f *fakeIdentity) DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error {
	f.SessionData = nil
	return nil
}
