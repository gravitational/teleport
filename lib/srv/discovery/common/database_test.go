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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/aws/aws-sdk-go-v2/aws"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awstesthelpers"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// TestDatabaseFromAzureDBServer tests converting an Azure DB Server to a database resource.
func TestDatabaseFromAzureDBServer(t *testing.T) {
	subscription := "sub1"
	resourceGroup := "defaultRG"
	resourceType := "Microsoft.DBforMySQL/servers"
	name := "testdb"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		resourceGroup,
		resourceType,
		name,
	)

	server := azure.DBServer{
		ID:       id,
		Location: "eastus",
		Name:     name,
		Port:     "3306",
		Properties: azure.ServerProperties{
			FullyQualifiedDomainName: name + ".mysql" + azureutils.DatabaseEndpointSuffix,
			UserVisibleState:         string(armmysql.ServerStateReady),
			Version:                  string(armmysql.ServerVersionFive7),
		},
		Protocol: defaults.ProtocolMySQL,
		Tags: map[string]string{
			"foo": "bar",
		},
	}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "testdb",
		Description: "Azure MySQL server in eastus",
		Labels: map[string]string{
			types.DiscoveryLabelRegion:              "eastus",
			types.DiscoveryLabelEngine:              "Microsoft.DBforMySQL/servers",
			types.DiscoveryLabelEngineVersion:       "5.7",
			types.DiscoveryLabelAzureResourceGroup:  "defaultRG",
			types.CloudLabel:                        types.CloudAzure,
			types.DiscoveryLabelAzureSubscriptionID: "sub1",
			"foo":                                   "bar",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "testdb.mysql.database.azure.com:3306",
		Azure: types.Azure{
			Name:       "testdb",
			ResourceID: id,
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromAzureServer(&server)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromAzureRedis(t *testing.T) {
	subscription := "test-sub"
	name := "test-azure-redis"
	group := "test-group"
	region := "eastus"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/Redis/%v", subscription, group, name)
	resourceInfo := &armredis.ResourceInfo{
		Name:     to.Ptr(name),
		ID:       to.Ptr(id),
		Location: to.Ptr(region),
		Tags: map[string]*string{
			"foo": to.Ptr("bar"),
		},
		Properties: &armredis.Properties{
			HostName:          to.Ptr(fmt.Sprintf("%v.redis.cache.windows.net", name)),
			SSLPort:           to.Ptr(int32(6380)),
			ProvisioningState: to.Ptr(armredis.ProvisioningStateSucceeded),
			RedisVersion:      to.Ptr("6.0"),
		},
	}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        name,
		Description: "Azure Redis server in eastus",
		Labels: map[string]string{
			types.DiscoveryLabelRegion:              region,
			types.DiscoveryLabelEngine:              "Microsoft.Cache/Redis",
			types.DiscoveryLabelEngineVersion:       "6.0",
			types.DiscoveryLabelAzureResourceGroup:  group,
			types.CloudLabel:                        types.CloudAzure,
			types.DiscoveryLabelAzureSubscriptionID: subscription,
			"foo":                                   "bar",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "test-azure-redis.redis.cache.windows.net:6380",
		Azure: types.Azure{
			Name:       "test-azure-redis",
			ResourceID: id,
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromAzureRedis(resourceInfo)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromAzureRedisEnterprise(t *testing.T) {
	subscription := "test-sub"
	name := "test-azure-redis"
	group := "test-group"
	region := "eastus"
	clusterID := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v", subscription, group, name)
	databaseID := fmt.Sprintf("%v/databases/default", clusterID)

	armCluster := &armredisenterprise.Cluster{
		Name:     to.Ptr(name),
		ID:       to.Ptr(clusterID),
		Location: to.Ptr(region),
		Tags: map[string]*string{
			"foo": to.Ptr("bar"),
		},
		Properties: &armredisenterprise.ClusterProperties{
			HostName:     to.Ptr(fmt.Sprintf("%v.%v.redisenterprise.cache.azure.net", name, region)),
			RedisVersion: to.Ptr("6.0"),
		},
	}
	armDatabase := &armredisenterprise.Database{
		Name: to.Ptr("default"),
		ID:   to.Ptr(databaseID),
		Properties: &armredisenterprise.DatabaseProperties{
			ProvisioningState: to.Ptr(armredisenterprise.ProvisioningStateSucceeded),
			Port:              to.Ptr(int32(10000)),
			ClusteringPolicy:  to.Ptr(armredisenterprise.ClusteringPolicyOSSCluster),
			ClientProtocol:    to.Ptr(armredisenterprise.ProtocolEncrypted),
		},
	}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        name,
		Description: "Azure Redis Enterprise server in eastus",
		Labels: map[string]string{
			types.DiscoveryLabelRegion:              region,
			types.DiscoveryLabelEngine:              "Microsoft.Cache/redisEnterprise",
			types.DiscoveryLabelEngineVersion:       "6.0",
			types.DiscoveryLabelAzureResourceGroup:  group,
			types.CloudLabel:                        types.CloudAzure,
			types.DiscoveryLabelAzureSubscriptionID: subscription,
			types.DiscoveryLabelEndpointType:        "OSSCluster",
			"foo":                                   "bar",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "test-azure-redis.eastus.redisenterprise.cache.azure.net:10000",
		Azure: types.Azure{
			Name:       "test-azure-redis",
			ResourceID: databaseID,
			Redis: types.AzureRedis{
				ClusteringPolicy: "OSSCluster",
			},
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromAzureRedisEnterprise(armCluster, armDatabase)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

// TestDatabaseFromRDSInstance tests converting an RDS instance to a database resource.
func TestDatabaseFromRDSInstance(t *testing.T) {
	instance := &rdstypes.DBInstance{
		DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:instance-1"),
		DBInstanceIdentifier:             aws.String("instance-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbiResourceId:                    aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(services.RDSEnginePostgres),
		EngineVersion:                    aws.String("13.0"),
		Endpoint: &rdstypes.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int32(5432),
		},
		TagList: []rdstypes.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
	}
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "instance-1",
		Description: "RDS instance in us-west-1",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:     "123456789012",
			types.CloudLabel:                  types.CloudAWS,
			types.DiscoveryLabelRegion:        "us-west-1",
			types.DiscoveryLabelEngine:        services.RDSEnginePostgres,
			types.DiscoveryLabelEngineVersion: "13.0",
			types.DiscoveryLabelEndpointType:  "instance",
			"key":                             "val",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-west-1",
			RDS: types.RDS{
				InstanceID: "instance-1",
				ClusterID:  "cluster-1",
				ResourceID: "resource-1",
				IAMAuth:    true,
			},
		},
	})
	require.NoError(t, err)
	actual, err := NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

// TestDatabaseFromRDSV2Instance tests converting an RDS instance (from aws sdk v2/rds) to a database resource.
func TestDatabaseFromRDSV2Instance(t *testing.T) {
	instance := &rdstypes.DBInstance{
		DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:instance-1"),
		DBInstanceIdentifier:             aws.String("instance-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DBInstanceStatus:                 aws.String("available"),
		DbiResourceId:                    aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(services.RDSEnginePostgres),
		EngineVersion:                    aws.String("13.0"),
		Endpoint: &rdstypes.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int32(5432),
		},
		TagList: []rdstypes.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
		DBSubnetGroup: &rdstypes.DBSubnetGroup{
			Subnets: []rdstypes.Subnet{
				{SubnetIdentifier: aws.String("")},
				{SubnetIdentifier: aws.String("subnet-1234567890abcdef0")},
				{SubnetIdentifier: aws.String("subnet-1234567890abcdef1")},
				{SubnetIdentifier: aws.String("subnet-1234567890abcdef2")},
			},
			VpcId: aws.String("vpc-asd"),
		},
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String("")},
			{VpcSecurityGroupId: aws.String("sg-1")},
			{VpcSecurityGroupId: aws.String("sg-2")},
		},
	}
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "instance-1",
		Description: "RDS instance in us-west-1",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:     "123456789012",
			types.CloudLabel:                  types.CloudAWS,
			types.DiscoveryLabelRegion:        "us-west-1",
			types.DiscoveryLabelEngine:        services.RDSEnginePostgres,
			types.DiscoveryLabelEngineVersion: "13.0",
			types.DiscoveryLabelEndpointType:  "instance",
			types.DiscoveryLabelStatus:        "available",
			types.DiscoveryLabelVPCID:         "vpc-asd",
			"key":                             "val",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-west-1",
			RDS: types.RDS{
				InstanceID: "instance-1",
				ClusterID:  "cluster-1",
				ResourceID: "resource-1",
				IAMAuth:    true,
				Subnets: []string{
					"subnet-1234567890abcdef0",
					"subnet-1234567890abcdef1",
					"subnet-1234567890abcdef2",
				},
				SecurityGroups: []string{
					"sg-1",
					"sg-2",
				},
				VPCID: "vpc-asd",
			},
		},
	})
	require.NoError(t, err)
	actual, err := NewDatabaseFromRDSV2Instance(instance)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))

	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		t.Run("with name override via "+overrideLabel, func(t *testing.T) {
			newName := "override-1"
			instance := instance
			instance.TagList = append(instance.TagList,
				rdstypes.Tag{
					Key:   aws.String(overrideLabel),
					Value: aws.String(newName),
				},
			)
			expected.Metadata.Name = newName

			actual, err := NewDatabaseFromRDSV2Instance(instance)
			require.NoError(t, err)
			require.Equal(t, actual.GetName(), newName)
		})
	}
}

