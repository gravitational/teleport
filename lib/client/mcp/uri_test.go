// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatabaseResourceURI(t *testing.T) {
	for name, tc := range map[string]struct {
		uri                  string
		expectError          bool
		expectedDatabase     bool
		expectedServiceName  string
		expectedDatabaseName string
		expectedDatabaseUser string
	}{
		"valid database": {
			uri:                  "teleport://databases/pg?dbName=database&dbUser=user",
			expectedDatabase:     true,
			expectedServiceName:  "pg",
			expectedDatabaseName: "database",
			expectedDatabaseUser: "user",
		},
		"valid database without params": {
			uri:                  "teleport://databases/pg",
			expectedDatabase:     true,
			expectedServiceName:  "pg",
			expectedDatabaseName: "",
			expectedDatabaseUser: "",
		},
		"random resource": {
			uri:                  "teleport://random/random-resource",
			expectedDatabase:     false,
			expectedServiceName:  "",
			expectedDatabaseName: "",
			expectedDatabaseUser: "",
		},
		"invalid schema": {
			uri:         "http://databases/database",
			expectError: true,
		},
		"invalid uri": {
			uri:         "random-value",
			expectError: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			uri, err := ParseResourceURI(tc.uri)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NotNil(t, uri)
			require.Equal(t, tc.expectedDatabase, uri.IsDatabase())
			require.Equal(t, tc.expectedServiceName, uri.GetDatabaseServiceName())
			require.Equal(t, tc.expectedDatabaseName, uri.GetDatabaseName())
			require.Equal(t, tc.expectedDatabaseUser, uri.GetDatabaseUser())
		})
	}
}
