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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshiftserverless/redshiftserverlessiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
	cloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func (c *urlChecker) checkAWS(describeCheck, basicEndpointCheck checkDatabaseFunc) checkDatabaseFunc {
	return func(ctx context.Context, database types.Database) error {
		err := describeCheck(ctx, database)

		// Database Service may not have enough permissions to permform the
		// describes. Log a warning and permform a basic endpoint validation
		// instead.
		if trace.IsAccessDenied(err) {
			c.logAWSAccessDeniedError(database, err)

			if err := basicEndpointCheck(ctx, database); err != nil {
				return trace.Wrap(err)
			}
			c.log.Debugf("AWS database %q URL validated by basic endpoint check.", database.GetName())
			return nil
		}

		if err != nil {
			return trace.Wrap(err)
		}
		c.log.Debugf("AWS database %q URL validated by describe check.", database.GetName())
		return nil
	}
}

func (c *urlChecker) logAWSAccessDeniedError(database types.Database, accessDeniedError error) {
	c.warnAWSOnce.Do(func() {
		// TODO(greedy52) add links to doc.
		c.log.Warn("No permissions to describe AWS resource metadata that is needed for validating databases created by Discovery Service. Basic AWS endpoint validation will be performed instead. For best security, please provide the Database Service with the proper IAM permissions. Enable --debug mode to see details on which databases require more IAM permissions. See Database Access documentation for more details.")
	})

	c.log.Debugf("No permissions to describe database %q for URL validation.", database.GetName())
}

func (c *urlChecker) checkRDS(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	rdsClient, err := c.clients.GetAWSRDSClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if meta.RDS.ClusterID != "" {
		return trace.Wrap(c.checkRDSCluster(ctx, database, rdsClient, meta.RDS.ClusterID))
	}
	return trace.Wrap(c.checkRDSInstance(ctx, database, rdsClient, meta.RDS.InstanceID))
}

func (c *urlChecker) checkRDSInstance(ctx context.Context, database types.Database, rdsClient rdsiface.RDSAPI, instanceID string) error {
	rdsInstance, err := describeRDSInstance(ctx, rdsClient, instanceID)
	if err != nil {
		return trace.Wrap(err)
	}
	if rdsInstance.Endpoint == nil {
		return trace.BadParameter("empty endpoint")
	}
	return trace.Wrap(requireDatabaseAddressPort(database, rdsInstance.Endpoint.Address, rdsInstance.Endpoint.Port))
}

func (c *urlChecker) checkRDSCluster(ctx context.Context, database types.Database, rdsClient rdsiface.RDSAPI, clusterID string) error {
	rdsCluster, err := describeRDSCluster(ctx, rdsClient, clusterID)
	if err != nil {
		return trace.Wrap(err)
	}
	databases, err := common.NewDatabasesFromRDSCluster(rdsCluster, []*rds.DBInstance{})
	if err != nil {
		c.log.Warnf("Could not convert RDS cluster %q to database resources: %v.",
			aws.StringValue(rdsCluster.DBClusterIdentifier), err)

		// services.NewDatabasesFromRDSCluster maybe partially successful.
		if len(databases) == 0 {
			return nil
		}
	}
	return trace.Wrap(requireContainsDatabaseURLAndEndpointType(databases, database, rdsCluster))
}

func (c *urlChecker) checkRDSProxy(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	rdsClient, err := c.clients.GetAWSRDSClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if meta.RDSProxy.CustomEndpointName != "" {
		return trace.Wrap(c.checkRDSProxyCustomEndpoint(ctx, database, rdsClient, meta.RDSProxy.CustomEndpointName))
	}
	return trace.Wrap(c.checkRDSProxyPrimaryEndpoint(ctx, database, rdsClient, meta.RDSProxy.Name))
}

func (c *urlChecker) checkRDSProxyPrimaryEndpoint(ctx context.Context, database types.Database, rdsClient rdsiface.RDSAPI, proxyName string) error {
	rdsProxy, err := describeRDSProxy(ctx, rdsClient, proxyName)
	if err != nil {
		return trace.Wrap(err)
	}
	// Port has to be fetched from a separate API. Instead of fetching that,
	// just validate the host domain.
	return requireDatabaseHost(database, aws.StringValue(rdsProxy.Endpoint))
}

func (c *urlChecker) checkRDSProxyCustomEndpoint(ctx context.Context, database types.Database, rdsClient rdsiface.RDSAPI, proxyEndpointName string) error {
	_, err := describeRDSProxyCustomEndpointAndFindURI(ctx, rdsClient, proxyEndpointName, database.GetURI())
	return trace.Wrap(err)
}

