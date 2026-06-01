/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"bytes"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

var (
	staticLabelsFixture = map[string]string{
		"label1": "val1",
		"label2": "val2",
		"label3": "val3",
	}
	longLabelFixture = map[string]string{
		"ultra_long_label_for_teleport_collection_text_table_formatting": "ultra_long_label_for_teleport_collection_text_table_formatting",
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
	require.Equal(t, "(Labels: x=[y])", databaseResourceMatchersToString(resMatch))
}

type writeTextTest struct {
	collection          resources.Collection
	wantVerboseTable    func() string
	wantNonVerboseTable func() string
}

func (test *writeTextTest) run(t *testing.T) {
	t.Helper()
	t.Run("verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := test.collection.WriteText(w, true)
		require.NoError(t, err)
		diff := cmp.Diff(test.wantVerboseTable(), w.String())
		require.Empty(t, diff)
	})
	t.Run("non-verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := test.collection.WriteText(w, false)
		require.NoError(t, err)
		diff := cmp.Diff(test.wantNonVerboseTable(), w.String())
		require.Empty(t, diff)
	})
}

func TestResourceCollection_writeText(t *testing.T) {
	t.Run("kube clusters", testKubeClusterCollection_writeText)
	t.Run("kube servers", testKubeServerCollection_writeText)
	t.Run("databases", testDatabaseCollection_writeText)
	t.Run("database servers", testDatabaseServerCollection_writeText)
	t.Run("nodes", testServerCollection_writeText)
	t.Run("apps", testAppCollection_writeText)
}

