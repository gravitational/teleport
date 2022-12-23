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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
		log.Warnf("Unknown server version: %q. Assuming Azure DB server %q is supported.",
			s.Properties.Version,
			s.Name,
		)
		return true
	}
}
