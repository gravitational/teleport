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
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"

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
		database.SetAWS(*metadata)
	}
	return nil
}

// fetchRDSMetadata fetches metadata for the provided RDS or Aurora database.
func (m *Metadata) fetchRDSMetadata(ctx context.Context, database types.Database) (*types.AWS, error) {
	rds, err := m.cfg.Clients.GetAWSRDSClient(database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// First try to fetch the RDS instance metadata.
	metadata, err := fetchRDSInstanceMetadata(ctx, rds, database.GetAWS().RDS.InstanceID)
	if err != nil && !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
		return nil, trace.Wrap(err)
	}
	// If RDS instance metadata wasn't found, it may be an Aurora cluster.
	if metadata == nil {
		// Aurora cluster ID may be either explicitly specified or parsed
		// from endpoint in which case it will be in InstanceID field.
		clusterID := database.GetAWS().RDS.ClusterID
		if clusterID == "" {
			clusterID = database.GetAWS().RDS.InstanceID
		}
		return fetchRDSClusterMetadata(ctx, rds, clusterID)
	}
	// If instance was found, it may be a part of an Aurora cluster.
	if metadata.RDS.ClusterID != "" {
		return fetchRDSClusterMetadata(ctx, rds, metadata.RDS.ClusterID)
	}
	return metadata, nil
}

// fetchRDSInstanceMetadata fetches metadata about specified RDS instance.
func fetchRDSInstanceMetadata(ctx context.Context, rdsClient rdsiface.RDSAPI, instanceID string) (*types.AWS, error) {
	rdsInstance, err := describeRDSInstance(ctx, rdsClient, instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedARN, err := arn.Parse(aws.StringValue(rdsInstance.DBInstanceArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RDS: types.RDS{
			InstanceID: aws.StringValue(rdsInstance.DBInstanceIdentifier),
			ClusterID:  aws.StringValue(rdsInstance.DBClusterIdentifier),
			ResourceID: aws.StringValue(rdsInstance.DbiResourceId),
			IAMAuth:    aws.BoolValue(rdsInstance.IAMDatabaseAuthenticationEnabled),
		},
	}, nil
}

// describeRDSInstance returns AWS RDS instance for the specified database.
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
	parsedARN, err := arn.Parse(aws.StringValue(rdsCluster.DBClusterArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RDS: types.RDS{
			ClusterID:  aws.StringValue(rdsCluster.DBClusterIdentifier),
			ResourceID: aws.StringValue(rdsCluster.DbClusterResourceId),
			IAMAuth:    aws.BoolValue(rdsCluster.IAMDatabaseAuthenticationEnabled),
		},
	}, nil
}

// describeRDSCluster returns AWS Aurora cluster for the specified database.
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
