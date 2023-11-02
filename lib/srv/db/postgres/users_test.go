/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
