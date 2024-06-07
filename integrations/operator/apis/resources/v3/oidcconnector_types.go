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

package v3

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportOIDCConnector{}, &TeleportOIDCConnectorList{})
}

// TeleportOIDCConnectorSpec defines the desired state of TeleportOIDCConnector
type TeleportOIDCConnectorSpec types.OIDCConnectorSpecV3

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportOIDCConnector is the Schema for the OIDCConnector API
type TeleportOIDCConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportOIDCConnectorSpec `json:"spec,omitempty"`
	Status resources.Status          `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportOIDCConnectorList contains a list of TeleportOIDCConnector
type TeleportOIDCConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportOIDCConnector `json:"items"`
}

func (c TeleportOIDCConnector) ToTeleport() types.OIDCConnector {
	return &types.OIDCConnectorV3{
		Kind:    types.KindOIDCConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:        c.Name,
			Labels:      c.Labels,
			Description: c.Annotations[resources.DescriptionKey],
		},
		Spec: types.OIDCConnectorSpecV3(c.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (c *TeleportOIDCConnector) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportOIDCConnectorSpec) Marshal() ([]byte, error) {
	return (*types.OIDCConnectorSpecV3)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportOIDCConnectorSpec) Unmarshal(data []byte) error {
	return (*types.OIDCConnectorSpecV3)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportOIDCConnectorSpec) DeepCopyInto(out *TeleportOIDCConnectorSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportOIDCConnectorSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// MarshalJSON serializes a spec into a JSON string
func (spec TeleportOIDCConnectorSpec) MarshalJSON() ([]byte, error) {
	type Alias TeleportOIDCConnectorSpec
	return json.Marshal(&struct {
		RedirectURLs []string `json:"redirect_url"`
		Alias
	}{
		RedirectURLs: spec.RedirectURLs,
		Alias:        (Alias)(spec),
	})
}
