/*
Copyright 2021 Gravitational, Inc.

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

package cloud

import (
	"context"
	"fmt"
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
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elasticache/elasticacheiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
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
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
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
	"github.com/gravitational/teleport/lib/utils"
)

// Clients provides interface for obtaining cloud provider clients.
type Clients interface {
	// GetInstanceMetadataClient returns instance metadata client based on which
	// cloud provider Teleport is running on, if any.
	GetInstanceMetadataClient(ctx context.Context) (InstanceMetadata, error)
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
}

// AWSClients is an interface for providing AWS API clients.
type AWSClients interface {
	// GetAWSSession returns AWS session for the specified region and any role(s).
	GetAWSSession(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (*awssession.Session, error)
	// GetAWSRDSClient returns AWS RDS client for the specified region.
	GetAWSRDSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (rdsiface.RDSAPI, error)
	// GetAWSRedshiftClient returns AWS Redshift client for the specified region.
	GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftiface.RedshiftAPI, error)
	// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
	GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftserverlessiface.RedshiftServerlessAPI, error)
	// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
	GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (elasticacheiface.ElastiCacheAPI, error)
	// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
	GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (memorydbiface.MemoryDBAPI, error)
	// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
	GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (opensearchserviceiface.OpenSearchServiceAPI, error)
	// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
	GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (secretsmanageriface.SecretsManagerAPI, error)
	// GetAWSIAMClient returns AWS IAM client for the specified region.
	GetAWSIAMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (iamiface.IAMAPI, error)
	// GetAWSSTSClient returns AWS STS client for the specified region.
	GetAWSSTSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (stsiface.STSAPI, error)
	// GetAWSEC2Client returns AWS EC2 client for the specified region.
	GetAWSEC2Client(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ec2iface.EC2API, error)
	// GetAWSSSMClient returns AWS SSM client for the specified region.
	GetAWSSSMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ssmiface.SSMAPI, error)
	// GetAWSEKSClient returns AWS EKS client for the specified region.
	GetAWSEKSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (eksiface.EKSAPI, error)
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

// NewClients returns a new instance of cloud clients retriever.
func NewClients() (Clients, error) {
	awsSessionsCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: 15 * time.Minute,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cloudClients{
		awsSessionsCache: awsSessionsCache,
		azureClients: azureClients{
			azureMySQLClients:               make(map[string]azure.DBServersClient),
			azurePostgresClients:            make(map[string]azure.DBServersClient),
			azureRedisClients:               azure.NewClientMap(azure.NewRedisClient),
			azureRedisEnterpriseClients:     azure.NewClientMap(azure.NewRedisEnterpriseClient),
			azureKubernetesClient:           make(map[string]azure.AKSClient),
			azureVirtualMachinesClients:     azure.NewClientMap(azure.NewVirtualMachinesClient),
			azureSQLServerClients:           azure.NewClientMap(azure.NewSQLClient),
			azureManagedSQLServerClients:    azure.NewClientMap(azure.NewManagedSQLClient),
			azureMySQLFlexServersClients:    azure.NewClientMap(azure.NewMySQLFlexServersClient),
			azurePostgresFlexServersClients: azure.NewClientMap(azure.NewPostgresFlexServersClient),
			azureRunCommandClients:          azure.NewClientMap(azure.NewRunCommandClient),
		},
	}, nil
}

// cloudClients implements Clients
var _ Clients = (*cloudClients)(nil)

type cloudClients struct {
	// awsSessionsCache is a cache of AWS sessions, where the cache key is
	// either "<region>" or "Region[<region>]:RoleARN[<arn>]:ExternalID[<id>]".
	awsSessionsCache *utils.FnCache
	// gcpIAM is the cached GCP IAM client.
	gcpIAM *gcpcredentials.IamCredentialsClient
	// gcpSQLAdmin is the cached GCP Cloud SQL Admin client.
	gcpSQLAdmin gcp.SQLAdminClient
	// instanceMetadata is the cached instance metadata client.
	instanceMetadata InstanceMetadata
	// gcpGKE is the cached GCP Cloud GKE client.
	gcpGKE gcp.GKEClient
	// azureClients contains Azure-specific clients.
	azureClients
	// mtx is used for locking.
	mtx sync.RWMutex
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
}

// awsAssumeRoleOpts a struct of additional options for assuming an AWS role
// when construction an underlying AWS session.
type awsAssumeRoleOpts struct {
	// baseSession is a session to use instead of the default session for an
	// AWS region, which is used to enable role chaining.
	baseSession *awssession.Session
	// assumeRoleARN is the AWS IAM Role ARN to assume.
	assumeRoleARN string
	// assumeRoleExternalID is used to assume an external AWS IAM Role.
	assumeRoleExternalID string
}

// AWSAssumeRoleOptionFn is an option function for setting additional options
// when getting an AWS session.
type AWSAssumeRoleOptionFn func(*awsAssumeRoleOpts)

// WithAssumeRole configures options needed for assuming an AWS role.
func WithAssumeRole(roleARN, externalID string) AWSAssumeRoleOptionFn {
	return func(options *awsAssumeRoleOpts) {
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// WithAssumeRoleFromAWSMeta extracts options needed from AWS metadata for
// assuming an AWS role.
func WithAssumeRoleFromAWSMeta(meta types.AWS) AWSAssumeRoleOptionFn {
	return WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID)
}

// WithChainedAssumeRole sets a role to assume with a base session to use
// for assuming the role, which enables role chaining.
func WithChainedAssumeRole(session *awssession.Session, roleARN, externalID string) AWSAssumeRoleOptionFn {
	return func(options *awsAssumeRoleOpts) {
		options.baseSession = session
		options.assumeRoleARN = roleARN
		options.assumeRoleExternalID = externalID
	}
}

// GetAWSSession returns AWS session for the specified region, optionally
// assuming AWS IAM Roles.
func (c *cloudClients) GetAWSSession(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (*awssession.Session, error) {
	var options awsAssumeRoleOpts
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
	return c.getAWSSessionForRole(ctx, region, options)
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *cloudClients) GetAWSRDSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (rdsiface.RDSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rds.New(session), nil
}

// GetAWSRedshiftClient returns AWS Redshift client for the specified region.
func (c *cloudClients) GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftiface.RedshiftAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return redshift.New(session), nil
}

// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
func (c *cloudClients) GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftserverlessiface.RedshiftServerlessAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return redshiftserverless.New(session), nil
}

// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
func (c *cloudClients) GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (elasticacheiface.ElastiCacheAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return elasticache.New(session), nil
}

// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
func (c *cloudClients) GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (opensearchserviceiface.OpenSearchServiceAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return opensearchservice.New(session), nil
}

// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
func (c *cloudClients) GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (memorydbiface.MemoryDBAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return memorydb.New(session), nil
}

// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
func (c *cloudClients) GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (secretsmanageriface.SecretsManagerAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return secretsmanager.New(session), nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *cloudClients) GetAWSIAMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (iamiface.IAMAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return iam.New(session), nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *cloudClients) GetAWSSTSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (stsiface.STSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sts.New(session), nil
}

// GetAWSEC2Client returns AWS EC2 client for the specified region.
func (c *cloudClients) GetAWSEC2Client(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ec2iface.EC2API, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ec2.New(session), nil
}

// GetAWSSSMClient returns AWS SSM client for the specified region.
func (c *cloudClients) GetAWSSSMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ssmiface.SSMAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssm.New(session), nil
}

// GetAWSEKSClient returns AWS EKS client for the specified region.
func (c *cloudClients) GetAWSEKSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (eksiface.EKSAPI, error) {
	session, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eks.New(session), nil
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
	c.mtx.RLock()
	if c.gcpSQLAdmin != nil {
		defer c.mtx.RUnlock()
		return c.gcpSQLAdmin, nil
	}
	c.mtx.RUnlock()
	return c.initGCPSQLAdminClient(ctx)
}

// GetInstanceMetadata returns the instance metadata.
func (c *cloudClients) GetInstanceMetadataClient(ctx context.Context) (InstanceMetadata, error) {
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
	c.mtx.RLock()
	if c.gcpGKE != nil {
		defer c.mtx.RUnlock()
		return c.gcpGKE, nil
	}
	c.mtx.RUnlock()
	return c.initGCPGKEClient(ctx)
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

// getAWSSessionForRegion returns AWS session for the specified region.
func (c *cloudClients) getAWSSessionForRegion(region string) (*awssession.Session, error) {
	return utils.FnCacheGet(context.Background(), c.awsSessionsCache, region, func(ctx context.Context) (*awssession.Session, error) {
		logrus.Debugf("Initializing AWS session for region %v.", region)
		session, err := awssession.NewSessionWithOptions(awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
			Config: aws.Config{
				Region: aws.String(region),
			},
		})
		return session, trace.Wrap(err)
	})
}

// getAWSSessionForRole returns AWS session for the specified region and role.
func (c *cloudClients) getAWSSessionForRole(ctx context.Context, region string, options awsAssumeRoleOpts) (*awssession.Session, error) {
	cacheKey := fmt.Sprintf("Region[%s]:RoleARN[%s]:ExternalID[%s]", region, options.assumeRoleARN, options.assumeRoleExternalID)
	return utils.FnCacheGet(ctx, c.awsSessionsCache, cacheKey, func(ctx context.Context) (*awssession.Session, error) {
		assumeRoler := sts.New(options.baseSession)
		return newSessionWithRole(ctx, assumeRoler, region, options.assumeRoleARN, options.assumeRoleExternalID)
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

func (c *cloudClients) initGCPSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpSQLAdmin != nil { // If some other thread already got here first.
		return c.gcpSQLAdmin, nil
	}
	logrus.Debug("Initializing GCP Cloud SQL Admin client.")
	gcpSQLAdmin, err := gcp.NewSQLAdminClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpSQLAdmin = gcpSQLAdmin
	return gcpSQLAdmin, nil
}

func (c *cloudClients) initGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpGKE != nil { // If some other thread already got here first.
		return c.gcpGKE, nil
	}
	logrus.Debug("Initializing GCP Cloud GKE client.")
	gcpGKE, err := gcp.NewGKEClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpGKE = gcpGKE
	return gcpGKE, nil
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
func (c *cloudClients) initInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.instanceMetadata != nil { // If some other thread already got here first.
		return c.instanceMetadata, nil
	}
	logrus.Debug("Initializing instance metadata client.")
	client, err := DiscoverInstanceMetadata(ctx)
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
	EC2                     ec2iface.EC2API
	SSM                     ssmiface.SSMAPI
	InstanceMetadata        InstanceMetadata
	EKS                     eksiface.EKSAPI
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

// GetAWSSession returns AWS session for the specified region, optionally
// assuming AWS IAM Roles.
func (c *TestCloudClients) GetAWSSession(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (*awssession.Session, error) {
	var options awsAssumeRoleOpts
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
	return awssession.NewSession(&aws.Config{
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     "fakeClientKeyID",
			SecretAccessKey: "fakeClientSecret",
		}}),
		Region: aws.String(region),
	})
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *TestCloudClients) GetAWSRDSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (rdsiface.RDSAPI, error) {
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
func (c *TestCloudClients) GetAWSRedshiftClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftiface.RedshiftAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.Redshift, nil
}

// GetAWSRedshiftServerlessClient returns AWS Redshift Serverless client for the specified region.
func (c *TestCloudClients) GetAWSRedshiftServerlessClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (redshiftserverlessiface.RedshiftServerlessAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.RedshiftServerless, nil
}

// GetAWSElastiCacheClient returns AWS ElastiCache client for the specified region.
func (c *TestCloudClients) GetAWSElastiCacheClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (elasticacheiface.ElastiCacheAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.ElastiCache, nil
}

// GetAWSOpenSearchClient returns AWS OpenSearch client for the specified region.
func (c *TestCloudClients) GetAWSOpenSearchClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (opensearchserviceiface.OpenSearchServiceAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.OpenSearch, nil
}

// GetAWSMemoryDBClient returns AWS MemoryDB client for the specified region.
func (c *TestCloudClients) GetAWSMemoryDBClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (memorydbiface.MemoryDBAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.MemoryDB, nil
}

// GetAWSSecretsManagerClient returns AWS Secrets Manager client for the specified region.
func (c *TestCloudClients) GetAWSSecretsManagerClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (secretsmanageriface.SecretsManagerAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.SecretsManager, nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *TestCloudClients) GetAWSIAMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (iamiface.IAMAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.IAM, nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *TestCloudClients) GetAWSSTSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (stsiface.STSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.STS, nil
}

// GetAWSEKSClient returns AWS EKS client for the specified region.
func (c *TestCloudClients) GetAWSEKSClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (eksiface.EKSAPI, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.EKS, nil
}

// GetAWSEC2Client returns AWS EC2 client for the specified region.
func (c *TestCloudClients) GetAWSEC2Client(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ec2iface.EC2API, error) {
	_, err := c.GetAWSSession(ctx, region, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.EC2, nil
}

// GetAWSSSMClient returns an AWS SSM client
func (c *TestCloudClients) GetAWSSSMClient(ctx context.Context, region string, opts ...AWSAssumeRoleOptionFn) (ssmiface.SSMAPI, error) {
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

// GetInstanceMetadata returns the instance metadata.
func (c *TestCloudClients) GetInstanceMetadataClient(ctx context.Context) (InstanceMetadata, error) {
	return c.InstanceMetadata, nil
}

// GetGCPGKEClient returns GKE client.
func (c *TestCloudClients) GetGCPGKEClient(ctx context.Context) (gcp.GKEClient, error) {
	return c.GCPGKE, nil
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

// GetAzureRunCommand returns an Azure Run Command client for the given subscription.
func (c *TestCloudClients) GetAzureRunCommandClient(subscription string) (azure.RunCommandClient, error) {
	return c.AzureRunCommand, nil
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

	// Create a new session with the credentials.
	roleSession, err := awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
		Config: aws.Config{
			Region:      aws.String(region),
			Credentials: cred,
		},
	})
	return roleSession, trace.Wrap(err)
}
