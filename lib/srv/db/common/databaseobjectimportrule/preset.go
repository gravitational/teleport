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
	log "github.com/sirupsen/logrus"

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
		log.WithError(err).Warn("failed to create import_all_objects database object import rule")
		return nil
	}
	return rule
}
