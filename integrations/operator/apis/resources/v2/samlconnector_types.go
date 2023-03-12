/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

// TeleportSAMLConnectorStatus defines the observed state of TeleportSAMLConnector
type TeleportSAMLConnectorStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportSAMLConnector is the Schema for the SAMLConnector API
type TeleportSAMLConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportSAMLConnectorSpec   `json:"spec,omitempty"`
	Status TeleportSAMLConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportSAMLConnectorList contains a list of TeleportSAMLConnector
type TeleportSAMLConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
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
