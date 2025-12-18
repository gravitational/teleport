// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

func makeDatabaseObjectImportRule(t *testing.T, name string, priority int) *dbobjectimportrulev1.DatabaseObjectImportRule {
	t.Helper()

	r, err := databaseobjectimportrule.NewDatabaseObjectImportRule(name, &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
		Priority: int32(priority),
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

func TestDatabaseImportRuleCollection_writeText(t *testing.T) {
	rules := []*dbobjectimportrulev1.DatabaseObjectImportRule{
		makeDatabaseObjectImportRule(t, "rule_1", 11),
		makeDatabaseObjectImportRule(t, "rule_2", 22),
		makeDatabaseObjectImportRule(t, "rule_3", 33),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Priority", "Mapping Count", "DB Label Count"},
		[]string{"rule_1", "11", "1", "2"},
		[]string{"rule_2", "22", "1", "2"},
		[]string{"rule_3", "33", "1", "2"},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, &databaseObjectImportRuleCollection{rules}, formatted, formatted)
}