// TestDatabaseFromRDSInstance tests converting an RDS instance to a database resource.
func TestDatabaseFromRDSInstanceNameOverride(t *testing.T) {
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		instance := &rdstypes.DBInstance{
			DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:instance-1"),
			DBInstanceIdentifier:             aws.String("instance-1"),
			DBClusterIdentifier:              aws.String("cluster-1"),
			DbiResourceId:                    aws.String("resource-1"),
			IAMDatabaseAuthenticationEnabled: aws.Bool(true),
			Engine:                           aws.String(services.RDSEnginePostgres),
			EngineVersion:                    aws.String("13.0"),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("localhost"),
				Port:    aws.Int32(5432),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("key"), Value: aws.String("val")},
				{Key: aws.String(overrideLabel), Value: aws.String("override-1")},
			},
		}
		t.Run("via "+overrideLabel, func(t *testing.T) {
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "override-1",
				Description: "RDS instance in us-west-1",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:     "123456789012",
					types.CloudLabel:                  types.CloudAWS,
					types.DiscoveryLabelRegion:        "us-west-1",
					types.DiscoveryLabelEngine:        services.RDSEnginePostgres,
					types.DiscoveryLabelEngineVersion: "13.0",
					types.DiscoveryLabelEndpointType:  "instance",
					overrideLabel:                     "override-1",
					"key":                             "val",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-west-1",
					RDS: types.RDS{
						InstanceID: "instance-1",
						ClusterID:  "cluster-1",
						ResourceID: "resource-1",
						IAMAuth:    true,
					},
				},
			})
			require.NoError(t, err)
			actual, err := NewDatabaseFromRDSInstance(instance)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})
	}
}

// TestDatabaseFromRDSCluster tests converting an RDS cluster to a database resource.
func TestDatabaseFromRDSCluster(t *testing.T) {
	vpcid := uuid.NewString()
	dbInstanceMembers := []rdstypes.DBInstance{{DBSubnetGroup: &rdstypes.DBSubnetGroup{VpcId: aws.String(vpcid)}}}
	cluster := &rdstypes.DBCluster{
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:cluster-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbClusterResourceId:              aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(services.RDSEngineAuroraMySQL),
		EngineVersion:                    aws.String("8.0.0"),
		Endpoint:                         aws.String("localhost"),
		ReaderEndpoint:                   aws.String("reader.host"),
		Port:                             aws.Int32(3306),
		CustomEndpoints: []string{
			"myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com",
			"myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com",
		},
		TagList: []rdstypes.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
	}

	expectedAWS := types.AWS{
		AccountID: "123456789012",
		Region:    "us-east-1",
		RDS: types.RDS{
			ClusterID:  "cluster-1",
			ResourceID: "resource-1",
			IAMAuth:    true,
		},
	}

	t.Run("primary", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1",
			Description: "Aurora cluster in us-east-1",
			Labels: map[string]string{
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
				types.DiscoveryLabelEndpointType:  "primary",
				types.DiscoveryLabelVPCID:         vpcid,
				"key":                             "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSCluster(cluster, dbInstanceMembers)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expected, actual))
	})

	t.Run("reader", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1-reader",
			Description: "Aurora cluster in us-east-1 (reader endpoint)",
			Labels: map[string]string{
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
				types.DiscoveryLabelEndpointType:  "reader",
				types.DiscoveryLabelVPCID:         vpcid,
				"key":                             "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "reader.host:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSClusterReaderEndpoint(cluster, dbInstanceMembers)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expected, actual))
	})

	t.Run("custom endpoints", func(t *testing.T) {
		expectedLabels := map[string]string{
			types.DiscoveryLabelAccountID:     "123456789012",
			types.CloudLabel:                  types.CloudAWS,
			types.DiscoveryLabelRegion:        "us-east-1",
			types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
			types.DiscoveryLabelEngineVersion: "8.0.0",
			types.DiscoveryLabelEndpointType:  "custom",
			types.DiscoveryLabelVPCID:         vpcid,
			"key":                             "val",
		}

		expectedMyEndpoint1, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1-custom-myendpoint1",
			Description: "Aurora cluster in us-east-1 (custom endpoint)",
			Labels:      expectedLabels,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com:3306",
			AWS:      expectedAWS,
			TLS: types.DatabaseTLS{
				ServerName: "localhost",
			},
		})
		require.NoError(t, err)

		expectedMyEndpoint2, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1-custom-myendpoint2",
			Description: "Aurora cluster in us-east-1 (custom endpoint)",
			Labels:      expectedLabels,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com:3306",
			AWS:      expectedAWS,
			TLS: types.DatabaseTLS{
				ServerName: "localhost",
			},
		})
		require.NoError(t, err)

		databases, err := NewDatabasesFromRDSClusterCustomEndpoints(cluster, dbInstanceMembers)
		require.NoError(t, err)
		require.Equal(t, types.Databases{expectedMyEndpoint1, expectedMyEndpoint2}, databases)
	})

	t.Run("bad custom endpoints ", func(t *testing.T) {
		badCluster := *cluster
		badCluster.CustomEndpoints = []string{
			"badendpoint1",
			"badendpoint2",
		}
		_, err := NewDatabasesFromRDSClusterCustomEndpoints(&badCluster, dbInstanceMembers)
		require.Error(t, err)
	})
}

// TestDatabaseFromRDSV2Cluster tests converting an RDS cluster to a database resource.
// It uses the V2 of the aws sdk.
func TestDatabaseFromRDSV2Cluster(t *testing.T) {
	cluster := &rdstypes.DBCluster{
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:cluster-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbClusterResourceId:              aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(services.RDSEngineAuroraMySQL),
		EngineVersion:                    aws.String("8.0.0"),
		Status:                           aws.String("available"),
		Endpoint:                         aws.String("localhost"),
		ReaderEndpoint:                   aws.String("reader.host"),
		Port:                             aws.Int32(3306),
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String("")},
			{VpcSecurityGroupId: aws.String("sg-1")},
			{VpcSecurityGroupId: aws.String("sg-2")},
		},
		CustomEndpoints: []string{
			"myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com",
			"myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com",
		},
		TagList: []rdstypes.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
	}

	expectedAWS := types.AWS{
		AccountID: "123456789012",
		Region:    "us-east-1",
		RDS: types.RDS{
			ClusterID:  "cluster-1",
			ResourceID: "resource-1",
			IAMAuth:    true,
			SecurityGroups: []string{
				"sg-1",
				"sg-2",
			},
		},
	}

	t.Run("primary", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1",
			Description: "Aurora cluster in us-east-1",
			Labels: map[string]string{
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
				types.DiscoveryLabelEndpointType:  "primary",
				types.DiscoveryLabelStatus:        "available",
				"key":                             "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSV2Cluster(cluster, nil)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expected, actual))

		for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
			t.Run("with name override via "+overrideLabel, func(t *testing.T) {
				newName := "override-1"

				cluster.TagList = append(cluster.TagList,
					rdstypes.Tag{
						Key:   aws.String(overrideLabel),
						Value: aws.String(newName),
					},
				)
				expected.Metadata.Name = newName

				actual, err := NewDatabaseFromRDSV2Cluster(cluster, nil)
				require.NoError(t, err)
				require.Equal(t, actual.GetName(), newName)
			})
		}
	})

	t.Run("DB Cluster uses network information from DB Instance when available", func(t *testing.T) {
		instance := &rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId: aws.String("vpc-123"),
				Subnets: []rdstypes.Subnet{
					{SubnetIdentifier: aws.String("subnet-123")},
					{SubnetIdentifier: aws.String("subnet-456")},
				},
			},
		}

		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "override-1",
			Description: "Aurora cluster in us-east-1",
			Labels: map[string]string{
				"TeleportDatabaseName":            "override-1",
				"teleport.dev/database_name":      "override-1",
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
				types.DiscoveryLabelEndpointType:  "primary",
				types.DiscoveryLabelStatus:        "available",
				types.DiscoveryLabelVPCID:         "vpc-123",
				"key":                             "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
			AWS: types.AWS{
				AccountID: "123456789012",
				Region:    "us-east-1",
				RDS: types.RDS{
					ClusterID:  "cluster-1",
					ResourceID: "resource-1",
					IAMAuth:    true,
					Subnets:    []string{"subnet-123", "subnet-456"},
					SecurityGroups: []string{
						"sg-1",
						"sg-2",
					},
					VPCID: "vpc-123",
				},
			},
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSV2Cluster(cluster, instance)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expected, actual), "NewDatabaseFromRDSV2Cluster diff (-want +got)")
	})
}

