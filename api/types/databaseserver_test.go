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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

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

func TestDatabaseServersToDatabases(t *testing.T) {
	t.Parallel()

	databaseServers := []DatabaseServer{
		makeDatabaseServer(t, "db1", "agent1"),
		makeDatabaseServer(t, "db1", "agent2"),
		makeDatabaseServer(t, "db2", "agent1"),
		makeDatabaseServer(t, "db3", "agent2"),
		makeDatabaseServer(t, "db3", "agent3"),
	}

	wantDatabases := []Database{
		databaseServers[0].GetDatabase(), // db1
		databaseServers[2].GetDatabase(), // db2
		databaseServers[3].GetDatabase(), // db3
	}

	actualDatabases := DatabaseServers(databaseServers).ToDatabases()
	require.Equal(t, wantDatabases, actualDatabases)
}

func makeDatabaseServer(t *testing.T, dbName, agentName string) DatabaseServer {
	t.Helper()

	databaseServer, err := NewDatabaseServerV3(Metadata{
		Name: dbName,
	}, DatabaseServerSpecV3{
		HostID:   agentName,
		Hostname: agentName,
		Database: &DatabaseV3{
			Metadata: Metadata{
				Name: dbName,
			},
			Spec: DatabaseSpecV3{
				Protocol: "postgres",
				URI:      "localhost:5432",
			},
		},
	})
	require.NoError(t, err)
	return databaseServer
}
