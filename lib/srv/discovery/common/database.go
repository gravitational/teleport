// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	rdsTypesV2 "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// setAWSDBName sets database name, overriding the first part if the database
// override label for AWS is present.
func setAWSDBName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName(types.AWSDatabaseNameOverrideLabels, meta, firstNamePart, extraNameParts...)
}

// setDBName sets database name, overriding the first part if the Azure database
// override label for Azure is present.
func setAzureDBName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.AzureDatabaseNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
}

// NewDatabaseFromAzureServer creates a database resource from an AzureDB server.
func NewDatabaseFromAzureServer(server *azure.DBServer) (types.Database, error) {
	fqdn := server.Properties.FullyQualifiedDomainName
	if fqdn == "" {
		return nil, trace.BadParameter("empty FQDN")
	}
	labels, err := labelsFromAzureServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure %v server in %v",
				defaults.ReadableDatabaseProtocol(server.Protocol),
				server.Location),
			Labels: labels,
		}, server.Name),
		types.DatabaseSpecV3{
			Protocol: server.Protocol,
			URI:      fmt.Sprintf("%v:%v", fqdn, server.Port),
			Azure: types.Azure{
				Name:       server.Name,
				ResourceID: server.ID,
			},
		})
}

// NewDatabaseFromAzureRedis creates a database resource from an Azure Redis server.
func NewDatabaseFromAzureRedis(server *armredis.ResourceInfo) (types.Database, error) {
	if server.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}
	if server.Properties.SSLPort == nil {
		return nil, trace.BadParameter("missing SSL port")
	}
	labels, err := labelsFromAzureRedis(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure Redis server in %v", azure.StringVal(server.Location)),
			Labels:      labels,
		}, azure.StringVal(server.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolRedis,
			URI:      fmt.Sprintf("%v:%v", azure.StringVal(server.Properties.HostName), *server.Properties.SSLPort),
			Azure: types.Azure{
				Name:       azure.StringVal(server.Name),
				ResourceID: azure.StringVal(server.ID),
			},
		})
}

// NewDatabaseFromAzureRedisEnterprise creates a database resource from an
// Azure Redis Enterprise database and its parent cluster.
func NewDatabaseFromAzureRedisEnterprise(cluster *armredisenterprise.Cluster, database *armredisenterprise.Database) (types.Database, error) {
	if cluster.Properties == nil || database.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}
	if database.Properties.Port == nil {
		return nil, trace.BadParameter("missing port")
	}
	labels, err := labelsFromAzureRedisEnterprise(cluster, database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the database name is "default", use only the cluster name as the name.
	// If the database name is not "default", use "<cluster>-<database>" as the name.
	var nameSuffix []string
	if azure.StringVal(database.Name) != azure.RedisEnterpriseClusterDefaultDatabase {
		nameSuffix = append(nameSuffix, azure.StringVal(database.Name))
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure Redis Enterprise server in %v", azure.StringVal(cluster.Location)),
			Labels:      labels,
		}, azure.StringVal(cluster.Name), nameSuffix...),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolRedis,
			URI:      fmt.Sprintf("%v:%v", azure.StringVal(cluster.Properties.HostName), *database.Properties.Port),
			Azure: types.Azure{
				ResourceID: azure.StringVal(database.ID),
				Redis: types.AzureRedis{
					ClusteringPolicy: azure.StringVal(database.Properties.ClusteringPolicy),
				},
			},
		})
}

// NewDatabaseFromAzureSQLServer creates a database resource from an Azure SQL
// server.
func NewDatabaseFromAzureSQLServer(server *armsql.Server) (types.Database, error) {
	if server.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}

	if server.Properties.FullyQualifiedDomainName == nil {
		return nil, trace.BadParameter("missing FQDN")
	}

	labels, err := labelsFromAzureSQLServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure SQL Server in %v", azure.StringVal(server.Location)),
			Labels:      labels,
		}, azure.StringVal(server.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolSQLServer,
			URI:      fmt.Sprintf("%v:%d", azure.StringVal(server.Properties.FullyQualifiedDomainName), azureSQLServerDefaultPort),
			Azure: types.Azure{
				Name:       azure.StringVal(server.Name),
				ResourceID: azure.StringVal(server.ID),
			},
		})
}

// NewDatabaseFromAzureManagedSQLServer creates a database resource from an
// Azure Managed SQL server.
func NewDatabaseFromAzureManagedSQLServer(server *armsql.ManagedInstance) (types.Database, error) {
	if server.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}

	if server.Properties.FullyQualifiedDomainName == nil {
		return nil, trace.BadParameter("missing FQDN")
	}

	labels, err := labelsFromAzureManagedSQLServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure Managed SQL Server in %v", azure.StringVal(server.Location)),
			Labels:      labels,
		}, azure.StringVal(server.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolSQLServer,
			URI:      fmt.Sprintf("%v:%d", azure.StringVal(server.Properties.FullyQualifiedDomainName), azureSQLServerDefaultPort),
			Azure: types.Azure{
				Name:       azure.StringVal(server.Name),
				ResourceID: azure.StringVal(server.ID),
			},
		})
}

// NewDatabaseFromAzureMySQLFlexServer creates a database resource from an Azure MySQL Flexible server.
func NewDatabaseFromAzureMySQLFlexServer(server *armmysqlflexibleservers.Server) (types.Database, error) {
	if server.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}

	if server.Properties.FullyQualifiedDomainName == nil {
		return nil, trace.BadParameter("missing FQDN")
	}

	labels, err := labelsFromAzureMySQLFlexServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var description string
	if replicaRole, ok := labels[types.DiscoveryLabelAzureReplicationRole]; ok {
		description = fmt.Sprintf("Azure MySQL Flexible server in %v (%v endpoint)",
			azure.StringVal(server.Location), strings.ToLower(replicaRole))
	} else {
		description = fmt.Sprintf("Azure MySQL Flexible server in %v", azure.StringVal(server.Location))
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: description,
			Labels:      labels,
		}, azure.StringVal(server.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      fmt.Sprintf("%v:%v", azure.StringVal(server.Properties.FullyQualifiedDomainName), azure.MySQLPort),
			Azure: types.Azure{
				Name:          azure.StringVal(server.Name),
				ResourceID:    azure.StringVal(server.ID),
				IsFlexiServer: true,
			},
		})
}

