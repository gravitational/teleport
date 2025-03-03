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

package endpoint

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

const testDir = "export-endpoint-test"

func Test_exportEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		expected      string
		initConfigDir func() string
	}{
		{
			name:     "create endpoint file and write value",
			endpoint: "v1/stable/cloud",
			expected: "v1/stable/cloud",
			initConfigDir: func() string {
				tmpDir, err := os.MkdirTemp("", testDir)
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(tmpDir) })
				return tmpDir
			},
		},
		{
			name:     "write value",
			endpoint: "v1/stable/cloud",
			expected: "v1/stable/cloud",
			initConfigDir: func() string {
				tmpDir, err := os.MkdirTemp("", testDir)
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(tmpDir) })

				endpointFile, err := os.Create(path.Join(tmpDir, "endpoint"))
				require.NoError(t, err)
				require.NoError(t, endpointFile.Close())
				return tmpDir
			},
		},
		{
			name:     "endpoint value already configured",
			endpoint: "v1/stable/cloud",
			expected: "existing/endpoint",
			initConfigDir: func() string {
				tmpDir, err := os.MkdirTemp("", testDir)
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(tmpDir) })

				endpointFile, err := os.Create(path.Join(tmpDir, "endpoint"))
				require.NoError(t, err)

				_, err = endpointFile.Write([]byte("existing/endpoint"))
				require.NoError(t, err)
				require.NoError(t, endpointFile.Close())
				return tmpDir
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := tt.initConfigDir()
			appliedEndpoint, err := exportEndpoint(configDir, tt.endpoint)
			require.NoError(t, err)
			require.Equal(t, tt.expected, appliedEndpoint)
		})
	}
}
