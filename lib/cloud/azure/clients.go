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
	// GetCredential returns Azure token credential. Uses default ambient credentials unless configured otherwise.
	GetCredential(ctx context.Context) (azcore.TokenCredential, error)
	// GetMySQLClient returns Azure MySQL client for the specified subscription.
	GetMySQLClient(ctx context.Context, subscription string) (DBServersClient, error)
	// GetPostgresClient returns Azure Postgres client for the specified subscription.
	GetPostgresClient(ctx context.Context, subscription string) (DBServersClient, error)
	// GetSubscriptionClient returns an Azure Subscriptions client
	GetSubscriptionClient(ctx context.Context) (*SubscriptionClient, error)
	// GetRedisClient returns an Azure Redis client for the given subscription.
	GetRedisClient(ctx context.Context, subscription string) (RedisClient, error)
	// GetRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
	GetRedisEnterpriseClient(ctx context.Context, subscription string) (RedisEnterpriseClient, error)
	// GetKubernetesClient returns an Azure AKS client for the specified subscription.
	GetKubernetesClient(ctx context.Context, subscription string) (AKSClient, error)
	// GetVirtualMachinesClient returns an Azure Virtual Machines client for the given subscription.
	GetVirtualMachinesClient(ctx context.Context, subscription string) (VirtualMachinesClient, error)
	// GetSQLServerClient returns an Azure SQL Server client for the
	// specified subscription.
	GetSQLServerClient(ctx context.Context, subscription string) (SQLServerClient, error)
	// GetManagedSQLServerClient returns an Azure ManagedSQL Server client
	// for the specified subscription.
	GetManagedSQLServerClient(ctx context.Context, subscription string) (ManagedSQLServerClient, error)
	// GetMySQLFlexServersClient returns an Azure MySQL Flexible Server client for the
	// specified subscription.
	GetMySQLFlexServersClient(ctx context.Context, subscription string) (MySQLFlexServersClient, error)
	// GetPostgresFlexServersClient returns an Azure PostgreSQL Flexible Server client for the
	// specified subscription.
	GetPostgresFlexServersClient(ctx context.Context, subscription string) (PostgresFlexServersClient, error)
	// GetRunCommandClient returns an Azure Run Command client for the given subscription.
	GetRunCommandClient(ctx context.Context, subscription string) (RunCommandClient, error)
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
		clt.credentialFunc = func(ctx context.Context) (azcore.TokenCredential, error) {
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

	azClients.credentialFunc = func(ctx context.Context) (azcore.TokenCredential, error) {
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

	// credentialFunc creates new Azure credential.
	credentialFunc func(ctx context.Context) (azcore.TokenCredential, error)
	credential     azcore.TokenCredential

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
}

// GetCredential returns default Azure token credential chain.
func (c *clients) GetCredential(ctx context.Context) (azcore.TokenCredential, error) {
	c.mtx.RLock()
	if c.credential != nil {
		defer c.mtx.RUnlock()
		return c.credential, nil
	}
	c.mtx.RUnlock()
	return c.initCredential(ctx)
}

// GetMySQLClient returns an AzureClient for MySQL for the given subscription.
func (c *clients) GetMySQLClient(ctx context.Context, subscription string) (DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.mySQLClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initMySQLClient(ctx, subscription)
}

// GetPostgresClient returns an AzureClient for Postgres for the given subscription.
func (c *clients) GetPostgresClient(ctx context.Context, subscription string) (DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.postgresClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initPostgresClient(ctx, subscription)
}

// GetSubscriptionClient returns an Azure client for listing subscriptions.
func (c *clients) GetSubscriptionClient(ctx context.Context) (*SubscriptionClient, error) {
	c.mtx.RLock()
	if c.subscriptionsClient != nil {
		defer c.mtx.RUnlock()
		return c.subscriptionsClient, nil
	}
	c.mtx.RUnlock()
	return c.initSubscriptionsClient(ctx)
}

// GetRedisClient returns an Azure Redis client for the given subscription.
func (c *clients) GetRedisClient(ctx context.Context, subscription string) (RedisClient, error) {
	return c.redisClients.Get(ctx, subscription, c.GetCredential)
}

// GetRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *clients) GetRedisEnterpriseClient(ctx context.Context, subscription string) (RedisEnterpriseClient, error) {
	return c.redisEnterpriseClients.Get(ctx, subscription, c.GetCredential)
}

// GetKubernetesClient returns an Azure client for listing AKS clusters.
func (c *clients) GetKubernetesClient(ctx context.Context, subscription string) (AKSClient, error) {
	c.mtx.RLock()
	if client, ok := c.kubernetesClient[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initKubernetesClient(ctx, subscription)
}

// GetVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *clients) GetVirtualMachinesClient(ctx context.Context, subscription string) (VirtualMachinesClient, error) {
	return c.virtualMachinesClients.Get(ctx, subscription, c.GetCredential)
}

// GetSQLServerClient returns an Azure client for listing SQL servers.
func (c *clients) GetSQLServerClient(ctx context.Context, subscription string) (SQLServerClient, error) {
	return c.sqlServerClients.Get(ctx, subscription, c.GetCredential)
}

// GetManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *clients) GetManagedSQLServerClient(ctx context.Context, subscription string) (ManagedSQLServerClient, error) {
	return c.managedSQLServerClients.Get(ctx, subscription, c.GetCredential)
}

// GetMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *clients) GetMySQLFlexServersClient(ctx context.Context, subscription string) (MySQLFlexServersClient, error) {
	return c.mySQLFlexServersClients.Get(ctx, subscription, c.GetCredential)
}

// GetPostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *clients) GetPostgresFlexServersClient(ctx context.Context, subscription string) (PostgresFlexServersClient, error) {
	return c.postgresFlexServersClients.Get(ctx, subscription, c.GetCredential)
}

// GetRunCommandClient returns an Azure Run Command client for the given
// subscription.
func (c *clients) GetRunCommandClient(ctx context.Context, subscription string) (RunCommandClient, error) {
	return c.runCommandClients.Get(ctx, subscription, c.GetCredential)
}

func (c *clients) initCredential(ctx context.Context) (azcore.TokenCredential, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.credential != nil { // If some other thread already got here first.
		return c.credential, nil
	}

	cred, err := c.credentialFunc(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.credential = cred
	return cred, nil
}

func (c *clients) initMySQLClient(ctx context.Context, subscription string) (DBServersClient, error) {
	cred, err := c.GetCredential(ctx)
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

func (c *clients) initPostgresClient(ctx context.Context, subscription string) (DBServersClient, error) {
	cred, err := c.GetCredential(ctx)
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

func (c *clients) initSubscriptionsClient(ctx context.Context) (*SubscriptionClient, error) {
	cred, err := c.GetCredential(ctx)
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

func (c *clients) initKubernetesClient(ctx context.Context, subscription string) (AKSClient, error) {
	cred, err := c.GetCredential(ctx)
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
