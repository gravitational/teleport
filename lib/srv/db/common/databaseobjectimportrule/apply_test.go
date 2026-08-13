// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package databaseobjectimportrule

import (
	"context"
	"log/slog"
	"maps"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	databaseobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
)

func mkDatabase(t *testing.T, name string, labels map[string]string) *types.DatabaseV3 {
	t.Helper()
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5252",
	})
	require.NoError(t, err)
	return db
}

type option func(db *dbobjectv1.DatabaseObject) error

func mkDatabaseObject(t *testing.T, name string, spec *dbobjectv1.DatabaseObjectSpec, options ...option) *dbobjectv1.DatabaseObject {
	t.Helper()
	spec.SetName(name)
	out, err := databaseobject.NewDatabaseObject(name, spec)
	require.NoError(t, err)
	for _, opt := range options {
		require.NoError(t, opt(out))
	}

	return out
}

func mkImportRule(t *testing.T, name string, spec *databaseobjectimportrulev1.DatabaseObjectImportRuleSpec) *databaseobjectimportrulev1.DatabaseObjectImportRule {
	t.Helper()
	out, err := NewDatabaseObjectImportRule(name, spec)
	require.NoError(t, err)
	return out
}

func mkImportRuleNoValidation(name string, spec *databaseobjectimportrulev1.DatabaseObjectImportRuleSpec) *databaseobjectimportrulev1.DatabaseObjectImportRule {
	out := databaseobjectimportrulev1.DatabaseObjectImportRule_builder{
		Kind:    types.KindDatabaseObjectImportRule,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:      name,
			Namespace: defaults.Namespace,
		}.Build(),
		Spec: spec,
	}.Build()
	return out
}

func TestApplyDatabaseObjectImportRules(t *testing.T) {
	tests := []struct {
		name     string
		rules    []*databaseobjectimportrulev1.DatabaseObjectImportRule
		database types.Database
		objs     []*dbobjectv1.DatabaseObject
		want     []*dbobjectv1.DatabaseObject
		errCount int
	}{
		{
			name:     "empty inputs",
			rules:    []*databaseobjectimportrulev1.DatabaseObjectImportRule{},
			database: mkDatabase(t, "dummy", map[string]string{"env": "prod"}),
			objs:     nil,
			want:     nil,
		},
		{
			name: "database labels are matched by the rules",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "foo", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"dev"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":    "rw",
								"flag_from_dev": "dummy",
							},
						}.Build(),
					},
				}.Build()),
				mkImportRule(t, "bar", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":     "ro",
								"flag_from_prod": "dummy",
							},
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "dummy", map[string]string{"env": "prod"}),
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build()),
			},
			want: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build(),
					func(db *dbobjectv1.DatabaseObject) error {
						db.GetMetadata().SetLabels(map[string]string{
							"dev_access":     "ro",
							"flag_from_prod": "dummy",
						})
						return nil
					}),
			},
		},
		{
			name: "rule priorities are applied",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "foo", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":    "rw",
								"flag_from_dev": "dummy",
							},
						}.Build(),
					},
				}.Build()),

				mkImportRule(t, "bar", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":     "ro",
								"flag_from_prod": "dummy",
							},
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "dummy", map[string]string{}),
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build()),
			},
			want: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build(), func(db *dbobjectv1.DatabaseObject) error {
					db.GetMetadata().SetLabels(map[string]string{
						"dev_access":     "ro",
						"flag_from_dev":  "dummy",
						"flag_from_prod": "dummy",
					})
					return nil
				}),
			},
		},
		{
			name: "errors are counted",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "foo", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":    "rw",
								"flag_from_dev": "dummy",
							},
						}.Build(),
					},
				}.Build()),

				mkImportRuleNoValidation("bar", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
								TableNames: []string{"*"},
							}.Build(),
							AddLabels: map[string]string{
								"dev_access":     "ro",
								"flag_from_prod": "dummy",
							},
						}.Build(),
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Match:     databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{TableNames: []string{"bar", "baz"}}.Build(),
							AddLabels: map[string]string{"error label": "{{foo()}}"},
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "dummy", map[string]string{}),
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build()),
				mkDatabaseObject(t, "bar", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build()),
				mkDatabaseObject(t, "baz", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build()),
			},
			want: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject(t, "foo", dbobjectv1.DatabaseObjectSpec_builder{ObjectKind: ObjectKindTable, Protocol: "postgres"}.Build(), func(db *dbobjectv1.DatabaseObject) error {
					db.GetMetadata().SetLabels(map[string]string{
						"dev_access":     "ro",
						"flag_from_dev":  "dummy",
						"flag_from_prod": "dummy",
					})
					return nil
				}),
			},
			errCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errCount := ApplyDatabaseObjectImportRules(context.Background(), slog.Default(), tt.rules, tt.database, tt.objs)
			require.Len(t, out, len(tt.want))
			for i, obj := range out {
				require.Equal(t, tt.want[i].String(), obj.String())
			}
			require.Equal(t, tt.errCount, errCount)
		})
	}
}

