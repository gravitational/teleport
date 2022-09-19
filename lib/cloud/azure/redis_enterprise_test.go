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
			resourceName    string
			expectedError   bool
			expectedToken   string
		}{
			{
				name:         "access denied",
				resourceName: "cluster-name",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					NoAuth: true,
				},
				expectedError: true,
			},
			{
				name:         "succeed (default database name)",
				resourceName: "cluster-name",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"default": "some-token",
					},
				},
				expectedToken: "some-token",
			},
			{
				name:         "succeed (specific database name)",
				resourceName: "cluster-name/databases/some-database",
				mockDatabaseAPI: &ARMRedisEnterpriseDatabaseMock{
					TokensByDatabaseName: map[string]string{
						"some-database": "some-token",
					},
				},
				expectedToken: "some-token",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				c := NewRedisEnterpriseClientByAPI(test.mockDatabaseAPI)
				token, err := c.GetToken(context.TODO(), "group", test.resourceName)
				if test.expectedError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, test.expectedToken, token)
				}
			})
		}
	})
}
