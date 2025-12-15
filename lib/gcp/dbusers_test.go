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

package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestAdjustDatabaseUsername(t *testing.T) {
	t.Parallel()

	gcpPostgresDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "gcp-postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		GCP: types.GCPCloudSQL{
			ProjectID:  "teleport-project-123",
			InstanceID: "pg-instance",
		},
	})
	require.NoError(t, err)

	gcpMySQLDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "gcp-mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
		GCP: types.GCPCloudSQL{
			ProjectID:  "teleport-project-123",
			InstanceID: "mysql-instance",
		},
	})
	require.NoError(t, err)

	nonGCPDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "non-gcp-postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	testCases := []struct {
		name             string
		username         string
		db               types.Database
		expectAdjusted   bool
		expectedUsername string
	}{
		{
			name:             "add suffix to short name",
			username:         "iam-user",
			db:               gcpPostgresDB,
			expectAdjusted:   true,
			expectedUsername: "iam-user@teleport-project-123.iam",
		},
		{
			name:             "add suffix to short name with whitespace",
			username:         "  iam-user  ",
			db:               gcpPostgresDB,
			expectAdjusted:   true,
			expectedUsername: "iam-user@teleport-project-123.iam",
		},
		{
			name:             "do not change long name",
			username:         "iam-user@other-project.iam",
			db:               gcpPostgresDB,
			expectAdjusted:   false,
			expectedUsername: "",
		},
		{
			name:             "ignore empty username",
			username:         "",
			db:               gcpPostgresDB,
			expectAdjusted:   false,
			expectedUsername: "",
		},
		{
			name:             "ignore non-GCP database",
			username:         "alice",
			db:               nonGCPDB,
			expectAdjusted:   false,
			expectedUsername: "",
		},
		{
			name:             "ignore non-Postgres database",
			username:         "iam-user",
			db:               gcpMySQLDB,
			expectAdjusted:   false,
			expectedUsername: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adjusted, newUsername := AdjustDatabaseUsername(tc.username, tc.db)
			require.Equal(t, tc.expectedUsername, newUsername)
			require.Equal(t, tc.expectAdjusted, adjusted)
		})
	}
}
