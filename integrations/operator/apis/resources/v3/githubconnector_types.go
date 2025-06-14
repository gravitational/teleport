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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportGithubConnector{}, &TeleportGithubConnectorList{})
}

// TeleportGithubConnectorSpec defines the desired state of TeleportGithubConnector
type TeleportGithubConnectorSpec types.GithubConnectorSpecV3

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportGithubConnector is the Schema for the GithubConnector API
type TeleportGithubConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportGithubConnectorSpec `json:"spec"`
	Status resources.Status            `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportGithubConnectorList contains a list of TeleportGithubConnector
type TeleportGithubConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportGithubConnector `json:"items"`
}

func (c TeleportGithubConnector) ToTeleport() types.GithubConnector {
	return &types.GithubConnectorV3{
		Kind:    types.KindGithubConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:        c.Name,
			Labels:      c.Labels,
			Description: c.Annotations[resources.DescriptionKey],
		},
		Spec: types.GithubConnectorSpecV3(c.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (c *TeleportGithubConnector) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportGithubConnectorSpec) Marshal() ([]byte, error) {
	return (*types.GithubConnectorSpecV3)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportGithubConnectorSpec) Unmarshal(data []byte) error {
	return (*types.GithubConnectorSpecV3)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportGithubConnectorSpec) DeepCopyInto(out *TeleportGithubConnectorSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportGithubConnectorSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