// TestDatabaseFromRDSClusterNameOverride tests converting an RDS cluster to a database resource with overridden name.
func TestDatabaseFromRDSClusterNameOverride(t *testing.T) {
	dbInstanceMembers := []rdstypes.DBInstance{{DBSubnetGroup: &rdstypes.DBSubnetGroup{VpcId: aws.String("vpc-123")}}}
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		cluster := &rdstypes.DBCluster{
			DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:cluster-1"),
			DBClusterIdentifier:              aws.String("cluster-1"),
			DbClusterResourceId:              aws.String("resource-1"),
			IAMDatabaseAuthenticationEnabled: aws.Bool(true),
			Engine:                           aws.String(services.RDSEngineAuroraMySQL),
			EngineVersion:                    aws.String("8.0.0"),
			Endpoint:                         aws.String("localhost"),
			ReaderEndpoint:                   aws.String("reader.host"),
			Port:                             aws.Int32(3306),
			CustomEndpoints: []string{
				"myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com",
				"myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com",
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("key"), Value: aws.String("val")},
				{Key: aws.String(overrideLabel), Value: aws.String("mycluster-2")},
			},
		}

		expectedAWS := types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			RDS: types.RDS{
				ClusterID:  "cluster-1",
				ResourceID: "resource-1",
				IAMAuth:    true,
			},
		}

		t.Run("primary", func(t *testing.T) {
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "mycluster-2",
				Description: "Aurora cluster in us-east-1",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:     "123456789012",
					types.CloudLabel:                  types.CloudAWS,
					types.DiscoveryLabelRegion:        "us-east-1",
					types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
					types.DiscoveryLabelEngineVersion: "8.0.0",
					types.DiscoveryLabelEndpointType:  "primary",
					types.DiscoveryLabelVPCID:         "vpc-123",
					overrideLabel:                     "mycluster-2",
					"key":                             "val",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				AWS:      expectedAWS,
			})
			require.NoError(t, err)
			actual, err := NewDatabaseFromRDSCluster(cluster, dbInstanceMembers)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})

		t.Run("reader", func(t *testing.T) {
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "mycluster-2-reader",
				Description: "Aurora cluster in us-east-1 (reader endpoint)",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:     "123456789012",
					types.CloudLabel:                  types.CloudAWS,
					types.DiscoveryLabelRegion:        "us-east-1",
					types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
					types.DiscoveryLabelEngineVersion: "8.0.0",
					types.DiscoveryLabelEndpointType:  "reader",
					types.DiscoveryLabelVPCID:         "vpc-123",
					overrideLabel:                     "mycluster-2",
					"key":                             "val",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      "reader.host:3306",
				AWS:      expectedAWS,
			})
			require.NoError(t, err)
			actual, err := NewDatabaseFromRDSClusterReaderEndpoint(cluster, dbInstanceMembers)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})

		t.Run("custom endpoints", func(t *testing.T) {
			expectedLabels := map[string]string{
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				types.DiscoveryLabelEngine:        services.RDSEngineAuroraMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
				types.DiscoveryLabelEndpointType:  "custom",
				types.DiscoveryLabelVPCID:         "vpc-123",
				overrideLabel:                     "mycluster-2",
				"key":                             "val",
			}

			expectedMyEndpoint1, err := types.NewDatabaseV3(types.Metadata{
				Name:        "mycluster-2-custom-myendpoint1",
				Description: "Aurora cluster in us-east-1 (custom endpoint)",
				Labels:      expectedLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      "myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com:3306",
				AWS:      expectedAWS,
				TLS: types.DatabaseTLS{
					ServerName: "localhost",
				},
			})
			require.NoError(t, err)

			expectedMyEndpoint2, err := types.NewDatabaseV3(types.Metadata{
				Name:        "mycluster-2-custom-myendpoint2",
				Description: "Aurora cluster in us-east-1 (custom endpoint)",
				Labels:      expectedLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      "myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com:3306",
				AWS:      expectedAWS,
				TLS: types.DatabaseTLS{
					ServerName: "localhost",
				},
			})
			require.NoError(t, err)

			databases, err := NewDatabasesFromRDSClusterCustomEndpoints(cluster, dbInstanceMembers)
			require.NoError(t, err)
			require.Equal(t, types.Databases{expectedMyEndpoint1, expectedMyEndpoint2}, databases)
		})

		t.Run("bad custom endpoints ", func(t *testing.T) {
			badCluster := *cluster
			badCluster.CustomEndpoints = []string{
				"badendpoint1",
				"badendpoint2",
			}
			_, err := NewDatabasesFromRDSClusterCustomEndpoints(&badCluster, dbInstanceMembers)
			require.Error(t, err)
		})
	}
}

func TestNewDatabasesFromDocumentDBCluster(t *testing.T) {
	inputLabels := map[string]string{
		"env": "test",
	}

	wantPrimaryDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-docdb",
		Description: "DocumentDB cluster in us-west-2",
		Labels: map[string]string{
			"account-id":         "123456789012",
			"endpoint-type":      "cluster",
			"engine":             "docdb",
			"engine-version":     "5.0.0",
			"env":                "test",
			"region":             "us-west-2",
			"teleport.dev/cloud": "AWS",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      "my-docdb.cluster-aabbccdd.us-west-2.docdb.amazonaws.com:27017",
		AWS: types.AWS{
			AccountID: "123456789012",
			DocumentDB: types.DocumentDB{
				ClusterID: "my-docdb",
			},
		},
	})
	require.NoError(t, err)

	wantReaderDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-docdb-reader",
		Description: "DocumentDB cluster in us-west-2 (reader endpoint)",
		Labels: map[string]string{
			"account-id":         "123456789012",
			"endpoint-type":      "reader",
			"engine":             "docdb",
			"engine-version":     "5.0.0",
			"env":                "test",
			"region":             "us-west-2",
			"teleport.dev/cloud": "AWS",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      "my-docdb.cluster-ro-aabbccdd.us-west-2.docdb.amazonaws.com:27017",
		AWS: types.AWS{
			AccountID: "123456789012",
			DocumentDB: types.DocumentDB{
				ClusterID: "my-docdb",
			},
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		inputCluster  *rdstypes.DBCluster
		wantDatabases types.Databases
	}{
		{
			name:          "primary-only",
			inputCluster:  mocks.DocumentDBCluster("my-docdb", "us-west-2", inputLabels),
			wantDatabases: types.Databases{wantPrimaryDatabase},
		},
		{
			name:          "with-reader",
			inputCluster:  mocks.DocumentDBCluster("my-docdb", "us-west-2", inputLabels, mocks.WithDocumentDBClusterReader),
			wantDatabases: types.Databases{wantPrimaryDatabase, wantReaderDatabase},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualDatabases, err := NewDatabasesFromDocumentDBCluster(test.inputCluster)
			require.NoError(t, err)
			require.Equal(t, test.wantDatabases, actualDatabases)
		})
	}
}