// NewDatabaseFromAzurePostgresFlexServer creates a database resource from an Azure PostgreSQL Flexible server.
func NewDatabaseFromAzurePostgresFlexServer(server *armpostgresqlflexibleservers.Server) (types.Database, error) {
	if server.Properties == nil {
		return nil, trace.BadParameter("missing properties")
	}

	if server.Properties.FullyQualifiedDomainName == nil {
		return nil, trace.BadParameter("missing FQDN")
	}

	labels, err := labelsFromAzurePostgresFlexServer(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAzureDBName(types.Metadata{
			Description: fmt.Sprintf("Azure PostgreSQL Flexible server in %v", azure.StringVal(server.Location)),
			Labels:      labels,
		}, azure.StringVal(server.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", azure.StringVal(server.Properties.FullyQualifiedDomainName), azure.PostgresPort),
			Azure: types.Azure{
				Name:          azure.StringVal(server.Name),
				ResourceID:    azure.StringVal(server.ID),
				IsFlexiServer: true,
			},
		})
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
	protocol, err := rdsEngineToProtocol(aws.StringValue(instance.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("RDS instance in %v", metadata.Region),
			Labels:      labelsFromRDSInstance(instance, metadata),
		}, aws.StringValue(instance.DBInstanceIdentifier)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint.Address), aws.Int64Value(endpoint.Port)),
			AWS:      *metadata,
		})
}

// NewDatabaseFromRDSV2Instance creates a database resource from an RDS instance.
// It uses aws sdk v2.
func NewDatabaseFromRDSV2Instance(instance *rdsTypesV2.DBInstance) (types.Database, error) {
	endpoint := instance.Endpoint
	if endpoint == nil {
		return nil, trace.BadParameter("empty endpoint")
	}
	metadata, err := MetadataFromRDSV2Instance(instance)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, err := rdsEngineToProtocol(aws.StringValue(instance.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uri := ""
	if instance.Endpoint != nil && instance.Endpoint.Address != nil {
		if instance.Endpoint.Port != nil {
			uri = fmt.Sprintf("%s:%d", aws.StringValue(instance.Endpoint.Address), *instance.Endpoint.Port)
		} else {
			uri = aws.StringValue(instance.Endpoint.Address)
		}
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("RDS instance in %v", metadata.Region),
			Labels:      labelsFromRDSV2Instance(instance, metadata),
		}, aws.StringValue(instance.DBInstanceIdentifier)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
			AWS:      *metadata,
		})
}

// MetadataFromRDSInstance creates AWS metadata from the provided RDS instance.
// It uses aws sdk v2.
func MetadataFromRDSV2Instance(rdsInstance *rdsTypesV2.DBInstance) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(rdsInstance.DBInstanceArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vpcID, subnets := rdsSubnetGroupToNetworkInfo(rdsInstance.DBSubnetGroup)

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RDS: types.RDS{
			InstanceID: aws.StringValue(rdsInstance.DBInstanceIdentifier),
			ClusterID:  aws.StringValue(rdsInstance.DBClusterIdentifier),
			ResourceID: aws.StringValue(rdsInstance.DbiResourceId),
			IAMAuth:    aws.BoolValue(rdsInstance.IAMDatabaseAuthenticationEnabled),
			Subnets:    subnets,
			VPCID:      vpcID,
		},
	}, nil
}

// labelsFromRDSV2Instance creates database labels for the provided RDS instance.
// It uses aws sdk v2.
func labelsFromRDSV2Instance(rdsInstance *rdsTypesV2.DBInstance, meta *types.AWS) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEngine] = aws.StringValue(rdsInstance.Engine)
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(rdsInstance.EngineVersion)
	labels[types.DiscoveryLabelEndpointType] = apiawsutils.RDSEndpointTypeInstance
	labels[types.DiscoveryLabelStatus] = aws.StringValue(rdsInstance.DBInstanceStatus)
	if rdsInstance.DBSubnetGroup != nil {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(rdsInstance.DBSubnetGroup.VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(rdsInstance.TagList))
}

// NewDatabaseFromRDSV2Cluster creates a database resource from an RDS cluster (Aurora).
// It uses aws sdk v2.
func NewDatabaseFromRDSV2Cluster(cluster *rdsTypesV2.DBCluster, firstInstance *rdsTypesV2.DBInstance) (types.Database, error) {
	metadata, err := MetadataFromRDSV2Cluster(cluster, firstInstance)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, err := rdsEngineToProtocol(aws.StringValue(cluster.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uri := ""
	if cluster.Endpoint != nil && cluster.Port != nil {
		uri = fmt.Sprintf("%v:%v", aws.StringValue(cluster.Endpoint), *cluster.Port)
	}
	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Aurora cluster in %v", metadata.Region),
			Labels:      labelsFromRDSV2Cluster(cluster, metadata, apiawsutils.RDSEndpointTypePrimary, firstInstance),
		}, aws.StringValue(cluster.DBClusterIdentifier)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
			AWS:      *metadata,
		})
}

func rdsSubnetGroupToNetworkInfo(subnetGroup *rdsTypesV2.DBSubnetGroup) (vpcID string, subnets []string) {
	if subnetGroup == nil {
		return
	}

	vpcID = aws.StringValue(subnetGroup.VpcId)
	subnets = make([]string, 0, len(subnetGroup.Subnets))
	for _, s := range subnetGroup.Subnets {
		subnetID := aws.StringValue(s.SubnetIdentifier)
		if subnetID != "" {
			subnets = append(subnets, subnetID)
		}
	}

	return
}

// MetadataFromRDSV2Cluster creates AWS metadata from the provided RDS cluster.
// It uses aws sdk v2.
// An optional [rdsTypesV2.DBInstance] can be passed to fill the network configuration of the Cluster.
func MetadataFromRDSV2Cluster(rdsCluster *rdsTypesV2.DBCluster, rdsInstance *rdsTypesV2.DBInstance) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(rdsCluster.DBClusterArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var vpcID string
	var subnets []string

	if rdsInstance != nil {
		vpcID, subnets = rdsSubnetGroupToNetworkInfo(rdsInstance.DBSubnetGroup)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RDS: types.RDS{
			ClusterID:  aws.StringValue(rdsCluster.DBClusterIdentifier),
			ResourceID: aws.StringValue(rdsCluster.DbClusterResourceId),
			IAMAuth:    aws.BoolValue(rdsCluster.IAMDatabaseAuthenticationEnabled),
			Subnets:    subnets,
			VPCID:      vpcID,
		},
	}, nil
}

