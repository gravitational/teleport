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
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/imds"
	awsimds "github.com/gravitational/teleport/lib/cloud/imds/aws"
	azureimds "github.com/gravitational/teleport/lib/cloud/imds/azure"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	oracleimds "github.com/gravitational/teleport/lib/cloud/imds/oracle"
)

// Clients provides interface for obtaining cloud provider clients.
type Clients interface {
	// GetInstanceMetadataClient returns instance metadata client based on which
	// cloud provider Teleport is running on, if any.
	GetInstanceMetadataClient(ctx context.Context) (imds.Client, error)
	// GCPClients is an interface for providing GCP API clients.
	GCPClients
	// AzureClients is an interface for Azure-specific API clients
	AzureClients
	// Closer closes all initialized clients.
	io.Closer
}

// GCPClients is an interface for providing GCP API clients.
type GCPClients interface {
	// GetGCPIAMClient returns GCP IAM client.
	GetGCPIAMClient(context.Context) (*gcpcredentials.IamCredentialsClient, error)
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
	// GetGCPGKEClient returns GKE client.
	GetGCPGKEClient(context.Context) (gcp.GKEClient, error)
	// GetGCPProjectsClient returns Projects client.
	GetGCPProjectsClient(context.Context) (gcp.ProjectsClient, error)
	// GetGCPInstancesClient returns instances client.
	GetGCPInstancesClient(context.Context) (gcp.InstancesClient, error)
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

type clientConstructor[T any] func(context.Context) (T, error)

// clientCache is a struct that holds a cloud client that will only be
// initialized once.
type clientCache[T any] struct {
	makeClient clientConstructor[T]
	client     T
	err        error
	once       sync.Once
}

// newClientCache creates a new client cache.
func newClientCache[T any](makeClient clientConstructor[T]) *clientCache[T] {
	return &clientCache[T]{makeClient: makeClient}
}

// GetClient gets the client, initializing it if necessary.
func (c *clientCache[T]) GetClient(ctx context.Context) (T, error) {
	c.once.Do(func() {
		c.client, c.err = c.makeClient(ctx)
	})
	return c.client, trace.Wrap(c.err)
}

func newAzureClients() (*azureClients, error) {
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

	return azClients, nil
}

// ClientsOption allows setting options as functional arguments to cloudClients.
type ClientsOption func(cfg *cloudClients)

// NewClients returns a new instance of cloud clients retriever.
func NewClients(opts ...ClientsOption) (Clients, error) {
	azClients, err := newAzureClients()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cloudClients := &cloudClients{
		gcpClients: gcpClients{
			gcpSQLAdmin:  newClientCache(gcp.NewSQLAdminClient),
			gcpGKE:       newClientCache(gcp.NewGKEClient),
			gcpProjects:  newClientCache(gcp.NewProjectsClient),
			gcpInstances: newClientCache(gcp.NewInstancesClient),
		},
		azureClients: azClients,
	}

	for _, opt := range opts {
		opt(cloudClients)
	}

	return cloudClients, nil
}

// cloudClients implements Clients
var _ Clients = (*cloudClients)(nil)

type cloudClients struct {
	// instanceMetadata is the cached instance metadata client.
	instanceMetadata imds.Client
	// gcpClients contains GCP-specific clients.
	gcpClients
	// azureClients contains Azure-specific clients.
	*azureClients
	// mtx is used for locking.
	mtx sync.RWMutex
}

// gcpClients contains GCP-specific clients.
type gcpClients struct {
	// gcpIAM is the cached GCP IAM client.
	gcpIAM *gcpcredentials.IamCredentialsClient
	// gcpSQLAdmin is the cached GCP Cloud SQL Admin client.
	gcpSQLAdmin *clientCache[gcp.SQLAdminClient]
	// gcpGKE is the cached GCP Cloud GKE client.
	gcpGKE *clientCache[gcp.GKEClient]
	// gcpProjects is the cached GCP Cloud Projects client.
	gcpProjects *clientCache[gcp.ProjectsClient]
	// gcpInstances is the cached GCP instances client.
	gcpInstances *clientCache[gcp.InstancesClient]
}

// azureClients contains Azure-specific clients.
type azureClients struct {
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
func (c *cloudClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.RLock()
	if c.gcpIAM != nil {
		defer c.mtx.RUnlock()
		return c.gcpIAM, nil
	}
	c.mtx.RUnlock()
	return c.initGCPIAMClient(ctx)
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *cloudClients) GetGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.gcpSQLAdmin.GetClient(ctx)
}

// GetInstanceMetadataClient returns the instance metadata.
func (c *cloudClients) GetInstanceMetadataClient(ctx context.Context) (imds.Client, error) {
	c.mtx.RLock()
	if c.instanceMetadata != nil {
		defer c.mtx.RUnlock()
		return c.instanceMetadata, nil
	}
	c.mtx.RUnlock()
	return c.initInstanceMetadata(ctx)
}

// GetGCPGKEClient returns GKE client.
func (c *cloudClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.gcpGKE.GetClient(ctx)
}

// GetGCPProjectsClient returns Project client.
func (c *cloudClients) GetGCPProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.gcpProjects.GetClient(ctx)
}

