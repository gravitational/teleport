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

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportLoginRule{}, &TeleportLoginRuleList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportLoginRule holds the kubernetes custom resources for login rules.
type TeleportLoginRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportLoginRuleSpec   `json:"spec,omitempty"`
	Status TeleportLoginRuleStatus `json:"status,omitempty"`
}

// TeleportLoginRuleSpec matches the JSON of generated CRD spec
// ([loginrulepb.LoginRule] does not actually have a spec field).
type TeleportLoginRuleSpec struct {
	Priority         int32               `json:"priority,omitempty"`
	TraitsExpression string              `json:"traits_expression,omitempty"`
	TraitsMap        map[string][]string `json:"traits_map,omitempty"`
}

type TeleportLoginRuleStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportLoginRuleList contains a list of TeleportLoginRule
type TeleportLoginRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
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

func (l *LoginRuleResource) GetMetadata() types.Metadata {
	return *l.LoginRule.Metadata
}
