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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportUser is the Schema for the users API
type TeleportUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportUserSpec `json:"spec"`
	Status resources.Status `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportUserList contains a list of TeleportUser
type TeleportUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
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

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
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
