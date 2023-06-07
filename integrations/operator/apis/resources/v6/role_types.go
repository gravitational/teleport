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

package v6

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
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
		Version: types.V6,
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
