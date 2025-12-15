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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/srv/db/common/permissions"
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

func TestCheckPgPermission(t *testing.T) {
	tests := []struct {
		name     string
		perm     string
		objKind  string
		checkErr require.ErrorAssertionFunc
	}{
		{
			name:     "valid permission",
			perm:     "SELECT",
			objKind:  databaseobjectimportrule.ObjectKindTable,
			checkErr: require.NoError,
		},
		{
			name:     "whitespace trimmed",
			perm:     "  SELECT   ",
			objKind:  databaseobjectimportrule.ObjectKindTable,
			checkErr: require.NoError,
		},
		{
			name:     "case-insensitive",
			perm:     "seLEct",
			objKind:  databaseobjectimportrule.ObjectKindTable,
			checkErr: require.NoError,
		},
		{
			name:    "invalid permission",
			perm:    "INVALID",
			objKind: databaseobjectimportrule.ObjectKindTable,
			checkErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "unrecognized \"table\" Postgres permission: \"INVALID\"")
			},
		},
		{
			name:    "multiple permissions not allowed",
			perm:    "SELECT, UPDATE",
			objKind: databaseobjectimportrule.ObjectKindTable,
			checkErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "unrecognized \"table\" Postgres permission: \"SELECT, UPDATE\"")
			},
		},
		{
			name:     "permissions for unknown object kinds are ignored",
			perm:     "invalid",
			objKind:  "unknown object kind",
			checkErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkErr(t, checkPgPermission(tt.objKind, tt.perm))
		})
	}
}

func TestConvertPermissions(t *testing.T) {
	mkObject := func(name, schema, kind string) *dbobjectv1.DatabaseObject {
		obj, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{
			ObjectKind: kind,
			Schema:     schema,
			Name:       name,
			Protocol:   "postgres",
		})
		require.NoError(t, err)
		return obj
	}

	// Define test cases
	tests := []struct {
		name          string
		input         permissions.PermissionSet
		expected      *Permissions
		expectedError error
	}{
		{
			name: "valid table permissions, ignoring procedure",
			input: permissions.PermissionSet{
				"SELECT":  {mkObject("my_table", "public", databaseobjectimportrule.ObjectKindTable)},
				"INSERT":  {mkObject("other_table", "secret", databaseobjectimportrule.ObjectKindTable)},
				"EXECUTE": {mkObject("my_proc", "public", databaseobjectimportrule.ObjectKindProcedure)},
			},
			expected: &Permissions{
				Tables: []TablePermission{
					{
						Privilege: "SELECT",
						Schema:    "public",
						Table:     "my_table",
					},
					{
						Privilege: "INSERT",
						Schema:    "secret",
						Table:     "other_table",
					},
				},
			},
		},
		{
			name: "invalid table permissions lead to an error",
			input: permissions.PermissionSet{
				"invalid": {mkObject("my_table", "public", databaseobjectimportrule.ObjectKindTable)},
			},
			expectedError: trace.BadParameter("unrecognized \"table\" Postgres permission: \"invalid\""),
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertPermissions(tt.input)
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				require.ElementsMatch(t, tt.expected.Tables, result.Tables)
			}
		})
	}
}