func testKubeClusterCollection_writeText(t *testing.T) {
	eksDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "cluster3",
	}
	kubeClusters := []types.KubeCluster{
		mustCreateNewKubeCluster(t, "cluster1", "", nil),
		mustCreateNewKubeCluster(t, "cluster2", "", longLabelFixture),
		mustCreateNewKubeCluster(t, "afirstCluster", "", longLabelFixture),
		mustCreateNewKubeCluster(t, "cluster3-eks-us-west-1-123456789012", "", eksDiscoveredNameLabel),
	}
	test := writeTextTest{
		collection: resources.NewKubeClusterCollection(kubeClusters),
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Name", "Labels"},
				[][]string{
					{"afirstCluster", formatTestLabels(staticLabelsFixture, longLabelFixture, false)},
					{"cluster1", formatTestLabels(staticLabelsFixture, nil, false)},
					{"cluster2", formatTestLabels(staticLabelsFixture, longLabelFixture, false)},
					{"cluster3", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, false)},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Name", "Labels"},
				[]string{"afirstCluster", formatTestLabels(staticLabelsFixture, longLabelFixture, true)},
				[]string{"cluster1", formatTestLabels(staticLabelsFixture, nil, true)},
				[]string{"cluster2", formatTestLabels(staticLabelsFixture, longLabelFixture, true)},
				[]string{"cluster3-eks-us-west-1-123456789012", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, true)},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func testKubeServerCollection_writeText(t *testing.T) {
	eksDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "cluster3",
	}
	kubeServers := []types.KubeServer{
		mustCreateNewKubeServer(t, "cluster1", "_", "", nil),
		mustCreateNewKubeServer(t, "cluster2", "_", "", longLabelFixture),
		mustCreateNewKubeServer(t, "afirstCluster", "_", "", longLabelFixture),
		mustCreateNewKubeServer(t, "cluster3-eks-us-west-1-123456789012", "_", "cluster3", nil),
	}
	test := writeTextTest{
		collection: resources.NewKubeServerCollection(kubeServers),
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Cluster", "Labels", "Version"},
				[][]string{
					{"afirstCluster", formatTestLabels(staticLabelsFixture, longLabelFixture, false), api.Version},
					{"cluster1", formatTestLabels(staticLabelsFixture, nil, false), api.Version},
					{"cluster2", formatTestLabels(staticLabelsFixture, longLabelFixture, false), api.Version},
					{"cluster3", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, false), api.Version},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Cluster", "Labels", "Version"},
				[]string{"afirstCluster", formatTestLabels(staticLabelsFixture, longLabelFixture, true), api.Version},
				[]string{"cluster1", formatTestLabels(staticLabelsFixture, nil, true), api.Version},
				[]string{"cluster2", formatTestLabels(staticLabelsFixture, longLabelFixture, true), api.Version},
				[]string{"cluster3-eks-us-west-1-123456789012", formatTestLabels(staticLabelsFixture, eksDiscoveredNameLabel, true), api.Version},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func testDatabaseCollection_writeText(t *testing.T) {
	rdsDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "database",
	}
	rdsURI := "database.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432"
	databases := []types.Database{
		mustCreateNewDatabase(t, "database-A", "mysql", "localhost:3306", nil),
		mustCreateNewDatabase(t, "database-B", "postgres", "localhost:5432", longLabelFixture),
		mustCreateNewDatabase(t, "afirstDatabase", "redis", "localhost:6379", longLabelFixture),
		mustCreateNewDatabase(t, "database-rds-us-west-1-123456789012", "postgres",
			rdsURI,
			rdsDiscoveredNameLabel),
	}
	test := writeTextTest{
		collection: resources.NewDatabaseCollection(databases),
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Name", "Protocol", "URI", "Labels"},
				[][]string{
					{"afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, longLabelFixture, false)},
					{"database", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, false)},
					{"database-A", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, false)},
					{"database-B", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, longLabelFixture, false)},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Name", "Protocol", "URI", "Labels"},
				[]string{"afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, longLabelFixture, true)},
				[]string{"database-A", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, true)},
				[]string{"database-B", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, longLabelFixture, true)},
				[]string{"database-rds-us-west-1-123456789012", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, true)},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func testDatabaseServerCollection_writeText(t *testing.T) {
	rdsDiscoveredNameLabel := map[string]string{
		types.DiscoveredNameLabel: "database",
	}
	rdsURI := "database.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432"
	dbServers := []types.DatabaseServer{
		mustCreateNewDatabaseServer(t, "database-A", "mysql", "localhost:3306", nil),
		mustCreateNewDatabaseServer(t, "database-B", "postgres", "localhost:5432", longLabelFixture),
		mustCreateNewDatabaseServer(t, "afirstDatabase", "redis", "localhost:6379", longLabelFixture),
		mustCreateNewDatabaseServer(t, "database-rds-us-west-1-123456789012", "postgres",
			rdsURI,
			rdsDiscoveredNameLabel),
	}
	test := writeTextTest{
		collection: &databaseServerCollection{servers: dbServers},
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Host", "Name", "Protocol", "URI", "Labels", "Version"},
				[][]string{
					{"some-host", "afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, longLabelFixture, false), api.Version},
					{"some-host", "database", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, false), api.Version},
					{"some-host", "database-A", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, false), api.Version},
					{"some-host", "database-B", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, longLabelFixture, false), api.Version},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Host", "Name", "Protocol", "URI", "Labels", "Version"},
				[]string{"some-host", "afirstDatabase", "redis", "localhost:6379", formatTestLabels(staticLabelsFixture, longLabelFixture, true), api.Version},
				[]string{"some-host", "database-A", "mysql", "localhost:3306", formatTestLabels(staticLabelsFixture, nil, true), api.Version},
				[]string{"some-host", "database-B", "postgres", "localhost:5432", formatTestLabels(staticLabelsFixture, longLabelFixture, true), api.Version},
				[]string{"some-host", "database-rds-us-west-1-123456789012", "postgres", rdsURI, formatTestLabels(staticLabelsFixture, rdsDiscoveredNameLabel, true), api.Version},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func mustCreateNewDatabase(t *testing.T, name, protocol, uri string, extraStaticLabels map[string]string) *types.DatabaseV3 {
	t.Helper()
	db, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   name,
			Labels: makeTestLabels(extraStaticLabels),
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

func mustCreateNewDatabaseServer(t *testing.T, name, protocol, uri string, extraStaticLabels map[string]string) types.DatabaseServer {
	t.Helper()
	dbServer, err := types.NewDatabaseServerV3(
		types.Metadata{
			Name:   name,
			Labels: makeTestLabels(extraStaticLabels),
		}, types.DatabaseServerSpecV3{
			HostID:   "some-hostid",
			Hostname: "some-host",
			Database: mustCreateNewDatabase(t, name, protocol, uri, extraStaticLabels),
		})
	require.NoError(t, err)

	return dbServer
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

func testServerCollection_writeText(t *testing.T) {
	nodes := []types.Server{
		mustCreateNewServer(t, "uuid-1", "node-B", "addr-b:22", nil),
		mustCreateNewServer(t, "uuid-2", "node-C", "addr-c:22", longLabelFixture),
		mustCreateNewServer(t, "uuid-3", "aFirstNode", "addr-a:22", longLabelFixture),
		mustCreateNewServer(t, "uuid-4", "node-D", "addr-d:22", nil),
	}
	// servers have only static labels (no dynamic "date" label), so format directly
	fmtLabels := func(extra map[string]string, verbose bool) string {
		return common.FormatLabels(makeTestLabels(extra), verbose)
	}
	test := writeTextTest{
		collection: resources.NewServerCollection(nodes),
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Host", "UUID", "Public Address", "Labels", "Version"},
				[][]string{
					{"aFirstNode", "uuid-3", "addr-a:22", fmtLabels(longLabelFixture, false), api.Version},
					{"node-B", "uuid-1", "addr-b:22", fmtLabels(nil, false), api.Version},
					{"node-C", "uuid-2", "addr-c:22", fmtLabels(longLabelFixture, false), api.Version},
					{"node-D", "uuid-4", "addr-d:22", fmtLabels(nil, false), api.Version},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Host", "UUID", "Public Address", "Labels", "Version"},
				[]string{"aFirstNode", "uuid-3", "addr-a:22", fmtLabels(longLabelFixture, true), api.Version},
				[]string{"node-B", "uuid-1", "addr-b:22", fmtLabels(nil, true), api.Version},
				[]string{"node-C", "uuid-2", "addr-c:22", fmtLabels(longLabelFixture, true), api.Version},
				[]string{"node-D", "uuid-4", "addr-d:22", fmtLabels(nil, true), api.Version},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func testAppCollection_writeText(t *testing.T) {
	apps := []types.Application{
		mustCreateNewApp(t, "app-B", "desc-b", "https://b.example.com", "b.example.com", nil),
		mustCreateNewApp(t, "app-C", "desc-c", "https://c.example.com", "c.example.com", longLabelFixture),
		mustCreateNewApp(t, "aFirstApp", "desc-a", "https://a.example.com", "a.example.com", longLabelFixture),
		mustCreateNewApp(t, "app-D", "desc-d", "https://d.example.com", "d.example.com", nil),
	}
	// apps have only static labels, so format directly without the dynamic "date" label
	fmtLabels := func(extra map[string]string, verbose bool) string {
		return common.FormatLabels(makeTestLabels(extra), verbose)
	}
	test := writeTextTest{
		collection: resources.NewAppCollection(apps),
		wantNonVerboseTable: func() string {
			table := asciitable.MakeTableWithTruncatedColumn(
				[]string{"Name", "Description", "URI", "Public Address", "Labels", "Version"},
				[][]string{
					{"aFirstApp", "desc-a", "https://a.example.com", "a.example.com", fmtLabels(longLabelFixture, false), "v3"},
					{"app-B", "desc-b", "https://b.example.com", "b.example.com", fmtLabels(nil, false), "v3"},
					{"app-C", "desc-c", "https://c.example.com", "c.example.com", fmtLabels(longLabelFixture, false), "v3"},
					{"app-D", "desc-d", "https://d.example.com", "d.example.com", fmtLabels(nil, false), "v3"},
				},
				"Labels")
			return table.AsBuffer().String()
		},
		wantVerboseTable: func() string {
			table := asciitable.MakeTable(
				[]string{"Name", "Description", "URI", "Public Address", "Labels", "Version"},
				[]string{"aFirstApp", "desc-a", "https://a.example.com", "a.example.com", fmtLabels(longLabelFixture, true), "v3"},
				[]string{"app-B", "desc-b", "https://b.example.com", "b.example.com", fmtLabels(nil, true), "v3"},
				[]string{"app-C", "desc-c", "https://c.example.com", "c.example.com", fmtLabels(longLabelFixture, true), "v3"},
				[]string{"app-D", "desc-d", "https://d.example.com", "d.example.com", fmtLabels(nil, true), "v3"},
			)
			return table.AsBuffer().String()
		},
	}
	test.run(t)
}

func mustCreateNewApp(t *testing.T, name, description, uri, publicAddr string, extraStaticLabels map[string]string) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name:        name,
		Description: description,
		Labels:      makeTestLabels(extraStaticLabels),
	}, types.AppSpecV3{
		URI:        uri,
		PublicAddr: publicAddr,
	})
	require.NoError(t, err)
	return app
}

func mustCreateNewServer(t *testing.T, uuid, hostname, addr string, extraStaticLabels map[string]string) types.Server {
	t.Helper()
	server, err := types.NewServer(uuid, types.KindNode, types.ServerSpecV2{
		Hostname: hostname,
		Addr:     addr,
		Version:  api.Version,
	})
	require.NoError(t, err)
	server.SetStaticLabels(makeTestLabels(extraStaticLabels))
	return server
}

func mustCreateNewKubeServer(t *testing.T, name, hostname, discoveredName string, extraStaticLabels map[string]string) *types.KubernetesServerV3 {
	t.Helper()
	cluster := mustCreateNewKubeCluster(t, name, discoveredName, extraStaticLabels)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(cluster, hostname, uuid.New().String())
	require.NoError(t, err)
	return kubeServer
}

func formatTestLabels(l1, l2 map[string]string, verbose bool) string {
	labels := map[string]string{
		"date": "Tue 11 Oct 2022 10:21:58 WEST",
	}

	maps.Copy(labels, l1)
	maps.Copy(labels, l2)
	return common.FormatLabels(labels, verbose)
}

func makeTestLabels(extraStaticLabels map[string]string) map[string]string {
	labels := make(map[string]string)
	maps.Copy(labels, staticLabelsFixture)
	maps.Copy(labels, extraStaticLabels)
	return labels
}
