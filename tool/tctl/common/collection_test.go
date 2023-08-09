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

package common

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
)

var (
	staticLabels = map[string]string{
		"label1": "val1",
		"label2": "val2",
		"label3": "val3",
	}
)

func TestDatabaseResourceMatchersToString(t *testing.T) {
	resMatch := []*types.DatabaseResourceMatcher{
		nil,
		{
			Labels: nil,
		},
		{
			Labels: &types.Labels{
				"x": []string{"y"},
			},
		},
	}
	require.Equal(t, databaseResourceMatchersToString(resMatch), "(Labels: x=[y])")
}

func Test_kubeClusterCollection_writeText(t *testing.T) {
	extraLabel := map[string]string{
		"ultra_long_label_for_teleport_kubernetes_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_list_kube_clusters_method",
	}
	kubeClusters := []types.KubeCluster{
		mustCreateNewKubeCluster(t, "cluster1", nil),
		mustCreateNewKubeCluster(t, "cluster2", extraLabel),
		mustCreateNewKubeCluster(t, "afirstCluster", extraLabel),
	}
	type fields struct {
		verbose bool
	}
	tests := []struct {
		name      string
		fields    fields
		wantTable func() string
	}{
		{
			name:   "non-verbose mode",
			fields: fields{verbose: false},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Labels"},
					[][]string{
						{"afirstCluster", formatTestLabels(staticLabels, extraLabel, false)},
						{"cluster1", formatTestLabels(staticLabels, nil, false)},
						{"cluster2", formatTestLabels(staticLabels, extraLabel, false)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name:   "verbose mode",
			fields: fields{verbose: true},
			wantTable: func() string {
				table := asciitable.MakeTable(
					[]string{"Name", "Labels"},
					[]string{"afirstCluster", formatTestLabels(staticLabels, extraLabel, true)},
					[]string{"cluster1", formatTestLabels(staticLabels, nil, true)},
					[]string{"cluster2", formatTestLabels(staticLabels, extraLabel, true)},
				)
				return table.AsBuffer().String()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &kubeClusterCollection{
				clusters: kubeClusters,
			}
			w := &bytes.Buffer{}
			err := c.writeText(w, tt.fields.verbose)
			require.NoError(t, err)
			require.Contains(t, w.String(), tt.wantTable())
		})
	}
}

func mustCreateNewKubeCluster(t *testing.T, name string, extraStaticLabels map[string]string) types.KubeCluster {
	labels := make(map[string]string)

	for k, v := range staticLabels {
		labels[k] = v
	}

	for k, v := range extraStaticLabels {
		labels[k] = v
	}

	cluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   name,
			Labels: labels,
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

func formatTestLabels(l1, l2 map[string]string, verbose bool) string {
	labels := map[string]string{
		"date": "Tue 11 Oct 2022 10:21:58 WEST",
	}

	for key, value := range l1 {
		labels[key] = value
	}
	for key, value := range l2 {
		labels[key] = value
	}
	return common.FormatLabels(labels, verbose)
}
