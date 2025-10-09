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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportRoleV6{}, &TeleportRoleV6List{})
}

// TeleportRoleV6Spec defines the desired state of TeleportRoleV6
type TeleportRoleV6Spec types.RoleSpecV6

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportRoleV6 is the Schema for the roles API
type TeleportRoleV6 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportRoleV6Spec `json:"spec,omitempty"`
	Status resources.Status   `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportRoleV6List contains a list of TeleportRoleV6
type TeleportRoleV6List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportRoleV6 `json:"items"`
}

func (r TeleportRoleV6) ToTeleport() types.Role {
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

// Marshal serializes a spec into binary data.
func (spec *TeleportRoleV6Spec) Marshal() ([]byte, error) {
	return (*types.RoleSpecV6)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportRoleV6Spec) Unmarshal(data []byte) error {
	return (*types.RoleSpecV6)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportRoleV6Spec) DeepCopyInto(out *TeleportRoleV6Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportRoleV6Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportRoleV6) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
