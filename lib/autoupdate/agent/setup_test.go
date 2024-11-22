/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestWriteConfigFiles(t *testing.T) {
	t.Parallel()
	linkDir := t.TempDir()
	ns, err := NewNamespace("")
	require.NoError(t, err)
	err = ns.writeConfigFiles(linkDir)
	require.NoError(t, err)

	nsTest, err := NewNamespace("test")
	require.NoError(t, err)
	err = nsTest.writeConfigFiles(linkDir)
	require.NoError(t, err)

	for _, p := range []string{
		filepath.Join(linkDir, serviceDir, "teleport-update.service"),
		filepath.Join(linkDir, serviceDir, "teleport-update.timer"),
		filepath.Join(linkDir, "teleport", "test", serviceDir, "teleport-update_test.service"),
		filepath.Join(linkDir, "teleport", "test", serviceDir, "teleport-update_test.timer"),
	} {
		t.Run(filepath.Base(p), func(t *testing.T) {
			data, err := os.ReadFile(p)
			require.NoError(t, err)
			data = replaceValues(data, map[string]string{
				DefaultLinkDir: linkDir,
			})
			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

func replaceValues(data []byte, m map[string]string) []byte {
	for k, v := range m {
		data = bytes.ReplaceAll(data, []byte(v),
			[]byte(k))
	}
	return data
}
