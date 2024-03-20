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

	"github.com/gravitational/teleport/api/defaults"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apilabels "github.com/gravitational/teleport/api/types/label"
)

func TestMarshalDatabaseObjectImportRuleRoundTrip(t *testing.T) {
	spec := &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
		Priority:       30,
		DatabaseLabels: apilabels.FromMap(map[string][]string{"env": {"staging", "prod"}, "owner_org": {"trading"}}),
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
	}
	obj := &dbobjectimportrulev1.DatabaseObjectImportRule{
		Kind:    types.KindDatabaseObjectImportRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      "import_all_staging_tables",
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}

	out, err := MarshalDatabaseObjectImportRule(obj)
	require.NoError(t, err)
	newObj, err := UnmarshalDatabaseObjectImportRule(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}
