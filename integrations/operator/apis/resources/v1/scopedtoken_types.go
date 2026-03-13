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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	tokenv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportScopedTokenV1{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportScopedTokenV1 represents a Kubernetes custom resource for
// Scoped Tokens.
type TeleportScopedTokenV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Scope  string                     `json:"scope"`
	Spec   *TeleportScopedTokenV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status          `json:"status"`
}

// TeleportScopedTokenV1Spec defines the desired state of the Scoped Token.
type TeleportScopedTokenV1Spec tokenv1.ScopedTokenSpec

//+kubebuilder:object:root = true

// TeleportScopedTokenV1List contains a list of [TeleportScopedTokenV1]
// objects.
type TeleportScopedTokenV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportScopedTokenV1 `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (m *TeleportScopedTokenV1) ToTeleport() *tokenv1.ScopedToken {
	resource := &tokenv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        m.Name,
			Description: m.Annotations[teleportcr.DescriptionKey],
			Labels:      m.Labels,
		},
		Scope: m.Scope,
		Spec:  (*tokenv1.ScopedTokenSpec)(m.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (m *TeleportScopedTokenV1) StatusConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportScopedTokenV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportScopedTokenV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*tokenv1.ScopedTokenSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportScopedTokenV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportScopedTokenV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*tokenv1.ScopedTokenSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportScopedTokenV1Spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportScopedTokenV1Spec) DeepCopyInto(out *TeleportScopedTokenV1Spec) {
	proto.Reset((*tokenv1.ScopedTokenSpec)(out))
	proto.Merge((*tokenv1.ScopedTokenSpec)(out), (*tokenv1.ScopedTokenSpec)(spec))
}
