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
	"time"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/opensearchservice/opensearchserviceiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/aws/aws-sdk-go/service/redshiftserverless/redshiftserverlessiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/types"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/imds"
	awsimds "github.com/gravitational/teleport/lib/cloud/imds/aws"
	azureimds "github.com/gravitational/teleport/lib/cloud/imds/azure"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// Clients provides interface for obtaining cloud provider clients.
type Clients interface {
	// GetInstanceMetadataClient returns instance metadata client based on which
	// cloud provider Teleport is running on, if any.
	GetInstanceMetadataClient(ctx context.Context) (imds.Client, error)
	// GCPClients is an interface for providing GCP API clients.
	GCPClients
	// AWSClients is an interface for providing AWS API clients.
	AWSClients
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

// AWSClients is an interface for providing AWS API clients.
type AWSClients interface {
	// GetAWSSession returns AWS session for the specified region and any role(s).
	GetAWSSession(ctx context.Context, region string, opts ...AWSOptionsFn) (*awssession.Session, error)
	// GetAWSRDSClient returns AWS RDS client for the specified region.
	GetAWSRDSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (rdsiface.RDSAPI, error)
	// GetAWSRedshiftClient returns AWS Redshift client for the specified region.
	GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftiface.RedshiftAPI, error)
	// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
	GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftserverlessiface.RedshiftServerlessAPI, error)
	// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
	GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSOptionsFn) (elasticacheiface.ElastiCacheAPI, error)
	// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
	GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSOptionsFn) (memorydbiface.MemoryDBAPI, error)
	// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
	GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSOptionsFn) (opensearchserviceiface.OpenSearchServiceAPI, error)
	// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
	GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSOptionsFn) (secretsmanageriface.SecretsManagerAPI, error)
	// GetAWSIAMClient returns AWS IAM client for the specified region.
	GetAWSIAMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (iamiface.IAMAPI, error)
	// GetAWSSTSClient returns AWS STS client for the specified region.
	GetAWSSTSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (stsiface.STSAPI, error)
	// GetAWSEC2Client returns AWS EC2 client for the specified region.
	GetAWSEC2Client(ctx context.Context, region string, opts ...AWSOptionsFn) (ec2iface.EC2API, error)
	// GetAWSSSMClient returns AWS SSM client for the specified region.
	GetAWSSSMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (ssmiface.SSMAPI, error)
	// GetAWSEKSClient returns AWS EKS client for the specified region.
	GetAWSEKSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (eksiface.EKSAPI, error)
	// GetAWSKMSClient returns AWS KMS client for the specified region.
	GetAWSKMSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (kmsiface.KMSAPI, error)
	// GetAWSS3Client returns AWS S3 client.
	GetAWSS3Client(ctx context.Context, region string, opts ...AWSOptionsFn) (s3iface.S3API, error)
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
	awsSessionsCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 15 * time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	azClients, err := newAzureClients()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cloudClients := &cloudClients{
		awsSessionsCache: awsSessionsCache,
		gcpClients: gcpClients{
			gcpSQLAdmin:  newClientCache[gcp.SQLAdminClient](gcp.NewSQLAdminClient),
			gcpGKE:       newClientCache[gcp.GKEClient](gcp.NewGKEClient),
			gcpProjects:  newClientCache[gcp.ProjectsClient](gcp.NewProjectsClient),
			gcpInstances: newClientCache[gcp.InstancesClient](gcp.NewInstancesClient),
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

// WithAWSIntegrationSessionProvider sets an integration session generator for AWS apis.
// If a client is requested for a specific Integration, instead of using the ambient credentials, this generator is used to fetch the AWS Session.
func WithAWSIntegrationSessionProvider(sessionProvider AWSIntegrationSessionProvider) func(*cloudClients) {
	return func(cc *cloudClients) {
		cc.awsIntegrationSessionProviderFn = sessionProvider
	}
}

// AWSIntegrationSessionProvider defines a function that creates an [awssession.Session] from a Region and an Integration.
// This is used to generate aws sessions for clients that must use an Integration instead of ambient credentials.
type AWSIntegrationSessionProvider func(ctx context.Context, region string, integration string) (*awssession.Session, error)

type awsSessionCacheKey struct {
	region      string
	integration string
	roleARN     string
	externalID  string
}

type cloudClients struct {
	// awsSessionsCache is a cache of AWS sessions, where the cache key is
	// an instance of awsSessionCacheKey.
	awsSessionsCache *utils.FnCache
	// awsIntegrationSessionProviderFn is a AWS Session Generator that uses an Integration to generate an AWS Session.
	awsIntegrationSessionProviderFn AWSIntegrationSessionProvider
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

// credentialsSource defines where the credentials must come from.
type credentialsSource int

const (
	// credentialsSourceAmbient uses the default Cloud SDK method to load the credentials.
	credentialsSourceAmbient = iota + 1
	// CredentialsSourceIntegration uses an Integration to load the credentials.
	credentialsSourceIntegration
)

// awsOptions a struct of additional options for assuming an AWS role
// when construction an underlying AWS session.
type awsOptions struct {
	// baseSession is a session to use instead of the default session for an
	// AWS region, which is used to enable role chaining.
	baseSession *awssession.Session
	// assumeRoleARN is the AWS IAM Role ARN to assume.
	assumeRoleARN string
	// assumeRoleExternalID is used to assume an external AWS IAM Role.
	assumeRoleExternalID string

	// credentialsSource describes which source to use to fetch credentials.
	credentialsSource credentialsSource

	// integration is the name of the integration to be used to fetch the credentials.
	integration string

	// customRetryer is a custom retryer to use for the session.
	customRetryer request.Retryer

	// maxRetries is the maximum number of retries to use for the session.
	maxRetries *int

	// withoutSessionCache disables the session cache for the AWS session.
	withoutSessionCache bool
}

func (a *awsOptions) checkAndSetDefaults() error {
	switch a.credentialsSource {
	case credentialsSourceAmbient:
		if a.integration != "" {
			return trace.BadParameter("integration and ambient credentials cannot be used at the same time")
		}
	case credentialsSourceIntegration:
		if a.integration == "" {
			return trace.BadParameter("missing integration name")
		}
	default:
		return trace.BadParameter("missing credentials source (ambient or integration)")
	}

	return nil
}

// AWSOptionsFn is an option function for setting additional options
// when getting an AWS session.
type AWSOptionsFn func(*awsOptions)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) AWSOptionsFn {
	return func(options *awsOptions) {
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// WithoutSessionCache disables the session cache for the AWS session.
func WithoutSessionCache() AWSOptionsFn {
	return func(options *awsOptions) {
		options.withoutSessionCache = true
	}
}

// WithAssumeRoleFromAWSMeta extracts options needed from AWS metadata for
// assuming an AWS role.
func WithAssumeRoleFromAWSMeta(meta types.AWS) AWSOptionsFn {
	return WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID)
}

// WithChainedAssumeRole sets a role to assume with a base session to use
// for assuming the role, which enables role chaining.
func WithChainedAssumeRole(session *awssession.Session, roleARN, externalID string) AWSOptionsFn {
	return func(options *awsOptions) {
		options.baseSession = session
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// WithRetryer sets a custom retryer for the session.
func WithRetryer(retryer request.Retryer) AWSOptionsFn {
	return func(options *awsOptions) {
		options.customRetryer = retryer
	}
}

// WithMaxRetries sets the maximum allowed value for the sdk to keep retrying.
func WithMaxRetries(maxRetries int) AWSOptionsFn {
	return func(options *awsOptions) {
		options.maxRetries = &maxRetries
	}
}

// WithCredentialsMaybeIntegration sets the credential source to be
// - ambient if the integration is an empty string
// - integration, otherwise
func WithCredentialsMaybeIntegration(integration string) AWSOptionsFn {
	if integration != "" {
		return withIntegrationCredentials(integration)
	}

	return WithAmbientCredentials()
}

// withIntegrationCredentials configures options with an Integration that must be used to fetch Credentials to assume a role.
// This prevents the usage of AWS environment credentials.
func withIntegrationCredentials(integration string) AWSOptionsFn {
	return func(options *awsOptions) {
		options.credentialsSource = credentialsSourceIntegration
		options.integration = integration
	}
}

// WithAmbientCredentials configures options to use the ambient credentials.
func WithAmbientCredentials() AWSOptionsFn {
	return func(options *awsOptions) {
		options.credentialsSource = credentialsSourceAmbient
	}
}

// GetAWSSession returns AWS session for the specified region, optionally
// assuming AWS IAM Roles.
func (c *cloudClients) GetAWSSession(ctx context.Context, region string, opts ...AWSOptionsFn) (*awssession.Session, error) {
	var options awsOptions
	for _, opt := range opts {
		opt(&options)
	}
	var err error
	if options.baseSession == nil {
		options.baseSession, err = c.getAWSSessionForRegion(ctx, region, options)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if options.assumeRoleARN == "" {
		return options.baseSession, nil
	}
	return c.getAWSSessionForRole(ctx, region, options)
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *cloudClients) GetAWSRDSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (rdsiface.RDSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rds.New(session), nil
}

// GetAWSRedshiftClient returns AWS Redshift client for the specified region.
func (c *cloudClients) GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftiface.RedshiftAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return redshift.New(session), nil
}

// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
func (c *cloudClients) GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftserverlessiface.RedshiftServerlessAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return redshiftserverless.New(session), nil
}

// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
func (c *cloudClients) GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSOptionsFn) (elasticacheiface.ElastiCacheAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return elasticache.New(session), nil
}

// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
func (c *cloudClients) GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSOptionsFn) (opensearchserviceiface.OpenSearchServiceAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return opensearchservice.New(session), nil
}

// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
func (c *cloudClients) GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSOptionsFn) (memorydbiface.MemoryDBAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return memorydb.New(session), nil
}

// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
func (c *cloudClients) GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSOptionsFn) (secretsmanageriface.SecretsManagerAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return secretsmanager.New(session), nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *cloudClients) GetAWSIAMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (iamiface.IAMAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return iam.New(session), nil
}

