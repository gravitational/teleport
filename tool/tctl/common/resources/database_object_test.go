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

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
)

func makeDatabaseObject(t *testing.T, name string) *dbobjectv1.DatabaseObject {
	t.Helper()
	r, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{
		Name:                name,
		Protocol:            "postgres",
		DatabaseServiceName: "pg",
		ObjectKind:          "table",
	})
	require.NoError(t, err)
	return r
}

func TestDatabaseObjectCollection_writeText(t *testing.T) {
	items := []*dbobjectv1.DatabaseObject{
		makeDatabaseObject(t, "object_1"),
		makeDatabaseObject(t, "object_2"),
		makeDatabaseObject(t, "object_3"),
	}

	table := asciitable.MakeTable(
		[]string{"Name", "Kind", "DB Service", "Protocol"},
		[]string{"object_1", "table", "pg", "postgres"},
		[]string{"object_2", "table", "pg", "postgres"},
		[]string{"object_3", "table", "pg", "postgres"},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, &databaseObjectCollection{items}, formatted, formatted)
}
