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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportUser{}, &TeleportUserList{})
}

// TeleportUserSpec defines the desired state of TeleportUser
type TeleportUserSpec types.UserSpecV2

// TeleportUserStatus defines the observed state of TeleportUser
type TeleportUserStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportUser is the Schema for the users API
type TeleportUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportUserSpec   `json:"spec,omitempty"`
	Status TeleportUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportUserList contains a list of TeleportUser
type TeleportUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportUser `json:"items"`
}

func (u TeleportUser) ToTeleport() types.User {
	return &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        u.Name,
			Labels:      u.Labels,
			Description: u.Annotations[resources.DescriptionKey],
		},
		Spec: types.UserSpecV2(u.Spec),
	}
}

func (u *TeleportUser) StatusConditions() *[]metav1.Condition {
	return &u.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportUserSpec) Marshal() ([]byte, error) {
	return (*types.UserSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportUserSpec) Unmarshal(data []byte) error {
	return (*types.UserSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportUserSpec) DeepCopyInto(out *TeleportUserSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportUserSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
