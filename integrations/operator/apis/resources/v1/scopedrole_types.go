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
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
	"github.com/gravitational/teleport/lib/scopes/access"
)

func init() {
	SchemeBuilder.Register(&TeleportScopedRoleV1{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportScopedRoleV1 represents a Kubernetes custom resource for
// Scoped Roles.
type TeleportScopedRoleV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Scope  string                    `json:"scope"`
	Spec   *TeleportScopedRoleV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status         `json:"status"`
}

// TeleportScopedRoleV1Spec defines the desired state of the Scoped Role.
type TeleportScopedRoleV1Spec accessv1.ScopedRoleSpec

//+kubebuilder:object:root = true

// TeleportScopedRoleV1List contains a list of [TeleportScopedRoleV1]
// objects.
type TeleportScopedRoleV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportScopedRoleV1 `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (m *TeleportScopedRoleV1) ToTeleport() *accessv1.ScopedRole {
	resource := &accessv1.ScopedRole{
		Kind:    access.KindScopedRole,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        m.Name,
			Description: m.Annotations[teleportcr.DescriptionKey],
			Labels:      m.Labels,
		},
		Scope: m.Scope,
		Spec:  (*accessv1.ScopedRoleSpec)(m.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (m *TeleportScopedRoleV1) StatusConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportScopedRoleV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportScopedRoleV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*accessv1.ScopedRoleSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportScopedRoleV1Spec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportScopedRoleV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*accessv1.ScopedRoleSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportScopedRoleV1Spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportScopedRoleV1Spec) DeepCopyInto(out *TeleportScopedRoleV1Spec) {
	proto.Reset((*accessv1.ScopedRoleSpec)(out))
	proto.Merge((*accessv1.ScopedRoleSpec)(out), (*accessv1.ScopedRoleSpec)(spec))
}
