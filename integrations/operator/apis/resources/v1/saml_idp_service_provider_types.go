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
	SchemeBuilder.Register(&TeleportSAMLIdPServiceProviderV1{}, &TeleportSAMLIdPServiceProviderV1List{})
}

// TeleportSAMLIdPServiceProviderV1Spec defines the desired state of TeleportSAMLIdPServiceProviderV1.
type TeleportSAMLIdPServiceProviderV1Spec types.SAMLIdPServiceProviderSpecV1

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportSAMLIdPServiceProviderV1 is the Schema for the SAMLIdPServiceProvider API.
type TeleportSAMLIdPServiceProviderV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportSAMLIdPServiceProviderV1Spec `json:"spec"`
	Status teleportcr.Status                    `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportSAMLIdPServiceProviderV1List contains a list of TeleportSAMLIdPServiceProviderV1.
type TeleportSAMLIdPServiceProviderV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportSAMLIdPServiceProviderV1 `json:"items"`
}

func (s TeleportSAMLIdPServiceProviderV1) ToTeleport() types.SAMLIdPServiceProvider {
	return &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindSAMLIdPServiceProvider,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:        s.Name,
				Labels:      s.Labels,
				Description: s.Annotations[teleportcr.DescriptionKey],
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1(s.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (s *TeleportSAMLIdPServiceProviderV1) StatusConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportSAMLIdPServiceProviderV1Spec) Marshal() ([]byte, error) {
	return (*types.SAMLIdPServiceProviderSpecV1)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportSAMLIdPServiceProviderV1Spec) Unmarshal(data []byte) error {
	return (*types.SAMLIdPServiceProviderSpecV1)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportSAMLIdPServiceProviderV1Spec) DeepCopyInto(out *TeleportSAMLIdPServiceProviderV1Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportSAMLIdPServiceProviderV1Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