// GetGCPInstancesClient returns instances client.
func (c *cloudClients) GetGCPInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.gcpInstances.GetClient(ctx)
}

// GetAzureCredential returns default Azure token credential chain.
func (c *cloudClients) GetAzureCredential() (azcore.TokenCredential, error) {
	c.mtx.RLock()
	if c.azureCredential != nil {
		defer c.mtx.RUnlock()
		return c.azureCredential, nil
	}
	c.mtx.RUnlock()
	return c.initAzureCredential()
}

// GetAzureMySQLClient returns an AzureClient for MySQL for the given subscription.
func (c *cloudClients) GetAzureMySQLClient(subscription string) (azure.DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.azureMySQLClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzureMySQLClient(subscription)
}

// GetAzurePostgresClient returns an AzureClient for Postgres for the given subscription.
func (c *cloudClients) GetAzurePostgresClient(subscription string) (azure.DBServersClient, error) {
	c.mtx.RLock()
	if client, ok := c.azurePostgresClients[subscription]; ok {
		c.mtx.RUnlock()
		return client, nil
	}
	c.mtx.RUnlock()
	return c.initAzurePostgresClient(subscription)
}

// GetAzureSubscriptionClient returns an Azure client for listing subscriptions.
func (c *cloudClients) GetAzureSubscriptionClient() (*azure.SubscriptionClient, error) {
	c.mtx.RLock()
	if c.azureSubscriptionsClient != nil {
		defer c.mtx.RUnlock()
		return c.azureSubscriptionsClient, nil
	}
	c.mtx.RUnlock()
	return c.initAzureSubscriptionsClient()
}

