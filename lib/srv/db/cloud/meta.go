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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// MetadataConfig is the cloud metadata service config.
type MetadataConfig struct {
	// Clients is an interface for retrieving cloud clients.
	Clients common.CloudClients
}

// Check validates the metadata service config.
func (c *MetadataConfig) Check() error {
	if c.Clients == nil {
		c.Clients = common.NewCloudClients()
	}
	return nil
}

// Metadata is a service that fetches cloud databases metadata.
type Metadata struct {
	cfg MetadataConfig
	log logrus.FieldLogger
}

// NewMetadata returns a new cloud metadata service.
func NewMetadata(config MetadataConfig) (*Metadata, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Metadata{
		cfg: config,
		log: logrus.WithField(trace.Component, "meta"),
	}, nil
}

// Update updates cloud metadata of the provided database.
func (m *Metadata) Update(ctx context.Context, database types.Database) error {
	if database.IsRDS() {
		metadata, err := m.fetchRDSMetadata(ctx, database)
		if err != nil {
			if trace.IsAccessDenied(err) { // Permission errors are expected.
				m.log.Debugf("No permissions to fetch RDS metadata for %q: %v.", database.GetName(), err)
				return nil
			}
			return trace.Wrap(err)
		}
		m.log.Debugf("Fetched RDS metadata for %q: %v.", database.GetName(), metadata)
		database.SetStatusAWS(*metadata)
	} else if database.IsRedshift() {
		metadata, err := m.fetchRedshiftMetadata(ctx, database)
		if err != nil {
			if trace.IsAccessDenied(err) { // Permission errros are expected.
				m.log.Debugf("No permissions to fetch Redshift metadata for %q: %v.", database.GetName(), err)
				return nil
			}
			return trace.Wrap(err)
		}
		m.log.Debugf("Fetched Redshift metadata for %q: %v.", database.GetName(), metadata)
		database.SetStatusAWS(*metadata)
	}
	return nil
}

// fetchRDSMetadata fetches metadata for the provided RDS or Aurora database.
func (m *Metadata) fetchRDSMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	rds, err := m.cfg.Clients.GetAWSRDSClient(database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsMetadata := database.GetAWS()

	if awsMetadata.RDS.ProxyEndpointName != "" {
		return fetchRDSProxyEndpointMetadata(ctx, rds, awsMetadata.RDS.ProxyEndpointName)
	}

	if awsMetadata.RDS.ProxyName != "" {
		return fetchRDSProxyMetadata(ctx, rds, awsMetadata.RDS.ProxyName)
	}

	if awsMetadata.RDS.ClusterID != "" {
		return fetchRDSClusterMetadata(ctx, rds, awsMetadata.RDS.ClusterID)
	}

	metadata, err := fetchRDSInstanceMetadata(ctx, rds, awsMetadata.RDS.InstanceID)

	// If instance was found, it may be a part of an Aurora cluster.
	if metadata != nil && metadata.RDS.ClusterID != "" {
		return fetchRDSClusterMetadata(ctx, rds, metadata.RDS.ClusterID)
	}
	return metadata, err
}

