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

package aaguid

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
)

func TestName(t *testing.T) {
	tests := []struct {
		name   string
		aaguid uuid.UUID
		want   string
	}{
		{
			name:   "known vendor",
			aaguid: uuid.MustParse("ee882879-721c-4913-9775-3dfcce97072a"),
			want:   "YubiKey 5 Series",
		},
		{
			name:   "browser-bound provider",
			aaguid: uuid.MustParse("adce0002-35bc-c60a-648b-0b25f1f05503"),
			want:   "Chrome on Mac",
		},
		{
			// Authenticators that decline to identify their make and model report all zeroes.
			name:   "zero AAGUID",
			aaguid: uuid.Nil,
		},
		{
			name:   "unknown AAGUID",
			aaguid: uuid.MustParse("ffffffff-0000-0000-0000-000000000000"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			name, ok := Name(test.aaguid)
			assert.Equal(t, test.want, name)
			assert.Equal(t, test.want != "", ok)
		})
	}
}

func TestNameFromBytes(t *testing.T) {
	id := uuid.MustParse("ee882879-721c-4913-9775-3dfcce97072a")
	raw, err := id.MarshalBinary()
	require.NoError(t, err, "MarshalBinary failed")

	name, ok := NameFromBytes(raw)
	assert.True(t, ok)
	assert.Equal(t, "YubiKey 5 Series", name)

	// Devices registered before Teleport stored the AAGUID have none at all.
	_, ok = NameFromBytes(nil)
	assert.False(t, ok)

	// Anything that is not 16 bytes is not an AAGUID.
	_, ok = NameFromBytes([]byte{1, 2, 3})
	assert.False(t, ok)
}

// Names are offered as device names, so one over the server's limit would be rejected for any client
// that names a device on the user's behalf. The generator clips them; this holds it to that.
func TestNamesFitDeviceNameLimit(t *testing.T) {
	require.NotEmpty(t, names, "embedded AAGUID name table is empty")

	for id, name := range names {
		assert.LessOrEqualf(t, len(name), defaults.MFADeviceNameMaxLen,
			"name %q for AAGUID %v is over the device name limit", name, id)
	}
}
