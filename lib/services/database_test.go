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
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDatabaseUnmarshal verifies a database resource can be unmarshaled.
func TestDatabaseUnmarshal(t *testing.T) {
	t.Parallel()
	tlsModes := map[string]types.DatabaseTLSMode{
		"":            types.DatabaseTLSMode_VERIFY_FULL,
		"verify-full": types.DatabaseTLSMode_VERIFY_FULL,
		"verify-ca":   types.DatabaseTLSMode_VERIFY_CA,
		"insecure":    types.DatabaseTLSMode_INSECURE,
	}
	for tlsModeName, tlsModeValue := range tlsModes {
		t.Run("tls mode "+tlsModeName, func(t *testing.T) {
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "test-database",
				Description: "Test description",
				Labels:      map[string]string{"env": "dev"},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				CACert:   fixtures.TLSCACertPEM,
				TLS: types.DatabaseTLS{
					Mode: tlsModeValue,
				},
			})
			require.NoError(t, err)
			caCert := indent(fixtures.TLSCACertPEM, 4)

			// verify it works with string tls mode.
			data, err := utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, tlsModeName, caCert)))
			require.NoError(t, err)
			actual, err := UnmarshalDatabase(data)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))

			// verify it works with integer tls mode.
			data, err = utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, int32(tlsModeValue), caCert)))
			require.NoError(t, err)
			actual, err = UnmarshalDatabase(data)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})
	}
}

// TestDatabaseMarshal verifies a marshaled database resource can be unmarshaled back.
func TestDatabaseMarshal(t *testing.T) {
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "test-database",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		CACert:   fixtures.TLSCACertPEM,
	})
	require.NoError(t, err)
	data, err := MarshalDatabase(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDatabase(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestValidateDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputName   string
		inputSpec   types.DatabaseSpecV3
		expectError bool
	}{
		{
			// Captured error:
			// a DNS-1035 label must consist of lower case alphanumeric
			// characters or '-', start with an alphabetic character, and end
			// with an alphanumeric character (e.g. 'my-name',  or 'abc-123',
			// regex used for validation is '[a-z]([-a-z0-9]*[a-z0-9])?')
			inputName: "invalid-database-name-",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: "unknown",
				URI:      "localhost:5432",
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "missing-port",
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-CA-cert",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				TLS: types.DatabaseTLS{
					CACert: "bad-cert",
				},
			},
			expectError: true,
		},
		{
			inputName: "valid-mongodb",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb://mongo-1:27017,mongo-2:27018/?replicaSet=rs0",
			},
			expectError: false,
		},
		{
			inputName: "valid-mongodb-srv",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb+srv://valid.but.cannot.be.resolved.com",
			},
			expectError: false,
		},
		{
			inputName: "invalid-mongodb-srv",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb+srv://valid.but.cannot.be.resolved.com/?readpreference=unknown",
			},
			expectError: true,
		},
		{
			inputName: "invalid-mongodb-missing-username",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb://mongo-1:27017/?authmechanism=plain",
			},
			expectError: true,
		},
		{
			inputName: "valid-redis",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "rediss://redis.example.com:6379",
			},
			expectError: false,
		},
		{
			inputName: "invalid-redis-incorrect-mode",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "rediss://redis.example.com:6379?mode=unknown",
			},
			expectError: true,
		},
		{
			inputName: "valid-snowflake",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSnowflake,
				URI:      "test.snowflakecomputing.com",
			},
			expectError: false,
		},
		{
			inputName: "invalid-snowflake",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSnowflake,
				URI:      "not.snow.flake.com",
			},
			expectError: true,
		},
		{
			inputName: "valid-cassandra-without-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolCassandra,
				AWS: types.AWS{
					Region:    "us-east-1",
					AccountID: "123456789012",
				},
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.inputName, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: test.inputName,
			}, test.inputSpec)
			require.NoError(t, err)

			err = ValidateDatabase(database)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// indent returns the string where each line is indented by the specified
// number of spaces.
func indent(s string, spaces int) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, fmt.Sprintf("%v%v", strings.Repeat(" ", spaces), scanner.Text()))
	}
	return strings.Join(lines, "\n")
}

