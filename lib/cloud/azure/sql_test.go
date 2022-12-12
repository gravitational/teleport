// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/stretchr/testify/require"
)

func TestSQLListAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range []struct {
		desc            string
		client          armSQLServerClient
		expectErr       require.ErrorAssertionFunc
		expectedServers []string
	}{
		{
			desc: "servers",
			client: &ARMSQLServerMock{AllServers: []*armsql.Server{
				makeSQLServer(t, "server1", "group1"),
				makeSQLServer(t, "server2", "group2"),
				makeSQLServer(t, "server3", "group1"),
			}},
			expectErr: require.NoError,
			expectedServers: []string{
				"server1.database.windows.net",
				"server2.database.windows.net",
				"server3.database.windows.net",
			},
		},
		{
			desc:            "empty list",
			client:          &ARMSQLServerMock{AllServers: []*armsql.Server{}},
			expectErr:       require.NoError,
			expectedServers: []string{},
		},
		{
			desc:            "auth error",
			client:          &ARMSQLServerMock{NoAuth: true},
			expectErr:       require.Error,
			expectedServers: []string{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewSQLClientByAPI(tc.client)

			servers, err := c.ListAll(ctx)
			tc.expectErr(t, err)
			requireSQLServers(t, tc.expectedServers, servers)
		})
	}
}

func TestSQLListWithinGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range []struct {
		desc            string
		client          armSQLServerClient
		expectErr       require.ErrorAssertionFunc
		expectedServers []string
	}{
		{
			desc: "servers",
			client: &ARMSQLServerMock{ResourceGroupServers: []*armsql.Server{
				makeSQLServer(t, "server1", "group1"),
				makeSQLServer(t, "server2", "group1"),
				makeSQLServer(t, "server3", "group1"),
			}},
			expectErr: require.NoError,
			expectedServers: []string{
				"server1.database.windows.net",
				"server2.database.windows.net",
				"server3.database.windows.net",
			},
		},
		{
			desc:            "empty list",
			client:          &ARMSQLServerMock{ResourceGroupServers: []*armsql.Server{}},
			expectErr:       require.NoError,
			expectedServers: []string{},
		},
		{
			desc:            "auth error",
			client:          &ARMSQLServerMock{NoAuth: true},
			expectErr:       require.Error,
			expectedServers: []string{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewSQLClientByAPI(tc.client)

			servers, err := c.ListWithinGroup(ctx, "group1")
			tc.expectErr(t, err)
			requireSQLServers(t, tc.expectedServers, servers)
		})
	}
}

func requireSQLServers(t *testing.T, expected []string, actual []*armsql.Server) {
	t.Helper()

	var serverNames []string
	for _, server := range actual {
		serverNames = append(serverNames, *server.Name)
	}

	require.ElementsMatch(t, expected, serverNames)
}

func makeSQLServer(t *testing.T, name, group string) *armsql.Server {
	t.Helper()

	return &armsql.Server{
		ID:   to.Ptr(fmt.Sprintf("/subscriptions/sub-id/resourceGroups/%v/providers/Microsoft.Sql/servers/%v", group, name)),
		Name: to.Ptr(fmt.Sprintf("%s.database.windows.net", name)),
	}
}
