/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

const DescriptionKey = "description"

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UserSpec defines the desired state of User
type UserSpec types.UserSpecV2

// UserStatus defines the observed state of User
type UserStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}

func (u User) ToTeleport() types.User {
	return &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        u.Name,
			Labels:      u.Labels,
			Description: u.Annotations[DescriptionKey],
		},
		Spec: types.UserSpecV2(u.Spec),
	}
}

// Marshal serializes a spec into binary data.
func (spec *UserSpec) Marshal() ([]byte, error) {
	return (*types.UserSpecV2)(spec).Marshal()
}

// Unmarshal deserializes a spec from binary data.
func (spec *UserSpec) Unmarshal(data []byte) error {
	return (*types.UserSpecV2)(spec).Unmarshal(data)
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *UserSpec) DeepCopyInto(out *UserSpec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = UserSpec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
