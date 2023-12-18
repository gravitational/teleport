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

package botfs

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReadWrite attempts to test read/write against all possible symlink
// modes.
func TestReadWrite(t *testing.T) {
	dir := t.TempDir()

	secureWriteExpected := HasSecureWriteSupport()

	expectedData := []byte{1, 2, 3, 4}

	for _, mode := range []SymlinksMode{SymlinksInsecure, SymlinksTrySecure, SymlinksSecure} {
		if mode == SymlinksSecure && !secureWriteExpected {
			t.Logf("skipping secure read/write test due to lack of platform support")
			continue
		}

		path := filepath.Join(dir, string(mode))

		err := Create(path, false, mode)
		require.NoError(t, err)

		err = Write(path, expectedData, mode)
		require.NoError(t, err)

		data, err := Read(path, mode)
		require.NoError(t, err)

		require.Equal(t, 0, bytes.Compare(data, expectedData), "read bytes must be equal to those written")
	}
}
