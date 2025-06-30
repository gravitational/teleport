/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	SchemeBuilder.Register(&TeleportRoleV8{}, &TeleportRoleV8List{})
}

// TeleportRoleV8Spec defines the desired state of TeleportRoleV8
type TeleportRoleV8Spec TeleportRoleV7Spec

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportRoleV8 is the Schema for the roles API
type TeleportRoleV8 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportRoleV8Spec `json:"spec"`
	Status resources.Status   `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportRoleV8List contains a list of TeleportRoleV8
type TeleportRoleV8List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportRoleV8 `json:"items"`
}

func (r TeleportRoleV8) ToTeleport() types.Role {
	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V8,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: types.RoleSpecV6(r.Spec),
	}
}

// Marshal serializes a spec into binary data.
func (spec *TeleportRoleV8Spec) Marshal() ([]byte, error) {
	return (*TeleportRoleV7Spec)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportRoleV8Spec) Unmarshal(data []byte) error {
	return (*TeleportRoleV7Spec)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportRoleV8Spec) DeepCopyInto(out *TeleportRoleV8Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportRoleV8Spec{}
	if err := out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportRoleV8) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
