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

package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	databaseobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/databaseobject"
	"github.com/gravitational/teleport/api/types/databaseobjectimportrule"
	"github.com/gravitational/teleport/api/types/label"
)

func TestApplyDatabaseObjectImportRules(t *testing.T) {
	mkDatabase := func(name string, labels map[string]string) *types.DatabaseV3 {
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

	mkDatabaseObject := func(name string, spec *dbobjectv1.DatabaseObjectSpec, options ...option) *dbobjectv1.DatabaseObject {
		spec.Name = name
		out, err := databaseobject.NewDatabaseObject(name, spec)
		require.NoError(t, err)
		for _, opt := range options {
			require.NoError(t, opt(out))
		}

		return out
	}

	mkImportRule := func(name string, spec *databaseobjectimportrulev1.DatabaseObjectImportRuleSpec) *databaseobjectimportrulev1.DatabaseObjectImportRule {
		out, err := databaseobjectimportrule.NewDatabaseObjectImportRule(name, spec)
		require.NoError(t, err)
		return out
	}
	tests := []struct {
		name     string
		rules    []*databaseobjectimportrulev1.DatabaseObjectImportRule
		database types.Database
		objs     []*dbobjectv1.DatabaseObject
		want     []*dbobjectv1.DatabaseObject
	}{
		{
			name:     "empty inputs",
			rules:    []*databaseobjectimportrulev1.DatabaseObjectImportRule{},
			database: mkDatabase("dummy", map[string]string{"env": "prod"}),
			objs:     nil,
			want:     nil,
		},
		{
			name: "database labels are matched by the rules",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule("foo", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"dev"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						{
							Match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
								TableNames: []string{"*"},
							},
							AddLabels: map[string]string{
								"dev_access":    "rw",
								"flag_from_dev": "dummy",
							},
						},
					},
				}),
				mkImportRule("bar", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"env": {"prod"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						{
							Match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
								TableNames: []string{"*"},
							},
							AddLabels: map[string]string{
								"dev_access":     "ro",
								"flag_from_prod": "dummy",
							},
						},
					},
				}),
			},
			database: mkDatabase("dummy", map[string]string{"env": "prod"}),
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", &dbobjectv1.DatabaseObjectSpec{ObjectKind: ObjectKindTable, Protocol: "postgres"}),
			},
			want: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", &dbobjectv1.DatabaseObjectSpec{ObjectKind: ObjectKindTable, Protocol: "postgres"},
					func(db *dbobjectv1.DatabaseObject) error {
						db.Metadata.Labels = map[string]string{
							"dev_access":     "ro",
							"flag_from_prod": "dummy",
						}
						return nil
					}),
			},
		},
		{
			name: "rule priorities are applied",
			rules: []*databaseobjectimportrulev1.DatabaseObjectImportRule{
				mkImportRule("foo", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
					Priority:       10,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						{
							Match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
								TableNames: []string{"*"},
							},
							AddLabels: map[string]string{
								"dev_access":    "rw",
								"flag_from_dev": "dummy",
							},
						},
					},
				}),

				mkImportRule("bar", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
					Priority:       20,
					DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
					Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
						{
							Match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
								TableNames: []string{"*"},
							},
							AddLabels: map[string]string{
								"dev_access":     "ro",
								"flag_from_prod": "dummy",
							},
						},
					},
				}),
			},
			database: mkDatabase("dummy", map[string]string{}),
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", &dbobjectv1.DatabaseObjectSpec{ObjectKind: ObjectKindTable, Protocol: "postgres"}),
			},
			want: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", &dbobjectv1.DatabaseObjectSpec{ObjectKind: ObjectKindTable, Protocol: "postgres"}, func(db *dbobjectv1.DatabaseObject) error {
					db.Metadata.Labels = map[string]string{
						"dev_access":     "ro",
						"flag_from_dev":  "dummy",
						"flag_from_prod": "dummy",
					}
					return nil
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := ApplyDatabaseObjectImportRules(tt.rules, tt.database, tt.objs)
			require.Len(t, out, len(tt.want))
			for i, obj := range out {
				require.Equal(t, tt.want[i].String(), obj.String())
			}
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
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
				TableNames: []string{"object1", "object2"},
			},
			spec: &dbobjectv1.DatabaseObjectSpec{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			},
			wantMatch: true,
		},
		{
			name: "glob",
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
				TableNames: []string{"*"},
			},
			spec: &dbobjectv1.DatabaseObjectSpec{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			},
			wantMatch: true,
		},
		{
			name: "mismatch",
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
				ViewNames: []string{"object1", "object2"},
			},
			spec: &dbobjectv1.DatabaseObjectSpec{
				Database:            "db3",
				DatabaseServiceName: "service3",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindView,
				Name:                "object3",
				Schema:              "schema3",
			},
			wantMatch: false,
		},
		{
			name: "empty name matches no objects",
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
				TableNames: []string{""},
			},
			spec: &dbobjectv1.DatabaseObjectSpec{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			},
			wantMatch: false,
		},
		{
			name:  "empty clause matches no objects",
			match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{},
			spec: &dbobjectv1.DatabaseObjectSpec{
				Database:            "db1",
				DatabaseServiceName: "service1",
				Protocol:            "postgres",
				ObjectKind:          ObjectKindTable,
				Name:                "object1",
				Schema:              "schema1",
			},
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
	spec := &dbobjectv1.DatabaseObjectSpec{
		Database: "foo",
		Schema:   "public",

		DatabaseServiceName: "service1",
		Protocol:            "postgres",
		ObjectKind:          ObjectKindTable,
		Name:                "object1",
	}

	tests := []struct {
		name  string
		scope *databaseobjectimportrulev1.DatabaseObjectImportScope
		want  bool
	}{
		{
			name: "empty db name and schema",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				DatabaseNames: nil,
				SchemaNames:   nil,
			},
			want: true,
		},
		{
			name: "just db name",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				DatabaseNames: []string{"foo", "bar", "baz"},
			},
			want: true,
		},
		{
			name: "db name glob match",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				DatabaseNames: []string{"f*o"},
			},
			want: true,
		},
		{
			name: "just schema",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				SchemaNames: []string{"public", "private"},
			},
			want: true,
		},
		{
			name: "schema name glob match",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				SchemaNames: []string{"pub*"},
			},
			want: true,
		},
		{
			name: "match db name and schema",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				DatabaseNames: []string{"foo", "bar", "baz"},
				SchemaNames:   []string{"public", "private"},
			},
			want: true,
		},
		{
			name: "mismatch db name and schema",
			scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
				DatabaseNames: []string{"dummy"},
				SchemaNames:   []string{"dummy"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, databaseObjectScopeMatch(tt.scope, spec))
		})
	}
}
