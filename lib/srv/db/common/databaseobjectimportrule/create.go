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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewDatabaseObjectImportRule creates a new dbobjectimportrulev1.DatabaseObjectImportRule.
func NewDatabaseObjectImportRule(name string, spec *dbobjectimportrulev1.DatabaseObjectImportRuleSpec) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	return NewDatabaseObjectImportRuleWithLabels(name, nil, spec)
}

// NewDatabaseObjectImportRuleWithLabels creates a new dbobjectimportrulev1.DatabaseObjectImportRule with specified labels.
func NewDatabaseObjectImportRuleWithLabels(name string, labels map[string]string, spec *dbobjectimportrulev1.DatabaseObjectImportRuleSpec) (*dbobjectimportrulev1.DatabaseObjectImportRule, error) {
	out := &dbobjectimportrulev1.DatabaseObjectImportRule{
		Kind:    types.KindDatabaseObjectImportRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}

	err := ValidateDatabaseObjectImportRule(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ValidateDatabaseObjectImportRule checks if dbobjectimportrulev1.DatabaseObjectImportRule is valid.
func ValidateDatabaseObjectImportRule(rule *dbobjectimportrulev1.DatabaseObjectImportRule) error {
	if rule == nil {
		return trace.BadParameter("database object import rule must be non-nil")
	}
	if rule.Metadata == nil {
		return trace.BadParameter("metadata: must be non-nil")
	}
	if rule.Metadata.Name == "" {
		return trace.BadParameter("metadata.name: must be non-empty")
	}
	if rule.Kind != types.KindDatabaseObjectImportRule {
		return trace.BadParameter("invalid kind %v, expected %v", rule.Kind, types.KindDatabaseObjectImportRule)
	}
	if rule.Spec == nil {
		return trace.BadParameter("missing spec")
	}
	if len(rule.Spec.DatabaseLabels) == 0 {
		return trace.BadParameter("missing database_labels")
	}
	if len(rule.Spec.Mappings) == 0 {
		return trace.BadParameter("missing mappings")
	}
	for _, mapping := range rule.Spec.Mappings {
		for key, template := range mapping.AddLabels {
			if strings.TrimSpace(key) == "" {
				return trace.BadParameter("invalid mapping: label name is empty or whitespace")
			}
			err := validateTemplate(template)
			if err != nil {
				return trace.Wrap(err, "mapping value failed to parse as template")
			}
		}
	}
	return nil
}
