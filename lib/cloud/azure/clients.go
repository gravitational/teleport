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
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Clients is an interface for Azure-specific API clients
type Clients interface {
	// GetAzureCredential returns Azure default token credential chain.
	GetAzureCredential(ctx context.Context) (azcore.TokenCredential, error)
	// GetAzureMySQLClient returns Azure MySQL client for the specified subscription.
	GetAzureMySQLClient(ctx context.Context, subscription string) (DBServersClient, error)
	// GetAzurePostgresClient returns Azure Postgres client for the specified subscription.
	GetAzurePostgresClient(ctx context.Context, subscription string) (DBServersClient, error)
	// GetAzureSubscriptionClient returns an Azure Subscriptions client
	GetAzureSubscriptionClient(ctx context.Context) (*SubscriptionClient, error)
	// GetAzureRedisClient returns an Azure Redis client for the given subscription.
	GetAzureRedisClient(ctx context.Context, subscription string) (RedisClient, error)
	// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
	GetAzureRedisEnterpriseClient(ctx context.Context, subscription string) (RedisEnterpriseClient, error)
	// GetAzureKubernetesClient returns an Azure AKS client for the specified subscription.
	GetAzureKubernetesClient(ctx context.Context, subscription string) (AKSClient, error)
	// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for the given subscription.
	GetAzureVirtualMachinesClient(ctx context.Context, subscription string) (VirtualMachinesClient, error)
	// GetAzureSQLServerClient returns an Azure SQL Server client for the
	// specified subscription.
	GetAzureSQLServerClient(ctx context.Context, subscription string) (SQLServerClient, error)
	// GetAzureManagedSQLServerClient returns an Azure ManagedSQL Server client
	// for the specified subscription.
	GetAzureManagedSQLServerClient(ctx context.Context, subscription string) (ManagedSQLServerClient, error)
	// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible Server client for the
	// specified subscription.
	GetAzureMySQLFlexServersClient(ctx context.Context, subscription string) (MySQLFlexServersClient, error)
	// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible Server client for the
	// specified subscription.
	GetAzurePostgresFlexServersClient(ctx context.Context, subscription string) (PostgresFlexServersClient, error)
	// GetAzureRunCommandClient returns an Azure Run Command client for the given subscription.
	GetAzureRunCommandClient(ctx context.Context, subscription string) (RunCommandClient, error)
}

// ClientsOption is an option to pass to NewAzureClients
type ClientsOption func(clients *clients)

type azureOIDCCredentials interface {
	GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error)
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// WithIntegrationCredentials configures Azure cloud clients to use integration credentials.
func WithIntegrationCredentials(integrationName string, auth azureOIDCCredentials) ClientsOption {
	return func(clt *clients) {
		clt.newAzureCredentialFunc = func(ctx context.Context) (azcore.TokenCredential, error) {
			integration, err := auth.GetIntegration(ctx, integrationName)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			spec := integration.GetAzureOIDCIntegrationSpec()
			if spec == nil {
				return nil, trace.BadParameter("expected %q to be an %q integration, was %q instead", integration.GetName(), types.IntegrationSubKindAzureOIDC, integration.GetSubKind())
			}
			cred, err := azidentity.NewClientAssertionCredential(spec.TenantID, spec.ClientID, func(ctx context.Context) (string, error) {
				return auth.GenerateAzureOIDCToken(ctx, integrationName)
				// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
			}, nil)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return cred, nil
		}
	}
}

