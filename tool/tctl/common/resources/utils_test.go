/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

func TestFilterBySQNOrDiscoveredName(t *testing.T) {
	unscopedFoo := mustCreateNewKubeCluster(t, "foo", "foo", nil)
	scopedFoo := mustCreateNewKubeCluster(t, "foo", "foo", nil)
	scopedFoo.Scope = "/foo"
	orthogonalFoo := mustCreateNewKubeCluster(t, "foo", "foo", nil)
	orthogonalFoo.Scope = "/bar"
	resources := []types.KubeCluster{
		unscopedFoo, scopedFoo, orthogonalFoo,
	}
	tests := []struct {
		desc   string
		filter scopes.QualifiedName
		want   []types.KubeCluster
	}{
		{
			desc: "filters by exact scoped SQN",
			filter: scopes.QualifiedName{
				Scope: "/foo",
				Name:  "foo",
			},
			want: []types.KubeCluster{scopedFoo},
		},
		{
			desc: "filters by exact unscoped SQN",
			filter: scopes.QualifiedName{
				Name: "foo",
			},
			want: []types.KubeCluster{unscopedFoo},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := FilterBySQNOrDiscoveredName(resources, test.filter)
			require.Empty(t, cmp.Diff(test.want, got))
		})
	}
}

func makeTestLabels(extraStaticLabels map[string]string) map[string]string {
	labels := make(map[string]string)
	maps.Copy(labels, staticLabelsFixture)
	maps.Copy(labels, extraStaticLabels)
	return labels
}

func mustCreateNewKubeCluster(t *testing.T, name, discoveredName string, extraStaticLabels map[string]string) *types.KubernetesClusterV3 {
	t.Helper()
	if extraStaticLabels == nil {
		extraStaticLabels = make(map[string]string)
	}
	if discoveredName != "" {
		extraStaticLabels[types.DiscoveredNameLabel] = discoveredName
	}
	cluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   name,
			Labels: makeTestLabels(extraStaticLabels),
		},
		types.KubernetesClusterSpecV3{
			DynamicLabels: map[string]types.CommandLabelV2{
				"date": {
					Period:  types.NewDuration(1 * time.Second),
					Command: []string{"date"},
					Result:  "Tue 11 Oct 2022 10:21:58 WEST",
				},
			},
		},
	)
	require.NoError(t, err)
	return cluster
}

var (
	staticLabelsFixture = map[string]string{
		"label1": "val1",
		"label2": "val2",
		"label3": "val3",
	}
)
