/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportWorkloadClusterV1{}, &TeleportWorkloadClusterV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportWorkloadClusterV1 holds the kubernetes custom resources for
// WorkloadCluster
type TeleportWorkloadClusterV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportWorkloadClusterV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status              `json:"status"`
}

// TeleportWorkloadClusterV1Spec defines the desired state of TeleportWorkloadClusterV1
type TeleportWorkloadClusterV1Spec workloadclusterv1.WorkloadClusterSpec

//+kubebuilder:object:root=true

// TeleportWorkloadClusterV1List contains a list of TeleportWorkloadClusterV1
type TeleportWorkloadClusterV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportWorkloadClusterV1 `json:"items"`
}

// ToTeleport returns a WorkloadCluster, which wraps the actual
// [workloadclusterv1.WorkloadCluster] and implements the necessary interface
// methods used by the TeleportResourceReconciler.
func (l *TeleportWorkloadClusterV1) ToTeleport() *workloadclusterv1.WorkloadCluster {
	resource := &workloadclusterv1.WorkloadCluster{
		Kind:    types.KindWorkloadCluster,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[teleportcr.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*workloadclusterv1.WorkloadClusterSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportWorkloadClusterV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the WorkloadClusterSpec to
// protojson, which is necessary for the WorkloadClusterSpec (and other Proto
// RFD153 resources) to be unmarshaled correctly from the unstructured object.
func (spec *TeleportWorkloadClusterV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*workloadclusterv1.WorkloadClusterSpec)(spec))
}

// MarshalJSON delegates marshaling of the WorkloadClusterSpec to protojson,
// which is necessary for the WorkloadClusterSpec (and other Proto RFD153
// resources) to be marshaled correctly into the unstructured object.
func (spec *TeleportWorkloadClusterV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*workloadclusterv1.WorkloadClusterSpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportWorkloadClusterV1Spec) DeepCopyInto(out *TeleportWorkloadClusterV1Spec) {
	proto.Reset((*workloadclusterv1.WorkloadClusterSpec)(out))
	proto.Merge((*workloadclusterv1.WorkloadClusterSpec)(out), (*workloadclusterv1.WorkloadClusterSpec)(spec))
}
