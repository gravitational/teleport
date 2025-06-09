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
	"fmt"
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
		expectedClusterName  string
	}{
		"valid database": {
			uri:                  "teleport://clusters/default/databases/pg?dbName=database&dbUser=user",
			expectedDatabase:     true,
			expectedServiceName:  "pg",
			expectedDatabaseName: "database",
			expectedDatabaseUser: "user",
			expectedClusterName:  "default",
		},
		"valid database without params": {
			uri:                  "teleport://clusters/default/databases/pg",
			expectedDatabase:     true,
			expectedServiceName:  "pg",
			expectedDatabaseName: "",
			expectedDatabaseUser: "",
			expectedClusterName:  "default",
		},
		"random resource": {
			uri:                  "teleport://clusters/default/random/random-resource",
			expectedDatabase:     false,
			expectedServiceName:  "",
			expectedDatabaseName: "",
			expectedDatabaseUser: "",
			expectedClusterName:  "default",
		},
		"generated uri": {
			uri:                  NewDatabaseResourceURI("default", "db").String(),
			expectedDatabase:     true,
			expectedServiceName:  "db",
			expectedDatabaseName: "",
			expectedDatabaseUser: "",
			expectedClusterName:  "default",
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
			fmt.Println(tc.uri)
			require.Equal(t, tc.expectedDatabase, IsDatabaseResourceURI(tc.uri))
			require.Equal(t, tc.expectedDatabase, uri.IsDatabase())
			require.Equal(t, tc.expectedServiceName, uri.GetDatabaseServiceName())
			require.Equal(t, tc.expectedDatabaseName, uri.GetDatabaseName())
			require.Equal(t, tc.expectedDatabaseUser, uri.GetDatabaseUser())
			require.Equal(t, tc.expectedClusterName, uri.GetClusterName())
		})
	}
}