// GetAzureRedisClient returns an Azure Redis client for the given subscription.
func (c *cloudClients) GetAzureRedisClient(subscription string) (azure.RedisClient, error) {
	return c.azureRedisClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *cloudClients) GetAzureRedisEnterpriseClient(subscription string) (azure.RedisEnterpriseClient, error) {
	return c.azureRedisEnterpriseClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureKubernetesClient returns an Azure client for listing AKS clusters.
func (c *cloudClients) GetAzureKubernetesClient(subscription string) (azure.AKSClient, error) {
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
func (c *cloudClients) GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error) {
	return c.azureVirtualMachinesClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureSQLServerClient returns an Azure client for listing SQL servers.
func (c *cloudClients) GetAzureSQLServerClient(subscription string) (azure.SQLServerClient, error) {
	return c.azureSQLServerClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *cloudClients) GetAzureManagedSQLServerClient(subscription string) (azure.ManagedSQLServerClient, error) {
	return c.azureManagedSQLServerClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *cloudClients) GetAzureMySQLFlexServersClient(subscription string) (azure.MySQLFlexServersClient, error) {
	return c.azureMySQLFlexServersClients.Get(subscription, c.GetAzureCredential)
}

// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *cloudClients) GetAzurePostgresFlexServersClient(subscription string) (azure.PostgresFlexServersClient, error) {
	return c.azurePostgresFlexServersClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRunCommandClient returns an Azure Run Command client for the given
// subscription.
func (c *cloudClients) GetAzureRunCommandClient(subscription string) (azure.RunCommandClient, error) {
	return c.azureRunCommandClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRoleDefinitionsClient returns an Azure Role Definitions client
func (c *cloudClients) GetAzureRoleDefinitionsClient(subscription string) (azure.RoleDefinitionsClient, error) {
	return c.azureRoleDefinitionsClients.Get(subscription, c.GetAzureCredential)
}

// GetAzureRoleAssignmentsClient returns an Azure Role Assignments client
func (c *cloudClients) GetAzureRoleAssignmentsClient(subscription string) (azure.RoleAssignmentsClient, error) {
	return c.azureRoleAssignmentsClients.Get(subscription, c.GetAzureCredential)
}

// Close closes all initialized clients.
func (c *cloudClients) Close() (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil {
		err = c.gcpIAM.Close()
		c.gcpIAM = nil
	}
	return trace.Wrap(err)
}

func (c *cloudClients) initGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
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

func (c *cloudClients) initAzureCredential() (azcore.TokenCredential, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.azureCredential != nil { // If some other thread already got here first.
		return c.azureCredential, nil
	}
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &azidentity.DefaultAzureCredentialOptions{}
	cred, err := azidentity.NewDefaultAzureCredential(options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.azureCredential = cred
	return cred, nil
}

func (c *cloudClients) initAzureMySQLClient(subscription string) (azure.DBServersClient, error) {
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

func (c *cloudClients) initAzurePostgresClient(subscription string) (azure.DBServersClient, error) {
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

func (c *cloudClients) initAzureSubscriptionsClient() (*azure.SubscriptionClient, error) {
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

// initInstanceMetadata initializes the instance metadata client.
func (c *cloudClients) initInstanceMetadata(ctx context.Context) (imds.Client, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.instanceMetadata != nil { // If some other thread already got here first.
		return c.instanceMetadata, nil
	}

	providers := []func(ctx context.Context) (imds.Client, error){
		func(ctx context.Context) (imds.Client, error) {
			clt, err := awsimds.NewInstanceMetadataClient(ctx)
			return clt, trace.Wrap(err)
		},
		func(ctx context.Context) (imds.Client, error) {
			return azureimds.NewInstanceMetadataClient(), nil
		},
		func(ctx context.Context) (imds.Client, error) {
			instancesClient, err := gcp.NewInstancesClient(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			clt, err := gcpimds.NewInstanceMetadataClient(instancesClient)
			return clt, trace.Wrap(err)
		},
		func(ctx context.Context) (imds.Client, error) {
			return oracleimds.NewInstanceMetadataClient(), nil
		},
	}

	client, err := DiscoverInstanceMetadata(ctx, providers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.instanceMetadata = client
	return client, nil
}

func (c *cloudClients) initAzureKubernetesClient(subscription string) (azure.AKSClient, error) {
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

// TestCloudClients implements Clients
var _ Clients = (*TestCloudClients)(nil)

// TestCloudClients are used in tests.
type TestCloudClients struct {
	GCPSQL                  gcp.SQLAdminClient
	GCPGKE                  gcp.GKEClient
	GCPProjects             gcp.ProjectsClient
	GCPInstances            gcp.InstancesClient
	InstanceMetadata        imds.Client
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

// GetGCPIAMClient returns GCP IAM client.
func (c *TestCloudClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	return gcpcredentials.NewIamCredentialsClient(ctx,
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())), // Insecure must be set for unauth client.
		option.WithoutAuthentication())
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *TestCloudClients) GetGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	return c.GCPSQL, nil
}

// GetInstanceMetadataClient returns the instance metadata.
func (c *TestCloudClients) GetInstanceMetadataClient(ctx context.Context) (imds.Client, error) {
	return c.InstanceMetadata, nil
}

// GetGCPGKEClient returns GKE client.
func (c *TestCloudClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.GCPGKE, nil
}

// GetGCPGKEClient returns GKE client.
func (c *TestCloudClients) GetGCPProjectsClient(ctx context.Context) (gcp.ProjectsClient, error) {
	return c.GCPProjects, nil
}

// GetGCPInstancesClient returns instances client.
func (c *TestCloudClients) GetGCPInstancesClient(ctx context.Context) (gcp.InstancesClient, error) {
	return c.GCPInstances, nil
}

// GetAzureCredential returns default Azure token credential chain.
func (c *TestCloudClients) GetAzureCredential() (azcore.TokenCredential, error) {
	return &azidentity.ChainedTokenCredential{}, nil
}

// GetAzureMySQLClient returns an AzureMySQLClient for the specified subscription
func (c *TestCloudClients) GetAzureMySQLClient(subscription string) (azure.DBServersClient, error) {
	if len(c.AzureMySQLPerSub) != 0 {
		return c.AzureMySQLPerSub[subscription], nil
	}
	return c.AzureMySQL, nil
}

// GetAzurePostgresClient returns an AzurePostgresClient for the specified subscription
func (c *TestCloudClients) GetAzurePostgresClient(subscription string) (azure.DBServersClient, error) {
	if len(c.AzurePostgresPerSub) != 0 {
		return c.AzurePostgresPerSub[subscription], nil
	}
	return c.AzurePostgres, nil
}

// GetAzureKubernetesClient returns an AKS client for the specified subscription
func (c *TestCloudClients) GetAzureKubernetesClient(subscription string) (azure.AKSClient, error) {
	if len(c.AzureAKSClientPerSub) != 0 {
		return c.AzureAKSClientPerSub[subscription], nil
	}
	return c.AzureAKSClient, nil
}

// GetAzureSubscriptionClient returns an Azure SubscriptionClient
func (c *TestCloudClients) GetAzureSubscriptionClient() (*azure.SubscriptionClient, error) {
	return c.AzureSubscriptionClient, nil
}

// GetAzureRedisClient returns an Azure Redis client for the given subscription.
func (c *TestCloudClients) GetAzureRedisClient(subscription string) (azure.RedisClient, error) {
	return c.AzureRedis, nil
}

// GetAzureRedisEnterpriseClient returns an Azure Redis Enterprise client for the given subscription.
func (c *TestCloudClients) GetAzureRedisEnterpriseClient(subscription string) (azure.RedisEnterpriseClient, error) {
	return c.AzureRedisEnterprise, nil
}

// GetAzureVirtualMachinesClient returns an Azure Virtual Machines client for
// the given subscription.
func (c *TestCloudClients) GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error) {
	return c.AzureVirtualMachines, nil
}

// GetAzureSQLServerClient returns an Azure client for listing SQL servers.
func (c *TestCloudClients) GetAzureSQLServerClient(subscription string) (azure.SQLServerClient, error) {
	return c.AzureSQLServer, nil
}

// GetAzureManagedSQLServerClient returns an Azure client for listing managed
// SQL servers.
func (c *TestCloudClients) GetAzureManagedSQLServerClient(subscription string) (azure.ManagedSQLServerClient, error) {
	return c.AzureManagedSQLServer, nil
}

// GetAzureMySQLFlexServersClient returns an Azure MySQL Flexible server client for listing MySQL Flexible servers.
func (c *TestCloudClients) GetAzureMySQLFlexServersClient(subscription string) (azure.MySQLFlexServersClient, error) {
	return c.AzureMySQLFlex, nil
}

// GetAzurePostgresFlexServersClient returns an Azure PostgreSQL Flexible server client for listing PostgreSQL Flexible servers.
func (c *TestCloudClients) GetAzurePostgresFlexServersClient(subscription string) (azure.PostgresFlexServersClient, error) {
	return c.AzurePostgresFlex, nil
}

// GetAzureRunCommandClient returns an Azure Run Command client for the given subscription.
func (c *TestCloudClients) GetAzureRunCommandClient(subscription string) (azure.RunCommandClient, error) {
	return c.AzureRunCommand, nil
}

// GetAzureRoleDefinitionsClient returns an Azure Role Definitions client for the given subscription.
func (c *TestCloudClients) GetAzureRoleDefinitionsClient(subscription string) (azure.RoleDefinitionsClient, error) {
	return c.AzureRoleDefinitions, nil
}

// GetAzureRoleAssignmentsClient returns an Azure Role Assignments client for the given subscription.
func (c *TestCloudClients) GetAzureRoleAssignmentsClient(subscription string) (azure.RoleAssignmentsClient, error) {
	return c.AzureRoleAssignments, nil
}

// Close closes all initialized clients.
func (c *TestCloudClients) Close() error {
	return nil
}
