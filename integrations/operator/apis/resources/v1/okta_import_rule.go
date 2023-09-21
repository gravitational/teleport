// Copyright 2023 Gravitational, Inc
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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
	"github.com/gravitational/teleport/lib/utils"
)

func init() {
	SchemeBuilder.Register(&TeleportOktaImportRule{}, &TeleportOktaImportRuleList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportOktaImportRule holds the kubernetes custom resources for okta import rules.
type TeleportOktaImportRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportOktaImportRuleSpec   `json:"spec,omitempty"`
	Status TeleportOktaImportRuleStatus `json:"status,omitempty"`
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
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportOktaImportRule `json:"items"`
}

type TeleportOktaImportRuleStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
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
			AddLabels: utils.CopyStringsMap(mapping.AddLabels),
		}
	}
	return importRule
}

func (l *TeleportOktaImportRule) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}
