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
			spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
				DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
				Mappings:       []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
			},
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
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings:       []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
				},
			},
			expectedError: nil,
		},
		{
			name:          "nil rule",
			rule:          nil,
			expectedError: trace.BadParameter("database object import rule must be non-nil"),
		},
		{
			name: "missing metadata",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:     types.KindDatabaseObjectImportRule,
				Version:  types.V1,
				Metadata: nil,
				Spec:     &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			},
			expectedError: trace.BadParameter("metadata: must be non-nil"),
		},
		{
			name: "missing name",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			},
			expectedError: trace.BadParameter("metadata.name: must be non-empty"),
		},
		{
			name: "invalid kind",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    "InvalidKind",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{},
			},
			expectedError: trace.BadParameter("invalid kind InvalidKind, expected db_object_import_rule"),
		},
		{
			name: "missing spec",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: nil,
			},
			expectedError: trace.BadParameter("missing spec"),
		},
		{
			name: "missing database_labels",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{}},
				},
			},
			expectedError: trace.BadParameter("missing database_labels"),
		},
		{
			name: "missing mappings",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
				},
			},
			expectedError: trace.BadParameter("missing mappings"),
		},
		{
			name: "invalid mapping key",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{
						AddLabels: map[string]string{"    ": "dummy"},
					}},
				},
			},
			expectedError: trace.BadParameter("invalid mapping: label name is empty or whitespace"),
		},
		{
			name: "invalid template in mapping",
			rule: &dbobjectimportrulev1.DatabaseObjectImportRule{
				Kind:    types.KindDatabaseObjectImportRule,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:      "test",
					Namespace: defaults.Namespace,
				},
				Spec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
					DatabaseLabels: label.FromMap(map[string][]string{"key": {"value"}}),
					Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{{
						AddLabels: map[string]string{"dummy": "  {{  "},
					}},
				},
			},
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
