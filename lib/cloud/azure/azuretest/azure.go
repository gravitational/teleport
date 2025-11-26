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
	AzureRoleDefinitions    azure.RoleDefinitionsClient
	AzureRoleAssignments    azure.RoleAssignmentsClient
}

var _ azure.Clients = (*Clients)(nil)

// GetAzureCredential returns default Azure token credential chain.
func (c *Clients) GetAzureCredential(ctx context.Context) (azcore.TokenCredential, error) {
	return &azidentity.ChainedTokenCredential{}, nil
}

// GetAzureMySQLClient returns an AzureMySQLClient for the specified subscription
func (c *Clients) GetAzureMySQLClient(ctx context.Context, subscription string) (azure.DBServersClient, error) {
	if len(c.AzureMySQLPerSub) != 0 {
		return c.AzureMySQLPerSub[subscription], nil
	}
	return c.AzureMySQL, nil
}

// GetAzurePostgresClient returns an AzurePostgresClient for the specified subscription
func (c *Clients) GetAzurePostgresClient(ctx context.Context, subscription string) (azure.DBServersClient, error) {
	if len(c.AzurePostgresPerSub) != 0 {
		return c.AzurePostgresPerSub[subscription], nil
	}
	return c.AzurePostgres, nil
}

// GetAzureKubernetesClient returns an AKS client for the specified subscription
func (c *Clients) GetAzureKubernetesClient(ctx context.Context, subscription string) (azure.AKSClient, error) {
	if len(c.AzureAKSClientPerSub) != 0 {
		return c.AzureAKSClientPerSub[subscription], nil
	}
	return c.AzureAKSClient, nil
}

// GetAzureSubscriptionClient returns an Azure SubscriptionClient
func (c *Clients) GetAzureSubscriptionClient(ctx context.Context) (*azure.SubscriptionClient, error) {
	return c.AzureSubscriptionClient, nil
}

// GetAzureRedisClient returns an Azure Redis client for the given subscription.
func (c *Clients) GetAzureRedisClient(ctx context.Context, subscription string) (azure.RedisClient, error) {
	return c.AzureRedis, nil
}

// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *Clients) GetAzureRedisEnterpriseClient(ctx context.Context, subscription string) (azure.RedisEnterpriseClient, error) {
	return c.AzureRedisEnterprise, nil
}

// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *Clients) GetAzureVirtualMachinesClient(ctx context.Context, subscription string) (azure.VirtualMachinesClient, error) {
	return c.AzureVirtualMachines, nil
}

// GetAzureSQLServerClient returns an Azure client for listing SQL servers.
func (c *Clients) GetAzureSQLServerClient(ctx context.Context, subscription string) (azure.SQLServerClient, error) {
	return c.AzureSQLServer, nil
}

// GetAzureManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *Clients) GetAzureManagedSQLServerClient(ctx context.Context, subscription string) (azure.ManagedSQLServerClient, error) {
	return c.AzureManagedSQLServer, nil
}

// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *Clients) GetAzureMySQLFlexServersClient(ctx context.Context, subscription string) (azure.MySQLFlexServersClient, error) {
	return c.AzureMySQLFlex, nil
}

// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *Clients) GetAzurePostgresFlexServersClient(ctx context.Context, subscription string) (azure.PostgresFlexServersClient, error) {
	return c.AzurePostgresFlex, nil
}

// GetAzureRunCommandClient returns an Azure Run Command client for the given subscription.
func (c *Clients) GetAzureRunCommandClient(ctx context.Context, subscription string) (azure.RunCommandClient, error) {
	return c.AzureRunCommand, nil
}

// GetAzureRoleDefinitionsClient returns an Azure Role Definitions client for the given subscription.
func (c *Clients) GetAzureRoleDefinitionsClient(ctx context.Context, subscription string) (azure.RoleDefinitionsClient, error) {
	return c.AzureRoleDefinitions, nil
}

// GetAzureRoleAssignmentsClient returns an Azure Role Assignments client for the given subscription.
func (c *Clients) GetAzureRoleAssignmentsClient(ctx context.Context, subscription string) (azure.RoleAssignmentsClient, error) {
	return c.AzureRoleAssignments, nil
}