// labelsFromRDSV2Cluster creates database labels for the provided RDS cluster.
// It uses aws sdk v2.
func labelsFromRDSV2Cluster(rdsCluster *rdsTypesV2.DBCluster, meta *types.AWS, endpointType string, memberInstance *rdsTypesV2.DBInstance) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEngine] = aws.StringValue(rdsCluster.Engine)
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(rdsCluster.EngineVersion)
	labels[types.DiscoveryLabelEndpointType] = endpointType
	labels[types.DiscoveryLabelStatus] = aws.StringValue(rdsCluster.Status)
	if memberInstance != nil && memberInstance.DBSubnetGroup != nil {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(memberInstance.DBSubnetGroup.VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(rdsCluster.TagList))
}

// NewDatabaseFromRDSCluster creates a database resource from an RDS cluster (Aurora).
func NewDatabaseFromRDSCluster(cluster *rds.DBCluster, memberInstances []*rds.DBInstance) (types.Database, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, err := rdsEngineToProtocol(aws.StringValue(cluster.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Aurora cluster in %v", metadata.Region),
			Labels:      labelsFromRDSCluster(cluster, metadata, apiawsutils.RDSEndpointTypePrimary, memberInstances),
		}, aws.StringValue(cluster.DBClusterIdentifier)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.Endpoint), aws.Int64Value(cluster.Port)),
			AWS:      *metadata,
		})
}

// NewDatabaseFromRDSClusterReaderEndpoint creates a database resource from an RDS cluster reader endpoint (Aurora).
func NewDatabaseFromRDSClusterReaderEndpoint(cluster *rds.DBCluster, memberInstances []*rds.DBInstance) (types.Database, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, err := rdsEngineToProtocol(aws.StringValue(cluster.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Aurora cluster in %v (%v endpoint)", metadata.Region, apiawsutils.RDSEndpointTypeReader),
			Labels:      labelsFromRDSCluster(cluster, metadata, apiawsutils.RDSEndpointTypeReader, memberInstances),
		}, aws.StringValue(cluster.DBClusterIdentifier), apiawsutils.RDSEndpointTypeReader),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.ReaderEndpoint), aws.Int64Value(cluster.Port)),
			AWS:      *metadata,
		})
}

// NewDatabasesFromRDSClusterCustomEndpoints creates database resources from RDS cluster custom endpoints (Aurora).
func NewDatabasesFromRDSClusterCustomEndpoints(cluster *rds.DBCluster, memberInstances []*rds.DBInstance) (types.Databases, error) {
	metadata, err := MetadataFromRDSCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, err := rdsEngineToProtocol(aws.StringValue(cluster.Engine))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var errors []error
	var databases types.Databases
	for _, endpoint := range cluster.CustomEndpoints {
		// RDS custom endpoint format:
		// <endpointName>.cluster-custom-<customerDnsIdentifier>.<dnsSuffix>
		endpointDetails, err := apiawsutils.ParseRDSEndpoint(aws.StringValue(endpoint))
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		if endpointDetails.ClusterCustomEndpointName == "" {
			errors = append(errors, trace.BadParameter("missing Aurora custom endpoint name"))
			continue
		}

		database, err := types.NewDatabaseV3(
			setAWSDBName(types.Metadata{
				Description: fmt.Sprintf("Aurora cluster in %v (%v endpoint)", metadata.Region, apiawsutils.RDSEndpointTypeCustom),
				Labels:      labelsFromRDSCluster(cluster, metadata, apiawsutils.RDSEndpointTypeCustom, memberInstances),
			}, aws.StringValue(cluster.DBClusterIdentifier), apiawsutils.RDSEndpointTypeCustom, endpointDetails.ClusterCustomEndpointName),
			types.DatabaseSpecV3{
				Protocol: protocol,
				URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint), aws.Int64Value(cluster.Port)),
				AWS:      *metadata,

				// Aurora instances update their certificates upon restart, and thus custom endpoint SAN may not be available right
				// away. Using primary endpoint instead as server name since it's always available.
				TLS: types.DatabaseTLS{
					ServerName: aws.StringValue(cluster.Endpoint),
				},
			})
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}

		databases = append(databases, database)
	}

	return databases, trace.NewAggregate(errors...)
}

// NewDatabasesFromRDSCluster creates all database resources from an RDS Aurora
// cluster.
func NewDatabasesFromRDSCluster(cluster *rds.DBCluster, memberInstances []*rds.DBInstance) (types.Databases, error) {
	var errors []error
	var databases types.Databases

	// Find out what types of instances the cluster has. Some examples:
	// - Aurora cluster with one instance: one writer
	// - Aurora cluster with three instances: one writer and two readers
	// - Secondary cluster of a global database: one or more readers
	var hasWriterInstance, hasReaderInstance bool
	for _, clusterMember := range cluster.DBClusterMembers {
		if clusterMember != nil {
			if aws.BoolValue(clusterMember.IsClusterWriter) {
				hasWriterInstance = true
			} else {
				hasReaderInstance = true
			}
		}
	}

	// Add a database from primary endpoint, if any writer instances.
	if cluster.Endpoint != nil && hasWriterInstance {
		database, err := NewDatabaseFromRDSCluster(cluster, memberInstances)
		if err != nil {
			errors = append(errors, err)
		} else {
			databases = append(databases, database)
		}
	}

	// Add a database from reader endpoint, if any reader instances.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html#Aurora.Endpoints.Reader
	if cluster.ReaderEndpoint != nil && hasReaderInstance {
		database, err := NewDatabaseFromRDSClusterReaderEndpoint(cluster, memberInstances)
		if err != nil {
			errors = append(errors, err)
		} else {
			databases = append(databases, database)
		}
	}

	// Add databases from custom endpoints
	if len(cluster.CustomEndpoints) > 0 {
		customEndpointDatabases, err := NewDatabasesFromRDSClusterCustomEndpoints(cluster, memberInstances)
		if err != nil {
			errors = append(errors, err)
		}
		databases = append(databases, customEndpointDatabases...)
	}

	return databases, trace.NewAggregate(errors...)
}

