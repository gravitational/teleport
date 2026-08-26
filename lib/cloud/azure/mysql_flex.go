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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/gravitational/trace"
)

// armMySQLFlexServersClient is an interface that defines a subset of functions of armmysqlflexibleservers.ServersClient.
type armMySQLFlexServersClient interface {
	NewListPager(*armmysqlflexibleservers.ServersClientListOptions) *runtime.Pager[armmysqlflexibleservers.ServersClientListResponse]
	NewListByResourceGroupPager(string, *armmysqlflexibleservers.ServersClientListByResourceGroupOptions) *runtime.Pager[armmysqlflexibleservers.ServersClientListByResourceGroupResponse]
}

// mySQLFlexServersClient is an Azure MySQL Flexible server client.
type mySQLFlexServersClient struct {
	api armMySQLFlexServersClient
}

var _ MySQLFlexServersClient = (*mySQLFlexServersClient)(nil)

// NewMySQLFlexServersClient creates a new Azure MySQL Flexible server client by subscription and credentials.
func NewMySQLFlexServersClient(subID string, cred azcore.TokenCredential, opts *arm.ClientOptions) (MySQLFlexServersClient, error) {
	api, err := armmysqlflexibleservers.NewServersClient(subID, cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewMySQLFlexServersClientByAPI(api), nil
}

// NewMySQLFlexServersClientByAPI creates a new Azure MySQL Flexible server client by ARM API client.
func NewMySQLFlexServersClientByAPI(api armMySQLFlexServersClient) MySQLFlexServersClient {
	return &mySQLFlexServersClient{
		api: api,
	}
}

// ListAll returns all Azure MySQL Flexible servers within an Azure subscription.
func (c *mySQLFlexServersClient) ListAll(ctx context.Context) ([]*armmysqlflexibleservers.Server, error) {
	var servers []*armmysqlflexibleservers.Server
	opts := &armmysqlflexibleservers.ServersClientListOptions{}
	pager := c.api.NewListPager(opts)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

// ListWithinGroup returns all Azure MySQL Flexible servers within an Azure resource group.
func (c *mySQLFlexServersClient) ListWithinGroup(ctx context.Context, group string) ([]*armmysqlflexibleservers.Server, error) {
	var servers []*armmysqlflexibleservers.Server
	opts := &armmysqlflexibleservers.ServersClientListByResourceGroupOptions{}
	pager := c.api.NewListByResourceGroupPager(group, opts)
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}
