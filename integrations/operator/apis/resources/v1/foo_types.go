// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package v1

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
	"github.com/gravitational/teleport/lib/foos"
)

func init() {
	SchemeBuilder.Register(&TeleportFooV1{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportFooV1 represents a Kubernetes custom resource for Foos
type TeleportFooV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Scope  string             `json:"scope"`
	Spec   *TeleportFooV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status  `json:"status"`
}

// TeleportFooV1Spec defines the desired state of the Foo.
type TeleportFooV1Spec foov1.FooSpec

//+kubebuilder:object:root = true

// TeleportFooV1List contains a list of [TeleportFooV1]
// objects.
type TeleportFooV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportFooV1 `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (m *TeleportFooV1) ToTeleport() *foov1.Foo {
	resource := foov1.Foo_builder{
		Kind:    foos.Kind,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        m.Name,
			Description: m.Annotations[teleportcr.DescriptionKey],
			Labels:      m.Labels,
		}.Build(),
		Scope: m.Scope,
		Spec:  (*foov1.FooSpec)(m.Spec),
	}.Build()
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (m *TeleportFooV1) StatusConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportFooV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportFooV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*foov1.FooSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportFooV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportFooV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*foov1.FooSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportFooV1Spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportFooV1Spec) DeepCopyInto(out *TeleportFooV1Spec) {
	proto.Reset((*foov1.FooSpec)(out))
	proto.Merge((*foov1.FooSpec)(out), (*foov1.FooSpec)(spec))
}
