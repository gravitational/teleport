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
	"context"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestRegister(t *testing.T) {
	resetU2FCallbacksAfterTest(t)

	const user = "llama"
	const rpID = "example.com"
	const origin = "https://example.com"

	u2fKey, err := newFakeDevice("u2fkey" /* name */, "" /* appID */)
	require.NoError(t, err)
	registeredKey, err := newFakeDevice("regkey" /* name */, rpID /* appID */)
	require.NoError(t, err)

	// Create a registration flow, we'll use it to both generate credential
	// requests and to validate them.
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
		Identity: &fakeIdentity{
			User: user,
			Devices: []*types.MFADevice{
				// Fake a WebAuthn device record, as U2F devices are not excluded from registration.
				{
					Device: &types.MFADevice_Webauthn{
						Webauthn: &types.WebauthnDevice{
							CredentialId: registeredKey.key.KeyHandle,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	tests := []struct {
		name            string
		devs            []*fakeDevice
		setUserPresence *fakeDevice
		wantErr         bool
		wantRawID       []byte
	}{
		{
			name:            "U2F-compatible registration",
			devs:            []*fakeDevice{u2fKey},
			setUserPresence: u2fKey,
			wantRawID:       u2fKey.key.KeyHandle,
		},
		{
			name:            "Registered key ignored",
			devs:            []*fakeDevice{u2fKey, registeredKey},
			setUserPresence: registeredKey,
			wantErr:         true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 100ms is an eternity when probing devices at 1ns intervals.
			ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			cc, err := webRegistration.Begin(ctx, user, false /* passwordless */)
			require.NoError(t, err)

			// Reset/set presence indicator.
			for _, dev := range test.devs {
				dev.SetUserPresence(false)
			}
			test.setUserPresence.SetUserPresence(true)

			// Replace U2F library functions with our mocked versions.
			fakeDevs := &fakeDevices{devs: test.devs}
			fakeDevs.assignU2FCallbacks()

			resp, err := wancli.U2FRegister(ctx, origin, cc)
			switch hasErr := err != nil; {
			case hasErr != test.wantErr:
				t.Fatalf("Register returned err = %v, wantErr = %v", err, test.wantErr)
			case hasErr: // OK.
				return
			}
			require.Equal(t, test.wantRawID, resp.GetWebauthn().RawId)

			_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
				User:             user,
				DeviceName:       u2fKey.name,
				CreationResponse: wantypes.CredentialCreationResponseFromProto(resp.GetWebauthn()),
			})
			require.NoError(t, err, "server-side registration failed")
		})
	}
}

func TestRegister_errors(t *testing.T) {
	ctx := context.Background()

	const origin = "https://example.com"
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: &types.Webauthn{
			RPID: "example.com",
		},
		Identity: &fakeIdentity{},
	}
	okCC, err := webRegistration.Begin(ctx, "llama" /* user */, false /* passwordless */)
	require.NoError(t, err)

	tests := []struct {
		name    string
		origin  string
		makeCC  func() *wantypes.CredentialCreation
		wantErr string
	}{
		{
			name:    "NOK empty origin",
			origin:  "",
			makeCC:  func() *wantypes.CredentialCreation { return okCC },
			wantErr: "origin",
		},
		{
			name:    "NOK nil credential creation",
			origin:  origin,
			makeCC:  func() *wantypes.CredentialCreation { return nil },
			wantErr: "credential creation required",
		},
		{
			name:    "NOK nil empty creation",
			origin:  origin,
			makeCC:  func() *wantypes.CredentialCreation { return &wantypes.CredentialCreation{} },
			wantErr: "relying party",
		},
		{
			name:   "NOK ES256 algorithm not allowed",
			origin: origin,
			makeCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				var params []wantypes.CredentialParameter
				for _, p := range cp.Response.Parameters {
					if p.Algorithm == webauthncose.AlgES256 {
						continue
					}
					params = append(params, p)
				}
				cp.Response.Parameters = params
				return &cp
			},
			wantErr: "ES256 not allowed",
		},
		{
			name:   "NOK platform attachment required",
			origin: origin,
			makeCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.Platform
				return &cp
			},
			wantErr: "platform",
		},
		{
			name:   "NOK resident key required",
			origin: origin,
			makeCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				rrk := true
				cp.Response.AuthenticatorSelection.RequireResidentKey = &rrk
				return &cp
			},
			wantErr: "resident key",
		},
		{
			name:   "NOK user verification required",
			origin: origin,
			makeCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired
				return &cp
			},
			wantErr: "user verification",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := wancli.U2FRegister(ctx, test.origin, test.makeCC())
			require.Error(t, err)
			require.Contains(t, err.Error(), test.wantErr)
		})
	}
}