func TestMatchPattern(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{
			name:     "plain text match",
			pattern:  "exactMatch",
			value:    "exactMatch",
			expected: true,
		},
		{
			name:     "substring mismatch",
			pattern:  "exact",
			value:    "exactMatch",
			expected: false,
		},
		{
			name:     "plain text mismatch",
			pattern:  "exactMatch",
			value:    "noMatch",
			expected: false,
		},
		{
			name:     "glob match",
			pattern:  "gl*b*",
			value:    "globMatch",
			expected: true,
		},
		{
			name:     "glob mismatch",
			pattern:  "glob*",
			value:    "noMatch",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchPattern(tc.pattern, tc.value)
			require.Equal(t, tc.expected, result, "Mismatch in test case: %s", tc.name)
		})
	}
}

func TestDatabaseObjectImportMatch(t *testing.T) {
	testCases := []struct {
		name      string
		match     *databaseobjectimportrulev1.DatabaseObjectImportMatch
		spec      *dbobjectv1.DatabaseObjectSpec
		wantMatch bool
	}{
		{
			name:      "empty",
			match:     &databaseobjectimportrulev1.DatabaseObjectImportMatch{},
			spec:      &dbobjectv1.DatabaseObjectSpec{},
			wantMatch: false,
		},
		{
			name: "match table name",
			match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
				TableNames: []string{"object1", "object2"},
			}.Build(),
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			}.Build(),
			wantMatch: true,
		},
		{
			name: "glob",
			match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
				TableNames: []string{"*"},
			}.Build(),
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			}.Build(),
			wantMatch: true,
		},
		{
			name: "mismatch",
			match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
				ViewNames: []string{"object1", "object2"},
			}.Build(),
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Database:            "db3",
				DatabaseServiceName: "service3",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindView,
				Name:                "object3",
				Schema:              "schema3",
			}.Build(),
			wantMatch: false,
		},
		{
			name: "empty name matches no objects",
			match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
				TableNames: []string{""},
			}.Build(),
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			}.Build(),
			wantMatch: false,
		},
		{
			name:  "empty clause matches no objects",
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{},
			spec: dbobjectv1.DatabaseObjectSpec_builder{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			}.Build(),
			wantMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMatch := databaseObjectImportMatch(tc.match, tc.spec)
			require.Equal(t, tc.wantMatch, gotMatch)
		})
	}
}

