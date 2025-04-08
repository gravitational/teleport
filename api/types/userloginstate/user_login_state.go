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

const (
	// OriginalRolesAndTraitsSet is an annotation that will indicate that the original roles and traits have been
	// set for this objects. For existing user login states, this will not be set, indicating that we can't
	// use these fields reliably.
	// DELETE IN 17 (mdwn)
	OriginalRolesAndTraitsSet = types.TeleportInternalLabelPrefix + "original-set"
)

// UserLoginState is the ephemeral user login state. This will hold data to differentiate
// from the User object. This will allow us to store derived roles and traits from
// access lists, login rules, and other mechanisms to more easily incorporate these
// bits of data into user created certificates. It will also allow us to leave the user
// object itself unmodified despite this ephemeral data, which is frequently needed
// despite the dynamic nature of user access.
type UserLoginState struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the user login state.
	Spec Spec `json:"spec" yaml:"spec"`
}

// Spec is the specification for the user login state.
type Spec struct {
	// OriginalRoles is the list of the original roles from the user login state.
	OriginalRoles []string `json:"original_roles" yaml:"original_roles"`

	// OriginalTraits is the list of the original traits from the user login state.
	OriginalTraits trait.Traits `json:"original_traits" yaml:"original_traits"`

	// Roles is the list of roles attached to the user login state.
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits attached to the user login state.
	Traits trait.Traits `json:"traits" yaml:"traits"`

	// UserType is the type of user that this state represents.
	UserType types.UserType `json:"user_type" yaml:"user_type"`
}

// New creates a new user login state.
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

	if err := trace.Wrap(u.ResourceHeader.CheckAndSetDefaults()); err != nil {
		return trace.Wrap(err)
	}

	if u.Spec.UserType == "" {
		u.Spec.UserType = types.UserTypeLocal
	}

	return nil
}

// GetOriginalRoles returns the original roles that the user login state was derived from.
func (u *UserLoginState) GetOriginalRoles() []string {
	return u.Spec.OriginalRoles
}

// GetOriginalTraits returns the original traits that the user login state was derived from.
func (u *UserLoginState) GetOriginalTraits() map[string][]string {
	return u.Spec.OriginalTraits
}

// IsOriginalRolesAndTraitsSet will return true if the original roles and traits annotation is present.
func (u *UserLoginState) IsOriginalRolesAndTraitsSet() bool {
	_, isSet := u.GetLabel(OriginalRolesAndTraitsSet)
	return isSet
}

// GetRoles returns the roles attached to the user login state.
func (u *UserLoginState) GetRoles() []string {
	return u.Spec.Roles
}

// GetTraits returns the traits attached to the user login state.
func (u *UserLoginState) GetTraits() map[string][]string {
	return u.Spec.Traits
}

// GetUserType returns the user type for the user login state.
func (u *UserLoginState) GetUserType() types.UserType {
	return u.Spec.UserType
}

// IsBot returns true if the user is a bot.
func (u *UserLoginState) IsBot() bool {
	_, ok := u.GetMetadata().Labels[types.BotGenerationLabel]
	return ok
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (u *UserLoginState) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(u.Metadata)
}
