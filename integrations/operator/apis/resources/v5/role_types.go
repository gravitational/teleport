/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v5

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/utils"
)

func init() {
	SchemeBuilder.Register(&TeleportRole{}, &TeleportRoleList{})
}

// TeleportRoleSpec defines the desired state of TeleportRole
type TeleportRoleSpec types.RoleSpecV6

// TeleportRoleStatus defines the observed state of TeleportRole
type TeleportRoleStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportRole is the Schema for the roles API
type TeleportRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportRoleSpec   `json:"spec,omitempty"`
	Status TeleportRoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportRoleList contains a list of TeleportRole
type TeleportRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportRole `json:"items"`
}

func (r TeleportRole) ToTeleport() types.Role {
	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: types.RoleSpecV6(r.Spec),
	}
}

func init() {
	SchemeBuilder.Register(&TeleportRole{}, &TeleportRoleList{})
}

// Marshal serializes a spec into binary data.
func (spec *TeleportRoleSpec) Marshal() ([]byte, error) {
	return (*types.RoleSpecV6)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportRoleSpec) Unmarshal(data []byte) error {
	return (*types.RoleSpecV6)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportRoleSpec) DeepCopyInto(out *TeleportRoleSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportRoleSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportRole) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}

// MarshalJSON serializes a spec into a JSON string
func (spec TeleportRoleSpec) MarshalJSON() ([]byte, error) {
	type Alias TeleportRoleSpec

	return json.Marshal(&struct {
		Allow TeleportRoleConditions `json:"allow,omitempty"`
		Deny  TeleportRoleConditions `json:"deny,omitempty"`
		Alias
	}{
		Allow: TeleportRoleConditions(spec.Allow),
		Deny:  TeleportRoleConditions(spec.Deny),
		Alias: (Alias)(spec),
	})
}

type TeleportRoleConditions types.RoleConditions

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
// This is here to make controller-gen happy, it's actually used.
func (cond *TeleportRoleConditions) DeepCopyInto(out *TeleportRoleConditions) {
	data, err := cond.MarshalJSON()
	if err != nil {
		panic(err)
	}
	*out = TeleportRoleConditions{}
	if err = json.Unmarshal(data, out); err != nil {
		panic(err)
	}
}

func (cond TeleportRoleConditions) MarshalJSON() ([]byte, error) {
	type Alias types.RoleConditions

	return json.Marshal(&struct {
		NodeLabels            map[string][]string `json:"node_labels,omitempty"`
		AppLabels             map[string][]string `json:"app_labels,omitempty"`
		ClusterLabels         map[string][]string `json:"cluster_labels,omitempty"`
		KubernetesLabels      map[string][]string `json:"kubernetes_labels,omitempty"`
		DatabaseLabels        map[string][]string `json:"db_labels,omitempty"`
		WindowsDesktopLabels  map[string][]string `json:"windows_desktop_labels,omitempty"`
		DatabaseServiceLabels map[string][]string `json:"database_service_labels,omitempty"`
		GroupLabels           map[string][]string `json:"group_labels,omitempty"`

		Alias
	}{
		NodeLabels:            utils.LabelsToMap(cond.NodeLabels),
		AppLabels:             utils.LabelsToMap(cond.AppLabels),
		ClusterLabels:         utils.LabelsToMap(cond.ClusterLabels),
		KubernetesLabels:      utils.LabelsToMap(cond.KubernetesLabels),
		DatabaseLabels:        utils.LabelsToMap(cond.DatabaseLabels),
		WindowsDesktopLabels:  utils.LabelsToMap(cond.WindowsDesktopLabels),
		DatabaseServiceLabels: utils.LabelsToMap(cond.DatabaseServiceLabels),
		GroupLabels:           utils.LabelsToMap(cond.GroupLabels),
		Alias:                 Alias(cond),
	})
}