func Test_databaseObjectScopeMatch(t *testing.T) {
	spec := dbobjectv1.DatabaseObjectSpec_builder{
		Database: "foo",
		Schema:   "public",

		DatabaseServiceName: "service1",
		Protocol:            "postgres",
		ObjectKind:          ObjectKindTable,
		Name:                "object1",
	}.Build()

	tests := []struct {
		name  string
		scope *databaseobjectimportrulev1.DatabaseObjectImportScope
		want  bool
	}{
		{
			name: "empty db name and schema",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				DatabaseNames: nil,
				SchemaNames:   nil,
			}.Build(),
			want: true,
		},
		{
			name: "just db name",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				DatabaseNames: []string{"foo", "bar", "baz"},
			}.Build(),
			want: true,
		},
		{
			name: "db name glob match",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				DatabaseNames: []string{"f*o"},
			}.Build(),
			want: true,
		},
		{
			name: "just schema",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				SchemaNames: []string{"public", "private"},
			}.Build(),
			want: true,
		},
		{
			name: "schema name glob match",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				SchemaNames: []string{"pub*"},
			}.Build(),
			want: true,
		},
		{
			name: "match db name and schema",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				DatabaseNames: []string{"foo", "bar", "baz"},
				SchemaNames:   []string{"public", "private"},
			}.Build(),
			want: true,
		},
		{
			name: "mismatch db name and schema",
			scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
				DatabaseNames: []string{"dummy"},
				SchemaNames:   []string{"dummy"},
			}.Build(),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, databaseObjectScopeMatch(tt.scope, spec))
		})
	}
}

func Test_applyMappingToObject(t *testing.T) {
	spec := dbobjectv1.DatabaseObjectSpec_builder{
		Database:            "db3",
		DatabaseServiceName: "service3",
		Protocol:            "postgres",
		ObjectKind:          ObjectKindTable,
		Name:                "object3",
		Schema:              "schema3",
	}.Build()

	tests := []struct {
		name       string
		mapping    *databaseobjectimportrulev1.DatabaseObjectImportRuleMapping
		labels     map[string]string
		wantLabels map[string]string
		wantMatch  bool
		wantError  bool
	}{
		{
			name: "simple templates",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label":           "rw",
					"protocol":              "{{obj.protocol}}",
					"database_service_name": "{{obj.database_service_name}}",
					"object_kind":           "{{obj.object_kind}}",
					"database":              "{{obj.database}}",
					"schema":                "{{obj.schema}}",
					"name":                  "{{obj.name}}",
				},
			}.Build(),
			labels: map[string]string{},
			wantLabels: map[string]string{
				"plain_label":           "rw",
				"protocol":              "postgres",
				"database_service_name": "service3",
				"object_kind":           "table",
				"database":              "db3",
				"schema":                "schema3",
				"name":                  "object3",
			},
			wantMatch: true,
		},
		{
			name: "add prefix",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "db-{{obj.object_kind}}",
				},
			}.Build(),
			labels: map[string]string{},
			wantLabels: map[string]string{
				"plain_label": "rw",
				"tag":         "db-table",
			},
			wantMatch: true,
		},
		{
			name: "spaces are trimmed prefix",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "  db-{{   obj.object_kind }}-bar  ",
				},
			}.Build(),
			labels: map[string]string{},
			wantLabels: map[string]string{
				"plain_label": "rw",
				"tag":         "db-table-bar",
			},
			wantMatch: true,
		},
		{
			name: "invalid object is rejected",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "db-{{obj.invalid}}",
				},
			}.Build(),
			labels:    map[string]string{},
			wantError: true,
		},
		{
			name: "invalid namespace is rejected",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "db-{{wrong.object_kind}}",
				},
			}.Build(),
			labels:    map[string]string{},
			wantError: true,
		},
		{
			name: "empty template is rejected",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "db-{{}}",
				},
			}.Build(),
			labels:    map[string]string{},
			wantError: true,
		},
		{
			name: "multi template is rejected",
			mapping: databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
				Match: databaseobjectimportrulev1.DatabaseObjectImportMatch_builder{
					TableNames: []string{"*"},
				}.Build(),
				AddLabels: map[string]string{
					"plain_label": "rw",
					"tag":         "db-{{obj.object_kind obj.object_kind}}",
				},
			}.Build(),
			labels:    map[string]string{},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := maps.Clone(tt.labels)
			match, err := applyMappingToObject(tt.mapping, spec, labels)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantMatch, match)
				require.Equal(t, tt.wantLabels, labels)
			}
		})
	}
}

