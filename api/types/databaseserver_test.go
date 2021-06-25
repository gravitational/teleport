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

// TestDatabaseServerRDSEndpoint verifies AWS info is correctly populated
// based on the RDS endpoint.
func TestDatabaseServerRDSEndpoint(t *testing.T) {
	server, err := NewDatabaseServerV3("rds", nil, DatabaseServerSpecV3{
		Protocol: "postgres",
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
		Hostname: "host-1",
		HostID:   "host-1",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-west-1",
	}, server.GetAWS())
}

// TestDatabaseServerRedshiftEndpoint verifies AWS info is correctly populated
// based on the Redshift endpoint.
func TestDatabaseServerRedshiftEndpoint(t *testing.T) {
	server, err := NewDatabaseServerV3("redshift", nil, DatabaseServerSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
		Hostname: "host-1",
		HostID:   "host-1",
	})
	require.NoError(t, err)
	require.Equal(t, AWS{
		Region: "us-east-1",
		Redshift: Redshift{
			ClusterID: "redshift-cluster-1",
		},
	}, server.GetAWS())
}