var databaseYAML = `---
kind: db
version: v3
metadata:
  name: test-database
  description: "Test description"
  labels:
    env: dev
spec:
  protocol: "postgres"
  uri: "localhost:5432"
  tls:
    mode: %v
  ca_cert: |-
%v`

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
			types.OriginLabel:   types.OriginCloud,
			labelRegion:         "eastus",
			labelEngine:         "Microsoft.DBforMySQL/servers",
			labelEngineVersion:  "5.7",
			labelResourceGroup:  "defaultRG",
			labelSubscriptionID: "sub1",
			"foo":               "bar",
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
	require.Equal(t, expected, actual)
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
			types.OriginLabel:   types.OriginCloud,
			labelRegion:         region,
			labelEngine:         "Microsoft.Cache/Redis",
			labelEngineVersion:  "6.0",
			labelResourceGroup:  group,
			labelSubscriptionID: subscription,
			"foo":               "bar",
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
	require.Equal(t, expected, actual)
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
			types.OriginLabel:   types.OriginCloud,
			labelRegion:         region,
			labelEngine:         "Microsoft.Cache/redisEnterprise",
			labelEngineVersion:  "6.0",
			labelResourceGroup:  group,
			labelSubscriptionID: subscription,
			labelEndpointType:   "OSSCluster",
			"foo":               "bar",
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
	require.Equal(t, expected, actual)
}

