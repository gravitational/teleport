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

package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func Test_prepareRoles(t *testing.T) {
	selfHostedDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "self-hosted",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	rdsDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)

	redshiftDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5439",
	})
	require.NoError(t, err)

	tests := []struct {
		inputDatabase types.Database
		expectRoles   any
	}{
		{
			inputDatabase: selfHostedDatabase,
			expectRoles:   []string{"role1", "role2"},
		},
		{
			inputDatabase: rdsDatabase,
			expectRoles:   []string{"role1", "role2", "rds_iam"},
		},
		{
			inputDatabase: redshiftDatabase,
			expectRoles:   `["role1","role2"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.inputDatabase.GetName(), func(t *testing.T) {
			sessionCtx := &common.Session{
				Database:      test.inputDatabase,
				DatabaseRoles: []string{"role1", "role2"},
			}

			actualRoles, err := prepareRoles(sessionCtx)
			require.NoError(t, err)
			require.Equal(t, test.expectRoles, actualRoles)
		})
	}
}
