/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	databaseobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	apilabels "github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

// TestDatabaseObjectImportRuleCRUD tests backend operations with DatabaseObject import rule resources.
func TestDatabaseObjectImportRuleCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewDatabaseObjectImportRuleService(backend)
	require.NoError(t, err)

	// Create a couple import rules.
	importRule1, err := databaseobjectimportrule.NewDatabaseObjectImportRule("r1", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
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
	})
	require.NoError(t, err)

	importRule2, err := databaseobjectimportrule.NewDatabaseObjectImportRule("r2", &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
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
	})
	require.NoError(t, err)

	// Initially we expect no import rules.
	out, nextToken, err := service.ListDatabaseObjectImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both import rules.
	importRule, err := service.CreateDatabaseObjectImportRule(ctx, importRule1)
	require.NoError(t, err)
	require.Equal(t, importRule1, importRule)

	importRule, err = service.CreateDatabaseObjectImportRule(ctx, importRule2)
	require.NoError(t, err)
	require.Equal(t, importRule2, importRule)

	// Fetch all import rules.
	out, nextToken, err = service.ListDatabaseObjectImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, out, 2)
	require.Equal(t, importRule1.String(), out[0].String())
	require.Equal(t, importRule2.String(), out[1].String())

	// Fetch a paginated list of import rules
	paginatedOut := make([]*databaseobjectimportrulev1.DatabaseObjectImportRule, 0, 2)
	for {
		out, nextToken, err = service.ListDatabaseObjectImportRules(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.True(t, proto.Equal(importRule1, paginatedOut[0]))
	require.True(t, proto.Equal(importRule2, paginatedOut[1]))

	// Fetch a specific import rule.
	importRule, err = service.GetDatabaseObjectImportRule(ctx, importRule2.Metadata.GetName())
	require.NoError(t, err)
	require.True(t, proto.Equal(importRule2, importRule))

	// Try to fetch an import rule that doesn't exist.
	_, err = service.GetDatabaseObjectImportRule(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Try to create the same import rule.
	_, err = service.CreateDatabaseObjectImportRule(ctx, importRule1)
	require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

	// Update an import rule.
	importRule1.Metadata.Expires = timestamppb.New(clock.Now().Add(30 * time.Minute))
	_, err = service.UpdateDatabaseObjectImportRule(ctx, importRule1)
	require.NoError(t, err)
	importRule, err = service.GetDatabaseObjectImportRule(ctx, importRule1.GetMetadata().GetName())
	require.NoError(t, err)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	importRule.Metadata.Id = importRule1.Metadata.Id
	require.True(t, proto.Equal(importRule1, importRule))

	// Delete an import rule
	err = service.DeleteDatabaseObjectImportRule(ctx, importRule1.GetMetadata().GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListDatabaseObjectImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.True(t, proto.Equal(importRule2, out[0]))

	// Try to delete an import rule that doesn't exist.
	err = service.DeleteDatabaseObjectImportRule(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all import rules.
	lst, nextToken, err := service.ListDatabaseObjectImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Equal(t, "", nextToken)
	for _, rule := range lst {
		err = service.DeleteDatabaseObjectImportRule(ctx, rule.GetMetadata().GetName())
		require.NoError(t, err)
	}
	out, nextToken, err = service.ListDatabaseObjectImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func TestMarshalDatabaseObjectImportRuleRoundTrip(t *testing.T) {
	spec := &databaseobjectimportrulev1.DatabaseObjectImportRuleSpec{
		Priority:       30,
		DatabaseLabels: apilabels.FromMap(map[string][]string{"env": {"staging", "prod"}, "owner_org": {"trading"}}),
		Mappings: []*databaseobjectimportrulev1.DatabaseObjectImportRuleMapping{
			{
				Scope: &databaseobjectimportrulev1.DatabaseObjectImportScope{
					SchemaNames:   []string{"public"},
					DatabaseNames: []string{"foo", "bar", "baz"},
				},
				Match: &databaseobjectimportrulev1.DatabaseObjectImportMatch{
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
	obj := &databaseobjectimportrulev1.DatabaseObjectImportRule{
		Kind:    types.KindDatabaseObjectImportRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      "import_all_staging_tables",
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}

	out, err := marshalDatabaseObjectImportRule(obj)
	require.NoError(t, err)
	newObj, err := unmarshalDatabaseObjectImportRule(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}
