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

package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/rds"

	"github.com/gravitational/trace"
)

// DatabaseGetter defines interface for fetching database resources.
type DatabaseGetter interface {
	// GetDatabases returns all database resources.
	GetDatabases(context.Context) ([]types.Database, error)
	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)
}

// Databases defines an interface for managing database resources.
type Databases interface {
	// DatabaseGetter provides methods for fetching database resources.
	DatabaseGetter
	// CreateDatabase creates a new database resource.
	CreateDatabase(context.Context, types.Database) error
	// UpdateDatabse updates an existing database resource.
	UpdateDatabase(context.Context, types.Database) error
	// DeleteDatabase removes the specified database resource.
	DeleteDatabase(ctx context.Context, name string) error
	// DeleteAllDatabases removes all database resources.
	DeleteAllDatabases(context.Context) error
}

// MarshalDatabase marshals the database resource to JSON.
func MarshalDatabase(database types.Database, opts ...MarshalOption) ([]byte, error) {
	if err := database.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch database := database.(type) {
	case *types.DatabaseV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *database
			copy.SetResourceID(0)
			database = &copy
		}
		return utils.FastMarshal(database)
	default:
		return nil, trace.BadParameter("unsupported database resource %T", database)
	}
}

// UnmarshalDatabase unmarshals the database resource from JSON.
func UnmarshalDatabase(data []byte, opts ...MarshalOption) (types.Database, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var database types.DatabaseV3
		if err := utils.FastUnmarshal(data, &database); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := database.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			database.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			database.SetExpiry(cfg.Expires)
		}
		return &database, nil
	}
	return nil, trace.BadParameter("unsupported database resource version %q", h.Version)
}

// NewDatabaseFromRDSInstance creates a database resource from an RDS instance.
func NewDatabaseFromRDSInstance(instance *rds.DBInstance) (types.Database, error) {
	endpoint := instance.Endpoint
	if endpoint == nil {
		return nil, trace.BadParameter("empty endpoint")
	}
	metadata, err := MetadataFromRDSInstance(instance)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:        aws.StringValue(instance.DBInstanceIdentifier),
		Description: fmt.Sprintf("RDS instance in %v", metadata.Region),
		Labels:      labelsFromRDSInstance(instance, metadata),
	}, types.DatabaseSpecV3{
		Protocol: engineToProtocol(aws.StringValue(instance.Engine)),
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint.Address), aws.Int64Value(endpoint.Port)),
		AWS:      *metadata,
	})
}

// NewDatabaseFromRDSCluster creates a database resource from an RDS cluster (Aurora).
func NewDatabaseFromRDSCluster(cluster *rds.DBCluster) (types.Database, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:        aws.StringValue(cluster.DBClusterIdentifier),
		Description: fmt.Sprintf("Aurora cluster in %v", metadata.Region),
		Labels:      labelsFromRDSCluster(cluster, metadata, RDSEndpointTypePrimary),
	}, types.DatabaseSpecV3{
		Protocol: engineToProtocol(aws.StringValue(cluster.Engine)),
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.Endpoint), aws.Int64Value(cluster.Port)),
		AWS:      *metadata,
	})
}

// NewDatabaseFromRDSClusterReader creates a database resource from an RDS cluster reader endpoint (Aurora).
func NewDatabaseFromRDSClusterReader(cluster *rds.DBCluster) (types.Database, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:        rdsCustomDatabaseName(cluster, string(RDSEndpointTypeReader)),
		Description: fmt.Sprintf("Aurora cluster in %v (%v endpoint)", metadata.Region, string(RDSEndpointTypeReader)),
		Labels:      labelsFromRDSCluster(cluster, metadata, RDSEndpointTypeReader),
	}, types.DatabaseSpecV3{
		Protocol: engineToProtocol(aws.StringValue(cluster.Engine)),
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.ReaderEndpoint), aws.Int64Value(cluster.Port)),
		AWS:      *metadata,

		// overwrite server name with the primary endpoint as the rds instances
		// may not have up-to-date certs that contains reader/custom endpoints
		TLS: types.DatabaseTLS{
			ServerName: aws.StringValue(cluster.Endpoint),
		},
	})
}

