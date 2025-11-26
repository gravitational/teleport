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

package cloud

import (
	"context"
	"io"
	"sync"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
)

// GCPClients is an interface for providing GCP API clients.
type GCPClients interface {
	// GetGCPIAMClient returns GCP IAM client.
	GetGCPIAMClient(context.Context) (*gcpcredentials.IamCredentialsClient, error)
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
	// GetGCPAlloyDBClient returns GCP AlloyDB Admin client.
	GetGCPAlloyDBClient(context.Context) (gcp.AlloyDBAdminClient, error)
	// GetGCPGKEClient returns GKE client.
	GetGCPGKEClient(context.Context) (gcp.GKEClient, error)
	// GetGCPProjectsClient returns Projects client.
	GetGCPProjectsClient(context.Context) (gcp.ProjectsClient, error)
	// GetGCPInstancesClient returns instances client.
	GetGCPInstancesClient(context.Context) (gcp.InstancesClient, error)

	io.Closer
}

// AzureClients is an interface for Azure-specific API clients
type AzureClients interface {
	// GetAzureCredential returns Azure default token credential chain.
	GetAzureCredential() (azcore.TokenCredential, error)
	// GetAzureMySQLClient returns Azure MySQL client for the specified subscription.
	GetAzureMySQLClient(subscription string) (azure.DBServersClient, error)
	// GetAzurePostgresClient returns Azure Postgres client for the specified subscription.
	GetAzurePostgresClient(subscription string) (azure.DBServersClient, error)
	// GetAzureSubscriptionClient returns an Azure Subscriptions client
	GetAzureSubscriptionClient() (*azure.SubscriptionClient, error)
	// GetAzureRedisClient returns an Azure Redis client for the given subscription.
	GetAzureRedisClient(subscription string) (azure.RedisClient, error)
	// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
	GetAzureRedisEnterpriseClient(subscription string) (azure.RedisEnterpriseClient, error)
	// GetAzureKubernetesClient returns an Azure AKS client for the specified subscription.
	GetAzureKubernetesClient(subscription string) (azure.AKSClient, error)
	// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for the given subscription.
	GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error)
	// GetAzureSQLServerClient returns an Azure SQL Server client for the
	// specified subscription.
	GetAzureSQLServerClient(subscription string) (azure.SQLServerClient, error)
	// GetAzureManagedSQLServerClient returns an Azure ManagedSQL Server client
	// for the specified subscription.
	GetAzureManagedSQLServerClient(subscription string) (azure.ManagedSQLServerClient, error)
	// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible Server client for the
	// specified subscription.
	GetAzureMySQLFlexServersClient(subscription string) (azure.MySQLFlexServersClient, error)
	// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible Server client for the
	// specified subscription.
	GetAzurePostgresFlexServersClient(subscription string) (azure.PostgresFlexServersClient, error)
	// GetAzureRunCommandClient returns an Azure Run Command client for the given subscription.
	GetAzureRunCommandClient(subscription string) (azure.RunCommandClient, error)
}

// AzureClientsOption is an option to pass to NewAzureClients
type AzureClientsOption func(clients *azureClients)

