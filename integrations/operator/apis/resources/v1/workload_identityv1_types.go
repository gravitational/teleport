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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportWorkloadIdentityV1{}, &TeleportWorkloadIdentityV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportWorkloadIdentityV1 holds the kubernetes custom resources for
// WorkloadIdentity
type TeleportWorkloadIdentityV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *TeleportWorkloadIdentityV1Spec `json:"spec,omitempty"`
	Status resources.Status                `json:"status,omitempty"`
}

// TeleportWorkloadIdentityV1Spec defines the desired state of TeleportWorkloadIdentityV1
type TeleportWorkloadIdentityV1Spec workloadidentityv1.WorkloadIdentitySpec

//+kubebuilder:object:root=true

// TeleportWorkloadIdentityV1List contains a list of TeleportWorkloadIdentityV1
type TeleportWorkloadIdentityV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportWorkloadIdentityV1 `json:"items"`
}

// ToTeleport returns a WorkloadIdentity, which wraps the actual
// [workloadidentityv1.WorkloadIdentity] and implements the necessary interface
// methods used by the TeleportResourceReconciler.
func (l *TeleportWorkloadIdentityV1) ToTeleport() *workloadidentityv1.WorkloadIdentity {
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
func (l *TeleportWorkloadIdentityV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the WorkloadIdentitySpec to
// protojson, which is necessary for the WorkloadIdentitySpec (and other Proto
// RFD153 resources) to be unmarshaled correctly from the unstructured object.
func (spec *TeleportWorkloadIdentityV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// MarshalJSON delegates marshaling of the WorkloadIdentitySpec to protojson,
// which is necessary for the WorkloadIdentitySpec (and other Proto RFD153
// resources) to be marshaled correctly into the unstructured object.
func (spec *TeleportWorkloadIdentityV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.Marshal((*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportWorkloadIdentityV1Spec) DeepCopyInto(out *TeleportWorkloadIdentityV1Spec) {
	proto.Reset((*workloadidentityv1.WorkloadIdentitySpec)(out))
	proto.Merge((*workloadidentityv1.WorkloadIdentitySpec)(out), (*workloadidentityv1.WorkloadIdentitySpec)(spec))
}
