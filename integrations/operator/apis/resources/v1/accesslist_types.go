// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	accesslist "github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportAccessList{}, &TeleportAccessListList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportAccessList holds the kubernetes custom resources for login rules.
type TeleportAccessList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportAccessListSpec `json:"spec"`
	Status resources.Status       `json:"status"`
}

// TeleportAccessListSpec defines the desired state of TeleportProvisionToken
type TeleportAccessListSpec accesslist.Spec

//+kubebuilder:object:root=true

// TeleportAccessListList contains a list of TeleportAccessList
type TeleportAccessListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportAccessList `json:"items"`
}

// ToTeleport returns a AccessListResource, which wraps the actual
// [accesslist.AccessList] and implements the necessary interface methods used
// by the TeleportResourceReconciler.
func (l TeleportAccessList) ToTeleport() *accesslist.AccessList {
	resource := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Kind:    "",
			SubKind: "",
			Version: types.V1,
			Metadata: header.Metadata{
				Name:        l.Name,
				Description: l.Annotations[resources.DescriptionKey],
				Labels:      l.Labels,
			},
		},
		Spec: accesslist.Spec(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportAccessList) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportAccessListSpec) Marshal() ([]byte, error) {
	return json.Marshal(spec)
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportAccessListSpec) Unmarshal(data []byte) error {
	return json.Unmarshal(data, spec)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportAccessListSpec) DeepCopyInto(out *TeleportAccessListSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportAccessListSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