func TestDatabaseFromRDSProxy(t *testing.T) {
	tests := []struct {
		desc         string
		engineFamily string
		wantProtocol string
		wantPort     int
	}{
		{
			desc:         "mysql",
			engineFamily: string(rdstypes.EngineFamilyMysql),
			wantProtocol: "mysql",
			wantPort:     3306,
		},
		{
			desc:         "postgres",
			engineFamily: string(rdstypes.EngineFamilyPostgresql),
			wantProtocol: "postgres",
			wantPort:     5432,
		},
		{
			desc:         "sqlserver",
			engineFamily: string(rdstypes.EngineFamilySqlserver),
			wantProtocol: "sqlserver",
			wantPort:     1433,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			dbProxy := &rdstypes.DBProxy{
				DBProxyArn:   aws.String("arn:aws:rds:ca-central-1:123456789012:db-proxy:prx-abcdef"),
				DBProxyName:  aws.String("testproxy"),
				EngineFamily: aws.String(test.engineFamily),
				Endpoint:     aws.String("proxy.rds.test"),
				VpcId:        aws.String("test-vpc-id"),
			}

			dbProxyEndpoint := &rdstypes.DBProxyEndpoint{
				Endpoint:            aws.String("custom.proxy.rds.test"),
				DBProxyEndpointName: aws.String("custom"),
				DBProxyName:         aws.String("testproxy"),
				DBProxyEndpointArn:  aws.String("arn:aws:rds:ca-central-1:123456789012:db-proxy-endpoint:prx-endpoint-abcdef"),
				TargetRole:          rdstypes.DBProxyEndpointTargetRoleReadOnly,
			}

			tags := []rdstypes.Tag{{
				Key:   aws.String("key"),
				Value: aws.String("val"),
			}}

			t.Run("default endpoint", func(t *testing.T) {
				expected, err := types.NewDatabaseV3(types.Metadata{
					Name:        "testproxy",
					Description: "RDS Proxy in ca-central-1",
					Labels: map[string]string{
						"key":                         "val",
						types.DiscoveryLabelAccountID: "123456789012",
						types.CloudLabel:              types.CloudAWS,
						types.DiscoveryLabelRegion:    "ca-central-1",
						types.DiscoveryLabelEngine:    test.engineFamily,
						types.DiscoveryLabelVPCID:     "test-vpc-id",
					},
				}, types.DatabaseSpecV3{
					Protocol: test.wantProtocol,
					URI:      fmt.Sprintf("proxy.rds.test:%d", test.wantPort),
					AWS: types.AWS{
						Region:    "ca-central-1",
						AccountID: "123456789012",
						RDSProxy: types.RDSProxy{
							ResourceID: "prx-abcdef",
							Name:       "testproxy",
						},
					},
				})
				require.NoError(t, err)

				actual, err := NewDatabaseFromRDSProxy(dbProxy, tags)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expected, actual))
			})

			t.Run("custom endpoint", func(t *testing.T) {
				expected, err := types.NewDatabaseV3(types.Metadata{
					Name:        "testproxy-custom",
					Description: "RDS Proxy endpoint in ca-central-1",
					Labels: map[string]string{
						"key":                            "val",
						types.DiscoveryLabelAccountID:    "123456789012",
						types.CloudLabel:                 types.CloudAWS,
						types.DiscoveryLabelRegion:       "ca-central-1",
						types.DiscoveryLabelEngine:       test.engineFamily,
						types.DiscoveryLabelVPCID:        "test-vpc-id",
						types.DiscoveryLabelEndpointType: "READ_ONLY",
					},
				}, types.DatabaseSpecV3{
					Protocol: test.wantProtocol,
					URI:      fmt.Sprintf("custom.proxy.rds.test:%d", test.wantPort),
					AWS: types.AWS{
						Region:    "ca-central-1",
						AccountID: "123456789012",
						RDSProxy: types.RDSProxy{
							ResourceID:         "prx-abcdef",
							Name:               "testproxy",
							CustomEndpointName: "custom",
						},
					},
					TLS: types.DatabaseTLS{
						ServerName: "proxy.rds.test",
					},
				})
				require.NoError(t, err)

				actual, err := NewDatabaseFromRDSProxyCustomEndpoint(dbProxy, dbProxyEndpoint, tags)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expected, actual))
			})
		})
	}
}

func TestAuroraMySQLVersion(t *testing.T) {
	tests := []struct {
		engineVersion        string
		expectedMySQLVersion string
	}{
		{
			engineVersion:        "5.6.10a",
			expectedMySQLVersion: "5.6.10a",
		},
		{
			engineVersion:        "5.6.mysql_aurora.1.22.1",
			expectedMySQLVersion: "1.22.1",
		},
		{
			engineVersion:        "5.6.mysql_aurora.1.22.1.3",
			expectedMySQLVersion: "1.22.1.3",
		},
	}
	for _, test := range tests {
		t.Run(test.engineVersion, func(t *testing.T) {
			require.Equal(t, test.expectedMySQLVersion, libcloudaws.AuroraMySQLVersion(&rdstypes.DBCluster{EngineVersion: aws.String(test.engineVersion)}))
		})
	}
}

func TestIsRDSClusterSupported(t *testing.T) {
	tests := []struct {
		name          string
		engineMode    string
		engineVersion string
		isSupported   bool
	}{
		{
			name:          "provisioned",
			engineMode:    services.RDSEngineModeProvisioned,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   true,
		},
		{
			name:          "serverless",
			engineMode:    services.RDSEngineModeServerless,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   false,
		},
		{
			name:          "parallel query supported",
			engineMode:    services.RDSEngineModeParallelQuery,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   true,
		},
		{
			name:          "parallel query unsupported",
			engineMode:    services.RDSEngineModeParallelQuery,
			engineVersion: "5.6.mysql_aurora.1.19.6",
			isSupported:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := &rdstypes.DBCluster{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:test"),
				DBClusterIdentifier: aws.String(test.name),
				DbClusterResourceId: aws.String(uuid.New().String()),
				Engine:              aws.String(services.RDSEngineAuroraMySQL),
				EngineMode:          aws.String(test.engineMode),
				EngineVersion:       aws.String(test.engineVersion),
			}

			got, want := libcloudaws.IsRDSClusterSupported(cluster), test.isSupported
			require.Equal(t, want, got, "IsRDSClusterSupported = %v, want = %v", got, want)
		})
	}
}

func TestIsRDSInstanceSupported(t *testing.T) {
	tests := []struct {
		name          string
		engine        string
		engineVersion string
		isSupported   bool
	}{
		{
			name:          "non-MariaDB engine",
			engine:        services.RDSEnginePostgres,
			engineVersion: "13.3",
			isSupported:   true,
		},
		{
			name:          "unsupported MariaDB",
			engine:        services.RDSEngineMariaDB,
			engineVersion: "10.3.28",
			isSupported:   false,
		},
		{
			name:          "min supported version",
			engine:        services.RDSEngineMariaDB,
			engineVersion: "10.6.2",
			isSupported:   true,
		},
		{
			name:          "supported version",
			engine:        services.RDSEngineMariaDB,
			engineVersion: "10.8.0",
			isSupported:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := &rdstypes.DBInstance{
				DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:instance:test"),
				DBClusterIdentifier: aws.String(test.name),
				DbiResourceId:       aws.String(uuid.New().String()),
				Engine:              aws.String(test.engine),
				EngineVersion:       aws.String(test.engineVersion),
			}

			got, want := libcloudaws.IsRDSInstanceSupported(cluster), test.isSupported
			require.Equal(t, want, got, "IsRDSInstanceSupported = %v, want = %v", got, want)
		})
	}
}

func TestAzureTagsToLabels(t *testing.T) {
	azureTags := map[string]string{
		"Env":     "dev",
		"foo:bar": "some-id",
		"Name":    "test",
	}
	labels := azureTagsToLabels(azureTags)
	wantLabels := map[string]string{
		"Name":           "test",
		"Env":            "dev",
		"foo:bar":        "some-id",
		types.CloudLabel: types.CloudAzure,
	}
	require.Equal(t, wantLabels, labels)
}