// NewClients returns a new instance of Azure SDK clients.
func NewClients(opts ...ClientsOption) (Clients, error) {
	azClients := &clients{
		mySQLClients:     make(map[string]DBServersClient),
		postgresClients:  make(map[string]DBServersClient),
		kubernetesClient: make(map[string]AKSClient),
	}
	var err error
	azClients.redisClients, err = NewClientMap(NewRedisClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.redisEnterpriseClients, err = NewClientMap(NewRedisEnterpriseClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.virtualMachinesClients, err = NewClientMap(NewVirtualMachinesClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.sqlServerClients, err = NewClientMap(NewSQLClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.managedSQLServerClients, err = NewClientMap(NewManagedSQLClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.mySQLFlexServersClients, err = NewClientMap(NewMySQLFlexServersClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.postgresFlexServersClients, err = NewClientMap(NewPostgresFlexServersClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.runCommandClients, err = NewClientMap(NewRunCommandClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	azClients.newAzureCredentialFunc = func(ctx context.Context) (azcore.TokenCredential, error) {
		// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
		return azidentity.NewDefaultAzureCredential(nil)
	}

	for _, opt := range opts {
		opt(azClients)
	}

	return azClients, nil
}

// clients contains Azure-specific clients.
type clients struct {
	// mtx is used for locking.
	mtx sync.RWMutex

	// newAzureCredentialFunc creates new Azure credential.
	newAzureCredentialFunc func(ctx context.Context) (azcore.TokenCredential, error)
	azureCredential        azcore.TokenCredential

	mySQLClients               map[string]DBServersClient
	postgresClients            map[string]DBServersClient
	subscriptionsClient        *SubscriptionClient
	redisClients               ClientMap[RedisClient]
	redisEnterpriseClients     ClientMap[RedisEnterpriseClient]
	kubernetesClient           map[string]AKSClient
	virtualMachinesClients     ClientMap[VirtualMachinesClient]
	sqlServerClients           ClientMap[SQLServerClient]
	managedSQLServerClients    ClientMap[ManagedSQLServerClient]
	mySQLFlexServersClients    ClientMap[MySQLFlexServersClient]
	postgresFlexServersClients ClientMap[PostgresFlexServersClient]
	runCommandClients          ClientMap[RunCommandClient]
	roleDefinitionsClients     ClientMap[RoleDefinitionsClient]
	assignmentsClients         ClientMap[RoleAssignmentsClient]
}

// GetAzureCredential returns default Azure token credential chain.
func (c *clients) GetAzureCredential(ctx context.Context) (azcore.TokenCredential, error) {
	c.mtx.RLock()
	if c.azureCredential != nil {
		defer c.mtx.RUnlock()
		return c.azureCredential, nil
	}
	c.mtx.RUnlock()
	return c.initAzureCredential(ctx)
}

// GetAzureMySQLClient returns an AzureClient for MySQL for the given subscription.
func (c *clients) GetAzureMySQLClient(ctx context.Context, subscription string) (DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.mySQLClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzureMySQLClient(ctx, subscription)
}

// GetAzurePostgresClient returns an AzureClient for Postgres for the given subscription.
func (c *clients) GetAzurePostgresClient(ctx context.Context, subscription string) (DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.postgresClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzurePostgresClient(ctx, subscription)
}

// GetAzureSubscriptionClient returns an Azure client for listing subscriptions.
func (c *clients) GetAzureSubscriptionClient(ctx context.Context) (*SubscriptionClient, error) {
	c.mtx.RLock()
	if c.subscriptionsClient != nil {
		defer c.mtx.RUnlock()
		return c.subscriptionsClient, nil
	}
	c.mtx.RUnlock()
	return c.initAzureSubscriptionsClient(ctx)
}

// GetAzureRedisClient returns an Azure Redis client for the given subscription.
func (c *clients) GetAzureRedisClient(ctx context.Context, subscription string) (RedisClient, error) {
	return c.redisClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *clients) GetAzureRedisEnterpriseClient(ctx context.Context, subscription string) (RedisEnterpriseClient, error) {
	return c.redisEnterpriseClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureKubernetesClient returns an Azure client for listing AKS clusters.
func (c *clients) GetAzureKubernetesClient(ctx context.Context, subscription string) (AKSClient, error) {
	c.mtx.RLock()
	if client, ok := c.kubernetesClient[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzureKubernetesClient(ctx, subscription)
}

// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *clients) GetAzureVirtualMachinesClient(ctx context.Context, subscription string) (VirtualMachinesClient, error) {
	return c.virtualMachinesClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureSQLServerClient returns an Azure client for listing SQL servers.
func (c *clients) GetAzureSQLServerClient(ctx context.Context, subscription string) (SQLServerClient, error) {
	return c.sqlServerClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *clients) GetAzureManagedSQLServerClient(ctx context.Context, subscription string) (ManagedSQLServerClient, error) {
	return c.managedSQLServerClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *clients) GetAzureMySQLFlexServersClient(ctx context.Context, subscription string) (MySQLFlexServersClient, error) {
	return c.mySQLFlexServersClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *clients) GetAzurePostgresFlexServersClient(ctx context.Context, subscription string) (PostgresFlexServersClient, error) {
	return c.postgresFlexServersClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureRunCommandClient returns an Azure Run Command client for the given
// subscription.
func (c *clients) GetAzureRunCommandClient(ctx context.Context, subscription string) (RunCommandClient, error) {
	return c.runCommandClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureRoleDefinitionsClient returns an Azure Role Definitions client
func (c *clients) GetAzureRoleDefinitionsClient(ctx context.Context, subscription string) (RoleDefinitionsClient, error) {
	return c.roleDefinitionsClients.Get(ctx, subscription, c.GetAzureCredential)
}

// GetAzureRoleAssignmentsClient returns an Azure Role Assignments client
func (c *clients) GetAzureRoleAssignmentsClient(ctx context.Context, subscription string) (RoleAssignmentsClient, error) {
	return c.assignmentsClients.Get(ctx, subscription, c.GetAzureCredential)
}

func (c *clients) initAzureCredential(ctx context.Context) (azcore.TokenCredential, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.azureCredential != nil { // If some other thread already got here first.
		return c.azureCredential, nil
	}

	cred, err := c.newAzureCredentialFunc(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.azureCredential = cred
	return cred, nil
}

func (c *clients) initAzureMySQLClient(ctx context.Context, subscription string) (DBServersClient, error) {
	cred, err := c.GetAzureCredential(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.mySQLClients[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}

	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armmysql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := NewMySQLServersClient(api)
	c.mySQLClients[subscription] = client
	return client, nil
}

func (c *clients) initAzurePostgresClient(ctx context.Context, subscription string) (DBServersClient, error) {
	cred, err := c.GetAzureCredential(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.postgresClients[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armpostgresql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := NewPostgresServerClient(api)
	c.postgresClients[subscription] = client
	return client, nil
}

func (c *clients) initAzureSubscriptionsClient(ctx context.Context) (*SubscriptionClient, error) {
	cred, err := c.GetAzureCredential(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.subscriptionsClient != nil { // If some other thread already got here first.
		return c.subscriptionsClient, nil
	}
	// TODO(gavin): if/when we support AzureChina/AzureGovernment,
	// we will need to specify the cloud in these options
	opts := &arm.ClientOptions{}
	armClient, err := armsubscription.NewSubscriptionsClient(cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := NewSubscriptionClient(armClient)
	c.subscriptionsClient = client
	return client, nil
}

func (c *clients) initAzureKubernetesClient(ctx context.Context, subscription string) (AKSClient, error) {
	cred, err := c.GetAzureCredential(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.kubernetesClient[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}
	// TODO(tigrato): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armcontainerservice.NewManagedClustersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := NewAKSClustersClient(
		api, func(options *azidentity.DefaultAzureCredentialOptions) (GetToken, error) {
			cc, err := azidentity.NewDefaultAzureCredential(options)
			return cc, err
		})
	c.kubernetesClient[subscription] = client
	return client, nil
}
