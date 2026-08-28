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
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

type mockGetter struct {
	allow []types.DatabasePermission
	deny  []types.DatabasePermission
}

func (m mockGetter) GetDatabasePermissions(_ types.Database) (allow types.DatabasePermissions, deny types.DatabasePermissions, err error) {
	return m.allow, m.deny, nil
}

func TestCalculatePermissions(t *testing.T) {
	mkDatabaseObject := func(name string, labels map[string]string) *dbobjectv1.DatabaseObject {
		out, err := databaseobject.NewDatabaseObjectWithLabels(name, maps.Clone(labels), &dbobjectv1.DatabaseObjectSpec{
			Protocol:            types.DatabaseProtocolPostgreSQL,
			DatabaseServiceName: "dummy",
			ObjectKind:          databaseobjectimportrule.ObjectKindTable,
			Database:            "dummy",
			Schema:              "public",
			Name:                name,
		})

		require.NoError(t, err)
		return out
	}

	tests := []struct {
		name    string
		getter  GetDatabasePermissions
		objs    []*dbobjectv1.DatabaseObject
		want    PermissionSet
		summary string
		details []events.DatabasePermissionEntry
	}{
		{
			name: "some permissions for some objects",
			getter: &mockGetter{
				allow: []types.DatabasePermission{
					{
						Permissions: []string{"SELECT"},
						Match:       map[string]utils.Strings{"kind": []string{"table"}},
					},
					{
						Permissions: []string{"DELETE"},
						Match:       map[string]utils.Strings{"kind": []string{"schema"}},
					},
				},
				deny: nil,
			},
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", map[string]string{"kind": "table"}),
				mkDatabaseObject("bar", map[string]string{"kind": "schema"}),
				mkDatabaseObject("myproc", map[string]string{"kind": "procedure"}),
				mkDatabaseObject("myunknown", map[string]string{"kind": "unknown"}),
			},
			want: PermissionSet{
				"SELECT": {
					mkDatabaseObject("foo", map[string]string{"kind": "table"}),
				},
				"DELETE": {
					mkDatabaseObject("bar", map[string]string{"kind": "schema"}),
				},
			},
			summary: "DELETE: 1 objects (table:1), SELECT: 1 objects (table:1)",
			details: []events.DatabasePermissionEntry{
				{
					Permission: "DELETE",
					Counts:     map[string]int32{"table": 1},
				},
				{
					Permission: "SELECT",
					Counts:     map[string]int32{"table": 1},
				},
			},
		},
		{
			name: "deny removes permissions",
			getter: &mockGetter{
				allow: []types.DatabasePermission{
					{
						Permissions: []string{"SELECT", "INSERT"},
						Match:       map[string]utils.Strings{"kind": []string{"table"}},
					},
					{
						Permissions: []string{"SELECT", "DELETE"},
						Match:       map[string]utils.Strings{"kind": []string{"schema"}},
					},
				},
				deny: []types.DatabasePermission{
					{
						Permissions: []string{"*"},
						Match:       map[string]utils.Strings{"kind": []string{"table"}},
					},
					{
						Permissions: []string{"DELETE"},
						Match:       map[string]utils.Strings{"kind": []string{"schema"}},
					},
				},
			},
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", map[string]string{"kind": "table"}),
				mkDatabaseObject("bar", map[string]string{"kind": "schema"}),
			},
			want: PermissionSet{
				"SELECT": {
					mkDatabaseObject("bar", map[string]string{"kind": "schema"}),
				},
			},
			summary: "SELECT: 1 objects (table:1)",
			details: []events.DatabasePermissionEntry{
				{
					Permission: "SELECT",
					Counts:     map[string]int32{"table": 1},
				},
			},
		},
		{
			name:   "no permissions",
			getter: &mockGetter{},
			objs: []*dbobjectv1.DatabaseObject{
				mkDatabaseObject("foo", map[string]string{"kind": "table"}),
				mkDatabaseObject("bar", map[string]string{"kind": "schema"}),
			},
			want:    PermissionSet{},
			summary: "(none)",
			details: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := types.NewDatabaseV3(types.Metadata{Name: "dummy"}, types.DatabaseSpecV3{Protocol: "postgres", URI: "dummy"})
			require.NoError(t, err)
			perms, err := CalculatePermissions(tt.getter, db, tt.objs)
			require.NoError(t, err)
			require.Equal(t, tt.want, perms)

			summary, details := SummarizePermissions(perms)
			require.Equal(t, tt.summary, summary)
			require.ElementsMatch(t, tt.details, details)
		})
	}
}

func TestDatabasePermissionMatch(t *testing.T) {
	mkDatabaseObject := func(labels map[string]string) *dbobjectv1.DatabaseObject {
		out, err := databaseobject.NewDatabaseObjectWithLabels("foo", maps.Clone(labels), &dbobjectv1.DatabaseObjectSpec{
			Protocol:            types.DatabaseProtocolPostgreSQL,
			DatabaseServiceName: "dummy",
			ObjectKind:          databaseobjectimportrule.ObjectKindTable,
			Database:            "dummy",
			Schema:              "public",
			Name:                "foo",
		})

		require.NoError(t, err)
		return out
	}

	tests := []struct {
		name      string
		obj       *dbobjectv1.DatabaseObject
		labels    types.Labels
		wantMatch bool
	}{
		{
			name: "object with some labels",
			obj:  mkDatabaseObject(map[string]string{"owner": "scrooge", "env": "dev"}),
			labels: types.Labels{
				"env":   {"production", "dev"},
				"owner": {"john_doe", "scrooge"},
			},
			wantMatch: true,
		},
		{
			name: "labels and glob patterns",
			obj:  mkDatabaseObject(map[string]string{"owner": "scrooge", "env": "dev"}),
			labels: types.Labels{
				"env":   {"*"},
				"owner": {"john_doe", "scrooge"},
			},
			wantMatch: true,
		},

		{
			name: "mismatch: object without labels",
			obj:  mkDatabaseObject(nil),
			labels: types.Labels{
				"env":   {"production", "dev"},
				"owner": {"john_doe", "scrooge"},
			},
			wantMatch: false,
		},
		{
			name:      "mismatch: no labels",
			obj:       mkDatabaseObject(map[string]string{"env": "dev"}),
			labels:    types.Labels{},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := databasePermissionMatch(types.DatabasePermission{Permissions: []string{"SELECT"}, Match: tt.labels}, tt.obj)
			require.Equal(t, tt.wantMatch, matched)
		})
	}
}