// TestDatabaseFromRedshiftCluster tests converting an Redshift cluster to a database resource.
func TestDatabaseFromRedshiftCluster(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cluster := &redshifttypes.Cluster{
			ClusterIdentifier:   aws.String("mycluster"),
			ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:u-u-i-d"),
			Endpoint: &redshifttypes.Endpoint{
				Address: aws.String("localhost"),
				Port:    aws.Int32(5439),
			},
			Tags: []redshifttypes.Tag{
				{
					Key:   aws.String("key"),
					Value: aws.String("val"),
				},
				{
					Key:   aws.String("elasticbeanstalk:environment-id"),
					Value: aws.String("id"),
				},
			},
		}
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "mycluster",
			Description: "Redshift cluster in us-east-1",
			Labels: map[string]string{
				types.DiscoveryLabelAccountID:     "123456789012",
				types.CloudLabel:                  types.CloudAWS,
				types.DiscoveryLabelRegion:        "us-east-1",
				"key":                             "val",
				"elasticbeanstalk:environment-id": "id",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5439",
			AWS: types.AWS{
				AccountID: "123456789012",
				Region:    "us-east-1",
				Redshift: types.Redshift{
					ClusterID: "mycluster",
				},
			},
		})

		require.NoError(t, err)

		actual, err := NewDatabaseFromRedshiftCluster(cluster)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expected, actual))
	})

	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		t.Run("success with name override via"+overrideLabel, func(t *testing.T) {
			cluster := &redshifttypes.Cluster{
				ClusterIdentifier:   aws.String("mycluster"),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:u-u-i-d"),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("localhost"),
					Port:    aws.Int32(5439),
				},
				Tags: []redshifttypes.Tag{
					{
						Key:   aws.String("key"),
						Value: aws.String("val"),
					},
					{
						Key:   aws.String("elasticbeanstalk:environment-id"),
						Value: aws.String("id"),
					},
					{
						Key:   aws.String(overrideLabel),
						Value: aws.String("mycluster-override-2"),
					},
				},
			}
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "mycluster-override-2",
				Description: "Redshift cluster in us-east-1",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:     "123456789012",
					types.CloudLabel:                  types.CloudAWS,
					types.DiscoveryLabelRegion:        "us-east-1",
					overrideLabel:                     "mycluster-override-2",
					"key":                             "val",
					"elasticbeanstalk:environment-id": "id",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5439",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-east-1",
					Redshift: types.Redshift{
						ClusterID: "mycluster",
					},
				},
			})

			require.NoError(t, err)

			actual, err := NewDatabaseFromRedshiftCluster(cluster)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})

		t.Run("missing endpoint", func(t *testing.T) {
			_, err := NewDatabaseFromRedshiftCluster(&redshifttypes.Cluster{
				ClusterIdentifier: aws.String("still-creating"),
			})
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err), "Expected trace.BadParameter, got %v", err)
		})
	}
}

func TestDatabaseFromElastiCacheConfigurationEndpoint(t *testing.T) {
	cluster := &ectypes.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(true),
		ConfigurationEndpoint: &ectypes.Endpoint{
			Address: aws.String("configuration.localhost"),
			Port:    aws.Int32(6379),
		},
		UserGroupIds: []string{"my-user-group"},
		NodeGroups: []ectypes.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				NodeGroupMembers: []ectypes.NodeGroupMember{
					{
						CacheClusterId: aws.String("my-cluster-0001-001"),
					},
					{
						CacheClusterId: aws.String("my-cluster-0001-002"),
					},
				},
			},
			{
				NodeGroupId: aws.String("0002"),
				NodeGroupMembers: []ectypes.NodeGroupMember{
					{
						CacheClusterId: aws.String("my-cluster-0002-001"),
					},
					{
						CacheClusterId: aws.String("my-cluster-0002-002"),
					},
				},
			},
		},
	}
	extraLabels := map[string]string{"key": "value"}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster",
		Description: "ElastiCache cluster in us-east-1 (configuration endpoint)",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "us-east-1",
			types.DiscoveryLabelEndpointType: "configuration",
			"key":                            "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "configuration.localhost:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID:       "my-cluster",
				UserGroupIDs:             []string{"my-user-group"},
				TransitEncryptionEnabled: true,
				EndpointType:             awsutils.ElastiCacheConfigurationEndpoint,
			},
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromElastiCacheConfigurationEndpointNameOverride(t *testing.T) {
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		t.Run("via "+overrideLabel, func(t *testing.T) {
			cluster := &ectypes.ReplicationGroup{
				ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
				ReplicationGroupId:       aws.String("my-cluster"),
				Status:                   aws.String("available"),
				TransitEncryptionEnabled: aws.Bool(true),
				ClusterEnabled:           aws.Bool(true),
				ConfigurationEndpoint: &ectypes.Endpoint{
					Address: aws.String("configuration.localhost"),
					Port:    aws.Int32(6379),
				},
				UserGroupIds: []string{"my-user-group"},
				NodeGroups: []ectypes.NodeGroup{
					{
						NodeGroupId: aws.String("0001"),
						NodeGroupMembers: []ectypes.NodeGroupMember{
							{
								CacheClusterId: aws.String("my-cluster-0001-001"),
							},
							{
								CacheClusterId: aws.String("my-cluster-0001-002"),
							},
						},
					},
					{
						NodeGroupId: aws.String("0002"),
						NodeGroupMembers: []ectypes.NodeGroupMember{
							{
								CacheClusterId: aws.String("my-cluster-0002-001"),
							},
							{
								CacheClusterId: aws.String("my-cluster-0002-002"),
							},
						},
					},
				},
			}
			extraLabels := map[string]string{
				overrideLabel: "my-override-cluster-2",
				"key":         "value",
			}

			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "my-override-cluster-2",
				Description: "ElastiCache cluster in us-east-1 (configuration endpoint)",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:    "123456789012",
					types.CloudLabel:                 types.CloudAWS,
					types.DiscoveryLabelRegion:       "us-east-1",
					types.DiscoveryLabelEndpointType: "configuration",
					overrideLabel:                    "my-override-cluster-2",
					"key":                            "value",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "configuration.localhost:6379",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-east-1",
					ElastiCache: types.ElastiCache{
						ReplicationGroupID:       "my-cluster",
						UserGroupIDs:             []string{"my-user-group"},
						TransitEncryptionEnabled: true,
						EndpointType:             awsutils.ElastiCacheConfigurationEndpoint,
					},
				},
			})
			require.NoError(t, err)

			actual, err := NewDatabaseFromElastiCacheConfigurationEndpoint(cluster, extraLabels)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})
	}
}

func TestDatabaseFromElastiCacheNodeGroups(t *testing.T) {
	cluster := &ectypes.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(false),
		UserGroupIds:             []string{"my-user-group"},
		NodeGroups: []ectypes.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				PrimaryEndpoint: &ectypes.Endpoint{
					Address: aws.String("primary.localhost"),
					Port:    aws.Int32(6379),
				},
				ReaderEndpoint: &ectypes.Endpoint{
					Address: aws.String("reader.localhost"),
					Port:    aws.Int32(6379),
				},
			},
		},
	}
	extraLabels := map[string]string{"key": "value"}

	expectedPrimary, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster",
		Description: "ElastiCache cluster in us-east-1 (primary endpoint)",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "us-east-1",
			types.DiscoveryLabelEndpointType: "primary",
			"key":                            "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "primary.localhost:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID:       "my-cluster",
				UserGroupIDs:             []string{"my-user-group"},
				TransitEncryptionEnabled: true,
				EndpointType:             awsutils.ElastiCachePrimaryEndpoint,
			},
		},
	})
	require.NoError(t, err)

	expectedReader, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster-reader",
		Description: "ElastiCache cluster in us-east-1 (reader endpoint)",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "us-east-1",
			types.DiscoveryLabelEndpointType: "reader",
			"key":                            "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "reader.localhost:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID:       "my-cluster",
				UserGroupIDs:             []string{"my-user-group"},
				TransitEncryptionEnabled: true,
				EndpointType:             awsutils.ElastiCacheReaderEndpoint,
			},
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
	require.NoError(t, err)
	require.Equal(t, types.Databases{expectedPrimary, expectedReader}, actual)
}

