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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportLoginRule{}, &TeleportLoginRuleList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportLoginRule holds the kubernetes custom resources for login rules.
type TeleportLoginRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportLoginRuleSpec `json:"spec"`
	Status resources.Status      `json:"status"`
}

// TeleportLoginRuleSpec matches the JSON of generated CRD spec
// ([loginrulepb.LoginRule] does not actually have a spec field).
type TeleportLoginRuleSpec struct {
	Priority         int32               `json:"priority,omitempty"`
	TraitsExpression string              `json:"traits_expression,omitempty"`
	TraitsMap        map[string][]string `json:"traits_map,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportLoginRuleList contains a list of TeleportLoginRule
type TeleportLoginRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportLoginRule `json:"items"`
}

// ToTeleport returns a LoginRuleResource, which wraps the actual
// [loginrulepb.LoginRule] and implements the necessary interface methods used
// by the TeleportResourceReconciler.
func (l TeleportLoginRule) ToTeleport() *LoginRuleResource {
	resource := &LoginRuleResource{
		LoginRule: &loginrulepb.LoginRule{
			Metadata: &types.Metadata{
				Name:        l.Name,
				Labels:      l.Labels,
				Description: l.Annotations[resources.DescriptionKey],
			},
			Version:          types.V1,
			Priority:         l.Spec.Priority,
			TraitsExpression: l.Spec.TraitsExpression,
		},
	}
	if len(l.Spec.TraitsMap) > 0 {
		resource.LoginRule.TraitsMap = make(map[string]*wrappers.StringValues, len(l.Spec.TraitsMap))
	}
	for traitKey, traitExpressions := range l.Spec.TraitsMap {
		resource.LoginRule.TraitsMap[traitKey] = &wrappers.StringValues{Values: traitExpressions}
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportLoginRule) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// +kubebuilder:object:generate=false

// LoginRuleResource wraps [loginrulepb.LoginRule] in order to implement the
// interface methods used by TeleportResourceReconciler.
type LoginRuleResource struct {
	LoginRule *loginrulepb.LoginRule
}

func (l *LoginRuleResource) GetName() string {
	return l.LoginRule.Metadata.Name
}

func (l *LoginRuleResource) SetOrigin(origin string) {
	l.LoginRule.Metadata.SetOrigin(origin)
}

func (l *LoginRuleResource) Origin() string {
	return l.LoginRule.Metadata.Origin()
}

func (l *LoginRuleResource) GetRevision() string {
	return l.LoginRule.Metadata.GetRevision()
}

func (l *LoginRuleResource) SetRevision(rev string) {
	l.LoginRule.Metadata.SetRevision(rev)
}
