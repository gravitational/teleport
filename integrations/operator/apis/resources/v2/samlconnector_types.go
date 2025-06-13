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
	SchemeBuilder.Register(&TeleportSAMLConnector{}, &TeleportSAMLConnectorList{})
}

// TeleportSAMLConnectorSpec defines the desired state of TeleportSAMLConnector
type TeleportSAMLConnectorSpec types.SAMLConnectorSpecV2

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportSAMLConnector is the Schema for the SAMLConnector API
type TeleportSAMLConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportSAMLConnectorSpec `json:"spec"`
	Status resources.Status          `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportSAMLConnectorList contains a list of TeleportSAMLConnector
type TeleportSAMLConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportSAMLConnector `json:"items"`
}

func (c TeleportSAMLConnector) ToTeleport() types.SAMLConnector {
	return &types.SAMLConnectorV2{
		Kind:    types.KindSAMLConnector,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        c.Name,
			Labels:      c.Labels,
			Description: c.Annotations[resources.DescriptionKey],
		},
		Spec: types.SAMLConnectorSpecV2(c.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (c *TeleportSAMLConnector) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportSAMLConnectorSpec) Marshal() ([]byte, error) {
	return (*types.SAMLConnectorSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportSAMLConnectorSpec) Unmarshal(data []byte) error {
	return (*types.SAMLConnectorSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportSAMLConnectorSpec) DeepCopyInto(out *TeleportSAMLConnectorSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportSAMLConnectorSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
