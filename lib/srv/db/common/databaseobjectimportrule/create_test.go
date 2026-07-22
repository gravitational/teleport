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

package databaseobjectimportrule

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
)

func TestNewDatabaseObjectImportRule(t *testing.T) {
	tests := []struct {
		name          string
		spec          *dbobjectimportrulev1.DatabaseObjectImportRuleSpec
		expectedError error
	}{
		{
			name: "valid rule",
			spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
				DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
				Mappings:       []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
			}.Build(),
			expectedError: nil,
		},
		{
			name:          "nil spec",
			spec:          nil,
			expectedError: trace.BadParameter("missing spec"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDatabaseObjectImportRule(tt.name, tt.spec)
			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func TestValidateDatabaseObjectImportRule(t *testing.T) {
	tests := []struct {
		name          string
		rule          *dbobjectimportrulev1.DatabaseObjectImportRule
		expectedError error
	}{
		{
			name: "valid rule",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings:       []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
				}.Build(),
			}.Build(),
			expectedError: nil,
		},
		{
			name:          "nil rule",
			rule:          nil,
			expectedError: trace.BadParameter("database object import rule must be non-nil"),
		},
		{
			name: "missing metadata",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:     types.KindDatabaseObjectImportRule,
				Version:  types.V1,
				Metadata: nil,
				Spec:     &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			}.Build(),
			expectedError: trace.BadParameter("metadata: must be non-nil"),
		},
		{
			name: "missing name",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			}.Build(),
			expectedError: trace.BadParameter("metadata.name: must be non-empty"),
		},
		{
			name: "invalid kind",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    "InvalidKind",
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			}.Build(),
			expectedError: trace.BadParameter("invalid kind InvalidKind, expected db_object_import_rule"),
		},
		{
			name: "missing spec",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: nil,
			}.Build(),
			expectedError: trace.BadParameter("missing spec"),
		},
		{
			name: "missing database_labels",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
				}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("missing database_labels"),
		},
		{
			name: "missing mappings",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
				}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("missing mappings"),
		},
		{
			name: "invalid mapping key",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{dbobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
						AddLabels: map[string]string{"    ": "dummy"},
					}.Build()},
				}.Build(),
			}.Build(),
			expectedError: trace.BadParameter("invalid mapping: label name is empty or whitespace"),
		},
		{
			name: "invalid template in mapping",
			rule: dbobjectimportrulev1.DatabaseObjectImportRule_builder{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name:      "test",
					Namespace: defaults.Namespace,
				}.Build(),
				Spec: dbobjectimportrulev1.DatabaseObjectImportRuleSpec_builder{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{dbobjectimportrulev1.DatabaseObjectImportRuleMapping_builder{
						AddLabels: map[string]string{"dummy": "  {{  "},
					}.Build()},
				}.Build(),
			}.Build(),
			expectedError: trace.Wrap(trace.BadParameter("\"  {{  \" is using template brackets '{{' or '}}', however expression does not parse, make sure the format is {{expression}}"), "mapping value failed to parse as template"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatabaseObjectImportRule(tt.rule)
			require.ErrorIs(t, err, tt.expectedError)
		})
	}
}