// NewDatabaseFromRDSProxy creates database resource from RDS Proxy.
func NewDatabaseFromRDSProxy(dbProxy *rds.DBProxy, tags []*rds.Tag) (types.Database, error) {
	metadata, err := MetadataFromRDSProxy(dbProxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, port, err := rdsEngineFamilyToProtocolAndPort(aws.StringValue(dbProxy.EngineFamily))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("RDS Proxy in %v", metadata.Region),
			Labels:      labelsFromRDSProxy(dbProxy, metadata, tags),
		}, aws.StringValue(dbProxy.DBProxyName)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      fmt.Sprintf("%s:%d", aws.StringValue(dbProxy.Endpoint), port),
			AWS:      *metadata,
		})
}

// NewDatabaseFromRDSProxyCustomEndpoint creates database resource from RDS
// Proxy custom endpoint.
func NewDatabaseFromRDSProxyCustomEndpoint(dbProxy *rds.DBProxy, customEndpoint *rds.DBProxyEndpoint, tags []*rds.Tag) (types.Database, error) {
	metadata, err := MetadataFromRDSProxyCustomEndpoint(dbProxy, customEndpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	protocol, port, err := rdsEngineFamilyToProtocolAndPort(aws.StringValue(dbProxy.EngineFamily))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("RDS Proxy endpoint in %v", metadata.Region),
			Labels:      labelsFromRDSProxyCustomEndpoint(dbProxy, customEndpoint, metadata, tags),
		}, aws.StringValue(dbProxy.DBProxyName), aws.StringValue(customEndpoint.DBProxyEndpointName)),
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      fmt.Sprintf("%s:%d", aws.StringValue(customEndpoint.Endpoint), port),
			AWS:      *metadata,

			// RDS proxies serve wildcard certificates like this:
			// *.proxy-<xxx>.<region>.rds.amazonaws.com
			//
			// However the custom endpoints have one extra level of subdomains like:
			// <name>.endpoint.proxy-<xxx>.<region>.rds.amazonaws.com
			// which will fail verify_full against the wildcard certificates.
			//
			// Using proxy's default endpoint as server name as it should always
			// succeed.
			TLS: types.DatabaseTLS{
				ServerName: aws.StringValue(dbProxy.Endpoint),
			},
		})
}

// NewDatabaseFromRedshiftCluster creates a database resource from a Redshift cluster.
func NewDatabaseFromRedshiftCluster(cluster *redshift.Cluster) (types.Database, error) {
	// Endpoint can be nil while the cluster is being created. Return an error
	// until the Endpoint is available.
	if cluster.Endpoint == nil {
		return nil, trace.BadParameter("missing endpoint in Redshift cluster %v", aws.StringValue(cluster.ClusterIdentifier))
	}

	metadata, err := MetadataFromRedshiftCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Redshift cluster in %v", metadata.Region),
			Labels:      labelsFromRedshiftCluster(cluster, metadata),
		}, aws.StringValue(cluster.ClusterIdentifier)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.Endpoint.Address), aws.Int64Value(cluster.Endpoint.Port)),
			AWS:      *metadata,
		})
}

// NewDatabaseFromElastiCacheConfigurationEndpoint creates a database resource
// from ElastiCache configuration endpoint.
func NewDatabaseFromElastiCacheConfigurationEndpoint(cluster *elasticache.ReplicationGroup, extraLabels map[string]string) (types.Database, error) {
	if cluster.ConfigurationEndpoint == nil {
		return nil, trace.BadParameter("missing configuration endpoint")
	}

	return newElastiCacheDatabase(cluster, cluster.ConfigurationEndpoint, apiawsutils.ElastiCacheConfigurationEndpoint, extraLabels)
}

