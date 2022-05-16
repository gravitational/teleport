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

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/stretchr/testify/require"
)

// TestDatabaseUnmarshal verifies a database resource can be unmarshaled.
func TestDatabaseUnmarshal(t *testing.T) {
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
	data, err := utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, indent(fixtures.TLSCACertPEM, 4))))
	require.NoError(t, err)
	actual, err := UnmarshalDatabase(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
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

var databaseYAML = `kind: db
version: v3
metadata:
  name: test-database
  description: "Test description"
  labels:
    env: dev
spec:
  protocol: "postgres"
  uri: "localhost:5432"
  ca_cert: |
%v`

// TestDatabaseFromRDSInstance tests converting an RDS instance to a database resource.
func TestDatabaseFromRDSInstance(t *testing.T) {
	instance := &rds.DBInstance{
		DBInstanceArn:                    aws.String("arn:aws:rds:us-west-1:1234567890:db:instance-1"),
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
			labelAccountID:     "1234567890",
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
			AccountID: "1234567890",
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
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:1234567890:cluster:cluster-1"),
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
		AccountID: "1234567890",
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
				labelAccountID:     "1234567890",
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
				labelAccountID:     "1234567890",
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
			labelAccountID:     "1234567890",
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
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:1234567890:cluster:test"),
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
				DBInstanceArn:       aws.String("arn:aws:rds:us-east-1:1234567890:instance:test"),
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

func TestRDSTagsToLabels(t *testing.T) {
	rdsTags := []*rds.Tag{
		&rds.Tag{
			Key:   aws.String("Env"),
			Value: aws.String("dev"),
		},
		&rds.Tag{
			Key:   aws.String("aws:cloudformation:stack-id"),
			Value: aws.String("some-id"),
		},
		&rds.Tag{
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
			ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:1234567890:namespace:u-u-i-d"),
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
				labelAccountID:                    "1234567890",
				labelRegion:                       "us-east-1",
				"key":                             "val",
				"elasticbeanstalk:environment-id": "id",
			},
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5439",
			AWS: types.AWS{
				AccountID: "1234567890",
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
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:1234567890:replicationgroup:my-cluster"),
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
			labelAccountID:    "1234567890",
			labelRegion:       "us-east-1",
			labelEndpointType: "configuration",
			"key":             "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "configuration.localhost:6379",
		AWS: types.AWS{
			AccountID: "1234567890",
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
		ARN:                      aws.String("arn:aws:elasticache:us-east-1:1234567890:replicationgroup:my-cluster"),
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
			labelAccountID:    "1234567890",
			labelRegion:       "us-east-1",
			labelEndpointType: "primary",
			"key":             "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "primary.localhost:6379",
		AWS: types.AWS{
			AccountID: "1234567890",
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
			labelAccountID:    "1234567890",
			labelRegion:       "us-east-1",
			labelEndpointType: "reader",
			"key":             "value",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "reader.localhost:6379",
		AWS: types.AWS{
			AccountID: "1234567890",
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
