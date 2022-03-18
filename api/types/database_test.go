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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDatabaseRDSEndpoint verifies AWS info is correctly populated
// based on the RDS endpoint.
func TestDatabaseRDSEndpoint(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "rds",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-west-1",
		RDS: RDS{
			InstanceID: "aurora-instance-1",
		},
	}, database.GetAWS())
}

// TestDatabaseRedshiftEndpoint verifies AWS info is correctly populated
// based on the Redshift endpoint.
func TestDatabaseRedshiftEndpoint(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "redshift",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-east-1",
		Redshift: Redshift{
			ClusterID: "redshift-cluster-1",
		},
	}, database.GetAWS())
}

// TestDatabaseStatus verifies database resource status field usage.
func TestDatabaseStatus(t *testing.T) {
	database, err := NewDatabaseV3(Metadata{
		Name: "test",
	}, DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	caCert := "test"
	database.SetStatusCA(caCert)
	require.Equal(t, caCert, database.GetCA())

	awsMeta := AWS{AccountID: "account-id"}
	database.SetStatusAWS(awsMeta)
	require.Equal(t, awsMeta, database.GetAWS())
}

// TestDatabaseElastiCacheEndpoint verifies if the Elasticache URI is correctly recognized.
func TestDatabaseElastiCacheEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		awsRedis AWSRedis
	}{
		{
			name: "cluster",
			uri:  "clustercfg.test-instance.abcdvg.use1.cache.amazonaws.com:6379",
			awsRedis: AWSRedis{
				ReplicationGroupID: "test-instance",
				EndpointType:       AWSRedis_ENDPOINT_CONFIGURATION,
				Mode:               AWSRedis_MODE_CLUSTER,
			},
		},
		{
			name: "standalone - primary",
			uri:  "master.test-instance-1.abcdvg.use1.cache.amazonaws.com:6379",
			awsRedis: AWSRedis{
				ReplicationGroupID: "test-instance-1",
				EndpointType:       AWSRedis_ENDPOINT_PRIMARY,
				Mode:               AWSRedis_MODE_SINGLE,
			},
		},
		{
			name: "standalone - reader",
			uri:  "replica.test-instance.abcdvg.use1.cache.amazonaws.com:6379",
			awsRedis: AWSRedis{
				ReplicationGroupID: "test-instance",
				EndpointType:       AWSRedis_ENDPOINT_READER,
				Mode:               AWSRedis_MODE_SINGLE,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, err := NewDatabaseV3(Metadata{
				Name: "test",
			}, DatabaseSpecV3{
				Protocol: "redis",
				URI:      tt.uri,
			})
			require.NoError(t, err)

			require.Equal(t, AWS{
				Elasticache: tt.awsRedis,
			}, database.GetAWS())
			require.True(t, database.IsElastiCache())
		})
	}
}