func TestDatabaseFromElastiCacheNodeGroupsNameOverride(t *testing.T) {
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		t.Run("via "+overrideLabel, func(t *testing.T) {
			cluster := &ectypes.ReplicationGroup{
				ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
				ReplicationGroupId:       aws.String("my-cluster"),
				Status:                   aws.String("available"),
				TransitEncryptionEnabled: aws.Bool(true),
				ClusterEnabled:           aws.Bool(false),
				UserGroupIds:             []string{"my-user-group"},
				NodeGroups: []ectypes.NodeGroup{
					{
						NodeGroupId: aws.String("0001"),
						PrimaryEndpoint: &ectypes.Endpoint{
							Address: aws.String("primary.localhost"),
							Port:    aws.Int32(6379),
						},
						ReaderEndpoint: &ectypes.Endpoint{
							Address: aws.String("reader.localhost"),
							Port:    aws.Int32(6379),
						},
					},
				},
			}
			extraLabels := map[string]string{
				overrideLabel: "my-override-cluster-2",
				"key":         "value",
			}

			expectedPrimary, err := types.NewDatabaseV3(types.Metadata{
				Name:        "my-override-cluster-2",
				Description: "ElastiCache cluster in us-east-1 (primary endpoint)",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:    "123456789012",
					types.CloudLabel:                 types.CloudAWS,
					types.DiscoveryLabelRegion:       "us-east-1",
					types.DiscoveryLabelEndpointType: "primary",
					overrideLabel:                    "my-override-cluster-2",
					"key":                            "value",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "primary.localhost:6379",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-east-1",
					ElastiCache: types.ElastiCache{
						ReplicationGroupID:       "my-cluster",
						UserGroupIDs:             []string{"my-user-group"},
						TransitEncryptionEnabled: true,
						EndpointType:             awsutils.ElastiCachePrimaryEndpoint,
					},
				},
			})
			require.NoError(t, err)

			expectedReader, err := types.NewDatabaseV3(types.Metadata{
				Name:        "my-override-cluster-2-reader",
				Description: "ElastiCache cluster in us-east-1 (reader endpoint)",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:    "123456789012",
					types.CloudLabel:                 types.CloudAWS,
					types.DiscoveryLabelRegion:       "us-east-1",
					types.DiscoveryLabelEndpointType: "reader",
					overrideLabel:                    "my-override-cluster-2",
					"key":                            "value",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "reader.localhost:6379",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-east-1",
					ElastiCache: types.ElastiCache{
						ReplicationGroupID:       "my-cluster",
						UserGroupIDs:             []string{"my-user-group"},
						TransitEncryptionEnabled: true,
						EndpointType:             awsutils.ElastiCacheReaderEndpoint,
					},
				},
			})
			require.NoError(t, err)

			actual, err := NewDatabasesFromElastiCacheNodeGroups(cluster, extraLabels)
			require.NoError(t, err)
			require.Equal(t, types.Databases{expectedPrimary, expectedReader}, actual)
		})
	}
}

func TestDatabaseFromMemoryDBCluster(t *testing.T) {
	cluster := &memorydbtypes.Cluster{
		ARN:        aws.String("arn:aws:memorydb:us-east-1:123456789012:cluster:my-cluster"),
		Name:       aws.String("my-cluster"),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ACLName:    aws.String("my-user-group"),
		ClusterEndpoint: &memorydbtypes.Endpoint{
			Address: aws.String("memorydb.localhost"),
			Port:    6379,
		},
	}
	extraLabels := map[string]string{"key": "value"}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster",
		Description: "MemoryDB cluster in us-east-1",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "us-east-1",
			types.DiscoveryLabelEndpointType: "cluster",
			"key":                            "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "memorydb.localhost:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
			MemoryDB: types.MemoryDB{
				ClusterName:  "my-cluster",
				ACLName:      "my-user-group",
				TLSEnabled:   true,
				EndpointType: awsutils.MemoryDBClusterEndpoint,
			},
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromRedshiftServerlessWorkgroup(t *testing.T) {
	workgroup := mocks.RedshiftServerlessWorkgroup("my-workgroup", "eu-west-2")
	tags := awstesthelpers.LabelsToRedshiftServerlessTags(map[string]string{"env": "prod"})
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-workgroup",
		Description: "Redshift Serverless workgroup in eu-west-2",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "eu-west-2",
			types.DiscoveryLabelEndpointType: "workgroup",
			types.DiscoveryLabelNamespace:    "my-namespace",
			types.DiscoveryLabelVPCID:        "vpc-id",
			"env":                            "prod",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "my-workgroup.123456789012.eu-west-2.redshift-serverless.amazonaws.com:5439",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "eu-west-2",
			RedshiftServerless: types.RedshiftServerless{
				WorkgroupName: "my-workgroup",
				WorkgroupID:   "some-uuid-for-my-workgroup",
			},
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromRedshiftServerlessWorkgroup(workgroup, tags)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromRedshiftServerlessVPCEndpoint(t *testing.T) {
	workgroup := mocks.RedshiftServerlessWorkgroup("my-workgroup", "eu-west-2")
	endpoint := mocks.RedshiftServerlessEndpointAccess(workgroup, "my-endpoint", "eu-west-2")
	tags := awstesthelpers.LabelsToRedshiftServerlessTags(map[string]string{"env": "prod"})
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-workgroup-my-endpoint",
		Description: "Redshift Serverless endpoint in eu-west-2",
		Labels: map[string]string{
			types.DiscoveryLabelAccountID:    "123456789012",
			types.CloudLabel:                 types.CloudAWS,
			types.DiscoveryLabelRegion:       "eu-west-2",
			types.DiscoveryLabelEndpointType: "vpc-endpoint",
			types.DiscoveryLabelWorkgroup:    "my-workgroup",
			types.DiscoveryLabelNamespace:    "my-namespace",
			types.DiscoveryLabelVPCID:        "vpc-id",
			"env":                            "prod",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "my-endpoint-endpoint-xxxyyyzzz.123456789012.eu-west-2.redshift-serverless.amazonaws.com:5439",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "eu-west-2",
			RedshiftServerless: types.RedshiftServerless{
				WorkgroupName: "my-workgroup",
				EndpointName:  "my-endpoint",
				WorkgroupID:   "some-uuid-for-my-workgroup",
			},
		},
		TLS: types.DatabaseTLS{
			ServerName: "my-workgroup.123456789012.eu-west-2.redshift-serverless.amazonaws.com",
		},
	})
	require.NoError(t, err)

	actual, err := NewDatabaseFromRedshiftServerlessVPCEndpoint(endpoint, workgroup, tags)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestDatabaseFromMemoryDBClusterNameOverride(t *testing.T) {
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		t.Run("via "+overrideLabel, func(t *testing.T) {
			cluster := &memorydbtypes.Cluster{
				ARN:        aws.String("arn:aws:memorydb:us-east-1:123456789012:cluster:my-cluster"),
				Name:       aws.String("my-cluster"),
				Status:     aws.String("available"),
				TLSEnabled: aws.Bool(true),
				ACLName:    aws.String("my-user-group"),
				ClusterEndpoint: &memorydbtypes.Endpoint{
					Address: aws.String("memorydb.localhost"),
					Port:    6379,
				},
			}
			extraLabels := map[string]string{
				overrideLabel: "override-1",
				"key":         "value",
			}

			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "override-1",
				Description: "MemoryDB cluster in us-east-1",
				Labels: map[string]string{
					types.DiscoveryLabelAccountID:    "123456789012",
					types.CloudLabel:                 types.CloudAWS,
					types.DiscoveryLabelRegion:       "us-east-1",
					types.DiscoveryLabelEndpointType: "cluster",
					overrideLabel:                    "override-1",
					"key":                            "value",
				},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "memorydb.localhost:6379",
				AWS: types.AWS{
					AccountID: "123456789012",
					Region:    "us-east-1",
					MemoryDB: types.MemoryDB{
						ClusterName:  "my-cluster",
						ACLName:      "my-user-group",
						TLSEnabled:   true,
						EndpointType: awsutils.MemoryDBClusterEndpoint,
					},
				},
			})
			require.NoError(t, err)

			actual, err := NewDatabaseFromMemoryDBCluster(cluster, extraLabels)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})
	}
}

