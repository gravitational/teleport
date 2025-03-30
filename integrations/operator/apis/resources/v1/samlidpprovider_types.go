/*
 * Teleport
 * Copyright (C) 2023 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportSAMLIdPServiceProvider{}, &TeleportSAMLIdPServiceProviderList{})
}

// TeleportSAMLIdPServiceProviderSpec defines the desired state of TeleportSAMLIdPServiceProvider.
// It aliases the underlying Teleport API spec.
type TeleportSAMLIdPServiceProviderSpec types.SAMLIdPServiceProviderSpecV1

// Marshal serializes the spec into binary data.
func (spec *TeleportSAMLIdPServiceProviderSpec) Marshal() ([]byte, error) {
	return (*types.SAMLIdPServiceProviderSpecV1)(spec).Marshal()
}

// Unmarshal deserializes the spec from binary data.
func (spec *TeleportSAMLIdPServiceProviderSpec) Unmarshal(data []byte) error {
	return (*types.SAMLIdPServiceProviderSpecV1)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one spec into another.
// This is similar to what is done for TeleportRoleV7Spec.
func (spec *TeleportSAMLIdPServiceProviderSpec) DeepCopyInto(out *TeleportSAMLIdPServiceProviderSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportSAMLIdPServiceProviderSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportSAMLIdPServiceProvider is the Schema for the SAML IdP Service Providers API.
type TeleportSAMLIdPServiceProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportSAMLIdPServiceProviderSpec `json:"spec,omitempty"`
	Status resources.Status                   `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportSAMLIdPServiceProviderList contains a list of TeleportSAMLIdPServiceProvider.
type TeleportSAMLIdPServiceProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportSAMLIdPServiceProvider `json:"items"`
}

// ToTeleport converts the custom resource into the corresponding Teleport API SAML IdP Service Provider type.
func (r TeleportSAMLIdPServiceProvider) ToTeleport() types.SAMLIdPServiceProvider {
	return &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:        r.Name,
				Labels:      r.Labels,
				Description: r.Annotations[resources.DescriptionKey],
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1(r.Spec),
	}
}