// NewDatabasesFromRDSClusterCustomEndpoints creates database resources from RDS cluster custom endpoints (Aurora).
func NewDatabasesFromRDSClusterCustomEndpoints(cluster *rds.DBCluster) (types.Databases, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var databases types.Databases
	for _, endpoint := range cluster.CustomEndpoints {
		endpointName, err := parseRDSCustomEndpoint(aws.StringValue(endpoint))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		database, err := types.NewDatabaseV3(types.Metadata{
			Name:        rdsCustomDatabaseName(cluster, endpointName),
			Description: fmt.Sprintf("Aurora cluster in %v (%v endpoint)", metadata.Region, string(RDSEndpointTypeCustom)),
			Labels:      labelsFromRDSCluster(cluster, metadata, RDSEndpointTypeCustom),
		}, types.DatabaseSpecV3{
			Protocol: engineToProtocol(aws.StringValue(cluster.Engine)),
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint), aws.Int64Value(cluster.Port)),
			AWS:      *metadata,

			// overwrite server name with the primary endpoint as the rds instances
			// may not have up-to-date certs that contains reader/custom endpoints
			TLS: types.DatabaseTLS{
				ServerName: aws.StringValue(cluster.Endpoint),
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		databases = append(databases, database)
	}
	return databases, nil
}

// MetadataFromRDSInstance creates AWS metadata from the provided RDS instance.
func MetadataFromRDSInstance(rdsInstance *rds.DBInstance) (*types.AWS, error) {
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

// MetadataFromRDSCluster creates AWS metadata from the provided RDS cluster.
func MetadataFromRDSCluster(rdsCluster *rds.DBCluster) (*types.AWS, error) {
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

// engineToProtocol converts RDS instance engine to the database protocol.
func engineToProtocol(engine string) string {
	switch engine {
	case RDSEnginePostgres, RDSEngineAuroraPostgres:
		return defaults.ProtocolPostgres
	case RDSEngineMySQL, RDSEngineAurora, RDSEngineAuroraMySQL:
		return defaults.ProtocolMySQL
	}
	return ""
}

// parseRDSCustomEndpoint endpoint name from the provided RDS custom endpoint.
func parseRDSCustomEndpoint(endpoint string) (name string, err error) {
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html#Aurora.Endpoints.Custom
	//
	// RDS custom endpoint format:
	// <endpointName>.cluster-custom-<customerDnsIdentifier>.<dnsSuffix>
	//
	// Note that endpoint name can only contain letters, numbers, and hyphens. So it'safe to to split on ".".
	parts := strings.Split(endpoint, ".")
	if !strings.HasSuffix(endpoint, rdsEndpointSuffix) || len(parts) != 6 {
		return "", trace.BadParameter("failed to parse %v as RDS custo endpoint", endpoint)
	}
	return parts[0], nil
}

// labelsFromRDSInstance creates database labels for the provided RDS instance.
func labelsFromRDSInstance(rdsInstance *rds.DBInstance, meta *types.AWS) map[string]string {
	labels := tagsToLabels(rdsInstance.TagList)
	labels[types.OriginLabel] = types.OriginCloud
	labels[labelAccountID] = meta.AccountID
	labels[labelRegion] = meta.Region
	labels[labelEngine] = aws.StringValue(rdsInstance.Engine)
	labels[labelEngineVersion] = aws.StringValue(rdsInstance.EngineVersion)
	labels[labelEndpointType] = string(RDSEndpointTypeInstance)
	return labels
}

// labelsFromRDSCluster creates database labels for the provided RDS cluster.
func labelsFromRDSCluster(rdsCluster *rds.DBCluster, meta *types.AWS, endpointType RDSEndpointType) map[string]string {
	labels := tagsToLabels(rdsCluster.TagList)
	labels[types.OriginLabel] = types.OriginCloud
	labels[labelAccountID] = meta.AccountID
	labels[labelRegion] = meta.Region
	labels[labelEngine] = aws.StringValue(rdsCluster.Engine)
	labels[labelEngineVersion] = aws.StringValue(rdsCluster.EngineVersion)
	labels[labelEndpointType] = string(endpointType)
	return labels
}

// tagsToLabels converts RDS tags to a labels map.
func tagsToLabels(tags []*rds.Tag) map[string]string {
	labels := make(map[string]string)
	for _, tag := range tags {
		labels[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return labels
}

// rdsCustomDatabaseName generates database names for RDS cluster reader and custom endpoints
func rdsCustomDatabaseName(cluster *rds.DBCluster, endpointName string) string {
	return fmt.Sprintf("%s-%s", aws.StringValue(cluster.DBClusterIdentifier), endpointName)
}

const (
	// labelAccountID is the label key containing AWS account ID.
	labelAccountID = "account-id"
	// labelRegion is the label key containing AWS region.
	labelRegion = "region"
	// labelEngine is the label key containing RDS database engine name.
	labelEngine = "engine"
	// labelEngineVersion is the label key containing RDS database engine version.
	labelEngineVersion = "engine-version"
	// labelEndpointType is the label key containing endpoint type.
	labelEndpointType = "endpoint-type"
)

const (
	// rdsEndpointSuffix is the RDS/Aurora endpoint suffix.
	rdsEndpointSuffix = ".rds.amazonaws.com"
)

const (
	// RDSEngineMySQL is RDS engine name for MySQL instances.
	RDSEngineMySQL = "mysql"
	// RDSEnginePostgres is RDS engine name for Postgres instances.
	RDSEnginePostgres = "postgres"
	// RDSEngineAurora is RDS engine name for Aurora MySQL 5.6 compatible clusters.
	RDSEngineAurora = "aurora"
	// RDSEngineAuroraMySQL is RDS engine name for Aurora MySQL 5.7 compatible clusters.
	RDSEngineAuroraMySQL = "aurora-mysql"
	// RDSEngineAuroraPostgres is RDS engine name for Aurora Postgres clusters.
	RDSEngineAuroraPostgres = "aurora-postgresql"
)

// RDSEndpointType specifies the endpoint type
type RDSEndpointType string

const (
	// RDSEndpointTypePrimary is the endpoint that specifies the connection for the primary instance of the RDS cluster.
	RDSEndpointTypePrimary RDSEndpointType = "primary"
	// RDSEndpointTypeReader is the endpoint that load-balances connections across the Aurora Replicas that are
	// available in a RDS cluster.
	RDSEndpointTypeReader RDSEndpointType = "reader"
	// RDSEndpointTypeCustom is a custom endpoint associated with the cluster.
	RDSEndpointTypeCustom RDSEndpointType = "custom"
	// RDSEndpointTypeInstance is the endpoint that the address of the DB instance.
	RDSEndpointTypeInstance RDSEndpointType = "instance"
)
