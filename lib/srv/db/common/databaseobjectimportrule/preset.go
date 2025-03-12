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

package databaseobjectimportrule

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/proto"

	dbobjectimportrulev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types/label"
)

// NewPresetImportAllObjectsRule creates new "import_all_objects" database object import rule, which applies `kind: <object kind>` label to all database objects.
// This is a convenience rule and users are free to modify it to suit their needs.
func NewPresetImportAllObjectsRule() *dbobjectimportrulev1pb.DatabaseObjectImportRule {
	rule, err := NewDatabaseObjectImportRule("import_all_objects", &dbobjectimportrulev1pb.DatabaseObjectImportRuleSpec{
		Priority:       0,
		DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
		Mappings: []*dbobjectimportrulev1pb.DatabaseObjectImportRuleMapping{
			{
				Match: &dbobjectimportrulev1pb.DatabaseObjectImportMatch{
					TableNames:     []string{"*"},
					ViewNames:      []string{"*"},
					ProcedureNames: []string{"*"},
				},
				AddLabels: map[string]string{
					"protocol":              "{{obj.protocol}}",
					"database_service_name": "{{obj.database_service_name}}",
					"object_kind":           "{{obj.object_kind}}",
					"database":              "{{obj.database}}",
					"schema":                "{{obj.schema}}",
					"name":                  "{{obj.name}}",
				},
			},
		},
	})

	if err != nil {
		slog.WarnContext(context.Background(), "failed to create import_all_objects database object import rule", "error", err)
		return nil
	}
	return rule
}

// IsOldImportAllObjectsRulePreset checks if the provided rule is the "old" preset.
// TODO(greedy52) DELETE in 18.0
func IsOldImportAllObjectsRulePreset(cur *dbobjectimportrulev1pb.DatabaseObjectImportRule) bool {
	// Skip no-zero expires.
	if cur.Metadata.Expires != nil && !cur.Metadata.Expires.AsTime().IsZero() {
		return false
	}

	// Make the old preset from https://github.com/gravitational/teleport/pull/37808
	old, err := NewDatabaseObjectImportRule("import_all_objects", &dbobjectimportrulev1pb.DatabaseObjectImportRuleSpec{
		Priority:       0,
		DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
		Mappings: []*dbobjectimportrulev1pb.DatabaseObjectImportRuleMapping{
			{
				Match:     &dbobjectimportrulev1pb.DatabaseObjectImportMatch{TableNames: []string{"*"}},
				AddLabels: map[string]string{"kind": ObjectKindTable},
			},
			{
				Match:     &dbobjectimportrulev1pb.DatabaseObjectImportMatch{ViewNames: []string{"*"}},
				AddLabels: map[string]string{"kind": ObjectKindView},
			},
			{
				Match:     &dbobjectimportrulev1pb.DatabaseObjectImportMatch{ProcedureNames: []string{"*"}},
				AddLabels: map[string]string{"kind": ObjectKindProcedure},
			},
		},
	})
	if err != nil {
		slog.WarnContext(context.Background(), "failed to create old import_all_objects database object import rule", "error", err)
		return false
	}

	// Ignore these fields.
	old.Metadata.Revision = cur.Metadata.Revision
	old.Metadata.Namespace = cur.Metadata.Namespace
	old.Metadata.Expires = cur.Metadata.Expires
	return proto.Equal(old, cur)
}
