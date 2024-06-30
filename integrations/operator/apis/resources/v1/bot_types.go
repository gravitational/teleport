/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportBot{}, &TeleportBotList{})
}

// TeleportBotSpec defines the desired state of TeleportBot
type TeleportBotSpec struct {
	Roles  []string                `json:"roles"`
	Traits []TeleportBotSpecTraits `json:"traits"`
}

type TeleportBotSpecTraits struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportBot is the Schema for the Bots API
type TeleportBot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeleportBotSpec  `json:"spec,omitempty"`
	Status resources.Status `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TeleportBotList contains a list of TeleportBot
type TeleportBotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportBot `json:"items"`
}

func (r TeleportBot) ToTeleport() *machineidv1.Bot {
	traits := make([]*machineidv1.Trait, 0, len(r.Spec.Traits))
	for _, trait := range r.Spec.Traits {
		machineidTrait := &machineidv1.Trait{
			Name:   trait.Name,
			Values: trait.Values,
		}
		traits = append(traits, machineidTrait)
	}

	return &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name:        r.Name,
			Labels:      r.Labels,
			Description: r.Annotations[resources.DescriptionKey],
		},
		Spec: &machineidv1.BotSpec{
			Roles:  r.Spec.Roles,
			Traits: traits,
		},
	}
}

// Marshal serializes a spec into binary data
func (spec *TeleportBotSpec) Marshal() ([]byte, error) {
	return json.Marshal(spec)
}

// Unmarshal deserializes a spec from binary data
func (spec *TeleportBotSpec) Unmarshal(data []byte) error {
	return json.Unmarshal(data, spec)
}

// DeepCopyInto deep-copies one role spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportBotSpec) DeepCopyInto(out *TeleportBotSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportBotSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}

// StatusConditions returns a pointer to Status.Conditions slice
func (r *TeleportBot) StatusConditions() *[]metav1.Condition {
	return &r.Status.Conditions
}
