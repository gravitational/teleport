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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/trace"
)

var _ DBServersClient = (*postgresClient)(nil)

// postgresClient wraps the ARMPostgres API so we can implement the DBServersClient interface.
type postgresClient struct {
	api ARMPostgres
}

// NewPostgresServerClient returns a DBServersClient for Azure PostgreSQL servers.
func NewPostgresServerClient(api ARMPostgres) DBServersClient {
	return &postgresClient{api: api}
}

func (c *postgresClient) Get(ctx context.Context, group, name string) (*DBServer, error) {
	res, err := c.api.Get(ctx, group, name, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}
	return ServerFromPostgresServer(&res.Server), nil
}

func (c *postgresClient) ListAll(ctx context.Context) ([]*DBServer, error) {
	var servers []*DBServer
	options := &armpostgresql.ServersClientListOptions{}
	pager := c.api.NewListPager(options)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			servers = append(servers, ServerFromPostgresServer(s))
		}
	}
	return servers, nil
}

func (c *postgresClient) ListWithinGroup(ctx context.Context, group string) ([]*DBServer, error) {
	var servers []*DBServer
	options := &armpostgresql.ServersClientListByResourceGroupOptions{}
	pager := c.api.NewListByResourceGroupPager(group, options)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			servers = append(servers, ServerFromPostgresServer(s))
		}
	}
	return servers, nil
}

// IsVersionSupported returns true if database supports AAD authentication.
// All Azure managed PostgreSQL single-server instances support AAD auth.
func isPostgresVersionSupported(s *DBServer) bool {
	return true
}