func (c *urlChecker) checkRedshift(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	redshift, err := c.clients.GetAWSRedshiftClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := describeRedshiftCluster(ctx, redshift, meta.Redshift.ClusterID)
	if err != nil {
		return trace.Wrap(err)
	}
	if cluster.Endpoint == nil {
		return trace.BadParameter("missing endpoint in Redshift cluster %v", aws.StringValue(cluster.ClusterIdentifier))
	}
	return trace.Wrap(requireDatabaseAddressPort(database, cluster.Endpoint.Address, cluster.Endpoint.Port))
}

func (c *urlChecker) checkRedshiftServerless(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	client, err := c.clients.GetAWSRedshiftServerlessClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if meta.RedshiftServerless.EndpointName != "" {
		return trace.Wrap(c.checkRedshiftServerlessVPCEndpoint(ctx, database, client, meta.RedshiftServerless.EndpointName))
	}
	return trace.Wrap(c.checkRedshiftServerlessWorkgroup(ctx, database, client, meta.RedshiftServerless.WorkgroupName))
}

func (c *urlChecker) checkRedshiftServerlessVPCEndpoint(ctx context.Context, database types.Database, client redshiftserverlessiface.RedshiftServerlessAPI, endpointName string) error {
	endpoint, err := describeRedshiftServerlessVCPEndpoint(ctx, client, endpointName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(requireDatabaseAddressPort(database, endpoint.Address, endpoint.Port))
}

func (c *urlChecker) checkRedshiftServerlessWorkgroup(ctx context.Context, database types.Database, client redshiftserverlessiface.RedshiftServerlessAPI, workgroupName string) error {
	workgroup, err := describeRedshiftServerlessWorkgroup(ctx, client, workgroupName)
	if err != nil {
		return trace.Wrap(err)
	}
	if workgroup.Endpoint == nil {
		return trace.BadParameter("missing endpoint")
	}
	return trace.Wrap(requireDatabaseAddressPort(database, workgroup.Endpoint.Address, workgroup.Endpoint.Port))
}

func (c *urlChecker) checkElastiCache(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	elastiCacheClient, err := c.clients.GetAWSElastiCacheClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := describeElastiCacheCluster(ctx, elastiCacheClient, meta.ElastiCache.ReplicationGroupID)
	if err != nil {
		return trace.Wrap(err)
	}
	databases, err := common.NewDatabasesFromElastiCacheReplicationGroup(cluster, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(requireContainsDatabaseURLAndEndpointType(databases, database, cluster))
}

func (c *urlChecker) checkMemoryDB(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	memoryDBClient, err := c.clients.GetAWSMemoryDBClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := describeMemoryDBCluster(ctx, memoryDBClient, meta.MemoryDB.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(requireDatabaseAddressPort(database, cluster.ClusterEndpoint.Address, cluster.ClusterEndpoint.Port))
}

func (c *urlChecker) checkOpenSearch(ctx context.Context, database types.Database) error {
	meta := database.GetAWS()
	client, err := c.clients.GetAWSOpenSearchClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	domains, err := client.DescribeDomainsWithContext(ctx, &opensearchservice.DescribeDomainsInput{
		DomainNames: []*string{aws.String(meta.OpenSearch.DomainName)},
	})
	if err != nil {
		return trace.Wrap(cloudaws.ConvertRequestFailureError(err))
	}
	if len(domains.DomainStatusList) != 1 {
		return trace.BadParameter("expect 1 domain but got %v", domains.DomainStatusList)
	}

	databases, err := common.NewDatabasesFromOpenSearchDomain(domains.DomainStatusList[0], nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(requireContainsDatabaseURLAndEndpointType(databases, database, domains.DomainStatusList[0]))
}

func (c *urlChecker) checkOpenSearchEndpoint(ctx context.Context, database types.Database) error {
	switch database.GetAWS().OpenSearch.EndpointType {
	case apiawsutils.OpenSearchDefaultEndpoint, apiawsutils.OpenSearchVPCEndpoint:
		return trace.Wrap(convIsEndpoint(apiawsutils.IsOpenSearchEndpoint)(ctx, database))
	default:
		// Custom endpoint can be anything. For best security, don't allow it.
		// Primary endpoint should also be discovered and users can still use
		// that.
		return trace.BadParameter(`cannot validate OpenSearch custom domain %v. Please provide Database Service "es:DescribeDomains" permission to validate the URL.`, database.GetURI())
	}
}
