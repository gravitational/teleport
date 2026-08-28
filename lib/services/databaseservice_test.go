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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDatabaseServiceUnmarshal verifies that DatabaseService resource can be unmarshaled.
func TestDatabaseServiceUnmarshal(t *testing.T) {
	expected, err := types.NewDatabaseServiceV1(types.Metadata{
		Name: "test-database-service",
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"prod", "stg"},
				},
			},
		},
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(dbServiceYAML))
	require.NoError(t, err)
	actual, err := UnmarshalDatabaseService(data)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}

// TestDatabaseServicenMarshal verifies a marshaled kubernetes resource resource can be unmarshaled back.
func TestDatabaseServiceMarshal(t *testing.T) {
	expected, err := types.NewDatabaseServiceV1(types.Metadata{
		Name: "test-database-service",
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"prod"},
				},
			},
		},
	})
	require.NoError(t, err)
	data, err := MarshalDatabaseService(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDatabaseService(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var dbServiceYAML = `---
kind: database_service
version: v1
metadata:
  name: test-database-service
spec:
  resources:
    - labels:
        env: [prod, stg]
`
