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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportGithubConnector{}, &TeleportGithubConnectorList{})
}

// TeleportGithubConnectorSpec defines the desired state of TeleportGithubConnector
type TeleportGithubConnectorSpec types.GithubConnectorSpecV3

// TeleportGithubConnectorStatus defines the observed state of TeleportGithubConnector
type TeleportGithubConnectorStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	TeleportResourceID int64 `json:"teleportResourceID,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportGithubConnector is the Schema for the GithubConnector API
type TeleportGithubConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportGithubConnectorSpec   `json:"spec,omitempty"`
	Status TeleportGithubConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportGithubConnectorList contains a list of TeleportGithubConnector
type TeleportGithubConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
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
