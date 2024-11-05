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

	"github.com/gravitational/trace"
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

// Custom json.Marshaller and json.Unmarshaler are here to cope with inconsistencies between our CRD and go types.
// They are invoked when the kubernetes client converts the unstructured object into a typed resource.
// We have two inconsistencies:
// - the utils.Strings typr that marshals inconsistently: single elements are strings, multiple elements are lists
// - the max_age setting which is an embedded pointer to another single-value message, which breaks JSON parsing

// MarshalJSON serializes a spec into a JSON string
func (spec TeleportOIDCConnectorSpec) MarshalJSON() ([]byte, error) {
	type Alias TeleportOIDCConnectorSpec

	var maxAge types.Duration
	if spec.MaxAge != nil {
		maxAge = spec.MaxAge.Value
	}

	return json.Marshal(&struct {
		RedirectURLs []string       `json:"redirect_url,omitempty"`
		MaxAge       types.Duration `json:"max_age,omitempty"`
		Alias
	}{
		RedirectURLs: spec.RedirectURLs,
		MaxAge:       maxAge,
		Alias:        (Alias)(spec),
	})
}

// UnmarshalJSON serializes a JSON string into a spec. This override is required to deal with the
// MaxAge field which is special case because it' an object embedded into the spec.
func (spec *TeleportOIDCConnectorSpec) UnmarshalJSON(data []byte) error {
	*spec = *new(TeleportOIDCConnectorSpec)
	type Alias TeleportOIDCConnectorSpec

	temp := &struct {
		MaxAge types.Duration `json:"max_age"`
		*Alias
	}{
		Alias: (*Alias)(spec),
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return trace.Wrap(err, "unmarshalling custom teleport oidc connector spec")
	}
	if temp.MaxAge != 0 {
		spec.MaxAge = &types.MaxAge{Value: temp.MaxAge}
	}
	return nil
}
