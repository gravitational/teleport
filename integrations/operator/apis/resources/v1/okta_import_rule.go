/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package v1

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportOktaImportRule{}, &TeleportOktaImportRuleList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportOktaImportRule holds the kubernetes custom resources for okta import rules.
type TeleportOktaImportRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportOktaImportRuleSpec `json:"spec"`
	Status resources.Status           `json:"status"`
}

// TeleportOktaImportRuleSpec matches the JSON of generated CRD spec
type TeleportOktaImportRuleSpec struct {
	Priority int32                           `json:"priority,omitempty"`
	Mappings []TeleportOktaImportRuleMapping `json:"mappings,omitempty"`
}

// TeleportOktaImportRuleMapping matches the JSON of a mapping definition
type TeleportOktaImportRuleMapping struct {
	Match     []TeleportOktaImportRuleMatch `json:"match,omitempty"`
	AddLabels map[string]string             `json:"add_labels,omitempty"`
}

// TeleportOktaImportRuleMatch matches the JSON of a match definition.
type TeleportOktaImportRuleMatch struct {
	AppIDs           []string `json:"app_ids,omitempty"`
	GroupIDs         []string `json:"group_ids,omitempty"`
	AppNameRegexes   []string `json:"app_name_regexes,omitempty"`
	GroupNameRegexes []string `json:"group_name_regexes,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportOktaImportRuleList contains a list of TeleportOktaImportRule
type TeleportOktaImportRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportOktaImportRule `json:"items"`
}

// ToTeleport returns an OktaImportRule, which wraps the actual
// [types.OktaImportRuleV1] and implements the necessary interface methods used
// by the TeleportResourceReconciler.
func (o TeleportOktaImportRule) ToTeleport() types.OktaImportRule {
	importRule := &types.OktaImportRuleV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:        o.Name,
				Labels:      o.Labels,
				Description: o.Annotations[resources.DescriptionKey],
			},
			Version: types.V1,
		},
		Spec: types.OktaImportRuleSpecV1{
			Priority: o.Spec.Priority,
			Mappings: make([]*types.OktaImportRuleMappingV1, len(o.Spec.Mappings)),
		},
	}

	for i, mapping := range o.Spec.Mappings {
		matches := make([]*types.OktaImportRuleMatchV1, len(mapping.Match))
		for j, match := range mapping.Match {
			matches[j] = &types.OktaImportRuleMatchV1{
				AppIDs:           match.AppIDs,
				GroupIDs:         match.GroupIDs,
				AppNameRegexes:   match.AppNameRegexes,
				GroupNameRegexes: match.GroupNameRegexes,
			}
		}
		importRule.Spec.Mappings[i] = &types.OktaImportRuleMappingV1{
			Match:     matches,
			AddLabels: maps.Clone(mapping.AddLabels),
		}
	}
	return importRule
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportOktaImportRule) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}
