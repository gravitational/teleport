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

package webauthncli_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/flynn/hid"
	"github.com/flynn/u2f/u2fhid"
	"github.com/flynn/u2f/u2ftoken"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestLogin(t *testing.T) {
	resetU2FCallbacksAfterTest(t)

	const appID = "https://example.com"
	const rpID = "example.com"
	const username = "llama"
	const origin = appID // URL including protocol

	devUnknown, err := newFakeDevice("unknown" /* name */, "unknown" /* appID */)
	require.NoError(t, err)
	devAppID, err := newFakeDevice("appid" /* name */, appID /* appID */)
	require.NoError(t, err)

	// Create a device that authenticates using the RPID.
	// In practice, it would be registered as a Webauthn device.
	devRPID, err := newFakeDevice("rpid" /* name */, rpID /* appID */)
	require.NoError(t, err)
	pubKeyI, err := x509.ParsePKIXPublicKey(devRPID.mfaDevice.GetU2F().PubKey)
	require.NoError(t, err)
	pubKeyCBOR, err := wanlib.U2FKeyToCBOR(pubKeyI.(*ecdsa.PublicKey))
	require.NoError(t, err)
	devRPID.mfaDevice.Device = &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:  devRPID.key.KeyHandle,
			PublicKeyCbor: pubKeyCBOR,
		},
	}

	// Use a LoginFlow to create and check assertions.
	identity := &fakeIdentity{
		Devices: []*types.MFADevice{
			devUnknown.mfaDevice,
			devAppID.mfaDevice,
			devRPID.mfaDevice,
		},
	}
	loginFlow := &wanlib.LoginFlow{
		U2F: &types.U2F{
			AppID: appID,
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

			assertion, err := loginFlow.Begin(ctx, username, &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
			require.NoError(t, err)
			if test.removeAppID {
				assertion.Response.Extensions = nil
			}

			fakeDevs := &fakeDevices{devs: test.devs}
			fakeDevs.assignU2FCallbacks()

			mfaResp, err := wancli.U2FLogin(ctx, origin, assertion)
			switch hasErr := err != nil; {
			case hasErr != test.wantErr:
				t.Fatalf("Login returned err = %v, wantErr = %v", err, test.wantErr)
			case hasErr:
				return // OK, error expected
			}
			require.NotNil(t, mfaResp.GetWebauthn())
			require.Equal(t, test.wantRawID, mfaResp.GetWebauthn().RawId)

			_, err = loginFlow.Finish(ctx, username, wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()), &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			})
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
	okAssertion, err := loginFlow.Begin(ctx, user, &mfav1.ChallengeExtensions{
		Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		origin       string
		getAssertion func() *wantypes.CredentialAssertion
	}{
		{
			name:   "NOK origin empty",
			origin: "",
			getAssertion: func() *wantypes.CredentialAssertion {
				return okAssertion
			},
		},
		{
			name:   "NOK assertion nil",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				return nil
			},
		},
		{
			name:   "NOK assertion empty",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				return &wantypes.CredentialAssertion{}
			},
		},
		{
			name:   "NOK assertion missing challenge",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				assertion.Response.Challenge = nil
				return assertion
			},
		},
		{
			name:   "NOK assertion missing RPID",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				assertion.Response.RelyingPartyID = ""
				return assertion
			},
		},
		{
			name:   "NOK assertion missing credentials",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
				require.NoError(t, err)
				assertion.Response.AllowedCredentials = nil
				return assertion
			},
		},
		{
			name:   "NOK assertion invalid user verification requirement",
			origin: origin,
			getAssertion: func() *wantypes.CredentialAssertion {
				assertion, err := loginFlow.Begin(ctx, user, &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				})
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

			_, err := wancli.U2FLogin(ctx, test.origin, test.getAssertion())
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func resetU2FCallbacksAfterTest(t *testing.T) {
	oldDevices, oldOpen, oldNewToken := *wancli.U2FDevices, *wancli.U2FOpen, *wancli.U2FNewToken
	oldPollInterval := wancli.DevicePollInterval
	t.Cleanup(func() {
		*wancli.U2FDevices = oldDevices
		*wancli.U2FOpen = oldOpen
		*wancli.U2FNewToken = oldNewToken
		wancli.DevicePollInterval = oldPollInterval
	})
}

type fakeDevices struct {
	devs []*fakeDevice
}

func (f *fakeDevices) assignU2FCallbacks() {
	*wancli.U2FDevices = f.devices
	*wancli.U2FOpen = f.open
	*wancli.U2FNewToken = f.newToken
	wancli.DevicePollInterval = 1 // as tight as possible.
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
	if err := f.checkUserPresent(); err != nil {
		return nil, err // Do no wrap.
	}

	rawResp, err := f.key.AuthenticateRaw(req.Application, req.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var counter uint32
	if err := binary.Read(bytes.NewReader(rawResp[1:5]), binary.BigEndian, &counter); err != nil {
		return nil, err
	}
	sign := rawResp[5:]

	return &u2ftoken.AuthenticateResponse{
		Counter:     counter,
		Signature:   sign,
		RawResponse: rawResp,
	}, nil
}

func (f *fakeDevice) Register(req u2ftoken.RegisterRequest) ([]byte, error) {
	if err := f.checkUserPresent(); err != nil {
		return nil, err // Do no wrap.
	}

	resp, err := f.key.RegisterRaw(req.Application, req.Challenge)
	return resp, trace.Wrap(err)
}

func (f *fakeDevice) checkUserPresent() error {
	// Fail presence tests a few times.
	const minPresenceCounter = 2
	if !f.userPresent || f.presenceCounter < minPresenceCounter {
		f.presenceCounter++
		return u2ftoken.ErrPresenceRequired
	}

	// Allowed.
	f.presenceCounter = 0
	return nil
}

type fakeIdentity struct {
	User        string
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

func (f *fakeIdentity) GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error) {
	if f.User == "" {
		return "", trace.NotFound("not found")
	}
	return f.User, nil
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
