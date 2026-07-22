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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/gravitational/trace"
)

type armPostgresFlexServersClient interface {
	NewListPager(*armpostgresqlflexibleservers.ServersClientListOptions) *runtime.Pager[armpostgresqlflexibleservers.ServersClientListResponse]
	NewListByResourceGroupPager(string, *armpostgresqlflexibleservers.ServersClientListByResourceGroupOptions) *runtime.Pager[armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse]
}

// armPostgresFlexServersClient is an interface that defines a subset of functions of armpostgresqlflexibleservers.ServersClient.
type postgresFlexServersClient struct {
	api armPostgresFlexServersClient
}

// postgresFlexServersClient is an Azure Postgres Flexible server client.
var _ PostgresFlexServersClient = (*postgresFlexServersClient)(nil)

// NewPostgresFlexServersClient creates a new Azure PostgreSQL Flexible server client by subscription and credentials.
func NewPostgresFlexServersClient(subID string, cred azcore.TokenCredential, opts *arm.ClientOptions) (PostgresFlexServersClient, error) {
	api, err := armpostgresqlflexibleservers.NewServersClient(subID, cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewPostgresFlexServersClientByAPI(api), nil
}

// NewPostgresFlexServersClientByAPI creates a new Azure PostgreSQL Flexible server client by ARM API client.
func NewPostgresFlexServersClientByAPI(api armPostgresFlexServersClient) PostgresFlexServersClient {
	return &postgresFlexServersClient{
		api: api,
	}
}

// ListAll returns all Azure PostgreSQL Flexible servers within an Azure subscription.
func (c *postgresFlexServersClient) ListAll(ctx context.Context) ([]*armpostgresqlflexibleservers.Server, error) {
	var servers []*armpostgresqlflexibleservers.Server
	opts := &armpostgresqlflexibleservers.ServersClientListOptions{}
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

// ListWithinGroup returns all Azure PostgreSQL Flexible servers within an Azure resource group.
func (c *postgresFlexServersClient) ListWithinGroup(ctx context.Context, group string) ([]*armpostgresqlflexibleservers.Server, error) {
	var servers []*armpostgresqlflexibleservers.Server
	opts := &armpostgresqlflexibleservers.ServersClientListByResourceGroupOptions{}
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
