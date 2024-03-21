// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
