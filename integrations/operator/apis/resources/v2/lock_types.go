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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportLock{}, &TeleportLockList{})
}

// TeleportLockSpec defines the desired state of TeleportLock
type TeleportLockSpec types.LockSpecV2

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportLock is the Schema for the Lock API
type TeleportLock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportLockSpec  `json:"spec"`
	Status teleportcr.Status `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportLockList contains a list of TeleportLock
type TeleportLockList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportLock `json:"items"`
}

func (c TeleportLock) ToTeleport() types.Lock {
	return &types.LockV2{
		Kind:    types.KindLock,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        c.Name,
			Labels:      c.Labels,
			Description: c.Annotations[teleportcr.DescriptionKey],
		},
		Spec: types.LockSpecV2(c.Spec),
	}
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (c *TeleportLock) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportLockSpec) Marshal() ([]byte, error) {
	return (*types.LockSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportLockSpec) Unmarshal(data []byte) error {
	return (*types.LockSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportLockSpec) DeepCopyInto(out *TeleportLockSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportLockSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
