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

package ui

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMakeMFADevices_exposesAAGUID(t *testing.T) {
	// Google Password Manager, one of the AAGUIDs the web UI resolves to a name and icon.
	const gpmAAGUID = "ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4"

	gpmBytes, err := uuid.MustParse(gpmAAGUID).MarshalBinary()
	require.NoError(t, err, "MarshalBinary failed")

	totp, err := types.NewMFADevice("t", "id4", time.Now(), &types.MFADevice_Totp{
		Totp: &types.TOTPDevice{Key: "secret"},
	})
	require.NoError(t, err, "NewMFADevice failed")

	out := MakeMFADevices([]*types.MFADevice{
		newWebauthnDevice(t, "k", "id1", gpmBytes),
		// An authenticator declining to identify its model reports an all-zero AAGUID, which carries no naming information.
		newWebauthnDevice(t, "z", "id2", make([]byte, 16)),
		// Devices registered before Teleport stored the AAGUID have none at all.
		newWebauthnDevice(t, "e", "id3", nil),
		totp,
	})

	require.Equal(t, gpmAAGUID, out[0].AAGUID)
	require.Empty(t, out[1].AAGUID)
	require.Empty(t, out[2].AAGUID)
	require.Empty(t, out[3].AAGUID)
}

func newWebauthnDevice(t *testing.T, name, id string, aaguid []byte) *types.MFADevice {
	t.Helper()

	dev, err := types.NewMFADevice(name, id, time.Now(), &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:  []byte{1},
			PublicKeyCbor: []byte{2},
			Aaguid:        aaguid,
		},
	})
	require.NoError(t, err, "NewMFADevice failed")

	return dev
}
