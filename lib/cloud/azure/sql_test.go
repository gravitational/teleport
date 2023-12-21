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
