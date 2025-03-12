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

func TestKubeClustersSorter(t *testing.T) {
	t.Parallel()

	makeClusters := func(testVals []string) []KubeCluster {
		servers := make([]KubeCluster, len(testVals))
		for i := 0; i < len(testVals); i++ {
			var err error
			servers[i], err = NewKubernetesClusterV3FromLegacyCluster("", &KubernetesCluster{
				Name: testVals[i],
			})
			require.NoError(t, err)
		}
		return servers
	}

	testValsUnordered := []string{"d", "b", "a", "c"}

	// Test descending.
	sortBy := SortBy{Field: ResourceMetadataName, IsDesc: true}
	clusters := KubeClusters(makeClusters(testValsUnordered))
	require.NoError(t, clusters.SortByCustom(sortBy))
	targetVals, err := clusters.GetFieldVals(ResourceMetadataName)
	require.NoError(t, err)
	require.IsDecreasing(t, targetVals)

	// Test ascending.
	sortBy = SortBy{Field: ResourceMetadataName}
	clusters = KubeClusters(makeClusters(testValsUnordered))
	require.NoError(t, clusters.SortByCustom(sortBy))
	targetVals, err = clusters.GetFieldVals(ResourceMetadataName)
	require.NoError(t, err)
	require.IsIncreasing(t, targetVals)

	// Test error.
	sortBy = SortBy{Field: "unsupported"}
	clusters = KubeClusters(makeClusters(testValsUnordered))
	require.True(t, trace.IsNotImplemented(clusters.SortByCustom(sortBy)))
}

func TestDeduplicateKubeClusters(t *testing.T) {
	t.Parallel()

	expected := []KubeCluster{
		&KubernetesClusterV3{Metadata: Metadata{Name: "a"}},
		&KubernetesClusterV3{Metadata: Metadata{Name: "b"}},
		&KubernetesClusterV3{Metadata: Metadata{Name: "c"}},
	}

	extra := []KubeCluster{
		&KubernetesClusterV3{Metadata: Metadata{Name: "a"}},
		&KubernetesClusterV3{Metadata: Metadata{Name: "a"}},
		&KubernetesClusterV3{Metadata: Metadata{Name: "b"}},
	}

	clusters := append(expected, extra...)

	result := DeduplicateKubeClusters(clusters)
	require.ElementsMatch(t, result, expected)
}