// TestDatabaseFromRDSInstance tests converting an RDS instance to a database resource.
func TestDatabaseFromRDSInstance(t *testing.T) {
	instance := &rds.DBInstance{
		DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:instance-1"),
		DBInstanceIdentifier:             aws.String("instance-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbiResourceId:                    aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(RDSEnginePostgres),
		EngineVersion:                    aws.String("13.0"),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: []*rds.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
	}
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "instance-1",
		Description: "RDS instance in us-west-1",
		Labels: map[string]string{
			types.OriginLabel:  types.OriginCloud,
			labelAccountID:     "123456789012",
			labelRegion:        "us-west-1",
			labelEngine:        RDSEnginePostgres,
			labelEngineVersion: "13.0",
			labelEndpointType:  "instance",
			"key":              "val",
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
	require.Equal(t, expected, actual)
}

// TestDatabaseFromRDSInstance tests converting an RDS instance to a database resource.
func TestDatabaseFromRDSInstanceNameOverride(t *testing.T) {
	instance := &rds.DBInstance{
		DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:123456789012:db:instance-1"),
		DBInstanceIdentifier:             aws.String("instance-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbiResourceId:                    aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(RDSEnginePostgres),
		EngineVersion:                    aws.String("13.0"),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: []*rds.Tag{
			{Key: aws.String("key"), Value: aws.String("val")},
			{Key: aws.String(labelTeleportDBName), Value: aws.String("override-1")},
		},
	}
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "override-1",
		Description: "RDS instance in us-west-1",
		Labels: map[string]string{
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-west-1",
			labelEngine:         RDSEnginePostgres,
			labelEngineVersion:  "13.0",
			labelEndpointType:   "instance",
			labelTeleportDBName: "override-1",
			"key":               "val",
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
	require.Equal(t, expected, actual)
}

// TestDatabaseFromRDSCluster tests converting an RDS cluster to a database resource.
func TestDatabaseFromRDSCluster(t *testing.T) {
	cluster := &rds.DBCluster{
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:cluster-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbClusterResourceId:              aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(RDSEngineAuroraMySQL),
		EngineVersion:                    aws.String("8.0.0"),
		Endpoint:                         aws.String("localhost"),
		ReaderEndpoint:                   aws.String("reader.host"),
		Port:                             aws.Int64(3306),
		CustomEndpoints: []*string{
			aws.String("myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com"),
			aws.String("myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com"),
		},
		TagList: []*rds.Tag{{
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
				types.OriginLabel:  types.OriginCloud,
				labelAccountID:     "123456789012",
				labelRegion:        "us-east-1",
				labelEngine:        RDSEngineAuroraMySQL,
				labelEngineVersion: "8.0.0",
				labelEndpointType:  "primary",
				"key":              "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSCluster(cluster)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("reader", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "cluster-1-reader",
			Description: "Aurora cluster in us-east-1 (reader endpoint)",
			Labels: map[string]string{
				types.OriginLabel:  types.OriginCloud,
				labelAccountID:     "123456789012",
				labelRegion:        "us-east-1",
				labelEngine:        RDSEngineAuroraMySQL,
				labelEngineVersion: "8.0.0",
				labelEndpointType:  "reader",
				"key":              "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "reader.host:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSClusterReaderEndpoint(cluster)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("custom endpoints", func(t *testing.T) {
		expectedLabels := map[string]string{
			types.OriginLabel:  types.OriginCloud,
			labelAccountID:     "123456789012",
			labelRegion:        "us-east-1",
			labelEngine:        RDSEngineAuroraMySQL,
			labelEngineVersion: "8.0.0",
			labelEndpointType:  "custom",
			"key":              "val",
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

		databases, err := NewDatabasesFromRDSClusterCustomEndpoints(cluster)
		require.NoError(t, err)
		require.Equal(t, types.Databases{expectedMyEndpoint1, expectedMyEndpoint2}, databases)
	})

	t.Run("bad custom endpoints ", func(t *testing.T) {
		badCluster := *cluster
		badCluster.CustomEndpoints = []*string{
			aws.String("badendpoint1"),
			aws.String("badendpoint2"),
		}
		_, err := NewDatabasesFromRDSClusterCustomEndpoints(&badCluster)
		require.Error(t, err)
	})
}

// TestDatabaseFromRDSClusterNameOverride tests converting an RDS cluster to a database resource with overridden name.
func TestDatabaseFromRDSClusterNameOverride(t *testing.T) {
	cluster := &rds.DBCluster{
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:cluster-1"),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbClusterResourceId:              aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String(RDSEngineAuroraMySQL),
		EngineVersion:                    aws.String("8.0.0"),
		Endpoint:                         aws.String("localhost"),
		ReaderEndpoint:                   aws.String("reader.host"),
		Port:                             aws.Int64(3306),
		CustomEndpoints: []*string{
			aws.String("myendpoint1.cluster-custom-example.us-east-1.rds.amazonaws.com"),
			aws.String("myendpoint2.cluster-custom-example.us-east-1.rds.amazonaws.com"),
		},
		TagList: []*rds.Tag{
			{Key: aws.String("key"), Value: aws.String("val")},
			{Key: aws.String(labelTeleportDBName), Value: aws.String("mycluster-2")},
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
				types.OriginLabel:   types.OriginCloud,
				labelAccountID:      "123456789012",
				labelRegion:         "us-east-1",
				labelEngine:         RDSEngineAuroraMySQL,
				labelEngineVersion:  "8.0.0",
				labelEndpointType:   "primary",
				labelTeleportDBName: "mycluster-2",
				"key":               "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSCluster(cluster)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("reader", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "mycluster-2-reader",
			Description: "Aurora cluster in us-east-1 (reader endpoint)",
			Labels: map[string]string{
				types.OriginLabel:   types.OriginCloud,
				labelAccountID:      "123456789012",
				labelRegion:         "us-east-1",
				labelEngine:         RDSEngineAuroraMySQL,
				labelEngineVersion:  "8.0.0",
				labelEndpointType:   "reader",
				labelTeleportDBName: "mycluster-2",
				"key":               "val",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "reader.host:3306",
			AWS:      expectedAWS,
		})
		require.NoError(t, err)
		actual, err := NewDatabaseFromRDSClusterReaderEndpoint(cluster)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("custom endpoints", func(t *testing.T) {
		expectedLabels := map[string]string{
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-east-1",
			labelEngine:         RDSEngineAuroraMySQL,
			labelEngineVersion:  "8.0.0",
			labelEndpointType:   "custom",
			labelTeleportDBName: "mycluster-2",
			"key":               "val",
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

		databases, err := NewDatabasesFromRDSClusterCustomEndpoints(cluster)
		require.NoError(t, err)
		require.Equal(t, types.Databases{expectedMyEndpoint1, expectedMyEndpoint2}, databases)
	})

	t.Run("bad custom endpoints ", func(t *testing.T) {
		badCluster := *cluster
		badCluster.CustomEndpoints = []*string{
			aws.String("badendpoint1"),
			aws.String("badendpoint2"),
		}
		_, err := NewDatabasesFromRDSClusterCustomEndpoints(&badCluster)
		require.Error(t, err)
	})
}

func TestDatabaseFromRDSProxy(t *testing.T) {
	var port int64 = 9999
	dbProxy := &rds.DBProxy{
		DBProxyArn:   aws.String("arn:aws:rds:ca-central-1:123456789012:db-proxy:prx-abcdef"),
		DBProxyName:  aws.String("testproxy"),
		EngineFamily: aws.String(rds.EngineFamilyMysql),
		Endpoint:     aws.String("proxy.rds.test"),
		VpcId:        aws.String("test-vpc-id"),
	}

	dbProxyEndpoint := &rds.DBProxyEndpoint{
		Endpoint:            aws.String("custom.proxy.rds.test"),
		DBProxyEndpointName: aws.String("custom"),
		DBProxyName:         aws.String("testproxy"),
		DBProxyEndpointArn:  aws.String("arn:aws:rds:ca-central-1:123456789012:db-proxy-endpoint:prx-endpoint-abcdef"),
		TargetRole:          aws.String(rds.DBProxyEndpointTargetRoleReadOnly),
	}

	tags := []*rds.Tag{{
		Key:   aws.String("key"),
		Value: aws.String("val"),
	}}

	t.Run("default endpoint", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "testproxy",
			Description: "RDS Proxy in ca-central-1",
			Labels: map[string]string{
				"key":             "val",
				types.OriginLabel: types.OriginCloud,
				labelAccountID:    "123456789012",
				labelRegion:       "ca-central-1",
				labelEngine:       "MYSQL",
				labelVPCID:        "test-vpc-id",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "proxy.rds.test:9999",
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

		actual, err := NewDatabaseFromRDSProxy(dbProxy, port, tags)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("custom endpoint", func(t *testing.T) {
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "testproxy-custom",
			Description: "RDS Proxy endpoint in ca-central-1",
			Labels: map[string]string{
				"key":             "val",
				types.OriginLabel: types.OriginCloud,
				labelAccountID:    "123456789012",
				labelRegion:       "ca-central-1",
				labelEngine:       "MYSQL",
				labelVPCID:        "test-vpc-id",
				labelEndpointType: "READ_ONLY",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolMySQL,
			URI:      "custom.proxy.rds.test:9999",
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

		actual, err := NewDatabaseFromRDSProxyCustomEndpoint(dbProxy, dbProxyEndpoint, port, tags)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
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
			require.Equal(t, test.expectedMySQLVersion, auroraMySQLVersion(&rds.DBCluster{EngineVersion: aws.String(test.engineVersion)}))
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
			engineMode:    RDSEngineModeProvisioned,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   true,
		},
		{
			name:          "serverless",
			engineMode:    RDSEngineModeServerless,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   false,
		},
		{
			name:          "parallel query supported",
			engineMode:    RDSEngineModeParallelQuery,
			engineVersion: "5.6.mysql_aurora.1.22.0",
			isSupported:   true,
		},
		{
			name:          "parallel query unsupported",
			engineMode:    RDSEngineModeParallelQuery,
			engineVersion: "5.6.mysql_aurora.1.19.6",
			isSupported:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := &rds.DBCluster{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:test"),
				DBClusterIdentifier: aws.String(test.name),
				DbClusterResourceId: aws.String(uuid.New().String()),
				Engine:              aws.String(RDSEngineAuroraMySQL),
				EngineMode:          aws.String(test.engineMode),
				EngineVersion:       aws.String(test.engineVersion),
			}

			got, want := IsRDSClusterSupported(cluster), test.isSupported
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
			engine:        RDSEnginePostgres,
			engineVersion: "13.3",
			isSupported:   true,
		},
		{
			name:          "unsupported MariaDB",
			engine:        RDSEngineMariaDB,
			engineVersion: "10.3.28",
			isSupported:   false,
		},
		{
			name:          "min supported version",
			engine:        RDSEngineMariaDB,
			engineVersion: "10.6.2",
			isSupported:   true,
		},
		{
			name:          "supported version",
			engine:        RDSEngineMariaDB,
			engineVersion: "10.8.0",
			isSupported:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := &rds.DBInstance{
				DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:123456789012:instance:test"),
				DBClusterIdentifier: aws.String(test.name),
				DbiResourceId:       aws.String(uuid.New().String()),
				Engine:              aws.String(test.engine),
				EngineVersion:       aws.String(test.engineVersion),
			}

			got, want := IsRDSInstanceSupported(cluster), test.isSupported
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
	require.Equal(t, map[string]string{"Name": "test", "Env": "dev",
		"foo:bar": "some-id"}, labels)
}

func TestRDSTagsToLabels(t *testing.T) {
	rdsTags := []*rds.Tag{
		{
			Key:   aws.String("Env"),
			Value: aws.String("dev"),
		},
		{
			Key:   aws.String("aws:cloudformation:stack-id"),
			Value: aws.String("some-id"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String("test"),
		},
	}
	labels := rdsTagsToLabels(rdsTags)
	require.Equal(t, map[string]string{"Name": "test", "Env": "dev",
		"aws:cloudformation:stack-id": "some-id"}, labels)
}

// TestDatabaseFromRedshiftCluster tests converting an Redshift cluster to a database resource.
func TestDatabaseFromRedshiftCluster(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cluster := &redshift.Cluster{
			ClusterIdentifier:   aws.String("mycluster"),
			ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:u-u-i-d"),
			Endpoint: &redshift.Endpoint{
				Address: aws.String("localhost"),
				Port:    aws.Int64(5439),
			},
			Tags: []*redshift.Tag{
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
				types.OriginLabel:                 types.OriginCloud,
				labelAccountID:                    "123456789012",
				labelRegion:                       "us-east-1",
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
		require.Equal(t, expected, actual)
	})

	t.Run("success with name override", func(t *testing.T) {
		cluster := &redshift.Cluster{
			ClusterIdentifier:   aws.String("mycluster"),
			ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:u-u-i-d"),
			Endpoint: &redshift.Endpoint{
				Address: aws.String("localhost"),
				Port:    aws.Int64(5439),
			},
			Tags: []*redshift.Tag{
				{
					Key:   aws.String("key"),
					Value: aws.String("val"),
				},
				{
					Key:   aws.String("elasticbeanstalk:environment-id"),
					Value: aws.String("id"),
				},
				{
					Key:   aws.String(labelTeleportDBName),
					Value: aws.String("mycluster-override-2"),
				},
			},
		}
		expected, err := types.NewDatabaseV3(types.Metadata{
			Name:        "mycluster-override-2",
			Description: "Redshift cluster in us-east-1",
			Labels: map[string]string{
				types.OriginLabel:                 types.OriginCloud,
				labelAccountID:                    "123456789012",
				labelRegion:                       "us-east-1",
				labelTeleportDBName:               "mycluster-override-2",
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
		require.Equal(t, expected, actual)
	})

	t.Run("missing endpoint", func(t *testing.T) {
		_, err := NewDatabaseFromRedshiftCluster(&redshift.Cluster{
			ClusterIdentifier: aws.String("still-creating"),
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "Expected trace.BadParameter, got %v", err)
	})
}

func TestDatabaseFromElastiCacheConfigurationEndpoint(t *testing.T) {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(true),
		ConfigurationEndpoint: &elasticache.Endpoint{
			Address: aws.String("configuration.localhost"),
			Port:    aws.Int64(6379),
		},
		UserGroupIds: []*string{aws.String("my-user-group")},
		NodeGroups: []*elasticache.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				NodeGroupMembers: []*elasticache.NodeGroupMember{
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
				NodeGroupMembers: []*elasticache.NodeGroupMember{
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
			types.OriginLabel: types.OriginCloud,
			labelAccountID:    "123456789012",
			labelRegion:       "us-east-1",
			labelEndpointType: "configuration",
			"key":             "value",
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
	require.Equal(t, expected, actual)
}

func TestDatabaseFromElastiCacheConfigurationEndpointNameOverride(t *testing.T) {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(true),
		ConfigurationEndpoint: &elasticache.Endpoint{
			Address: aws.String("configuration.localhost"),
			Port:    aws.Int64(6379),
		},
		UserGroupIds: []*string{aws.String("my-user-group")},
		NodeGroups: []*elasticache.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				NodeGroupMembers: []*elasticache.NodeGroupMember{
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
				NodeGroupMembers: []*elasticache.NodeGroupMember{
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
		labelTeleportDBName: "my-override-cluster-2",
		"key":               "value",
	}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-override-cluster-2",
		Description: "ElastiCache cluster in us-east-1 (configuration endpoint)",
		Labels: map[string]string{
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-east-1",
			labelEndpointType:   "configuration",
			labelTeleportDBName: "my-override-cluster-2",
			"key":               "value",
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
	require.Equal(t, expected, actual)
}

func TestDatabaseFromElastiCacheNodeGroups(t *testing.T) {
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(false),
		UserGroupIds:             []*string{aws.String("my-user-group")},
		NodeGroups: []*elasticache.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				PrimaryEndpoint: &elasticache.Endpoint{
					Address: aws.String("primary.localhost"),
					Port:    aws.Int64(6379),
				},
				ReaderEndpoint: &elasticache.Endpoint{
					Address: aws.String("reader.localhost"),
					Port:    aws.Int64(6379),
				},
			},
		},
	}
	extraLabels := map[string]string{"key": "value"}

	expectedPrimary, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster",
		Description: "ElastiCache cluster in us-east-1 (primary endpoint)",
		Labels: map[string]string{
			types.OriginLabel: types.OriginCloud,
			labelAccountID:    "123456789012",
			labelRegion:       "us-east-1",
			labelEndpointType: "primary",
			"key":             "value",
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
			types.OriginLabel: types.OriginCloud,
			labelAccountID:    "123456789012",
			labelRegion:       "us-east-1",
			labelEndpointType: "reader",
			"key":             "value",
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
	cluster := &elasticache.ReplicationGroup{
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:my-cluster"),
		ReplicationGroupId:       aws.String("my-cluster"),
		Status:                   aws.String("available"),
		TransitEncryptionEnabled: aws.Bool(true),
		ClusterEnabled:           aws.Bool(false),
		UserGroupIds:             []*string{aws.String("my-user-group")},
		NodeGroups: []*elasticache.NodeGroup{
			{
				NodeGroupId: aws.String("0001"),
				PrimaryEndpoint: &elasticache.Endpoint{
					Address: aws.String("primary.localhost"),
					Port:    aws.Int64(6379),
				},
				ReaderEndpoint: &elasticache.Endpoint{
					Address: aws.String("reader.localhost"),
					Port:    aws.Int64(6379),
				},
			},
		},
	}
	extraLabels := map[string]string{
		labelTeleportDBName: "my-override-cluster-2",
		"key":               "value",
	}

	expectedPrimary, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-override-cluster-2",
		Description: "ElastiCache cluster in us-east-1 (primary endpoint)",
		Labels: map[string]string{
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-east-1",
			labelEndpointType:   "primary",
			labelTeleportDBName: "my-override-cluster-2",
			"key":               "value",
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
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-east-1",
			labelEndpointType:   "reader",
			labelTeleportDBName: "my-override-cluster-2",
			"key":               "value",
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

func TestDatabaseFromMemoryDBCluster(t *testing.T) {
	cluster := &memorydb.Cluster{
		ARN:        aws.String("arn:aws:memorydb:us-east-1:123456789012:cluster:my-cluster"),
		Name:       aws.String("my-cluster"),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ACLName:    aws.String("my-user-group"),
		ClusterEndpoint: &memorydb.Endpoint{
			Address: aws.String("memorydb.localhost"),
			Port:    aws.Int64(6379),
		},
	}
	extraLabels := map[string]string{"key": "value"}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-cluster",
		Description: "MemoryDB cluster in us-east-1",
		Labels: map[string]string{
			types.OriginLabel: types.OriginCloud,
			labelAccountID:    "123456789012",
			labelRegion:       "us-east-1",
			labelEndpointType: "cluster",
			"key":             "value",
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
	require.Equal(t, expected, actual)
}

func TestDatabaseFromMemoryDBClusterNameOverride(t *testing.T) {
	cluster := &memorydb.Cluster{
		ARN:        aws.String("arn:aws:memorydb:us-east-1:123456789012:cluster:my-cluster"),
		Name:       aws.String("my-cluster"),
		Status:     aws.String("available"),
		TLSEnabled: aws.Bool(true),
		ACLName:    aws.String("my-user-group"),
		ClusterEndpoint: &memorydb.Endpoint{
			Address: aws.String("memorydb.localhost"),
			Port:    aws.Int64(6379),
		},
	}
	extraLabels := map[string]string{
		labelTeleportDBName: "override-1",
		"key":               "value",
	}

	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "override-1",
		Description: "MemoryDB cluster in us-east-1",
		Labels: map[string]string{
			types.OriginLabel:   types.OriginCloud,
			labelAccountID:      "123456789012",
			labelRegion:         "us-east-1",
			labelEndpointType:   "cluster",
			labelTeleportDBName: "override-1",
			"key":               "value",
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
	require.Equal(t, expected, actual)
}

func TestExtraElastiCacheLabels(t *testing.T) {
	cluster := &elasticache.ReplicationGroup{
		ReplicationGroupId: aws.String("my-redis"),
	}
	tags := []*elasticache.Tag{
		{Key: aws.String("key1"), Value: aws.String("value1")},
		{Key: aws.String("key2"), Value: aws.String("value2")},
	}

	nodes := []*elasticache.CacheCluster{
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

	subnetGroups := []*elasticache.CacheSubnetGroup{
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
		inputTags         []*elasticache.Tag
		inputNodes        []*elasticache.CacheCluster
		inputSubnetGroups []*elasticache.CacheSubnetGroup
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
				"key1": "value1",
				"key2": "value2",
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
	cluster := &memorydb.Cluster{
		Name:            aws.String("my-cluster"),
		SubnetGroupName: aws.String("my-subnet-group"),
		EngineVersion:   aws.String("6.6.6"),
	}

	allSubnetGroups := []*memorydb.SubnetGroup{
		{
			Name:  aws.String("other-subnet-group"),
			VpcId: aws.String("other-vpc-id"),
		},
		{
			Name:  aws.String("my-subnet-group"),
			VpcId: aws.String("my-vpc-id"),
		},
	}

	resourceTags := []*memorydb.Tag{
		{Key: aws.String("key1"), Value: aws.String("value1")},
		{Key: aws.String("key2"), Value: aws.String("value2")},
	}

	expected := map[string]string{
		"key1":           "value1",
		"key2":           "value2",
		"engine-version": "6.6.6",
		"vpc-id":         "my-vpc-id",
	}

	actual := ExtraMemoryDBLabels(cluster, resourceTags, allSubnetGroups)
	require.Equal(t, expected, actual)
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
				labelEngine:        RDSEngineMySQL,
				labelEngineVersion: "8.0.0",
			},
			want: "8.0.0",
		},
		{
			name: "mariadb returns nothing",
			labels: map[string]string{
				labelEngine:        RDSEngineMariaDB,
				labelEngineVersion: "10.6.7",
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
				labelEngine:        AzureEngineMySQL,
				labelEngineVersion: "8.0.0",
			},
			want: "8.0.0",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := GetMySQLEngineVersion(tt.labels); got != tt.want {
				t.Errorf("GetMySQLEngineVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setDBName(t *testing.T) {
	tests := []struct {
		name           string
		meta           types.Metadata
		firstNamePart  string
		extraNameParts []string
		want           types.Metadata
	}{
		{
			name:           "no override, one part name",
			meta:           types.Metadata{},
			firstNamePart:  "foo",
			extraNameParts: nil,
			want:           types.Metadata{Name: "foo"},
		},
		{
			name:           "no override, multi part name",
			meta:           types.Metadata{},
			firstNamePart:  "foo",
			extraNameParts: []string{"bar", "baz"},
			want:           types.Metadata{Name: "foo-bar-baz"},
		},
		{
			name:           "override, one part name",
			meta:           types.Metadata{Labels: map[string]string{labelTeleportDBName: "gizmo"}},
			firstNamePart:  "foo",
			extraNameParts: nil,
			want:           types.Metadata{Name: "gizmo", Labels: map[string]string{labelTeleportDBName: "gizmo"}},
		},
		{
			name:           "override, multi part name",
			meta:           types.Metadata{Labels: map[string]string{labelTeleportDBName: "gizmo"}},
			firstNamePart:  "foo",
			extraNameParts: []string{"bar", "baz"},
			want:           types.Metadata{Name: "gizmo-bar-baz", Labels: map[string]string{labelTeleportDBName: "gizmo"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setDBName(tt.meta, tt.firstNamePart, tt.extraNameParts...)
			require.Equal(t, tt.want, result)
		})
	}
}
