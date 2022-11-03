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
