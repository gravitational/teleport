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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
	logrus.Debug("Initializing Azure PostgreSQL Flexible servers client.")
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