func TestExtraElastiCacheLabels(t *testing.T) {
	cluster := &ectypes.ReplicationGroup{
		ReplicationGroupId: aws.String("my-redis"),
		Engine:             aws.String("redis"),
	}
	tags := []ectypes.Tag{
		{Key: aws.String("key1"), Value: aws.String("value1")},
		{Key: aws.String("key2"), Value: aws.String("value2")},
	}

	nodes := []ectypes.CacheCluster{
		{
			ReplicationGroupId:   aws.String("some-other-redis"),
			EngineVersion:        aws.String("8.8.8"),
			CacheSubnetGroupName: aws.String("some-other-subnet-group"),
		},
		{
			ReplicationGroupId:   aws.String("my-redis"),
			EngineVersion:        aws.String("6.6.6"),
			CacheSubnetGroupName: aws.String("my-subnet-group"),
		},
	}

	subnetGroups := []ectypes.CacheSubnetGroup{
		{
			CacheSubnetGroupName: aws.String("some-other-subnet-group"),
			VpcId:                aws.String("some-other-vpc"),
		},
		{
			CacheSubnetGroupName: aws.String("my-subnet-group"),
			VpcId:                aws.String("my-vpc"),
		},
	}

	tests := []struct {
		name              string
		inputTags         []ectypes.Tag
		inputNodes        []ectypes.CacheCluster
		inputSubnetGroups []ectypes.CacheSubnetGroup
		expectLabels      map[string]string
	}{
		{
			name:              "all tags",
			inputTags:         tags,
			inputNodes:        nodes,
			inputSubnetGroups: subnetGroups,
			expectLabels: map[string]string{
				"key1":           "value1",
				"key2":           "value2",
				"engine":         "redis",
				"engine-version": "6.6.6",
				"vpc-id":         "my-vpc",
			},
		},
		{
			name:              "no resource tags",
			inputTags:         nil,
			inputNodes:        nodes,
			inputSubnetGroups: subnetGroups,
			expectLabels: map[string]string{
				"engine":         "redis",
				"engine-version": "6.6.6",
				"vpc-id":         "my-vpc",
			},
		},
		{
			name:              "no nodes",
			inputTags:         tags,
			inputNodes:        nil,
			inputSubnetGroups: subnetGroups,

			// Without subnet group name from nodes, VPC ID cannot be found.
			expectLabels: map[string]string{
				"key1":   "value1",
				"key2":   "value2",
				"engine": "redis",
			},
		},
		{
			name:              "no subnet groups",
			inputTags:         tags,
			inputNodes:        nodes,
			inputSubnetGroups: nil,
			expectLabels: map[string]string{
				"key1":           "value1",
				"key2":           "value2",
				"engine":         "redis",
				"engine-version": "6.6.6",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualLabels := ExtraElastiCacheLabels(cluster, test.inputTags, test.inputNodes, test.inputSubnetGroups)
			require.Equal(t, test.expectLabels, actualLabels)
		})
	}
}

func TestExtraMemoryDBLabels(t *testing.T) {
	cluster := &memorydbtypes.Cluster{
		Name:            aws.String("my-cluster"),
		SubnetGroupName: aws.String("my-subnet-group"),
		Engine:          aws.String("redis"),
		EngineVersion:   aws.String("6.6.6"),
	}

	allSubnetGroups := []memorydbtypes.SubnetGroup{
		{
			Name:  aws.String("other-subnet-group"),
			VpcId: aws.String("other-vpc-id"),
		},
		{
			Name:  aws.String("my-subnet-group"),
			VpcId: aws.String("my-vpc-id"),
		},
	}

	resourceTags := []memorydbtypes.Tag{
		{Key: aws.String("key1"), Value: aws.String("value1")},
		{Key: aws.String("key2"), Value: aws.String("value2")},
	}

	expected := map[string]string{
		"key1":           "value1",
		"key2":           "value2",
		"engine":         "redis",
		"engine-version": "6.6.6",
		"vpc-id":         "my-vpc-id",
	}

	actual := ExtraMemoryDBLabels(cluster, resourceTags, allSubnetGroups)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestGetLabelEngineVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name: "mysql-8.0.0",
			labels: map[string]string{
				types.DiscoveryLabelEngine:        services.RDSEngineMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
			},
			want: "8.0.0",
		},
		{
			name: "mariadb returns nothing",
			labels: map[string]string{
				types.DiscoveryLabelEngine:        services.RDSEngineMariaDB,
				types.DiscoveryLabelEngineVersion: "10.6.7",
			},
			want: "",
		},
		{
			name:   "missing labels",
			labels: map[string]string{},
			want:   "",
		},
		{
			name: "azure-mysql-8.0.0",
			labels: map[string]string{
				types.DiscoveryLabelEngine:        services.AzureEngineMySQL,
				types.DiscoveryLabelEngineVersion: "8.0.0",
			},
			want: "8.0.0",
		},
		{
			name: "azure-mysql-8.0.0 flex server",
			labels: map[string]string{
				types.DiscoveryLabelEngine:        services.AzureEngineMySQLFlex,
				types.DiscoveryLabelEngineVersion: string(armmysqlflexibleservers.ServerVersionEight021),
			},
			want: "8.0.21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := GetMySQLEngineVersion(tt.labels); got != tt.want {
				t.Errorf("GetMySQLEngineVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDatabaseFromAzureSQLServer(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		server      *armsql.Server
		expectedErr require.ErrorAssertionFunc
		expectedDB  require.ValueAssertionFunc
	}{
		{
			desc: "complete server",
			server: &armsql.Server{
				ID:       to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-groupd/providers/Microsoft.Sql/servers/sqlserver"),
				Name:     to.Ptr("sqlserver"),
				Location: to.Ptr("westus"),
				Properties: &armsql.ServerProperties{
					FullyQualifiedDomainName: to.Ptr("sqlserver.database.windows.net"),
					Version:                  to.Ptr("12.0"),
				},
			},
			expectedErr: require.NoError,
			expectedDB: func(t require.TestingT, i any, _ ...any) {
				db, ok := i.(types.Database)
				require.True(t, ok, "expected types.Database, got %T", i)

				require.Equal(t, defaults.ProtocolSQLServer, db.GetProtocol())
				require.Equal(t, "sqlserver", db.GetName())
				require.Equal(t, "sqlserver.database.windows.net:1433", db.GetURI())
				require.Equal(t, "sqlserver", db.GetAzure().Name)
				require.Equal(t, "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-groupd/providers/Microsoft.Sql/servers/sqlserver", db.GetAzure().ResourceID)

				// Assert labels
				labels := db.GetMetadata().Labels
				require.Equal(t, "westus", labels[types.DiscoveryLabelRegion])
				require.Equal(t, "12.0", labels[types.DiscoveryLabelEngineVersion])
			},
		},
		{
			desc:        "empty properties",
			server:      &armsql.Server{Properties: nil},
			expectedErr: require.Error,
			expectedDB:  require.Nil,
		},
		{
			desc:        "empty FQDN",
			server:      &armsql.Server{Properties: &armsql.ServerProperties{FullyQualifiedDomainName: nil}},
			expectedErr: require.Error,
			expectedDB:  require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			database, err := NewDatabaseFromAzureSQLServer(tc.server)
			tc.expectedErr(t, err)
			tc.expectedDB(t, database)
		})
	}
}

func TestNewDatabaseFromAzureManagedSQLServer(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		server      *armsql.ManagedInstance
		expectedErr require.ErrorAssertionFunc
		expectedDB  require.ValueAssertionFunc
	}{
		{
			desc: "complete server",
			server: &armsql.ManagedInstance{
				ID:       to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-groupd/providers/Microsoft.Sql/servers/sqlserver"),
				Name:     to.Ptr("sqlserver"),
				Location: to.Ptr("westus"),
				Properties: &armsql.ManagedInstanceProperties{
					FullyQualifiedDomainName: to.Ptr("sqlserver.database.windows.net"),
				},
			},
			expectedErr: require.NoError,
			expectedDB: func(t require.TestingT, i any, _ ...any) {
				db, ok := i.(types.Database)
				require.True(t, ok, "expected types.Database, got %T", i)

				require.Equal(t, defaults.ProtocolSQLServer, db.GetProtocol())
				require.Equal(t, "sqlserver", db.GetName())
				require.Equal(t, "sqlserver.database.windows.net:1433", db.GetURI())
				require.Equal(t, "sqlserver", db.GetAzure().Name)
				require.Equal(t, "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-groupd/providers/Microsoft.Sql/servers/sqlserver", db.GetAzure().ResourceID)

				// Assert labels
				labels := db.GetMetadata().Labels
				require.Equal(t, "westus", labels[types.DiscoveryLabelRegion])
			},
		},
		{
			desc:        "empty properties",
			server:      &armsql.ManagedInstance{Properties: nil},
			expectedErr: require.Error,
			expectedDB:  require.Nil,
		},
		{
			desc:        "empty FQDN",
			server:      &armsql.ManagedInstance{Properties: &armsql.ManagedInstanceProperties{FullyQualifiedDomainName: nil}},
			expectedErr: require.Error,
			expectedDB:  require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			database, err := NewDatabaseFromAzureManagedSQLServer(tc.server)
			tc.expectedErr(t, err)
			tc.expectedDB(t, database)
		})
	}
}

