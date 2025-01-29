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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
)

// DBServersClient provides an interface for fetching Azure DB Servers.
type DBServersClient interface {
	// ListAll returns all Azure DB servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*DBServer, error)
	// ListWithinGroup returns all Azure DB servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*DBServer, error)
	// Get returns a DBServer within an Azure subscription, queried by group and name
	Get(ctx context.Context, group, name string) (*DBServer, error)
}

// ARMMySQL is an interface for armmysql.ServersClient.
// It exists so that the client can be mocked.
type ARMMySQL interface {
	// Get - gets information about an Azure DB server.
	Get(ctx context.Context, group, name string, opts *armmysql.ServersClientGetOptions) (armmysql.ServersClientGetResponse, error)
	// NewListPager - List all the servers in a given subscription.
	NewListPager(opts *armmysql.ServersClientListOptions) *runtime.Pager[armmysql.ServersClientListResponse]
	// NewListByResourceGroupPager - List all the servers in a given resource group.
	NewListByResourceGroupPager(group string, opts *armmysql.ServersClientListByResourceGroupOptions) *runtime.Pager[armmysql.ServersClientListByResourceGroupResponse]
}

var _ ARMMySQL = (*armmysql.ServersClient)(nil)

// ARMPostgres is an interface for armpostgresql.ServersClient.
// It exists so that the client can be mocked.
type ARMPostgres interface {
	// Get - gets information about an Azure DB server.
	Get(ctx context.Context, group, name string, opts *armpostgresql.ServersClientGetOptions) (armpostgresql.ServersClientGetResponse, error)
	// NewListPager - List all the servers in a given subscription.
	NewListPager(opts *armpostgresql.ServersClientListOptions) *runtime.Pager[armpostgresql.ServersClientListResponse]
	// NewListByResourceGroupPager - List all the servers in a given resource group.
	NewListByResourceGroupPager(group string, opts *armpostgresql.ServersClientListByResourceGroupOptions) *runtime.Pager[armpostgresql.ServersClientListByResourceGroupResponse]
}

var _ ARMPostgres = (*armpostgresql.ServersClient)(nil)

// CacheForRedisClient provides an interface for an Azure Redis For Cache client.
type CacheForRedisClient interface {
	// GetToken retrieves the auth token for provided resource ID.
	GetToken(ctx context.Context, resourceID string) (string, error)
}

// RedisClient is an interface for a Redis client.
type RedisClient interface {
	CacheForRedisClient

	// ListAll returns all Azure Redis servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*armredis.ResourceInfo, error)
	// ListWithinGroup returns all Azure Redis servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*armredis.ResourceInfo, error)
}

// RedisEnterpriseClient is an interface for a Redis Enterprise client.
type RedisEnterpriseClient interface {
	CacheForRedisClient

	// ListAll returns all Azure Redis Enterprise databases within an Azure subscription.
	ListAll(ctx context.Context) ([]*RedisEnterpriseDatabase, error)
	// ListWithinGroup returns all Azure Redis Enterprise databases within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*RedisEnterpriseDatabase, error)
}

// SQLServerClient is an interface for a SQL Server client.
type SQLServerClient interface {
	// ListAll returns all Azure SQL servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*armsql.Server, error)
	// ListWithinGroup returns all Azure SQL servers databases within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*armsql.Server, error)
}

// ManagedSQLServerClient is an interface for a Managed SQL Server client.
type ManagedSQLServerClient interface {
	// ListAll returns all Azure Managed SQL servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*armsql.ManagedInstance, error)
	// ListWithinGroup returns all Azure Managed SQL servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*armsql.ManagedInstance, error)
}

// MySQLFlexServersClient is an interface for an Azure MySQL Flexible server client.
type MySQLFlexServersClient interface {
	// ListAll returns all Azure MySQL Flex servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*armmysqlflexibleservers.Server, error)
	// ListWithinGroup returns all Azure MySQL Flex servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*armmysqlflexibleservers.Server, error)
}

// PostgresFlexServersClient is an interface for an Azure PostgreSQL Flexible server client.
type PostgresFlexServersClient interface {
	// ListAll returns all Azure Postgres Flex servers within an Azure subscription.
	ListAll(ctx context.Context) ([]*armpostgresqlflexibleservers.Server, error)
	// ListWithinGroup returns all Azure Postgres Flex servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*armpostgresqlflexibleservers.Server, error)
}