// NewDatabasesFromElastiCacheNodeGroups creates database resources from
// ElastiCache node groups.
func NewDatabasesFromElastiCacheNodeGroups(cluster *elasticache.ReplicationGroup, extraLabels map[string]string) (types.Databases, error) {
	var databases types.Databases
	for _, nodeGroup := range cluster.NodeGroups {
		if nodeGroup.PrimaryEndpoint != nil {
			database, err := newElastiCacheDatabase(cluster, nodeGroup.PrimaryEndpoint, apiawsutils.ElastiCachePrimaryEndpoint, extraLabels)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			databases = append(databases, database)
		}

		if nodeGroup.ReaderEndpoint != nil {
			database, err := newElastiCacheDatabase(cluster, nodeGroup.ReaderEndpoint, apiawsutils.ElastiCacheReaderEndpoint, extraLabels)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// NewDatabasesFromElastiCacheReplicationGroup creates all database resources
// from an ElastiCache ReplicationGroup.
func NewDatabasesFromElastiCacheReplicationGroup(cluster *elasticache.ReplicationGroup, extraLabels map[string]string) (types.Databases, error) {
	// Create database using configuration endpoint for Redis with cluster
	// mode enabled.
	if aws.BoolValue(cluster.ClusterEnabled) {
		database, err := NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Databases{database}, nil
	}

	// Create databases using primary and reader endpoints for Redis with
	// cluster mode disabled. When cluster mode is disabled, it is expected
	// there is only one node group (aka shard) with one primary endpoint
	// and one reader endpoint.
	databases, err := NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	return databases, trace.Wrap(err)
}

// newElastiCacheDatabase returns a new ElastiCache database.
func newElastiCacheDatabase(cluster *elasticache.ReplicationGroup, endpoint *elasticache.Endpoint, endpointType string, extraLabels map[string]string) (types.Database, error) {
	metadata, err := MetadataFromElastiCacheCluster(cluster, endpointType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	suffix := make([]string, 0)
	if endpointType == apiawsutils.ElastiCacheReaderEndpoint {
		suffix = []string{endpointType}
	}

	return types.NewDatabaseV3(setAWSDBName(types.Metadata{
		Description: fmt.Sprintf("ElastiCache cluster in %v (%v endpoint)", metadata.Region, endpointType),
		Labels:      labelsFromMetaAndEndpointType(metadata, endpointType, extraLabels),
	}, aws.StringValue(cluster.ReplicationGroupId), suffix...), types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint.Address), aws.Int64Value(endpoint.Port)),
		AWS:      *metadata,
	})
}

// NewDatabasesFromOpenSearchDomain creates database resources from an OpenSearch domain.
func NewDatabasesFromOpenSearchDomain(domain *opensearchservice.DomainStatus, tags []*opensearchservice.Tag) (types.Databases, error) {
	var databases types.Databases

	if aws.StringValue(domain.Endpoint) != "" {
		metadata, err := MetadataFromOpenSearchDomain(domain, apiawsutils.OpenSearchDefaultEndpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		meta := types.Metadata{
			Description: fmt.Sprintf("OpenSearch domain in %v (default endpoint)", metadata.Region),
			Labels:      labelsFromOpenSearchDomain(domain, metadata, apiawsutils.OpenSearchDefaultEndpoint, tags),
		}

		meta = setAWSDBName(meta, aws.StringValue(domain.DomainName))
		spec := types.DatabaseSpecV3{
			Protocol: defaults.ProtocolOpenSearch,
			URI:      fmt.Sprintf("%v:443", aws.StringValue(domain.Endpoint)),
			AWS:      *metadata,
		}

		db, err := types.NewDatabaseV3(meta, spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		databases = append(databases, db)
	}

	if domain.DomainEndpointOptions != nil && aws.StringValue(domain.DomainEndpointOptions.CustomEndpoint) != "" {
		metadata, err := MetadataFromOpenSearchDomain(domain, apiawsutils.OpenSearchCustomEndpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		meta := types.Metadata{
			Description: fmt.Sprintf("OpenSearch domain in %v (custom endpoint)", metadata.Region),
			Labels:      labelsFromOpenSearchDomain(domain, metadata, apiawsutils.OpenSearchCustomEndpoint, tags),
		}

		meta = setAWSDBName(meta, aws.StringValue(domain.DomainName), "custom")
		spec := types.DatabaseSpecV3{
			Protocol: defaults.ProtocolOpenSearch,
			URI:      fmt.Sprintf("%v:443", aws.StringValue(domain.DomainEndpointOptions.CustomEndpoint)),
			AWS:      *metadata,
		}

		db, err := types.NewDatabaseV3(meta, spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		databases = append(databases, db)
	}

	for name, url := range domain.Endpoints {
		metadata, err := MetadataFromOpenSearchDomain(domain, apiawsutils.OpenSearchVPCEndpoint)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		meta := types.Metadata{
			Description: fmt.Sprintf("OpenSearch domain in %v (endpoint %q)", metadata.Region, name),
			Labels:      labelsFromOpenSearchDomain(domain, metadata, apiawsutils.OpenSearchVPCEndpoint, tags),
		}

		if domain.VPCOptions != nil {
			meta.Labels[types.DiscoveryLabelVPCID] = aws.StringValue(domain.VPCOptions.VPCId)
		}

		meta = setAWSDBName(meta, aws.StringValue(domain.DomainName), name)
		spec := types.DatabaseSpecV3{
			Protocol: defaults.ProtocolOpenSearch,
			URI:      fmt.Sprintf("%v:443", aws.StringValue(url)),
			AWS:      *metadata,
		}

		db, err := types.NewDatabaseV3(meta, spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		databases = append(databases, db)
	}

	return databases, nil
}

// NewDatabaseFromMemoryDBCluster creates a database resource from a MemoryDB
// cluster.
func NewDatabaseFromMemoryDBCluster(cluster *memorydb.Cluster, extraLabels map[string]string) (types.Database, error) {
	endpointType := apiawsutils.MemoryDBClusterEndpoint

	metadata, err := MetadataFromMemoryDBCluster(cluster, endpointType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("MemoryDB cluster in %v", metadata.Region),
			Labels:      labelsFromMetaAndEndpointType(metadata, endpointType, extraLabels),
		}, aws.StringValue(cluster.Name)),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolRedis,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(cluster.ClusterEndpoint.Address), aws.Int64Value(cluster.ClusterEndpoint.Port)),
			AWS:      *metadata,
		})
}

// NewDatabaseFromRedshiftServerlessWorkgroup creates a database resource from
// a Redshift Serverless Workgroup.
func NewDatabaseFromRedshiftServerlessWorkgroup(workgroup *redshiftserverless.Workgroup, tags []*redshiftserverless.Tag) (types.Database, error) {
	if workgroup.Endpoint == nil {
		return nil, trace.BadParameter("missing endpoint")
	}

	metadata, err := MetadataFromRedshiftServerlessWorkgroup(workgroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Redshift Serverless workgroup in %v", metadata.Region),
			Labels:      labelsFromRedshiftServerlessWorkgroup(workgroup, metadata, tags),
		}, metadata.RedshiftServerless.WorkgroupName),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(workgroup.Endpoint.Address), aws.Int64Value(workgroup.Endpoint.Port)),
			AWS:      *metadata,
		})
}

