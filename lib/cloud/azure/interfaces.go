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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

// DBServersClient provides an interface for getting Azure DB servers.
type DBServersClient interface {
	// ListServers lists all Azure DB servers within an Azure subscription by resource group.
	// If the resource group is "*", then all resources within the subscription are queried.
	ListServers(ctx context.Context, group string, maxPages int) ([]*DBServer, error)
	// Get returns a DBServer with an Azure subscription, queried by group and name
	Get(ctx context.Context, group, name string) (*DBServer, error)
}

// SubscriptionsAPI provides an interface for armsubscription.SubscriptionsClient so that client can be mocked.
type SubscriptionsAPI interface {
	NewListPager(opts *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
}

var _ SubscriptionsAPI = (*armsubscription.SubscriptionsClient)(nil)

// MySQLAPI is an interface for armmysql.ServersClient so that the client can be mocked.
type MySQLAPI interface {
	Get(ctx context.Context, group, name string, opts *armmysql.ServersClientGetOptions) (armmysql.ServersClientGetResponse, error)
	NewListPager(opts *armmysql.ServersClientListOptions) *runtime.Pager[armmysql.ServersClientListResponse]
	NewListByResourceGroupPager(group string, opts *armmysql.ServersClientListByResourceGroupOptions) *runtime.Pager[armmysql.ServersClientListByResourceGroupResponse]
}

var _ MySQLAPI = (*armmysql.ServersClient)(nil)

// PostgresAPI is an interface for armpostgresql.ServersClient so that the client can be mocked.
type PostgresAPI interface {
	Get(ctx context.Context, group, name string, opts *armpostgresql.ServersClientGetOptions) (armpostgresql.ServersClientGetResponse, error)
	NewListPager(opts *armpostgresql.ServersClientListOptions) *runtime.Pager[armpostgresql.ServersClientListResponse]
	NewListByResourceGroupPager(group string, opts *armpostgresql.ServersClientListByResourceGroupOptions) *runtime.Pager[armpostgresql.ServersClientListByResourceGroupResponse]
}

var _ PostgresAPI = (*armpostgresql.ServersClient)(nil)
