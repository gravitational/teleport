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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisClient(t *testing.T) {
	t.Run("GetToken", func(t *testing.T) {
		tests := []struct {
			name        string
			mockAPI     armRedisClient
			expectError bool
			expectToken string
		}{
			{
				name: "access denied",
				mockAPI: &ARMRedisMock{
					NoAuth: true,
				},
				expectError: true,
			},
			{
				name: "succeed",
				mockAPI: &ARMRedisMock{
					Token: "some-token",
				},
				expectToken: "some-token",
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				c := NewRedisClientByAPI(test.mockAPI)
				token, err := c.GetToken(context.TODO(), "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/Redis/example-teleport")
				if test.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, test.expectToken, token)
				}
			})
		}
	})

	t.Run("List", func(t *testing.T) {
		mockAPI := &ARMRedisMock{
			Token: "some-token",
			Servers: []*armredis.ResourceInfo{
				makeRedisResourceInfo("redis-prod-1", "group-prod"),
				makeRedisResourceInfo("redis-prod-2", "group-prod"),
				makeRedisResourceInfo("redis-dev", "group-dev"),
			},
		}

		t.Run("ListAll", func(t *testing.T) {
			t.Parallel()

			expectServers := []string{"redis-prod-1", "redis-prod-2", "redis-dev"}

			c := NewRedisClientByAPI(mockAPI)
			resources, err := c.ListAll(context.TODO())
			require.NoError(t, err)
			requireRedisServers(t, expectServers, resources)
		})
		t.Run("ListWithinGroup", func(t *testing.T) {
			t.Parallel()

			expectServers := []string{"redis-prod-1", "redis-prod-2"}

			c := NewRedisClientByAPI(mockAPI)
			resources, err := c.ListWithinGroup(context.TODO(), "group-prod")
			require.NoError(t, err)
			requireRedisServers(t, expectServers, resources)
		})
	})
}

func requireRedisServers(t *testing.T, expectServers []string, servers []*armredis.ResourceInfo) {
	require.Len(t, servers, len(expectServers))
	for i, server := range servers {
		require.Equal(t, expectServers[i], StringVal(server.Name))
	}
}

func makeRedisResourceInfo(name, group string) *armredis.ResourceInfo {
	return &armredis.ResourceInfo{
		Name:     to.Ptr(name),
		ID:       to.Ptr(fmt.Sprintf("/subscriptions/sub-id/resourceGroups/%v/providers/Microsoft.Cache/Redis/%v", group, name)),
		Type:     to.Ptr("Microsoft.Cache/Redis"),
		Location: to.Ptr("local"),
	}
}
