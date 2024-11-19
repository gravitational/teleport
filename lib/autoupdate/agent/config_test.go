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

	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestWriteConfigFiles(t *testing.T) {
	t.Parallel()
	linkDir := t.TempDir()
	dataDir := t.TempDir()
	err := writeConfigFiles(linkDir, dataDir)
	require.NoError(t, err)

	for _, p := range []string{
		filepath.Join(linkDir, serviceDir, updateServiceName),
		filepath.Join(linkDir, serviceDir, updateTimerName),
	} {
		t.Run(filepath.Base(p), func(t *testing.T) {
			data, err := os.ReadFile(p)
			require.NoError(t, err)
			data = replaceValues(data, map[string]string{
				DefaultLinkDir:      linkDir,
				libdefaults.DataDir: dataDir,
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
