// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types/databaseobjectimportrule"
)

func TestMarshalDatabaseObjectImportRuleRoundTrip(t *testing.T) {
	mkImportRule := func(name string, spec *dbobjectimportrulev1.DatabaseObjectImportRuleSpec) *dbobjectimportrulev1.DatabaseObjectImportRule {
		out, err := databaseobjectimportrule.NewDatabaseObjectImportRule(name, spec)
		require.NoError(t, err)
		return out
	}

	tests := []struct {
		name string
		obj  *dbobjectimportrulev1.DatabaseObjectImportRule
	}{
		{name: "dbImportRule-import_all_staging_tables", obj: mkImportRule("import_all_staging_tables", &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
			Priority: 30,
			DbLabels: map[string]string{"env": "staging"},
			Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{
				{
					Scope: &dbobjectimportrulev1.DatabaseObjectImportScope{
						SchemaNames:   []string{"public"},
						DatabaseNames: []string{"foo", "bar", "baz"},
					},
					Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
						TableNames:     []string{"*"},
						ViewNames:      []string{"1", "2", "3"},
						ProcedureNames: []string{"aaa", "bbb", "ccc"},
					},
					AddLabels: map[string]string{
						"env":          "staging",
						"custom_label": "my_custom_value",
					},
				},
			},
		})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := MarshalDatabaseObjectImportRule(tt.obj)
			require.NoError(t, err)
			obj, err := UnmarshalDatabaseObjectImportRule(out)
			require.NoError(t, err)
			require.True(t, proto.Equal(tt.obj, obj), "messages are not equal")
		})
	}
}