// NewDatabaseFromRedshiftServerlessVPCEndpoint creates a database resource from
// a Redshift Serverless VPC endpoint.
func NewDatabaseFromRedshiftServerlessVPCEndpoint(endpoint *redshiftserverless.EndpointAccess, workgroup *redshiftserverless.Workgroup, tags []*redshiftserverless.Tag) (types.Database, error) {
	if workgroup.Endpoint == nil {
		return nil, trace.BadParameter("missing endpoint")
	}

	metadata, err := MetadataFromRedshiftServerlessVPCEndpoint(endpoint, workgroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return types.NewDatabaseV3(
		setAWSDBName(types.Metadata{
			Description: fmt.Sprintf("Redshift Serverless endpoint in %v", metadata.Region),
			Labels:      labelsFromRedshiftServerlessVPCEndpoint(endpoint, workgroup, metadata, tags),
		}, metadata.RedshiftServerless.WorkgroupName, metadata.RedshiftServerless.EndpointName),
		types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      fmt.Sprintf("%v:%v", aws.StringValue(endpoint.Address), aws.Int64Value(endpoint.Port)),
			AWS:      *metadata,

			// Use workgroup's default address as the server name.
			TLS: types.DatabaseTLS{
				ServerName: aws.StringValue(workgroup.Endpoint.Address),
			},
		})
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

// MetadataFromRDSProxy creates AWS metadata from the provided RDS Proxy.
func MetadataFromRDSProxy(rdsProxy *rds.DBProxy) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(rdsProxy.DBProxyArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// rds.DBProxy has no resource ID attribute. The resource ID can be found
	// in the ARN, e.g.:
	//
	// arn:aws:rds:ca-central-1:123456789012:db-proxy:prx-xxxyyyzzz
	//
	// In this example, the arn.Resource is "db-proxy:prx-xxxyyyzzz", where the
	// resource type is "db-proxy" and the resource ID is "prx-xxxyyyzzz".
	_, resourceID, ok := strings.Cut(parsedARN.Resource, ":")
	if !ok {
		return nil, trace.BadParameter("failed to find resource ID from %v", aws.StringValue(rdsProxy.DBProxyArn))
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RDSProxy: types.RDSProxy{
			Name:       aws.StringValue(rdsProxy.DBProxyName),
			ResourceID: resourceID,
		},
	}, nil
}

// MetadataFromRDSProxyCustomEndpoint creates AWS metadata from the provided
// RDS Proxy custom endpoint.
func MetadataFromRDSProxyCustomEndpoint(rdsProxy *rds.DBProxy, customEndpoint *rds.DBProxyEndpoint) (*types.AWS, error) {
	// Using resource ID from the default proxy for IAM policies to gain the
	// RDS connection access.
	metadata, err := MetadataFromRDSProxy(rdsProxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metadata.RDSProxy.CustomEndpointName = aws.StringValue(customEndpoint.DBProxyEndpointName)
	return metadata, nil
}

// MetadataFromRedshiftCluster creates AWS metadata from the provided Redshift cluster.
func MetadataFromRedshiftCluster(cluster *redshift.Cluster) (*types.AWS, error) {
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

// MetadataFromElastiCacheCluster creates AWS metadata for the provided
// ElastiCache cluster.
func MetadataFromElastiCacheCluster(cluster *elasticache.ReplicationGroup, endpointType string) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(cluster.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// aws.StringValueSlice will return an empty slice is the input slice
	// is empty, but when cloning protobuf messages a cloned empty slice
	// will return nil. Keep this behavior so tests comparing cloned
	// messages don't fail.
	var userGroupIDs []string
	if len(cluster.UserGroupIds) != 0 {
		userGroupIDs = aws.StringValueSlice(cluster.UserGroupIds)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		ElastiCache: types.ElastiCache{
			ReplicationGroupID:       aws.StringValue(cluster.ReplicationGroupId),
			UserGroupIDs:             userGroupIDs,
			TransitEncryptionEnabled: aws.BoolValue(cluster.TransitEncryptionEnabled),
			EndpointType:             endpointType,
		},
	}, nil
}

// MetadataFromOpenSearchDomain creates AWS metadata for the provided OpenSearch domain.
func MetadataFromOpenSearchDomain(domain *opensearchservice.DomainStatus, endpointType string) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(domain.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		OpenSearch: types.OpenSearch{
			DomainName:   aws.StringValue(domain.DomainName),
			DomainID:     aws.StringValue(domain.DomainId),
			EndpointType: endpointType,
		},
	}, nil
}

// MetadataFromMemoryDBCluster creates AWS metadata for the provided MemoryDB
// cluster.
func MetadataFromMemoryDBCluster(cluster *memorydb.Cluster, endpointType string) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(cluster.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		MemoryDB: types.MemoryDB{
			ClusterName:  aws.StringValue(cluster.Name),
			ACLName:      aws.StringValue(cluster.ACLName),
			TLSEnabled:   aws.BoolValue(cluster.TLSEnabled),
			EndpointType: endpointType,
		},
	}, nil
}

// MetadataFromRedshiftServerlessWorkgroup creates AWS metadata for the
// provided Redshift Serverless Workgroup.
func MetadataFromRedshiftServerlessWorkgroup(workgroup *redshiftserverless.Workgroup) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(workgroup.WorkgroupArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RedshiftServerless: types.RedshiftServerless{
			WorkgroupName: aws.StringValue(workgroup.WorkgroupName),
			WorkgroupID:   aws.StringValue(workgroup.WorkgroupId),
		},
	}, nil
}

