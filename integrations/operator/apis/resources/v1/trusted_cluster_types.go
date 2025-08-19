/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	SchemeBuilder.Register(&TeleportTrustedClusterV2{}, &TeleportTrustedClusterV2List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportTrustedClusterV2 is the Schema for the trusted_clusters API
type TeleportTrustedClusterV2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportTrustedClusterV2Spec `json:"spec"`
	Status resources.Status             `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportTrustedClusterV2List contains a list of TeleportTrustedClusterV2
type TeleportTrustedClusterV2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportTrustedClusterV2 `json:"items"`
}

// ToTeleport converts the resource to the teleport trusted_cluster API type.
func (r TeleportTrustedClusterV2) ToTeleport() types.TrustedCluster {
	return &types.TrustedClusterV2{
		Kind:    types.KindTrustedCluster,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: types.TrustedClusterSpecV2(r.Spec),
	}
}

// TeleportTrustedClusterV2Spec defines the desired state of TeleportTrustedClusterV2
type TeleportTrustedClusterV2Spec types.TrustedClusterSpecV2

// Marshal serializes a spec into binary data.
func (spec *TeleportTrustedClusterV2Spec) Marshal() ([]byte, error) {
	return (*types.TrustedClusterSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportTrustedClusterV2Spec) Unmarshal(data []byte) error {
	return (*types.TrustedClusterSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one trusted_cluster spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportTrustedClusterV2Spec) DeepCopyInto(out *TeleportTrustedClusterV2Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportTrustedClusterV2Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportTrustedClusterV2) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
