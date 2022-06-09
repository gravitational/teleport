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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

const DescriptionKey = "description"

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RoleSpec defines the desired state of Role
type RoleSpec types.RoleSpecV5

// RoleStatus defines the observed state of Role
type RoleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Role is the Schema for the roles API
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec,omitempty"`
	Status RoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}

func (r Role) ToTeleport() types.Role {
	return &types.RoleV5{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[DescriptionKey],
		},
		Spec: types.RoleSpecV5(r.Spec),
	}
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}

// Marshal serializes a spec into binary data.
func (spec *RoleSpec) Marshal() ([]byte, error) {
	return (*types.RoleSpecV5)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *RoleSpec) Unmarshal(data []byte) error {
	return (*types.RoleSpecV5)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *RoleSpec) DeepCopyInto(out *RoleSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = RoleSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