// fetchRedshiftMetadata fetches metadata for the provided Redshift database.
func (m *Metadata) fetchRedshiftMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	redshift, err := m.cfg.Clients.GetAWSRedshiftClient(database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := describeRedshiftCluster(ctx, redshift, database.GetAWS().Redshift.ClusterID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedARN, err := arn.Parse(aws.StringValue(cluster.ClusterNamespaceArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		Redshift: types.Redshift{
			ClusterID: aws.StringValue(cluster.ClusterIdentifier),
		},
	}, nil
}

// fetchRDSInstanceMetadata fetches metadata about specified RDS instance.
func fetchRDSInstanceMetadata(ctx context.Context, rdsClient rdsiface.RDSAPI, instanceID string) (*types.AWS, error) {
	rdsInstance, err := describeRDSInstance(ctx, rdsClient, instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.MetadataFromRDSInstance(rdsInstance)
}

// describeRDSInstance returns AWS RDS instance for the specified ID.
func describeRDSInstance(ctx context.Context, rdsClient rdsiface.RDSAPI, instanceID string) (*rds.DBInstance, error) {
	out, err := rdsClient.DescribeDBInstancesWithContext(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBInstances) != 1 {
		return nil, trace.BadParameter("expected 1 RDS instance for %v, got %s", instanceID, out.DBInstances)
	}
	return out.DBInstances[0], nil
}

// fetchRDSClusterMetadata fetches metadata about specified Aurora cluster.
func fetchRDSClusterMetadata(ctx context.Context, rdsClient rdsiface.RDSAPI, clusterID string) (*types.AWS, error) {
	rdsCluster, err := describeRDSCluster(ctx, rdsClient, clusterID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.MetadataFromRDSCluster(rdsCluster)
}

// describeRDSCluster returns AWS Aurora cluster for the specified ID.
func describeRDSCluster(ctx context.Context, rdsClient rdsiface.RDSAPI, clusterID string) (*rds.DBCluster, error) {
	out, err := rdsClient.DescribeDBClustersWithContext(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBClusters) != 1 {
		return nil, trace.BadParameter("expected 1 RDS cluster for %v, got %s", clusterID, out.DBClusters)
	}
	return out.DBClusters[0], nil
}

// describeRedshiftCluster returns AWS Redshift cluster for the specified ID.
func describeRedshiftCluster(ctx context.Context, redshiftClient redshiftiface.RedshiftAPI, clusterID string) (*redshift.Cluster, error) {
	out, err := redshiftClient.DescribeClustersWithContext(ctx, &redshift.DescribeClustersInput{
		ClusterIdentifier: aws.String(clusterID),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.Clusters) != 1 {
		return nil, trace.BadParameter("expected 1 Redshift cluster for %v, got %s", clusterID, out.Clusters)
	}
	return out.Clusters[0], nil
}

// fetchRDSProxyMetadata fetches metadata about specified RDS proxy name.
func fetchRDSProxyMetadata(ctx context.Context, rdsClient rdsiface.RDSAPI, proxyName string) (*types.AWS, error) {
	rdsProxy, err := describeRDSProxy(ctx, rdsClient, proxyName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.MetadataFromRDSProxy(rdsProxy)
}

// describeRDSProxy returns AWS RDS proxy for the specified RDS proxy name.
func describeRDSProxy(ctx context.Context, rdsClient rdsiface.RDSAPI, proxyName string) (*rds.DBProxy, error) {
	out, err := rdsClient.DescribeDBProxiesWithContext(ctx, &rds.DescribeDBProxiesInput{
		DBProxyName: aws.String(proxyName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBProxies) != 1 {
		return nil, trace.BadParameter("expected 1 RDS proxy for %v, got %s", proxyName, out.DBProxies)
	}
	return out.DBProxies[0], nil
}

// fetchRDSProxyEndpointMetadata fetches metadata about specified RDS proxy endpoint.
func fetchRDSProxyEndpointMetadata(ctx context.Context, rdsClient rdsiface.RDSAPI, proxyEndpointName string) (*types.AWS, error) {
	rdsProxyEndpoint, err := describeRDSProxyEndpoint(ctx, rdsClient, proxyEndpointName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rdsProxy, err := describeRDSProxy(ctx, rdsClient, aws.StringValue(rdsProxyEndpoint.DBProxyName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return services.MetadataFromRDSProxyEndpoint(rdsProxy, rdsProxyEndpoint)
}

// describeRDSProxyEndpoint returns AWS RDS proxy endpoint for the specified RDS proxy endpoint.
func describeRDSProxyEndpoint(ctx context.Context, rdsClient rdsiface.RDSAPI, proxyEndpointName string) (*rds.DBProxyEndpoint, error) {
	out, err := rdsClient.DescribeDBProxyEndpointsWithContext(ctx, &rds.DescribeDBProxyEndpointsInput{
		DBProxyEndpointName: aws.String(proxyEndpointName),
	})
	if err != nil {
		return nil, common.ConvertError(err)
	}
	if len(out.DBProxyEndpoints) != 1 {
		return nil, trace.BadParameter("expected 1 RDS proxy endpoint for %v, got %s", proxyEndpointName, out.DBProxyEndpoints)
	}
	return out.DBProxyEndpoints[0], nil
}
