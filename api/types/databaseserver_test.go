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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestDatabaseServerGetDatabase verifies that older agents get adapted to
// the new database server interface for backward compatibility.
//
// DELETE IN 9.0.
func TestDatabaseServerGetDatabase(t *testing.T) {
	server, err := NewDatabaseServerV3(Metadata{
		Name:   "server-1",
		Labels: map[string]string{"a": "b"},
	}, DatabaseServerSpecV3{
		Description:   "description",
		Protocol:      "postgres",
		URI:           "localhost:5432",
		CACert:        []byte("cert"),
		AWS:           AWS{Region: "us-east-1", Redshift: Redshift{ClusterID: "cluster-1"}},
		Version:       "1.0.0",
		Hostname:      "host",
		HostID:        "host-1",
		DynamicLabels: map[string]CommandLabelV2{"c": {Period: Duration(time.Minute), Command: []string{"/bin/date"}}},
		GCP:           GCPCloudSQL{ProjectID: "project-1", InstanceID: "instance-1"},
	})
	require.NoError(t, err)
	database, err := NewDatabaseV3(Metadata{
		Name:        "server-1",
		Description: "description",
		Labels:      map[string]string{"a": "b"},
	}, DatabaseSpecV3{
		Protocol:      "postgres",
		URI:           "localhost:5432",
		CACert:        "cert",
		AWS:           AWS{Region: "us-east-1", Redshift: Redshift{ClusterID: "cluster-1"}},
		DynamicLabels: map[string]CommandLabelV2{"c": {Period: Duration(time.Minute), Command: []string{"/bin/date"}}},
		GCP:           GCPCloudSQL{ProjectID: "project-1", InstanceID: "instance-1"},
	})
	require.NoError(t, err)
	require.Equal(t, database, server.GetDatabase())
}

func TestDatabaseServerSorter(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c"}

	// DB types are hardcoded and types are determined
	// by which spec fields are set, values don't matter.
	// Used to randomly assign db types.
	dbSpecs := []DatabaseSpecV3{
		// type redshift
		{
			Protocol: "_",
			URI:      "_",
			AWS: AWS{
				Redshift: Redshift{
					ClusterID: "_",
				},
			},
		},
		// type azure
		{
			Protocol: "_",
			URI:      "_",
			Azure: Azure{
				Name: "_",
			},
		},
		// type rds
		{
			Protocol: "_",
			URI:      "_",
			AWS: AWS{
				Region: "_",
			},
		},
		// type gcp
		{
			Protocol: "_",
			URI:      "_",
			GCP: GCPCloudSQL{
				ProjectID:  "_",
				InstanceID: "_",
			},
		},
	}

	cases := []struct {
		name      string
		wantErr   bool
		fieldName string
	}{
		{
			name:      "by name",
			fieldName: ResourceMetadataName,
		},
		{
			name:      "by description",
			fieldName: ResourceSpecDescription,
		},
		{
			name:      "by type",
			fieldName: ResourceSpecType,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s desc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName, IsDesc: true}
			servers := DatabaseServers(makeServers(t, testValsUnordered, dbSpecs, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsDecreasing(t, targetVals)
		})

		t.Run(fmt.Sprintf("%s asc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName}
			servers := DatabaseServers(makeServers(t, testValsUnordered, dbSpecs, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsIncreasing(t, targetVals)
		})
	}

	// Test error.
	sortBy := SortBy{Field: "unsupported"}
	servers := makeServers(t, testValsUnordered, dbSpecs, "does-not-matter")
	require.True(t, trace.IsNotImplemented(DatabaseServers(servers).SortByCustom(sortBy)))
}

func makeServers(t *testing.T, testVals []string, dbSpecs []DatabaseSpecV3, testField string) []DatabaseServer {
	t.Helper()
	servers := make([]DatabaseServer, len(testVals))
	for i := 0; i < len(testVals); i++ {
		testVal := testVals[i]
		dbSpec := dbSpecs[i%len(dbSpecs)]
		var err error

		servers[i], err = NewDatabaseServerV3(Metadata{
			Name: "foo",
		}, DatabaseServerSpecV3{
			HostID:   "_",
			Hostname: "_",
			Database: &DatabaseV3{
				Metadata: Metadata{
					Name:        getTestVal(testField == ResourceMetadataName, testVal),
					Description: getTestVal(testField == ResourceSpecDescription, testVal),
				},
				Spec: dbSpec,
			},
		})
		require.NoError(t, err)
	}
	return servers
}
