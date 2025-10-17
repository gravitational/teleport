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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestGetOneResourceNameToDelete(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo-eks", "host-foo1", "foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo-eks", "host-foo2", "foo", nil)
	fooBar1 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-1", "host-foo-bar1", "foo-bar", nil)
	fooBar2 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-2", "host-foo-bar2", "foo-bar", nil)
	tests := []struct {
		desc            string
		refName         string
		wantErrContains string
		resources       []types.KubeServer
		wantName        string
	}{
		{
			desc:      "one resource is ok",
			refName:   "foo-bar-eks-us-west-1",
			resources: []types.KubeServer{fooBar1},
			wantName:  "foo-bar-eks-us-west-1",
		},
		{
			desc:      "multiple resources with same name is ok",
			refName:   "foo",
			resources: []types.KubeServer{foo1, foo2},
			wantName:  "foo-eks",
		},
		{
			desc:            "zero resources is an error",
			refName:         "xxx",
			wantErrContains: `kubernetes server "xxx" not found`,
		},
		{
			desc:            "multiple resources with different names is an error",
			refName:         "foo-bar",
			resources:       []types.KubeServer{fooBar1, fooBar2},
			wantErrContains: "matches multiple",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ref := services.Ref{Kind: types.KindKubeServer, Name: test.refName}
			resDesc := "kubernetes server"
			name, err := GetOneResourceNameToDelete(test.resources, ref, resDesc)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.Equal(t, test.wantName, name)
		})
	}
}

func TestFilterByNameOrDiscoveredName(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo-eks-us-west-1", "host-foo", "foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo-eks-us-west-2", "host-foo", "foo", nil)
	fooBar1 := mustCreateNewKubeServer(t, "foo-bar", "host-foo-bar1", "", nil)
	fooBar2 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-2", "host-foo-bar2", "foo-bar", nil)
	resources := []types.KubeServer{
		foo1, foo2, fooBar1, fooBar2,
	}
	hostNameGetter := func(ks types.KubeServer) string { return ks.GetHostname() }
	tests := []struct {
		desc           string
		filter         string
		altNameGetters []AltResourceNameFunc[types.KubeServer]
		want           []types.KubeServer
	}{
		{
			desc:   "filters by exact name",
			filter: "foo-eks-us-west-1",
			want:   []types.KubeServer{foo1},
		},
		{
			desc:   "filters by exact name over discovered names",
			filter: "foo-bar",
			want:   []types.KubeServer{fooBar1},
		},
		{
			desc:   "filters by discovered name",
			filter: "foo",
			want:   []types.KubeServer{foo1, foo2},
		},
		{
			desc:           "checks alt names for exact matches",
			filter:         "host-foo",
			altNameGetters: []AltResourceNameFunc[types.KubeServer]{hostNameGetter},
			want:           []types.KubeServer{foo1, foo2},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := FilterByNameOrDiscoveredName(resources, test.filter, test.altNameGetters...)
			require.Empty(t, cmp.Diff(test.want, got))
		})
	}
}

func TestFormatAmbiguousDeleteMessage(t *testing.T) {
	ref := services.Ref{Kind: types.KindDatabase, Name: "x"}
	resDesc := "database"
	names := []string{"xbbb", "xaaa", "xccc", "xb"}
	got := formatAmbiguousDeleteMessage(ref, resDesc, names)
	require.Contains(t, got, "db/x matches multiple auto-discovered databases",
		"should have formatted the ref used and pluralized the resource description")
	wantSortedNames := strings.Join([]string{"xaaa", "xb", "xbbb", "xccc"}, "\n")
	require.Contains(t, got, wantSortedNames, "should have sorted the matching names")
	require.Contains(t, got, "$ tctl rm db/xaaa", "should have contained an example command")
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

func mustCreateNewKubeServer(t *testing.T, name, hostname, discoveredName string, extraStaticLabels map[string]string) *types.KubernetesServerV3 {
	t.Helper()
	cluster := mustCreateNewKubeCluster(t, name, discoveredName, extraStaticLabels)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(cluster, hostname, uuid.New().String())
	require.NoError(t, err)
	return kubeServer
}

var (
	staticLabelsFixture = map[string]string{
		"label1": "val1",
		"label2": "val2",
		"label3": "val3",
	}
)