// GetAWSS3Client returns AWS S3 client.
func (c *cloudClients) GetAWSS3Client(ctx context.Context, region string, opts ...AWSOptionsFn) (s3iface.S3API, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s3.New(session), nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *cloudClients) GetAWSSTSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (stsiface.STSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return stsutils.NewV1(session), nil
}

// GetAWSEC2Client returns AWS EC2 client for the specified region.
func (c *cloudClients) GetAWSEC2Client(ctx context.Context, region string, opts ...AWSOptionsFn) (ec2iface.EC2API, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ec2.New(session), nil
}

// GetAWSSSMClient returns AWS SSM client for the specified region.
func (c *cloudClients) GetAWSSSMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (ssmiface.SSMAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssm.New(session), nil
}

// GetAWSEKSClient returns AWS EKS client for the specified region.
func (c *cloudClients) GetAWSEKSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (eksiface.EKSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eks.New(session), nil
}

// GetAWSKMSClient returns AWS KMS client for the specified region.
func (c *cloudClients) GetAWSKMSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (kmsiface.KMSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return kms.New(session), nil
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

// awsAmbientSessionProvider loads a new session using the environment variables.
// Describe in detail here: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
func awsAmbientSessionProvider(ctx context.Context, region string) (*awssession.Session, error) {
	awsSessionOptions := buildAWSSessionOptions(region, nil /* credentials */)

	session, err := awssession.NewSessionWithOptions(awsSessionOptions)
	return session, trace.Wrap(err)
}

// getAWSSessionForRegion returns AWS session for the specified region.
func (c *cloudClients) getAWSSessionForRegion(ctx context.Context, region string, opts awsOptions) (*awssession.Session, error) {
	if err := opts.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	createSession := func(ctx context.Context) (*awssession.Session, error) {
		if opts.credentialsSource == credentialsSourceIntegration {
			if c.awsIntegrationSessionProviderFn == nil {
				return nil, trace.BadParameter("missing aws integration session provider")
			}

			logrus.Debugf("Initializing AWS session for region %v with integration %q.", region, opts.integration)
			session, err := c.awsIntegrationSessionProviderFn(ctx, region, opts.integration)
			return session, trace.Wrap(err)
		}

		logrus.Debugf("Initializing AWS session for region %v using environment credentials.", region)
		session, err := awsAmbientSessionProvider(ctx, region)
		return session, trace.Wrap(err)
	}

	if opts.withoutSessionCache {
		sess, err := createSession(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if opts.customRetryer != nil || opts.maxRetries != nil {
			return sess.Copy(&aws.Config{
				Retryer:    opts.customRetryer,
				MaxRetries: opts.maxRetries,
			}), nil
		}
		return sess, trace.Wrap(err)
	}

	cacheKey := awsSessionCacheKey{
		region:      region,
		integration: opts.integration,
	}

	sess, err := utils.FnCacheGet(ctx, c.awsSessionsCache, cacheKey, func(ctx context.Context) (*awssession.Session, error) {
		session, err := createSession(ctx)
		return session, trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if opts.customRetryer != nil || opts.maxRetries != nil {
		return sess.Copy(&aws.Config{
			Retryer:    opts.customRetryer,
			MaxRetries: opts.maxRetries,
		}), nil
	}
	return sess, err
}

// getAWSSessionForRole returns AWS session for the specified region and role.
func (c *cloudClients) getAWSSessionForRole(ctx context.Context, region string, options awsOptions) (*awssession.Session, error) {
	if err := options.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	createSession := func(ctx context.Context) (*awssession.Session, error) {
		stsClient := stsutils.NewV1(options.baseSession)
		return newSessionWithRole(ctx, stsClient, region, options.assumeRoleARN, options.assumeRoleExternalID)
	}

	if options.withoutSessionCache {
		session, err := createSession(ctx)
		return session, trace.Wrap(err)
	}

	cacheKey := awsSessionCacheKey{
		region:      region,
		integration: options.integration,
		roleARN:     options.assumeRoleARN,
		externalID:  options.assumeRoleExternalID,
	}
	return utils.FnCacheGet(ctx, c.awsSessionsCache, cacheKey, func(ctx context.Context) (*awssession.Session, error) {
		session, err := createSession(ctx)
		return session, trace.Wrap(err)
	})
}

func (c *cloudClients) initGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil { // If some other thread already got here first.
		return c.gcpIAM, nil
	}
	logrus.Debug("Initializing GCP IAM client.")
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
	logrus.Debug("Initializing Azure default credential chain.")
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

	logrus.Debug("Initializing Azure MySQL servers client.")
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
	logrus.Debug("Initializing Azure Postgres servers client.")
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
	logrus.Debug("Initializing Azure subscriptions client.")
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
	logrus.Debug("Initializing instance metadata client.")

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
	logrus.Debug("Initializing Azure AKS client.")
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
	RDS                     rdsiface.RDSAPI
	RDSPerRegion            map[string]rdsiface.RDSAPI
	Redshift                redshiftiface.RedshiftAPI
	RedshiftServerless      redshiftserverlessiface.RedshiftServerlessAPI
	ElastiCache             elasticacheiface.ElastiCacheAPI
	OpenSearch              opensearchserviceiface.OpenSearchServiceAPI
	MemoryDB                memorydbiface.MemoryDBAPI
	SecretsManager          secretsmanageriface.SecretsManagerAPI
	IAM                     iamiface.IAMAPI
	STS                     stsiface.STSAPI
	GCPSQL                  gcp.SQLAdminClient
	GCPGKE                  gcp.GKEClient
	GCPProjects             gcp.ProjectsClient
	GCPInstances            gcp.InstancesClient
	EC2                     ec2iface.EC2API
	SSM                     ssmiface.SSMAPI
	InstanceMetadata        imds.Client
	EKS                     eksiface.EKSAPI
	KMS                     kmsiface.KMSAPI
	S3                      s3iface.S3API
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

// GetAWSSession returns AWS session for the specified region, optionally
// assuming AWS IAM Roles.
func (c *TestCloudClients) GetAWSSession(ctx context.Context, region string, opts ...AWSOptionsFn) (*awssession.Session, error) {
	var options awsOptions
	for _, opt := range opts {
		opt(&options)
	}
	var err error
	if options.baseSession == nil {
		options.baseSession, err = c.getAWSSessionForRegion(region)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if options.assumeRoleARN == "" {
		return options.baseSession, nil
	}
	return newSessionWithRole(ctx, c.STS, region, options.assumeRoleARN, options.assumeRoleExternalID)
}

// GetAWSSession returns AWS session for the specified region.
func (c *TestCloudClients) getAWSSessionForRegion(region string) (*awssession.Session, error) {
	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if modules.GetModules().IsBoringBinary() {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	return awssession.NewSession(&aws.Config{
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     "fakeClientKeyID",
			SecretAccessKey: "fakeClientSecret",
		}}),
		Region:          aws.String(region),
		UseFIPSEndpoint: useFIPSEndpoint,
	})
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *TestCloudClients) GetAWSRDSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (rdsiface.RDSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(c.RDSPerRegion) != 0 {
		return c.RDSPerRegion[region], nil
	}
	return c.RDS, nil
}

// GetAWSRedshiftClient returns AWS Redshift client for the specified region.
func (c *TestCloudClients) GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftiface.RedshiftAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.Redshift, nil
}

// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
func (c *TestCloudClients) GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSOptionsFn) (redshiftserverlessiface.RedshiftServerlessAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.RedshiftServerless, nil
}

// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
func (c *TestCloudClients) GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSOptionsFn) (elasticacheiface.ElastiCacheAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.ElastiCache, nil
}

// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
func (c *TestCloudClients) GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSOptionsFn) (opensearchserviceiface.OpenSearchServiceAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.OpenSearch, nil
}

// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
func (c *TestCloudClients) GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSOptionsFn) (memorydbiface.MemoryDBAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.MemoryDB, nil
}

// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
func (c *TestCloudClients) GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSOptionsFn) (secretsmanageriface.SecretsManagerAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.SecretsManager, nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *TestCloudClients) GetAWSIAMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (iamiface.IAMAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.IAM, nil
}

// GetAWSS3Client returns AWS S3 client.
func (c *TestCloudClients) GetAWSS3Client(ctx context.Context, region string, opts ...AWSOptionsFn) (s3iface.S3API, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.S3, nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *TestCloudClients) GetAWSSTSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (stsiface.STSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.STS, nil
}

// GetAWSEKSClient returns AWS EKS client for the specified region.
func (c *TestCloudClients) GetAWSEKSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (eksiface.EKSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.EKS, nil
}

// GetAWSKMSClient returns AWS KMS client for the specified region.
func (c *TestCloudClients) GetAWSKMSClient(ctx context.Context, region string, opts ...AWSOptionsFn) (kmsiface.KMSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.KMS, nil
}

// GetAWSEC2Client returns AWS EC2 client for the specified region.
func (c *TestCloudClients) GetAWSEC2Client(ctx context.Context, region string, opts ...AWSOptionsFn) (ec2iface.EC2API, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.EC2, nil
}