func TestDatabaseFromAzureMySQLFlexServer(t *testing.T) {
	t.Parallel()
	subID := "sub123"
	group := "test-group"
	region := "eastus"
	provider := services.AzureEngineMySQLFlex

	tests := []struct {
		desc                     string
		serverName               string
		replicationRole          armmysqlflexibleservers.ReplicationRole
		sourceServerID           *string
		wantReplicationRoleLabel string
		wantSourceServerLabel    string
		wantDBDesc               string
	}{
		{
			desc:            "server without replication",
			serverName:      "azure-mysql-flex",
			replicationRole: armmysqlflexibleservers.ReplicationRoleNone,
			wantDBDesc:      "Azure MySQL Flexible server in eastus",
		},
		{
			desc:                     "source server",
			serverName:               "azure-mysql-flex-source",
			replicationRole:          armmysqlflexibleservers.ReplicationRoleSource,
			wantReplicationRoleLabel: "Source",
			wantDBDesc:               "Azure MySQL Flexible server in eastus (source endpoint)",
		},
		{
			desc:                     "replica server",
			serverName:               "azure-mysql-flex-replica",
			replicationRole:          armmysqlflexibleservers.ReplicationRoleReplica,
			sourceServerID:           to.Ptr(makeAzureResourceID(subID, group, provider, "azure-mysql-flex-source")),
			wantReplicationRoleLabel: "Replica",
			wantSourceServerLabel:    "azure-mysql-flex-source",
			wantDBDesc:               "Azure MySQL Flexible server in eastus (replica endpoint)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			rid := makeAzureResourceID(subID, group, provider, tt.serverName)
			server := armmysqlflexibleservers.Server{
				Name:     to.Ptr(tt.serverName),
				ID:       to.Ptr(rid),
				Location: to.Ptr(region),
				Tags: map[string]*string{
					"foo": to.Ptr("bar"),
				},
				Properties: &armmysqlflexibleservers.ServerProperties{
					FullyQualifiedDomainName: to.Ptr(tt.serverName + ".mysql" + azureutils.DatabaseEndpointSuffix),
					State:                    to.Ptr(armmysqlflexibleservers.ServerStateReady),
					Version:                  to.Ptr(armmysqlflexibleservers.ServerVersionEight021),
					ReplicationRole:          &tt.replicationRole,
					SourceServerResourceID:   tt.sourceServerID,
				},
			}

			wantLabels := map[string]string{
				types.DiscoveryLabelRegion:              region,
				types.DiscoveryLabelEngine:              provider,
				types.DiscoveryLabelEngineVersion:       "8.0.21",
				types.DiscoveryLabelAzureResourceGroup:  group,
				types.CloudLabel:                        types.CloudAzure,
				types.DiscoveryLabelAzureSubscriptionID: subID,
				"foo":                                   "bar",
			}
			if tt.wantReplicationRoleLabel != "" {
				wantLabels[types.DiscoveryLabelAzureReplicationRole] = tt.wantReplicationRoleLabel
			}
			if tt.wantSourceServerLabel != "" {
				wantLabels[types.DiscoveryLabelAzureSourceServer] = tt.wantSourceServerLabel
			}
			wantDB, err := types.NewDatabaseV3(types.Metadata{
				Name:        tt.serverName,
				Description: tt.wantDBDesc,
				Labels:      wantLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      tt.serverName + ".mysql.database.azure.com:3306",
				Azure: types.Azure{
					Name:          tt.serverName,
					ResourceID:    rid,
					IsFlexiServer: true,
				},
			})
			require.NoError(t, err)

			actual, err := NewDatabaseFromAzureMySQLFlexServer(&server)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(wantDB, actual))
		})
	}
}

func TestDatabaseFromAzurePostgresFlexServer(t *testing.T) {
	t.Parallel()
	subID := "sub123"
	group := "test-group"
	region := "eastus"
	provider := services.AzureEnginePostgresFlex

	tests := []struct {
		desc       string
		serverName string
		wantDBDesc string
	}{
		{
			desc:       "server without replication",
			serverName: "azure-postgres-flex",
			wantDBDesc: "Azure PostgreSQL Flexible server in eastus",
		},
		// TODO(gavin): add more tests if replication is done somehow in postgres. it's different than the azure mysql flex setup for some reason.
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			rid := makeAzureResourceID(subID, group, provider, tt.serverName)
			server := armpostgresqlflexibleservers.Server{
				Name:     to.Ptr(tt.serverName),
				ID:       to.Ptr(rid),
				Location: to.Ptr(region),
				Tags: map[string]*string{
					"foo": to.Ptr("bar"),
				},
				Properties: &armpostgresqlflexibleservers.ServerProperties{
					FullyQualifiedDomainName: to.Ptr(tt.serverName + ".postgres" + azureutils.DatabaseEndpointSuffix),
					State:                    to.Ptr(armpostgresqlflexibleservers.ServerStateReady),
					Version:                  to.Ptr(armpostgresqlflexibleservers.ServerVersionFourteen),
				},
			}

			wantLabels := map[string]string{
				types.DiscoveryLabelRegion:              region,
				types.DiscoveryLabelEngine:              provider,
				types.DiscoveryLabelEngineVersion:       "14",
				types.DiscoveryLabelAzureResourceGroup:  group,
				types.CloudLabel:                        types.CloudAzure,
				types.DiscoveryLabelAzureSubscriptionID: subID,
				"foo":                                   "bar",
			}
			wantDB, err := types.NewDatabaseV3(types.Metadata{
				Name:        tt.serverName,
				Description: tt.wantDBDesc,
				Labels:      wantLabels,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      tt.serverName + ".postgres.database.azure.com:5432",
				Azure: types.Azure{
					Name:          tt.serverName,
					ResourceID:    rid,
					IsFlexiServer: true,
				},
			})
			require.NoError(t, err)

			actual, err := NewDatabaseFromAzurePostgresFlexServer(&server)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(wantDB, actual))
		})
	}
}

func TestMakeAzureDatabaseLoginUsername(t *testing.T) {
	t.Parallel()
	subID := "sub123"
	group := "group"
	serverName := "test-server"
	tests := []struct {
		desc         string
		protocol     string
		engine       string
		staticIsFlex bool
		wantIsFlex   bool
	}{
		{
			desc:       "mysql flex",
			protocol:   defaults.ProtocolMySQL,
			engine:     services.AzureEngineMySQLFlex,
			wantIsFlex: true,
		},
		{
			desc:       "postgres flex",
			protocol:   defaults.ProtocolPostgres,
			engine:     services.AzureEnginePostgresFlex,
			wantIsFlex: true,
		},
		{
			desc:         "static config mysql flex",
			protocol:     defaults.ProtocolMySQL,
			staticIsFlex: true,
			wantIsFlex:   true,
		},
		{
			desc:         "static config postgres flex",
			protocol:     defaults.ProtocolPostgres,
			staticIsFlex: true,
			wantIsFlex:   true,
		},
		{
			desc:       "mysql single server",
			protocol:   defaults.ProtocolMySQL,
			engine:     services.AzureEngineMySQL,
			wantIsFlex: false,
		},
		{
			desc:       "postgres single server",
			protocol:   defaults.ProtocolPostgres,
			engine:     services.AzureEnginePostgres,
			wantIsFlex: false,
		},
		{
			desc:       "mysql non-azure",
			protocol:   defaults.ProtocolMySQL,
			wantIsFlex: false,
		},
		{
			desc:       "postgres non-azure",
			protocol:   defaults.ProtocolPostgres,
			wantIsFlex: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			db, err := types.NewDatabaseV3(types.Metadata{
				Name:        serverName,
				Description: "test azure db server",
				Labels: map[string]string{
					types.DiscoveryLabelRegion:              "eastus",
					types.DiscoveryLabelEngine:              tt.engine,
					types.DiscoveryLabelEngineVersion:       "1.2.3",
					types.DiscoveryLabelAzureResourceGroup:  group,
					types.CloudLabel:                        types.CloudAzure,
					types.DiscoveryLabelAzureSubscriptionID: subID,
					"foo":                                   "bar",
				},
			}, types.DatabaseSpecV3{
				Protocol: tt.protocol,
				URI:      "example.com:1234",
				Azure: types.Azure{
					Name:          serverName,
					ResourceID:    makeAzureResourceID(subID, group, tt.engine, serverName),
					IsFlexiServer: tt.staticIsFlex,
				},
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantIsFlex, IsAzureFlexServer(db))

			user := MakeAzureDatabaseLoginUsername(db, "alice")
			if tt.wantIsFlex {
				require.Equal(t, "alice", user)
			} else {
				require.Equal(t, "alice@test-server", user)
			}
		})
	}
}

func makeAzureResourceID(subID, group, provider, resourceName string) string {
	return fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subID, group, provider, resourceName)
}