// MetadataFromRedshiftServerlessVPCEndpoint creates AWS metadata for the
// provided Redshift Serverless VPC endpoint.
func MetadataFromRedshiftServerlessVPCEndpoint(endpoint *redshiftserverless.EndpointAccess, workgroup *redshiftserverless.Workgroup) (*types.AWS, error) {
	parsedARN, err := arn.Parse(aws.StringValue(endpoint.EndpointArn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.AWS{
		Region:    parsedARN.Region,
		AccountID: parsedARN.AccountID,
		RedshiftServerless: types.RedshiftServerless{
			WorkgroupName: aws.StringValue(endpoint.WorkgroupName),
			EndpointName:  aws.StringValue(endpoint.EndpointName),
			WorkgroupID:   aws.StringValue(workgroup.WorkgroupId),
		},
	}, nil
}

// ExtraElastiCacheLabels returns a list of extra labels for provided
// ElastiCache cluster.
func ExtraElastiCacheLabels(cluster *elasticache.ReplicationGroup, tags []*elasticache.Tag, allNodes []*elasticache.CacheCluster, allSubnetGroups []*elasticache.CacheSubnetGroup) map[string]string {
	replicationGroupID := aws.StringValue(cluster.ReplicationGroupId)
	subnetGroupName := ""
	labels := make(map[string]string)

	// Find any node belongs to this cluster and set engine version label.
	for _, node := range allNodes {
		if aws.StringValue(node.ReplicationGroupId) == replicationGroupID {
			subnetGroupName = aws.StringValue(node.CacheSubnetGroupName)
			labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(node.EngineVersion)
			break
		}
	}

	// Find the subnet group used by this cluster and set VPC ID label.
	//
	// ElastiCache servers do not have public IPs so they are usually only
	// accessible within the same VPC. Having a VPC ID label can be very useful
	// for filtering.
	for _, subnetGroup := range allSubnetGroups {
		if aws.StringValue(subnetGroup.CacheSubnetGroupName) == subnetGroupName {
			labels[types.DiscoveryLabelVPCID] = aws.StringValue(subnetGroup.VpcId)
			break
		}
	}

	// Add AWS resource tags.
	return addLabels(labels, libcloudaws.TagsToLabels(tags))
}

// ExtraMemoryDBLabels returns a list of extra labels for provided MemoryDB
// cluster.
func ExtraMemoryDBLabels(cluster *memorydb.Cluster, tags []*memorydb.Tag, allSubnetGroups []*memorydb.SubnetGroup) map[string]string {
	labels := make(map[string]string)

	// Engine version.
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(cluster.EngineVersion)

	// VPC ID.
	for _, subnetGroup := range allSubnetGroups {
		if aws.StringValue(subnetGroup.Name) == aws.StringValue(cluster.SubnetGroupName) {
			labels[types.DiscoveryLabelVPCID] = aws.StringValue(subnetGroup.VpcId)
			break
		}
	}

	// Add AWS resource tags.
	return addLabels(labels, libcloudaws.TagsToLabels(tags))
}

// rdsEngineToProtocol converts RDS instance engine to the database protocol.
func rdsEngineToProtocol(engine string) (string, error) {
	switch engine {
	case services.RDSEnginePostgres, services.RDSEngineAuroraPostgres:
		return defaults.ProtocolPostgres, nil
	case services.RDSEngineMySQL, services.RDSEngineAurora, services.RDSEngineAuroraMySQL, services.RDSEngineMariaDB:
		return defaults.ProtocolMySQL, nil
	}
	return "", trace.BadParameter("unknown RDS engine type %q", engine)
}

// rdsEngineFamilyToProtocolAndPort converts RDS engine family to the database protocol and port.
func rdsEngineFamilyToProtocolAndPort(engineFamily string) (string, int, error) {
	switch engineFamily {
	case rds.EngineFamilyMysql:
		return defaults.ProtocolMySQL, services.RDSProxyMySQLPort, nil
	case rds.EngineFamilyPostgresql:
		return defaults.ProtocolPostgres, services.RDSProxyPostgresPort, nil
	case rds.EngineFamilySqlserver:
		return defaults.ProtocolSQLServer, services.RDSProxySQLServerPort, nil
	}
	return "", 0, trace.BadParameter("unknown RDS engine family type %q", engineFamily)
}

// labelsFromAzureServer creates database labels for the provided Azure DB server.
func labelsFromAzureServer(server *azure.DBServer) (map[string]string, error) {
	labels := azureTagsToLabels(server.Tags)
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(server.Location)
	labels[types.DiscoveryLabelEngineVersion] = server.Properties.Version
	return withLabelsFromAzureResourceID(labels, server.ID)
}

// withLabelsFromAzureResourceID adds labels extracted from the Azure resource ID.
func withLabelsFromAzureResourceID(labels map[string]string, resourceID string) (map[string]string, error) {
	rid, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labels[types.DiscoveryLabelEngine] = rid.ResourceType.String()
	labels[types.DiscoveryLabelAzureResourceGroup] = rid.ResourceGroupName
	labels[types.DiscoveryLabelAzureSubscriptionID] = rid.SubscriptionID
	return labels, nil
}

// labelsFromAzureRedis creates database labels from the provided Azure Redis instance.
func labelsFromAzureRedis(server *armredis.ResourceInfo) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(server.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(server.Location))
	labels[types.DiscoveryLabelEngineVersion] = azure.StringVal(server.Properties.RedisVersion)
	return withLabelsFromAzureResourceID(labels, azure.StringVal(server.ID))
}

// labelsFromAzureRedisEnterprise creates database labels from the provided Azure Redis Enterprise server.
func labelsFromAzureRedisEnterprise(cluster *armredisenterprise.Cluster, database *armredisenterprise.Database) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(cluster.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(cluster.Location))
	labels[types.DiscoveryLabelEngineVersion] = azure.StringVal(cluster.Properties.RedisVersion)
	labels[types.DiscoveryLabelEndpointType] = azure.StringVal(database.Properties.ClusteringPolicy)
	return withLabelsFromAzureResourceID(labels, azure.StringVal(cluster.ID))
}

// labelsFromAzureSQLServer creates database labels from the provided Azure SQL
// server.
func labelsFromAzureSQLServer(server *armsql.Server) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(server.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(server.Location))
	labels[types.DiscoveryLabelEngineVersion] = azure.StringVal(server.Properties.Version)
	return withLabelsFromAzureResourceID(labels, azure.StringVal(server.ID))
}

// labelsFromAzureManagedSQLServer creates database labels from the provided
// Azure Managed SQL server.
func labelsFromAzureManagedSQLServer(server *armsql.ManagedInstance) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(server.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(server.Location))
	return withLabelsFromAzureResourceID(labels, azure.StringVal(server.ID))
}

// labelsFromAzureMySQLFlexServer creates database labels for the provided Azure MySQL flex server.
func labelsFromAzureMySQLFlexServer(server *armmysqlflexibleservers.Server) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(server.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(server.Location))
	labels[types.DiscoveryLabelEngineVersion] = azure.StringVal(server.Properties.Version)

	role := azure.StringVal(server.Properties.ReplicationRole)
	switch armmysqlflexibleservers.ReplicationRole(role) {
	case armmysqlflexibleservers.ReplicationRoleNone:
		// don't add a label if this server has 'None' replication.
	case armmysqlflexibleservers.ReplicationRoleSource:
		labels[types.DiscoveryLabelAzureReplicationRole] = role
	case armmysqlflexibleservers.ReplicationRoleReplica:
		labels[types.DiscoveryLabelAzureReplicationRole] = role
		ssrid, err := arm.ParseResourceID(azure.StringVal(server.Properties.SourceServerResourceID))
		if err != nil {
			log.WithError(err).Debugf("Skipping malformed %q label for Azure MySQL Flexible server replica.", types.DiscoveryLabelAzureSourceServer)
		} else {
			labels[types.DiscoveryLabelAzureSourceServer] = ssrid.Name
		}
	}
	return withLabelsFromAzureResourceID(labels, azure.StringVal(server.ID))
}

// labelsFromAzurePostgresFlexServer creates database labels for the provided Azure postgres flex server.
func labelsFromAzurePostgresFlexServer(server *armpostgresqlflexibleservers.Server) (map[string]string, error) {
	labels := azureTagsToLabels(azure.ConvertTags(server.Tags))
	labels[types.DiscoveryLabelRegion] = azureutils.NormalizeLocation(azure.StringVal(server.Location))
	labels[types.DiscoveryLabelEngineVersion] = azure.StringVal(server.Properties.Version)
	return withLabelsFromAzureResourceID(labels, azure.StringVal(server.ID))
}

// labelsFromRDSInstance creates database labels for the provided RDS instance.
func labelsFromRDSInstance(rdsInstance *rds.DBInstance, meta *types.AWS) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEngine] = aws.StringValue(rdsInstance.Engine)
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(rdsInstance.EngineVersion)
	labels[types.DiscoveryLabelEndpointType] = apiawsutils.RDSEndpointTypeInstance
	if rdsInstance.DBSubnetGroup != nil {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(rdsInstance.DBSubnetGroup.VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(rdsInstance.TagList))
}

