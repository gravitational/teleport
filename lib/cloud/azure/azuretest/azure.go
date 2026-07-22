// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package azuretest

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

type Clients struct {
	AzureMySQL              azure.DBServersClient
	AzureMySQLPerSub        map[string]azure.DBServersClient
	AzurePostgres           azure.DBServersClient
	AzurePostgresPerSub     map[string]azure.DBServersClient
	AzureSubscriptionClient *azure.SubscriptionClient
	AzureRedis              azure.RedisClient
	AzureRedisEnterprise    azure.RedisEnterpriseClient
	AzureAKSClientPerSub    map[string]azure.AKSClient
	AzureAKSClient          azure.AKSClient
	AzureVirtualMachines    azure.VirtualMachinesClient
	AzureSQLServer          azure.SQLServerClient
	AzureManagedSQLServer   azure.ManagedSQLServerClient
	AzureMySQLFlex          azure.MySQLFlexServersClient
	AzurePostgresFlex       azure.PostgresFlexServersClient
	AzureRunCommand         azure.RunCommandClient
}

var _ azure.Clients = (*Clients)(nil)

// GetCredential returns default Azure token credential chain.
func (c *Clients) GetCredential(ctx context.Context) (azcore.TokenCredential, error) {
	return &azidentity.ChainedTokenCredential{}, nil
}

// GetMySQLClient returns an AzureMySQLClient for the specified subscription
func (c *Clients) GetMySQLClient(ctx context.Context, subscription string) (azure.DBServersClient, error) {
	if len(c.AzureMySQLPerSub) != 0 {
		return c.AzureMySQLPerSub[subscription], nil
	}
	return c.AzureMySQL, nil
}

// GetPostgresClient returns an AzurePostgresClient for the specified subscription
func (c *Clients) GetPostgresClient(ctx context.Context, subscription string) (azure.DBServersClient, error) {
	if len(c.AzurePostgresPerSub) != 0 {
		return c.AzurePostgresPerSub[subscription], nil
	}
	return c.AzurePostgres, nil
}

// GetKubernetesClient returns an AKS client for the specified subscription
func (c *Clients) GetKubernetesClient(ctx context.Context, subscription string) (azure.AKSClient, error) {
	if len(c.AzureAKSClientPerSub) != 0 {
		return c.AzureAKSClientPerSub[subscription], nil
	}
	return c.AzureAKSClient, nil
}

// GetSubscriptionClient returns an Azure SubscriptionClient
func (c *Clients) GetSubscriptionClient(ctx context.Context) (*azure.SubscriptionClient, error) {
	return c.AzureSubscriptionClient, nil
}

// GetRedisClient returns an Azure Redis client for the given subscription.
func (c *Clients) GetRedisClient(ctx context.Context, subscription string) (azure.RedisClient, error) {
	return c.AzureRedis, nil
}

// GetRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *Clients) GetRedisEnterpriseClient(ctx context.Context, subscription string) (azure.RedisEnterpriseClient, error) {
	return c.AzureRedisEnterprise, nil
}

// GetVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *Clients) GetVirtualMachinesClient(ctx context.Context, subscription string) (azure.VirtualMachinesClient, error) {
	return c.AzureVirtualMachines, nil
}

// GetSQLServerClient returns an Azure client for listing SQL servers.
func (c *Clients) GetSQLServerClient(ctx context.Context, subscription string) (azure.SQLServerClient, error) {
	return c.AzureSQLServer, nil
}

// GetManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *Clients) GetManagedSQLServerClient(ctx context.Context, subscription string) (azure.ManagedSQLServerClient, error) {
	return c.AzureManagedSQLServer, nil
}

// GetMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *Clients) GetMySQLFlexServersClient(ctx context.Context, subscription string) (azure.MySQLFlexServersClient, error) {
	return c.AzureMySQLFlex, nil
}

// GetPostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *Clients) GetPostgresFlexServersClient(ctx context.Context, subscription string) (azure.PostgresFlexServersClient, error) {
	return c.AzurePostgresFlex, nil
}

// GetRunCommandClient returns an Azure Run Command client for the given subscription.
func (c *Clients) GetRunCommandClient(ctx context.Context, subscription string) (azure.RunCommandClient, error) {
	return c.AzureRunCommand, nil
}
