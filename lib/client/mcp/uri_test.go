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
