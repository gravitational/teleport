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
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/gravitational/trace"
)

var _ DBServersClient = (*mySQLClient)(nil)

// mySQLClient wraps the ARMMySQL API so we can implement the DBServersClient interface.
type mySQLClient struct {
	api ARMMySQL
}

// NewMySQLServersClient returns a DBServersClient for Azure MySQL servers.
func NewMySQLServersClient(api ARMMySQL) DBServersClient {
	return &mySQLClient{api: api}
}

func (c *mySQLClient) Get(ctx context.Context, group, name string) (*DBServer, error) {
	res, err := c.api.Get(ctx, group, name, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}
	return ServerFromMySQLServer(&res.Server), nil
}

func (c *mySQLClient) ListAll(ctx context.Context) ([]*DBServer, error) {
	var servers []*DBServer
	options := &armmysql.ServersClientListOptions{}
	pager := c.api.NewListPager(options)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			servers = append(servers, ServerFromMySQLServer(s))
		}
	}
	return servers, nil
}

func (c *mySQLClient) ListWithinGroup(ctx context.Context, group string) ([]*DBServer, error) {
	var servers []*DBServer
	options := &armmysql.ServersClientListByResourceGroupOptions{}
	pager := c.api.NewListByResourceGroupPager(group, options)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			servers = append(servers, ServerFromMySQLServer(s))
		}
	}
	return servers, nil
}

// IsVersionSupported returns true if database supports AAD authentication.
// Only available for MySQL 5.7 and newer.
func isMySQLVersionSupported(s *DBServer) bool {
	switch armmysql.ServerVersion(s.Properties.Version) {
	case armmysql.ServerVersionEight0, armmysql.ServerVersionFive7:
		return true
	case armmysql.ServerVersionFive6:
		return false
	default:
		slog.WarnContext(context.Background(), "Assuming Azure DB server with unknown server version is supported",
			"version", s.Properties.Version,
			"server", s.Name,
		)
		return true
	}
}