func Test_splitExpression(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []eval
		wantErr bool
	}{
		{
			name:    "empty string",
			value:   "",
			want:    []eval{literal{text: ""}},
			wantErr: false,
		},
		{
			name:    "literal",
			value:   "literal",
			want:    []eval{literal{text: "literal"}},
			wantErr: false,
		},
		{
			name:    "literal with whitespace",
			value:   "      literal   ",
			want:    []eval{literal{text: "literal"}},
			wantErr: false,
		},
		{
			name:    "prefix, expr, suffix",
			value:   "prefix-{{expr}}-suffix",
			want:    []eval{literal{text: "prefix-"}, expression{text: "expr"}, literal{text: "-suffix"}},
			wantErr: false,
		},
		{
			name:    "prefix, expr, suffix with extra whitespace",
			value:   "    prefix-{{expr}}-suffix        ",
			want:    []eval{literal{text: "prefix-"}, expression{text: "expr"}, literal{text: "-suffix"}},
			wantErr: false,
		},
		{
			name:    "unmatched {{",
			value:   "foo bar {{ baz",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "unmatched }}",
			value:   "foo bar }} baz",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "multiple templates",
			value:   "foo {{bar}} {{baz}}",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitExpression(tt.value)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFilterRulesForDatabase(t *testing.T) {
	tests := []struct {
		name  string
		rules []*databaseobjectimportrulev1.DatabaseObjectImportRule

		want []*databaseobjectimportrulev1.DatabaseObjectImportRule
	}{
		{
			name: "all matching",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRuleNoValidation("rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
				mkImportRuleNoValidation("rule2", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
			},
			want: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRuleNoValidation("rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
				mkImportRuleNoValidation("rule2", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
			},
		},
		{
			name: "one matching",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRuleNoValidation("rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
				mkImportRuleNoValidation("rule2", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"dev"}}),
				}.Build()),
			},
			want: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRuleNoValidation("rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
				}.Build()),
			},
		},
		{
			name:  "empty rules",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := mkDatabase(t, "testdb", map[string]string{"env": "prod"})
			result := filterRulesForDatabase(tt.rules, database)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestCalculateDatabaseNameFilter(t *testing.T) {
	tests := []struct {
		name     string
		rules    []*databaseobjectimportrulev1.DatabaseObjectImportRule
		database *types.DatabaseV3
		dbNames  map[string]bool
	}{
		{
			name: "accept any database",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
								// empty list => match any database name.
								DatabaseNames: []string{},
							}.Build(),
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "testdb", map[string]string{"env": "prod"}),
			dbNames:  map[string]bool{"random-name-" + uuid.New().String(): true},
		},
		{
			name: "match specific database name",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
								DatabaseNames: []string{"testdb", "devdb"},
							}.Build(),
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "testdb", map[string]string{"env": "prod"}),
			dbNames:  map[string]bool{"testdb": true, "devdb": true, "baddb": false},
		},
		{
			name: "no matching rules",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule(t, "rule1", databaseobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"dev"}}), // env:dev does not match env:prod below.
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						databaseobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
							Scope: databaseobjectimportrulev1.DatabaseObjectImportScope_builder{
								DatabaseNames: []string{"devdb"},
							}.Build(),
						}.Build(),
					},
				}.Build()),
			},
			database: mkDatabase(t, "testdb", map[string]string{"env": "prod"}),
			dbNames:  map[string]bool{"testdb": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := CalculateDatabaseNameFilter(tt.rules, tt.database)
			for dbName, expected := range tt.dbNames {
				t.Run(dbName, func(tt *testing.T) {
					result := filter(dbName)
					require.Equal(t, expected, result)
				})
			}
		})
	}
}
