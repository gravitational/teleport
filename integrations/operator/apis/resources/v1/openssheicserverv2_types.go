/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportOpenSSHEICEServerV2{}, &TeleportOpenSSHEICEServerV2List{})
}

// TeleportOpenSSHEICEServerV2Spec defines the desired state of TeleportOpenSSHEICEServerV2
type TeleportOpenSSHEICEServerV2Spec types.ServerSpecV2

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportOpenSSHEICEServerV2 is the Schema for the roles API
type TeleportOpenSSHEICEServerV2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TeleportOpenSSHEICEServerV2Spec `json:"spec"`
	Status resources.Status                `json:"status"`
}

//+kubebuilder:object:root=true

// TeleportOpenSSHEICEServerV2List contains a list of TeleportOpenSSHEICEServerV2
type TeleportOpenSSHEICEServerV2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportOpenSSHEICEServerV2 `json:"items"`
}

func (r TeleportOpenSSHEICEServerV2) ToTeleport() types.Server {
	return &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHEICENode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: types.ServerSpecV2(r.Spec),
	}
}

// Marshal serializes a spec into binary data.
func (spec *TeleportOpenSSHEICEServerV2Spec) Marshal() ([]byte, error) {
	return (*types.ServerSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportOpenSSHEICEServerV2Spec) Unmarshal(data []byte) error {
	return (*types.ServerSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportOpenSSHEICEServerV2Spec) DeepCopyInto(out *TeleportOpenSSHEICEServerV2Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportOpenSSHEICEServerV2Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (r *TeleportOpenSSHEICEServerV2) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
