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
	"google.golang.org/protobuf/types/known/durationpb"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/tool/common"
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
		diff := cmp.Diff(test.wantVerboseTable(), w.String())
		require.Empty(t, diff)
	})
	t.Run("non-verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := test.collection.writeText(w, false)
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
		collection: &kubeClusterCollection{clusters: kubeClusters},
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
		collection: &kubeServerCollection{servers: kubeServers},
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
		collection: &databaseCollection{databases: databases},
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

func TestDatabaseImportRuleCollection_writeText(t *testing.T) {
	mkRule := func(name string) *dbobjectimportrulev1.DatabaseObjectImportRule {
		r, err := databaseobjectimportrule.NewDatabaseObjectImportRule(name, &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
			Priority: 123,
			DatabaseLabels: label.FromMap(map[string][]string{
				"foo":   {"bar"},
				"beast": {"dragon", "phoenix"},
			}),
			Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{
				{
					Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
						TableNames: []string{"dummy"},
					},
					AddLabels: map[string]string{
						"dummy_table": "true",
						"another":     "label"},
				},
			},
		})
		require.NoError(t, err)
		return r
	}

	rules := []*dbobjectimportrulev1.DatabaseObjectImportRule{
		mkRule("rule_1"),
		mkRule("rule_2"),
		mkRule("rule_3"),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Priority", "Mapping Count", "DB Label Count"},
		[]string{"rule_1", "123", "1", "2"},
		[]string{"rule_2", "123", "1", "2"},
		[]string{"rule_3", "123", "1", "2"},
	)

	formatted := table.AsBuffer().String()

	test := writeTextTest{
		collection:          &databaseObjectImportRuleCollection{rules},
		wantVerboseTable:    func() string { return formatted },
		wantNonVerboseTable: func() string { return formatted },
	}
	test.run(t)
}

func TestDatabaseObjectCollection_writeText(t *testing.T) {
	mkObj := func(name string) *dbobjectv1.DatabaseObject {
		r, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{
			Name:                name,
			Protocol:            "postgres",
			DatabaseServiceName: "pg",
			ObjectKind:          "table",
		})
		require.NoError(t, err)
		return r
	}

	items := []*dbobjectv1.DatabaseObject{
		mkObj("object_1"),
		mkObj("object_2"),
		mkObj("object_3"),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Kind", "DB Service", "Protocol"},
		[]string{"object_1", "table", "pg", "postgres"},
		[]string{"object_2", "table", "pg", "postgres"},
		[]string{"object_3", "table", "pg", "postgres"},
	)

	formatted := table.AsBuffer().String()

	test := writeTextTest{
		collection:          &databaseObjectCollection{items},
		wantVerboseTable:    func() string { return formatted },
		wantNonVerboseTable: func() string { return formatted },
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

// autoUpdateConfigBrokenCollection is an intentionally broken version of the
// autoUpdateConfigCollection that is not marshaling resources properly because
// it's doing json marshaling instead of protojson marshaling.
type autoUpdateConfigBrokenCollection struct {
	autoUpdateConfigCollection
}

func (c *autoUpdateConfigBrokenCollection) resources() []types.Resource {
	// We use Resource153ToLegacy instead of ProtoResource153ToLegacy.
	return []types.Resource{types.Resource153ToLegacy(c.config)}
}

// This test makes sure we marshal and unmarshal proto-based Resource153 properly.
// We had a bug where types.Resource153 implemented by protobuf structs were not
// marshaled properly (they should be marshaled using protojson). This test
// checks we can do a round-trip with one of those proto-struct resource.
func TestRoundTripProtoResource153(t *testing.T) {
	// Test setup: generate fixture.
	initial, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Agents: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
			Mode:                      autoupdate.AgentsUpdateModeEnabled,
			Strategy:                  autoupdate.AgentsStrategyTimeBased,
			MaintenanceWindowDuration: durationpb.New(1 * time.Hour),
			Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
					{
						Name: "group1",
						Days: []string{types.Wildcard},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Test execution: dump the resource into a YAML manifest.
	collection := &autoUpdateConfigCollection{config: initial}
	buf := &bytes.Buffer{}
	require.NoError(t, writeYAML(collection, buf))

	// Test execution: load the YAML manifest back.
	decoder := kyaml.NewYAMLOrJSONDecoder(buf, defaults.LookaheadBufSize)
	var raw services.UnknownResource
	require.NoError(t, decoder.Decode(&raw))
	result, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw)
	require.NoError(t, err)

	// Test validation: check that the loaded content matches what we had before.
	require.Equal(t, result, initial)

	// Test execution: now dump the resource into a YAML manifest with a
	// collection using types.Resource153ToLegacy instead of types.ProtoResource153ToLegacy
	brokenCollection := &autoUpdateConfigBrokenCollection{autoUpdateConfigCollection{initial}}
	buf = &bytes.Buffer{}
	require.NoError(t, writeYAML(brokenCollection, buf))

	// Test execution: load the YAML manifest back and see that we can't unmarshal it.
	decoder = kyaml.NewYAMLOrJSONDecoder(buf, defaults.LookaheadBufSize)
	require.NoError(t, decoder.Decode(&raw))
	_, err = services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw)
	require.Error(t, err)
}
