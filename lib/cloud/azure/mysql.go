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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
)

// mySQLClient implements ServersClient
var _ DBServersClient = (*mySQLClient)(nil)

// mySQLClient wraps the Azure MySQLAPI so we can implement the ServersClient interface.
type mySQLClient struct {
	api MySQLAPI
}

// NewMySQLClient returns a DBServersClient for MySQL servers
func NewMySQLClient(api MySQLAPI) DBServersClient {
	return &mySQLClient{api: api}
}

// ListServers lists all database servers within an Azure subscription.
func (c *mySQLClient) ListServers(ctx context.Context, group string, maxPages int) ([]*DBServer, error) {
	var servers []*armmysql.Server
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
		server, err := ServerFromMySQLServer(s)
		if err != nil {
			continue
		}
		result = append(result, server)
	}
	return result, nil
}

func (c *mySQLClient) Get(ctx context.Context, group, name string) (*DBServer, error) {
	res, err := c.api.Get(ctx, group, name, nil)
	if err != nil {
		return nil, ConvertResponseError(err)
	}
	return ServerFromMySQLServer(&res.Server)
}

func (c *mySQLClient) listAll(ctx context.Context, maxPages int) ([]*armmysql.Server, error) {
	var servers []*armmysql.Server
	options := &armmysql.ServersClientListOptions{}
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

func (c *mySQLClient) listByGroup(ctx context.Context, group string, maxPages int) ([]*armmysql.Server, error) {
	var servers []*armmysql.Server
	options := &armmysql.ServersClientListByResourceGroupOptions{}
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
// Only available for MySQL 5.7 and newer.
func isMySQLVersionSupported(s *DBServer) bool {
	switch armmysql.ServerVersion(s.Properties.Version) {
	case armmysql.ServerVersionEight0, armmysql.ServerVersionFive7:
		return true
	case armmysql.ServerVersionFive6:
		return false
	}
	return false
}
