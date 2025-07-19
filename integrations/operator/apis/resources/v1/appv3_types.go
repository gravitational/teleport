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
	SchemeBuilder.Register(&TeleportAppV3{}, &TeleportAppV3List{})
}

// TeleportAppV3Spec defines the desired state of TeleportAppV3
type TeleportAppV3Spec types.AppSpecV3

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportAppV3 is the Schema for the roles API
type TeleportAppV3 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportAppV3Spec `json:"spec,omitempty"`
	Status resources.Status  `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportAppV3List contains a list of TeleportAppV3
type TeleportAppV3List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportAppV3 `json:"items"`
}

func (r TeleportAppV3) ToTeleport() types.Application {
	return &types.AppV3{
		Kind:    types.KindApp,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: types.AppSpecV3(r.Spec),
	}
}

// Marshal serializes a spec into binary data.
func (spec *TeleportAppV3Spec) Marshal() ([]byte, error) {
	return (*types.AppSpecV3)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportAppV3Spec) Unmarshal(data []byte) error {
	return (*types.AppSpecV3)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportAppV3Spec) DeepCopyInto(out *TeleportAppV3Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportAppV3Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportAppV3) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
