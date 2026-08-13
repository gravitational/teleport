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
	"bytes"
	"compress/gzip"
	"io"
	"os"
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

// Go embeds the gzipped table while the web imports the plain JSON, so the two are only ever as
// consistent as the generator that emits them. Hand-editing either one, or regenerating without
// committing both, is caught here rather than by tsh and the browser naming a device differently.
func TestEmbeddedMatchesJSON(t *testing.T) {
	plain, err := os.ReadFile("aaguids.json")
	require.NoError(t, err, "reading aaguids.json failed")

	zr, err := gzip.NewReader(bytes.NewReader(embedded))
	require.NoError(t, err, "opening aaguids.json.gz failed")
	defer zr.Close()

	fromBlob, err := io.ReadAll(zr)
	require.NoError(t, err, "decompressing aaguids.json.gz failed")

	assert.True(t, bytes.Equal(plain, fromBlob),
		"aaguids.json.gz does not decompress to aaguids.json, re-run build.assets/generate-aaguids.sh")
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
