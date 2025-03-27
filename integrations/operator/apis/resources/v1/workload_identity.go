// Copyright 2025 Gravitational, Inc
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

	"google.golang.org/protobuf/encoding/protojson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportWorkloadIdentity{}, &TeleportWorkloadIdentityList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportWorkloadIdentity holds the kubernetes custom resources for
// WorkloadIdentity
type TeleportWorkloadIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *TeleportWorkloadIdentitySpec `json:"spec,omitempty"`
	Status resources.Status              `json:"status,omitempty"`
}

// TeleportWorkloadIdentitySpec defines the desired state of TeleportWorkloadIdentity
type TeleportWorkloadIdentitySpec workloadidentityv1.WorkloadIdentitySpec

//+kubebuilder:object:root=true

// TeleportWorkloadIdentityList contains a list of TeleportWorkloadIdentity
type TeleportWorkloadIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportWorkloadIdentity `json:"items"`
}

// ToTeleport returns a WorkloadIdentity, which wraps the actual
// [workloadidentityv1.WorkloadIdentity] and implements the necessary interface
// methods used by the TeleportResourceReconciler.
func (l *TeleportWorkloadIdentity) ToTeleport() *workloadidentityv1.WorkloadIdentity {
	resource := &workloadidentityv1.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[resources.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*workloadidentityv1.WorkloadIdentitySpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportWorkloadIdentity) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportWorkloadIdentitySpec) Marshal() ([]byte, error) {
	// TODO(noah): use protojson??
	return json.Marshal(spec)
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportWorkloadIdentitySpec) Unmarshal(data []byte) error {
	// TODO(noah): use protojson??
	return json.Unmarshal(data, spec)
}

// UnmarshalJSON delegates unmarshaling of the WorkloadIdentitySpec to
// protojson, which is necessary for the WorkloadIdentitySpec (and other Proto
// RFD153 resources) to be unmarshaled correctly from the unstructured object.
func (spec *TeleportWorkloadIdentitySpec) UnmarshalJSON(data []byte) error {
	return protojson.Unmarshal(data, (*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// MarshalJSON delegates marshaling of the WorkloadIdentitySpec to protojson,
// which is necessary for the WorkloadIdentitySpec (and other Proto RFD153
// resources) to be marshaled correctly into the unstructured object.
func (spec *TeleportWorkloadIdentitySpec) MarshalJSON() ([]byte, error) {
	return protojson.Marshal((*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportWorkloadIdentitySpec) DeepCopyInto(out *TeleportWorkloadIdentitySpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportWorkloadIdentitySpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
