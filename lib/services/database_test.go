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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
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
		Port:                             aws.Int64(3306),
		TagList: []*rds.Tag{{
			Key:   aws.String("key"),
			Value: aws.String("val"),
		}},
	}
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "cluster-1",
		Description: "Aurora cluster in us-east-1",
		Labels: map[string]string{
			types.OriginLabel:  types.OriginCloud,
			labelAccountID:     "1234567890",
			labelRegion:        "us-east-1",
			labelEngine:        RDSEngineAuroraMySQL,
			labelEngineVersion: "8.0.0",
			"key":              "val",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
		AWS: types.AWS{
			AccountID: "1234567890",
			Region:    "us-east-1",
			RDS: types.RDS{
				ClusterID:  "cluster-1",
				ResourceID: "resource-1",
				IAMAuth:    true,
			},
		},
	})
	require.NoError(t, err)
	actual, err := NewDatabaseFromRDSCluster(cluster)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
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

			require.Equal(t, test.isSupported, IsRDSClusterSupported(cluster))

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
	require.Equal(t, map[string]string{"Name": "test", "Env": "dev"}, labels)
}
