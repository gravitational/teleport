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

// TeleportOIDCConnectorStatus defines the observed state of TeleportOIDCConnector
type TeleportOIDCConnectorStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportOIDCConnector is the Schema for the OIDCConnector API
type TeleportOIDCConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportOIDCConnectorSpec   `json:"spec,omitempty"`
	Status TeleportOIDCConnectorStatus `json:"status,omitempty"`
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
