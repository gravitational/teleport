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
	SchemeBuilder.Register(&TeleportProvisionToken{}, &TeleportProvisionTokenList{})
}

// TeleportProvisionTokenSpec defines the desired state of TeleportProvisionToken
type TeleportProvisionTokenSpec types.ProvisionTokenSpecV2

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportProvisionToken is the Schema for the ProvisionToken API
type TeleportProvisionToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportProvisionTokenSpec `json:"spec"`
	Status resources.Status           `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportProvisionTokenList contains a list of TeleportProvisionToken
type TeleportProvisionTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportProvisionToken `json:"items"`
}

func (c TeleportProvisionToken) ToTeleport() types.ProvisionToken {
	return &types.ProvisionTokenV2{
		Kind:    types.KindToken,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        c.Name,
			Labels:      c.Labels,
			Description: c.Annotations[resources.DescriptionKey],
		},
		Spec: types.ProvisionTokenSpecV2(c.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (c *TeleportProvisionToken) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportProvisionTokenSpec) Marshal() ([]byte, error) {
	return (*types.ProvisionTokenSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportProvisionTokenSpec) Unmarshal(data []byte) error {
	return (*types.ProvisionTokenSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportProvisionTokenSpec) DeepCopyInto(out *TeleportProvisionTokenSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportProvisionTokenSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