// GetAWSSSMClient returns an AWS SSM client
func (c *TestCloudClients) GetAWSSSMClient(ctx context.Context, region string, opts ...AWSOptionsFn) (ssmiface.SSMAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.SSM, nil
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

// newSessionWithRole assumes a given AWS IAM Role, passing an external ID if given,
// and returns a new AWS session with the assumed role in the given region.
func newSessionWithRole(ctx context.Context, svc stscreds.AssumeRoler, region, roleARN, externalID string) (*awssession.Session, error) {
	logrus.Debugf("Initializing AWS session for assumed role %q for region %v.", roleARN, region)
	// Make a credentials with AssumeRoleProvider and test it out.
	cred := stscreds.NewCredentialsWithClient(svc, roleARN, func(p *stscreds.AssumeRoleProvider) {
		if externalID != "" {
			p.ExternalID = aws.String(externalID)
		}
	})
	if _, err := cred.GetWithContext(ctx); err != nil {
		return nil, trace.Wrap(libcloudaws.ConvertRequestFailureError(err))
	}

	awsSessionOptions := buildAWSSessionOptions(region, cred)

	// Create a new session with the credentials.
	roleSession, err := awssession.NewSessionWithOptions(awsSessionOptions)
	return roleSession, trace.Wrap(err)
}

func buildAWSSessionOptions(region string, cred *credentials.Credentials) awssession.Options {
	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if modules.GetModules().IsBoringBinary() {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	return awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
		Config: aws.Config{
			Region:          aws.String(region),
			Credentials:     cred,
			UseFIPSEndpoint: useFIPSEndpoint,
		},
	}
}