type azureOIDCCredentials interface {
	GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error)
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// WithAzureIntegrationCredentials configures Azure cloud clients to use integration credentials.
func WithAzureIntegrationCredentials(integrationName string, auth azureOIDCCredentials) AzureClientsOption {
	return func(clt *azureClients) {
		clt.newAzureCredentialFunc = func() (azcore.TokenCredential, error) {
			ctx := context.TODO()
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

// NewAzureClients returns a new instance of Azure SDK clients.
func NewAzureClients(opts ...AzureClientsOption) (AzureClients, error) {
	azClients := &azureClients{
		azureMySQLClients:     make(map[string]azure.DBServersClient),
		azurePostgresClients:  make(map[string]azure.DBServersClient),
		azureKubernetesClient: make(map[string]azure.AKSClient),
	}
	var err error
	azClients.azureRedisClients, err = azure.NewClientMap(azure.NewRedisClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureRedisEnterpriseClients, err = azure.NewClientMap(azure.NewRedisEnterpriseClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureVirtualMachinesClients, err = azure.NewClientMap(azure.NewVirtualMachinesClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureSQLServerClients, err = azure.NewClientMap(azure.NewSQLClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureManagedSQLServerClients, err = azure.NewClientMap(azure.NewManagedSQLClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureMySQLFlexServersClients, err = azure.NewClientMap(azure.NewMySQLFlexServersClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azurePostgresFlexServersClients, err = azure.NewClientMap(azure.NewPostgresFlexServersClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients.azureRunCommandClients, err = azure.NewClientMap(azure.NewRunCommandClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	azClients.newAzureCredentialFunc = func() (azcore.TokenCredential, error) {
		// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
		return azidentity.NewDefaultAzureCredential(nil)
	}

	for _, opt := range opts {
		opt(azClients)
	}

	return azClients, nil
}

// NewGCPClients returns a new instance of GCP SDK clients.
func NewGCPClients() GCPClients {
	return &gcpClients{
		gcpSQLAdmin:     newClientCache(gcp.NewSQLAdminClient),
		gcpAlloyDBAdmin: newClientCache(gcp.NewAlloyDBAdminClient),
		gcpGKE:          newClientCache(gcp.NewGKEClient),
		gcpProjects:     newClientCache(gcp.NewProjectsClient),
		gcpInstances:    newClientCache(gcp.NewInstancesClient),
	}
}

// gcpClients contains GCP-specific clients.
type gcpClients struct {
	// mtx is used for locking.
	mtx sync.RWMutex

	// gcpIAM is the cached GCP IAM client.
	gcpIAM *gcpcredentials.IamCredentialsClient
	// gcpSQLAdmin is the cached GCP Cloud SQL Admin client.
	gcpSQLAdmin *clientCache[gcp.SQLAdminClient]
	// gcpAlloyDBAdmin is the cached GCP AlloyDB Admin client.
	gcpAlloyDBAdmin *clientCache[gcp.AlloyDBAdminClient]
	// gcpGKE is the cached GCP Cloud GKE client.
	gcpGKE *clientCache[gcp.GKEClient]
	// gcpProjects is the cached GCP Cloud Projects client.
	gcpProjects *clientCache[gcp.ProjectsClient]
	// gcpInstances is the cached GCP instances client.
	gcpInstances *clientCache[gcp.InstancesClient]
}

// azureClients contains Azure-specific clients.
type azureClients struct {
	// mtx is used for locking.
	mtx sync.RWMutex

	// newAzureCredentialFunc creates new Azure credential.
	newAzureCredentialFunc func() (azcore.TokenCredential, error)
	// azureCredential is the cached Azure credential.
	azureCredential azcore.TokenCredential

	// azureMySQLClients is the cached Azure MySQL Server clients.
	azureMySQLClients map[string]azure.DBServersClient
	// azurePostgresClients is the cached Azure Postgres Server clients.
	azurePostgresClients map[string]azure.DBServersClient
	// azureSubscriptionsClient is the cached Azure Subscriptions client.
	azureSubscriptionsClient *azure.SubscriptionClient
	// azureRedisClients contains the cached Azure Redis clients.
	azureRedisClients azure.ClientMap[azure.RedisClient]
	// azureRedisEnterpriseClients contains the cached Azure Redis Enterprise clients.
	azureRedisEnterpriseClients azure.ClientMap[azure.RedisEnterpriseClient]
	// azureKubernetesClient is the cached Azure Kubernetes client.
	azureKubernetesClient map[string]azure.AKSClient
	// azureVirtualMachinesClients contains the cached Azure Virtual Machines clients.
	azureVirtualMachinesClients azure.ClientMap[azure.VirtualMachinesClient]
	// azureSQLServerClient is the cached Azure SQL Server client.
	azureSQLServerClients azure.ClientMap[azure.SQLServerClient]
	// azureManagedSQLServerClient is the cached Azure Managed SQL Server
	// client.
	azureManagedSQLServerClients azure.ClientMap[azure.ManagedSQLServerClient]
	// azureMySQLFlexServersClients is the cached Azure MySQL Flexible Server
	// client.
	azureMySQLFlexServersClients azure.ClientMap[azure.MySQLFlexServersClient]
	// azurePostgresFlexServersClients is the cached Azure PostgreSQL Flexible Server
	// client.
	azurePostgresFlexServersClients azure.ClientMap[azure.PostgresFlexServersClient]
	// azureRunCommandClients contains the cached Azure Run Command clients.
	azureRunCommandClients azure.ClientMap[azure.RunCommandClient]
	// azureRoleDefinitionsClients contains the cached Azure Role Definitions clients.
	azureRoleDefinitionsClients azure.ClientMap[azure.RoleDefinitionsClient]
	// azureRoleAssignmentsClients contains the cached Azure Role Assignments clients.
	azureRoleAssignmentsClients azure.ClientMap[azure.RoleAssignmentsClient]
}

// GetGCPIAMClient returns GCP IAM client.
func (c *gcpClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.RLock()
	if c.gcpIAM != nil {
		defer c.mtx.RUnlock()
		return c.gcpIAM, nil
	}
	c.mtx.RUnlock()
	return c.initGCPIAMClient(ctx)
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *gcpClients) GetGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.gcpSQLAdmin.GetClient(ctx)
}

// GetGCPAlloyDBClient returns GCP AlloyDB Admin client.
func (c *gcpClients) GetGCPAlloyDBClient(ctx context.Context) (gcp.AlloyDBAdminClient, error) {
	return c.gcpAlloyDBAdmin.GetClient(ctx)
}

// GetGCPGKEClient returns GKE client.
func (c *gcpClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.gcpGKE.GetClient(ctx)
}

// GetGCPProjectsClient returns Project client.
func (c *gcpClients) GetGCPProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.gcpProjects.GetClient(ctx)
}

// GetGCPInstancesClient returns instances client.
func (c *gcpClients) GetGCPInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.gcpInstances.GetClient(ctx)
}

// GetAzureCredential returns default Azure token credential chain.
func (c *azureClients) GetAzureCredential() (azcore.TokenCredential, error) {
	c.mtx.RLock()
	if c.azureCredential != nil {
		defer c.mtx.RUnlock()
		return c.azureCredential, nil
	}
	c.mtx.RUnlock()
	return c.initAzureCredential()
}

// GetAzureMySQLClient returns an AzureClient for MySQL for the given subscription.
func (c *azureClients) GetAzureMySQLClient(subscription string) (azure.DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.azureMySQLClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzureMySQLClient(subscription)
}

// GetAzurePostgresClient returns an AzureClient for Postgres for the given subscription.
func (c *azureClients) GetAzurePostgresClient(subscription string) (azure.DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.azurePostgresClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzurePostgresClient(subscription)
}

// GetAzureSubscriptionClient returns an Azure client for listing subscriptions.
func (c *azureClients) GetAzureSubscriptionClient() (*azure.SubscriptionClient, error) {
	c.mtx.RLock()
	if c.azureSubscriptionsClient != nil {
		defer c.mtx.RUnlock()
		return c.azureSubscriptionsClient, nil
	}
	c.mtx.RUnlock()
	return c.initAzureSubscriptionsClient()
}

// GetAzureRedisClient returns an Azure Redis client for the given subscription.
func (c *azureClients) GetAzureRedisClient(subscription string) (azure.RedisClient, error) {
	return c.azureRedisClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *azureClients) GetAzureRedisEnterpriseClient(subscription string) (azure.RedisEnterpriseClient, error) {
	return c.azureRedisEnterpriseClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureKubernetesClient returns an Azure client for listing AKS clusters.
func (c *azureClients) GetAzureKubernetesClient(subscription string) (azure.AKSClient, error) {
	c.mtx.RLock()
	if client, ok := c.azureKubernetesClient[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzureKubernetesClient(subscription)
}

// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *azureClients) GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error) {
	return c.azureVirtualMachinesClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureSQLServerClient returns an Azure client for listing SQL servers.
func (c *azureClients) GetAzureSQLServerClient(subscription string) (azure.SQLServerClient, error) {
	return c.azureSQLServerClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *azureClients) GetAzureManagedSQLServerClient(subscription string) (azure.ManagedSQLServerClient, error) {
	return c.azureManagedSQLServerClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *azureClients) GetAzureMySQLFlexServersClient(subscription string) (azure.MySQLFlexServersClient, error) {
	return c.azureMySQLFlexServersClients.Get(subscription, c.GetAzureCredential)
}

// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *azureClients) GetAzurePostgresFlexServersClient(subscription string) (azure.PostgresFlexServersClient, error) {
	return c.azurePostgresFlexServersClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRunCommandClient returns an Azure Run Command client for the given
// subscription.
func (c *azureClients) GetAzureRunCommandClient(subscription string) (azure.RunCommandClient, error) {
	return c.azureRunCommandClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRoleDefinitionsClient returns an Azure Role Definitions client
func (c *azureClients) GetAzureRoleDefinitionsClient(subscription string) (azure.RoleDefinitionsClient, error) {
	return c.azureRoleDefinitionsClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRoleAssignmentsClient returns an Azure Role Assignments client
func (c *azureClients) GetAzureRoleAssignmentsClient(subscription string) (azure.RoleAssignmentsClient, error) {
	return c.azureRoleAssignmentsClients.Get(subscription, c.GetAzureCredential)
}

// Close closes all initialized clients.
func (c *gcpClients) Close() (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil {
		err = c.gcpIAM.Close()
		c.gcpIAM = nil
	}
	return trace.Wrap(err)
}

func (c *gcpClients) initGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil { // If some other thread already got here first.
		return c.gcpIAM, nil
	}
	gcpIAM, err := gcpcredentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpIAM = gcpIAM
	return gcpIAM, nil
}

func (c *azureClients) initAzureCredential() (azcore.TokenCredential, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.azureCredential != nil { // If some other thread already got here first.
		return c.azureCredential, nil
	}

	cred, err := c.newAzureCredentialFunc()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.azureCredential = cred
	return cred, nil
}

func (c *azureClients) initAzureMySQLClient(subscription string) (azure.DBServersClient, error) {
	cred, err := c.GetAzureCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.azureMySQLClients[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}

	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armmysql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := azure.NewMySQLServersClient(api)
	c.azureMySQLClients[subscription] = client
	return client, nil
}

func (c *azureClients) initAzurePostgresClient(subscription string) (azure.DBServersClient, error) {
	cred, err := c.GetAzureCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.azurePostgresClients[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armpostgresql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := azure.NewPostgresServerClient(api)
	c.azurePostgresClients[subscription] = client
	return client, nil
}

func (c *azureClients) initAzureSubscriptionsClient() (*azure.SubscriptionClient, error) {
	cred, err := c.GetAzureCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.azureSubscriptionsClient != nil { // If some other thread already got here first.
		return c.azureSubscriptionsClient, nil
	}
	// TODO(gavin): if/when we support AzureChina/AzureGovernment,
	// we will need to specify the cloud in these options
	opts := &arm.ClientOptions{}
	armClient, err := armsubscription.NewSubscriptionsClient(cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := azure.NewSubscriptionClient(armClient)
	c.azureSubscriptionsClient = client
	return client, nil
}

func (c *azureClients) initAzureKubernetesClient(subscription string) (azure.AKSClient, error) {
	cred, err := c.GetAzureCredential()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if client, ok := c.azureKubernetesClient[subscription]; ok { // If some other thread already got here first.
		return client, nil
	}
	// TODO(tigrato): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	api, err := armcontainerservice.NewManagedClustersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := azure.NewAKSClustersClient(
		api, func(options *azidentity.DefaultAzureCredentialOptions) (azure.GetToken, error) {
			cc, err := azidentity.NewDefaultAzureCredential(options)
			return cc, err
		})
	c.azureKubernetesClient[subscription] = client
	return client, nil
}