// labelsFromRDSCluster creates database labels for the provided RDS cluster.
func labelsFromRDSCluster(rdsCluster *rds.DBCluster, meta *types.AWS, endpointType string, memberInstances []*rds.DBInstance) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEngine] = aws.StringValue(rdsCluster.Engine)
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(rdsCluster.EngineVersion)
	labels[types.DiscoveryLabelEndpointType] = endpointType
	if len(memberInstances) > 0 && memberInstances[0].DBSubnetGroup != nil {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(memberInstances[0].DBSubnetGroup.VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(rdsCluster.TagList))
}

// labelsFromRDSProxy creates database labels for the provided RDS Proxy.
func labelsFromRDSProxy(rdsProxy *rds.DBProxy, meta *types.AWS, tags []*rds.Tag) map[string]string {
	// rds.DBProxy has no TagList.
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelVPCID] = aws.StringValue(rdsProxy.VpcId)
	labels[types.DiscoveryLabelEngine] = aws.StringValue(rdsProxy.EngineFamily)
	return addLabels(labels, libcloudaws.TagsToLabels(tags))
}

// labelsFromRDSProxyCustomEndpoint creates database labels for the provided
// RDS Proxy custom endpoint.
func labelsFromRDSProxyCustomEndpoint(rdsProxy *rds.DBProxy, customEndpoint *rds.DBProxyEndpoint, meta *types.AWS, tags []*rds.Tag) map[string]string {
	labels := labelsFromRDSProxy(rdsProxy, meta, tags)
	labels[types.DiscoveryLabelEndpointType] = aws.StringValue(customEndpoint.TargetRole)
	return labels
}

// labelsFromRedshiftCluster creates database labels for the provided Redshift cluster.
func labelsFromRedshiftCluster(cluster *redshift.Cluster, meta *types.AWS) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	return addLabels(labels, libcloudaws.TagsToLabels(cluster.Tags))
}

func labelsFromRedshiftServerlessWorkgroup(workgroup *redshiftserverless.Workgroup, meta *types.AWS, tags []*redshiftserverless.Tag) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEndpointType] = services.RedshiftServerlessWorkgroupEndpoint
	labels[types.DiscoveryLabelNamespace] = aws.StringValue(workgroup.NamespaceName)
	if workgroup.Endpoint != nil && len(workgroup.Endpoint.VpcEndpoints) > 0 {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(workgroup.Endpoint.VpcEndpoints[0].VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(tags))
}

func labelsFromRedshiftServerlessVPCEndpoint(endpoint *redshiftserverless.EndpointAccess, workgroup *redshiftserverless.Workgroup, meta *types.AWS, tags []*redshiftserverless.Tag) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEndpointType] = services.RedshiftServerlessVPCEndpoint
	labels[types.DiscoveryLabelWorkgroup] = aws.StringValue(endpoint.WorkgroupName)
	labels[types.DiscoveryLabelNamespace] = aws.StringValue(workgroup.NamespaceName)
	if endpoint.VpcEndpoint != nil {
		labels[types.DiscoveryLabelVPCID] = aws.StringValue(endpoint.VpcEndpoint.VpcId)
	}
	return addLabels(labels, libcloudaws.TagsToLabels(tags))
}

// labelsFromAWSMetadata returns labels from provided AWS metadata.
func labelsFromAWSMetadata(meta *types.AWS) map[string]string {
	labels := make(map[string]string)
	if meta != nil {
		labels[types.DiscoveryLabelAccountID] = meta.AccountID
		labels[types.DiscoveryLabelRegion] = meta.Region
	}
	labels[types.CloudLabel] = types.CloudAWS
	return labels
}

func labelsFromOpenSearchDomain(domain *opensearchservice.DomainStatus, meta *types.AWS, endpointType string, tags []*opensearchservice.Tag) map[string]string {
	labels := labelsFromMetaAndEndpointType(meta, endpointType, libcloudaws.TagsToLabels(tags))
	labels[types.DiscoveryLabelEngineVersion] = aws.StringValue(domain.EngineVersion)
	return labels
}

// labelsFromMetaAndEndpointType creates database labels from provided AWS meta and endpoint type.
func labelsFromMetaAndEndpointType(meta *types.AWS, endpointType string, extraLabels map[string]string) map[string]string {
	labels := labelsFromAWSMetadata(meta)
	labels[types.DiscoveryLabelEndpointType] = endpointType
	return addLabels(labels, extraLabels)
}

// GetMySQLEngineVersion returns MySQL engine version from provided metadata labels.
// An empty string is returned if label doesn't exist.
func GetMySQLEngineVersion(labels map[string]string) string {
	engine, ok := labels[types.DiscoveryLabelEngine]
	if !ok {
		return ""
	}
	switch engine {
	case services.RDSEngineMySQL, services.AzureEngineMySQL, services.AzureEngineMySQLFlex:
	default:
		// unrecognized MySQL engine label
		return ""
	}

	version, ok := labels[types.DiscoveryLabelEngineVersion]
	if !ok {
		return ""
	}
	return version
}

// IsAzureFlexServer returns true if the database engine label matches the Azure PostgreSQL or MySQL Flex server engine name.
// Matching engines are "Microsoft.DBforMySQL/flexibleServers" or "Microsoft.DBforPostgreSQL/flexibleServers".
func IsAzureFlexServer(db types.Database) bool {
	if db.GetAzure().IsFlexiServer {
		return true
	}
	engine, ok := db.GetMetadata().Labels[types.DiscoveryLabelEngine]
	return ok && (engine == services.AzureEngineMySQLFlex || engine == services.AzureEnginePostgresFlex)
}

// MakeAzureDatabaseLoginUsername returns a user name appropriate for Azure database logins.
// Azure requires database login to be <user>@<server-name>,
// for example: alice@mysql-server-name.
// Flexible server is an exception to this format and returns the provided username unmodified.
func MakeAzureDatabaseLoginUsername(db types.Database, user string) string {
	// https://learn.microsoft.com/en-us/azure/mysql/flexible-server/how-to-azure-ad
	if IsAzureFlexServer(db) {
		return user
	}
	return fmt.Sprintf("%v@%v", user, db.GetAzure().Name)
}

const (
	// azureSQLServerDefaultPort is the default port for Azure SQL Server.
	azureSQLServerDefaultPort = 1433
)
