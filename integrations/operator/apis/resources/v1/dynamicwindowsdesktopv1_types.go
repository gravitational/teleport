/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportDynamicWindowsDesktopV1{}, &TeleportDynamicWindowsDesktopV1List{})
}

// TeleportDynamicWindowsDesktopV1Spec defines the desired state of TeleportDynamicWindowsDesktopV1
type TeleportDynamicWindowsDesktopV1Spec types.DynamicWindowsDesktopSpecV1

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportDynamicWindowsDesktopV1 is the Schema for the dynamicwindowsdesktopsv1 API
type TeleportDynamicWindowsDesktopV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportDynamicWindowsDesktopV1Spec `json:"spec"`
	Status teleportcr.Status                   `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportDynamicWindowsDesktopV1List contains a list of TeleportDynamicWindowsDesktopV1
type TeleportDynamicWindowsDesktopV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportDynamicWindowsDesktopV1 `json:"items"`
}

// ToTeleport returns a DynamicWindowsDesktop that can be sent to Teleport.
func (r TeleportDynamicWindowsDesktopV1) ToTeleport() types.DynamicWindowsDesktop {
	return &types.DynamicWindowsDesktopV1{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindDynamicWindowsDesktop,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:        r.Name,
				Labels:      r.Labels,
				Description: r.Annotations[teleportcr.DescriptionKey],
			},
		},
		Spec: types.DynamicWindowsDesktopSpecV1(r.Spec),
	}
}

// Marshal serializes a spec into binary data.
func (spec *TeleportDynamicWindowsDesktopV1Spec) Marshal() ([]byte, error) {
	return (*types.DynamicWindowsDesktopSpecV1)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportDynamicWindowsDesktopV1Spec) Unmarshal(data []byte) error {
	return (*types.DynamicWindowsDesktopSpecV1)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one dynamic Windows desktop spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportDynamicWindowsDesktopV1Spec) DeepCopyInto(out *TeleportDynamicWindowsDesktopV1Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportDynamicWindowsDesktopV1Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportDynamicWindowsDesktopV1) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
