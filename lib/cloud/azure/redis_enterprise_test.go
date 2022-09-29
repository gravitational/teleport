/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisEnterpriseClient(t *testing.T) {
	t.Run("GetToken", func(t *testing.T) {
		tests := []struct {
			name            string
			mockDatabaseAPI armRedisEnterpriseDatabaseClient
			resourceID      string
			expectError     bool
			expectToken     string
		}{
			{
				name:       "access denied",
				resourceID: "cluster-name",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					NoAuth: true,
				},
				expectError: true,
			},
			{
				name:       "succeed (default database name)",
				resourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"default": "some-token",
					},
				},
				expectToken: "some-token",
			},
			{
				name:       "succeed (specific database name)",
				resourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport/databases/some-database",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"some-database": "some-token",
					},
				},
				expectToken: "some-token",
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				c := NewRedisEnterpriseClientByAPI(test.mockDatabaseAPI)
				token, err := c.GetToken(context.TODO(), test.resourceID)
				if test.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, test.expectToken, token)
				}
			})
		}
	})
}
