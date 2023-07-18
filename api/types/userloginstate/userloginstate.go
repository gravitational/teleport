/*
Copyright 2023 Gravitational, Inc.

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

package userloginstate

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/types/trait"
)

// UserLoginState is the ephemeral user login state. This will hold data to differentiate
// from the User object.
type UserLoginState struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the user login state.
	Spec Spec `json:"spec" yaml:"spec"`
}

// Spec is the specification for the user login st ate.
type Spec struct {
	// Roles is the list of roles attached to the user login state.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits attached to the user login state.
	Traits trait.Traits `json:"traits" yaml:"traits"`
}

// New will create a new user login state.
func New(metadata header.Metadata, spec Spec) (*UserLoginState, error) {
	userLoginState := &UserLoginState{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}

	if err := userLoginState.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return userLoginState, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (u *UserLoginState) CheckAndSetDefaults() error {
	u.SetKind(types.KindUserLoginState)
	u.SetVersion(types.V1)

	if err := u.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetRoles returns the roles attached to the user login state.
func (u *UserLoginState) GetRoles() []string {
	return u.Spec.Roles
}

// GetTraits returns the traits attached to the user login state.
func (u *UserLoginState) GetTraits() trait.Traits {
	return u.Spec.Traits
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (u *UserLoginState) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(u.Metadata)
}
