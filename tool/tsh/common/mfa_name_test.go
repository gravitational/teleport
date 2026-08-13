/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestDefaultDeviceName(t *testing.T) {
	// Google Password Manager, one of the AAGUIDs the generated table names.
	gpmAAGUID, err := uuid.MustParse("ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4").MarshalBinary()
	require.NoError(t, err, "MarshalBinary failed")

	tests := []struct {
		name         string
		devType      string
		aaguid       []byte
		passwordless bool
		existing     []string
		want         string
	}{
		{
			name:    "named after the authenticator",
			devType: webauthnDeviceType,
			aaguid:  gpmAAGUID,
			want:    "Google Password Manager",
		},
		{
			// Authenticators that decline to identify their make and model report all zeroes, leaving
			// only what the credential is for to name it after.
			name:    "unidentified authenticator registering an MFA device",
			devType: webauthnDeviceType,
			want:    "Security key",
		},
		{
			name:         "unidentified authenticator registering a passkey",
			devType:      webauthnDeviceType,
			passwordless: true,
			want:         "Passkey",
		},
		{
			// Touch ID reports no AAGUID, but the user asked for it by name.
			name:         "touch ID",
			devType:      touchIDDeviceType,
			passwordless: true,
			want:         "Touch ID",
		},
		{
			// Every credential from one authenticator resolves to the same name, so the second one
			// registered has to be told apart from the first.
			name:     "counter appended when the name is taken",
			devType:  webauthnDeviceType,
			aaguid:   gpmAAGUID,
			existing: []string{"Google Password Manager"},
			want:     "Google Password Manager (2)",
		},
		{
			name:     "counter skips names already taken",
			devType:  webauthnDeviceType,
			aaguid:   gpmAAGUID,
			existing: []string{"Google Password Manager", "google password manager (2)"},
			want:     "Google Password Manager (3)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := signRegistration(t, test.aaguid, test.passwordless)

			name := defaultDeviceName(
				t.Context(), newFakeDeviceLister(t, test.existing...), resp, test.devType, test.passwordless)

			require.Equal(t, test.want, name)
			require.LessOrEqual(t, len(name), defaults.MFADeviceNameMaxLen)
		})
	}
}

// The caller prompts for a name when no default can be settled on, so both give-up paths report it the
// same way: an empty name, never an error.
func TestDefaultDeviceName_noDefault(t *testing.T) {
	gpmAAGUID, err := uuid.MustParse("ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4").MarshalBinary()
	require.NoError(t, err, "MarshalBinary failed")

	resp := signRegistration(t, gpmAAGUID, false /* passwordless */)

	t.Run("every candidate name is taken", func(t *testing.T) {
		taken := []string{"Google Password Manager"}
		for n := 2; n <= maxDefaultNameAttempts; n++ {
			taken = append(taken, fmt.Sprintf("Google Password Manager (%d)", n))
		}

		name := defaultDeviceName(
			t.Context(), newFakeDeviceLister(t, taken...), resp, webauthnDeviceType, false /* passwordless */)
		require.Empty(t, name)
	})

	t.Run("the device list cannot be read", func(t *testing.T) {
		// Registration has already happened by this point, so a failure here is not worth undoing it.
		name := defaultDeviceName(
			t.Context(), failingDeviceLister{}, resp, webauthnDeviceType, false /* passwordless */)
		require.Empty(t, name)
	})
}

// signRegistration runs a registration ceremony against a mock authenticator reporting the given
// AAGUID, so the response carries a real attestation object for the naming code to read.
func signRegistration(t *testing.T, aaguid []byte, passwordless bool) *proto.MFARegisterResponse {
	t.Helper()

	key, err := mocku2f.Create()
	require.NoError(t, err, "Create failed")
	key.AAGUID = aaguid
	key.SetUV = passwordless
	key.AllowResidentKey = passwordless

	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge:    []byte("challenge"),
			RelyingParty: wantypes.RelyingPartyEntity{ID: "localhost"},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
		},
	}

	ccr, err := key.SignCredentialCreation("https://localhost", cc)
	require.NoError(t, err, "SignCredentialCreation failed")

	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
		},
	}
}

type fakeDeviceLister struct {
	devices []*types.MFADevice
}

func newFakeDeviceLister(t *testing.T, names ...string) *fakeDeviceLister {
	t.Helper()

	lister := &fakeDeviceLister{}
	for _, name := range names {
		dev, err := types.NewMFADevice(name, uuid.NewString(), time.Now(), &types.MFADevice_Webauthn{
			Webauthn: &types.WebauthnDevice{
				CredentialId:  []byte(name),
				PublicKeyCbor: []byte{1},
			},
		})
		require.NoError(t, err, "NewMFADevice failed")

		lister.devices = append(lister.devices, dev)
	}

	return lister
}

func (f *fakeDeviceLister) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	return &proto.GetMFADevicesResponse{Devices: f.devices}, nil
}

type failingDeviceLister struct{}

func (failingDeviceLister) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	return nil, trace.ConnectionProblem(nil, "auth is unreachable")
}
