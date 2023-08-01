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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

var (
	staticLabelsFixture = map[string]string{
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

type writeTextTest struct {
	collection          ResourceCollection
	wantVerboseTable    func() string
	wantNonVerboseTable func() string
}

func (test *writeTextTest) run(t *testing.T) {
	t.Helper()
	t.Run("verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := test.collection.writeText(w, true)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(test.wantVerboseTable(), w.String()))
	})
	t.Run("non-verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := test.collection.writeText(w, false)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(test.wantNonVerboseTable(), w.String()))
	})
}

func TestResourceCollection_writeText(t *testing.T) {
	t.Run("kube clusters", testKubeClusterCollection_writeText)
	t.Run("databases", testKubeClusterCollection_writeText)
}

func testKubeClusterCollection_writeText(t *testing.T) {
	extraLabel := map[string]string{
		"ultra_long_label_for_teleport_kubernetes_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_list_kube_clusters_method",
	}
	eksDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "cluster3",
	}
	kubeClusters := []types.KubeCluster{
		mustCreateNewKubeCluster(t, "cluster1", nil),
		mustCreateNewKubeCluster(t, "cluster2", extraLabel),
		mustCreateNewKubeCluster(t, "afirstCluster", extraLabel),
		mustCreateNewKubeCluster(t, "cluster3-eks-us-west-1-123456789012", eksDiscoveredNameLabel),
	}
	test := writeTextTest{
		collection: &kubeClusterCollection{clusters: kubeClusters},
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Name", "Labels"},
				[][]string{
					{"afirstCluster", formatTestLabels(staticLabelsFixture, extraLabel, false)},
					{"cluster1", formatTestLabels(staticLabelsFixture, nil, false)},
					{"cluster2", formatTestLabels(staticLabelsFixture, extraLabel, false)},
					{"cluster3", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, false)},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Name", "Labels"},
				[]string{"afirstCluster", formatTestLabels(staticLabelsFixture, extraLabel, true)},
				[]string{"cluster1", formatTestLabels(staticLabelsFixture, nil, true)},
				[]string{"cluster2", formatTestLabels(staticLabelsFixture, extraLabel, true)},
				[]string{"cluster3-eks-us-west-1-123456789012", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, false)},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func testDatabaseCollection_writeText(t *testing.T) {
	rdsDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "database3",
	}
	rdsURI := "database3.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432"
	databases := []types.Database{
		mustCreateNewDatabase(t, "database1", "mysql", "localhost:3306", nil),
		mustCreateNewDatabase(t, "database2", "postgres", "localhost:5432", nil),
		mustCreateNewDatabase(t, "afirstDatabase", "redis", "localhost:6379", nil),
		mustCreateNewDatabase(t, "database3-rds-us-west-1-123456789012", "postgres",
			rdsURI,
			rdsDiscoveredNameLabel),
	}
	test := writeTextTest{
		collection: &databaseCollection{databases: databases},
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Name", "Protocol", "URI", "Labels"},
				[][]string{
					{"afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, nil, false)},
					{"database1", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, false)},
					{"database2", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, nil, false)},
					{"database3", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, false)},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Name", "Protocol", "URI", "Labels"},
				[]string{"afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, nil, false)},
				[]string{"database1", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, false)},
				[]string{"database2", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, nil, false)},
				[]string{"database3-rds-us-west-1-123456789012", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, false)},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func mustCreateNewDatabase(t *testing.T, name, protocol, uri string, extraStaticLabels map[string]string) types.Database {
	t.Helper()
	labels := make(map[string]string)

	for k, v := range staticLabelsFixture {
		labels[k] = v
	}

	for k, v := range extraStaticLabels {
		labels[k] = v
	}

	db, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   name,
			Labels: labels,
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
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
	return db
}

func mustCreateNewKubeCluster(t *testing.T, name string, extraStaticLabels map[string]string) types.KubeCluster {
	t.Helper()
	labels := make(map[string]string)

	for k, v := range staticLabelsFixture {
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
	return stripInternalTeleportLabels(verbose, labels)
}
