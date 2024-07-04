/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestKubeServerSorter(t *testing.T) {
	t.Parallel()

	makeClusters := func(testVals []string) []KubeServer {
		servers := make([]KubeServer, len(testVals))
		for i := 0; i < len(testVals); i++ {
			var err error
			servers[i], err = NewKubernetesServerV3(Metadata{Name: testVals[i]}, KubernetesServerSpecV3{
				Hostname: "ss",
				HostID:   "hostid",
				Cluster: &KubernetesClusterV3{
					Metadata: Metadata{Name: testVals[i]},
				},
			})
			require.NoError(t, err)
		}
		return servers
	}

	testValsUnordered := []string{"d", "b", "a", "c"}

	// Test descending.
	sortBy := SortBy{Field: ResourceMetadataName, IsDesc: true}
	clusters := KubeServers(makeClusters(testValsUnordered))
	require.NoError(t, clusters.SortByCustom(sortBy))
	targetVals, err := clusters.GetFieldVals(ResourceMetadataName)
	require.NoError(t, err)
	require.IsDecreasing(t, targetVals)

	// Test ascending.
	sortBy = SortBy{Field: ResourceMetadataName}
	clusters = KubeServers(makeClusters(testValsUnordered))
	require.NoError(t, clusters.SortByCustom(sortBy))
	targetVals, err = clusters.GetFieldVals(ResourceMetadataName)
	require.NoError(t, err)
	require.IsIncreasing(t, targetVals)

	// Test error.
	sortBy = SortBy{Field: "unsupported"}
	clusters = KubeServers(makeClusters(testValsUnordered))
	require.True(t, trace.IsNotImplemented(clusters.SortByCustom(sortBy)))
}
