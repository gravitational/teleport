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

func TestMySQLVersionValidation(t *testing.T) {
	t.Parallel()

	t.Run("correct config", func(t *testing.T) {
		database, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "mysql",
			URI:      "localhost:5432",
			MySQL: MySQLOptions{
				ServerVersion: "8.0.18",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "8.0.18", database.GetMySQLServerVersion())
	})

	t.Run("incorrect config - wrong protocol", func(t *testing.T) {
		_, err := NewDatabaseV3(Metadata{
			Name: "test",
		}, DatabaseSpecV3{
			Protocol: "Postgres",
			URI:      "localhost:5432",
			MySQL: MySQLOptions{
				ServerVersion: "8.0.18",
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "ServerVersion")
	})
}

func TestMySQLServerVersion(t *testing.T) {
	t.Parallel()

	database, err := NewDatabaseV3(Metadata{
		Name: "test",
	}, DatabaseSpecV3{
		Protocol: "mysql",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	require.Equal(t, "", database.GetMySQLServerVersion())

	database.SetMySQLServerVersion("8.0.1")
	require.Equal(t, "8.0.1", database.GetMySQLServerVersion())
}
