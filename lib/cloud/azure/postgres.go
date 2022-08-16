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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// postgresClient implements ServersClient
var _ DBServersClient = (*postgresClient)(nil)

// postgresClient wraps the Azure Postgres API so we can implement the ServersClient interface.
type postgresClient struct {
	api PostgresAPI
}

// NewPostgresClient returns a DBServersClient for postgres servers
func NewPostgresClient(api PostgresAPI) DBServersClient {
	return &postgresClient{api: api}
}

// ListServers lists all database servers within an Azure subscription.
func (c *postgresClient) ListServers(ctx context.Context, group string, maxPages int) ([]*DBServer, error) {
	var servers []*armpostgresql.Server
	var err error
	if group == types.Wildcard {
		servers, err = c.listAll(ctx, maxPages)
	} else {
		servers, err = c.listByGroup(ctx, group, maxPages)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := make([]*DBServer, 0, len(servers))
	for _, s := range servers {
		server, err := ServerFromPostgresServer(s)
		if err != nil {
			continue
		}
		result = append(result, server)
	}
	return result, nil
}

func (c *postgresClient) Get(ctx context.Context, group, name string) (*DBServer, error) {
	res, err := c.api.Get(ctx, group, name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ServerFromPostgresServer(&res.Server)
}

func (c *postgresClient) listAll(ctx context.Context, maxPages int) ([]*armpostgresql.Server, error) {
	var servers []*armpostgresql.Server
	options := &armpostgresql.ServersClientListOptions{}
	pager := c.api.NewListPager(options)
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, ConvertResponseError(err)
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

func (c *postgresClient) listByGroup(ctx context.Context, group string, maxPages int) ([]*armpostgresql.Server, error) {
	var servers []*armpostgresql.Server
	options := &armpostgresql.ServersClientListByResourceGroupOptions{}
	pager := c.api.NewListByResourceGroupPager(group, options)
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, ConvertResponseError(err)
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

// IsVersionSupported returns true if database supports AAD authentication.
// All Azure managed PostgreSQL single-server instances support AAD auth.
func isPostgresVersionSupported(s *DBServer) bool {
	switch armpostgresql.ServerVersion(s.Properties.Version) {
	case armpostgresql.ServerVersionNine5, armpostgresql.ServerVersionNine6,
		armpostgresql.ServerVersionTen, armpostgresql.ServerVersionTen0,
		armpostgresql.ServerVersionTen2, armpostgresql.ServerVersionEleven:
		return true
	}
	return false
}
