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
	// OriginalRoles are the user roles that are part of the user's static definition. These roles
	// are not affected by access granted by access lists and are obtained prior to granting access
	// list access. Basically, [OriginalRoles] = [Roles] - [AccessListRoles].
	OriginalRoles []string `json:"original_roles" yaml:"original_roles"`

	// OriginalTraits are the user traits that are part of the user's static definition. These
	// traits are not affected by access granted by access lists and are obtained prior to granting
	// access list access. Basically, [OriginalTraits] = [Traits] - [AccessListTraits].
	OriginalTraits trait.Traits `json:"original_traits" yaml:"original_traits"`

	// Roles are the user roles attached to the user. Basically, [Roles] = [OriginalRoles] +
	// [AccessListRoles].
	Roles []string `json:"roles" yaml:"roles"`

	// Traits are the traits attached to the user. Basically, [roles] = [original_traits] +
	// [access_list_traits].
	Traits trait.Traits `json:"traits" yaml:"traits"`

	// AccessListRoles are roles granted to this user by the Access Lists
	// membership/ownership.  Basically, [AccessListRoles] = [Roles] - [OriginalRoles].
	AccessListRoles []string `json:"access_list_roles,omitempty" yaml:"access_list_roles"`

	// AccessListTraits are traits granted to this user by the Access Lists membership/ownership.
	// Basically, [AccessListTraits] = [Traits] - [OriginalTraits].
	AccessListTraits trait.Traits `json:"access_list_traits" yaml:"access_list_traits"`

	// UserType is the type of user that this state represents.
	UserType types.UserType `json:"user_type" yaml:"user_type"`

	// GitHubIdentity is user's attached GitHub identity
	GitHubIdentity *ExternalIdentity `json:"github_identity,omitempty" yaml:"github_identity"`

	// SAMLIdentities are the identities created from the SAML connectors used to log in by
	// this user name.
	//
	// NOTE: There is no mechanism to clean those identities. If the the user is deleted, the
	// user_login_state and it's saml_identities will not be deleted. Or even if the user still
	// exists, but it's SAML identity expires it isn't cleared from the user_login_state. This means
	// the information stored here can be used only as long as there is a background sync running and
	// making sure the user's info is up-to-date. E.g. Okta assignment creator is using this
	// information, but it is running only when Okta user sync is active and periodically updates the
	// user which in turn updates the user_login_state.
	//
	// NOTE2: This field isn't currently used. It's introduced so we can resolve the
	// https://github.com/gravitational/teleport.e/issues/6723 issue in stages.
	// The STAGE 1 is to introduce this field and give enough time to get existing Teleport
	// installations to get updated and populate this field.
	// The STAGE 2 is in the v19 release (or maybe even v20) to deploy the actual fix PR
	// (https://github.com/gravitational/teleport.e/pull/7168) reading this field and calculating
	// access to Okta resources. See more details in the description of the fix PR.
	//
	// TODO(kopiczko) v19: consider proceeding with the STAGE 2 described above.
	SAMLIdentities []ExternalIdentity `json:"saml_identities,omitempty" yaml:"saml_identities"`
}

// ExternalIdentity defines an external identity attached to this user state.
type ExternalIdentity struct {
	// ConnectorID is the connector this identity was created with. It's empty for the local
	// user.
	ConnectorID string
	// UserId is the unique identifier of the external identity such as GitHub
	// user ID.
	UserID string
	// Username is the username of the external identity.
	Username string
	// GrantedRoles specific for this identity. E.g.: from connector attributes mapping.
	GrantedRoles []string
	// GrantedTraits specific for this identity. E.g.: from connector roles attributes mapping.
	GrantedTraits trait.Traits
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

// Clone returns a copy of the member.
func (u *UserLoginState) Clone() *UserLoginState {
	if u == nil {
		return nil
	}
	out := &UserLoginState{}
	deriveDeepCopyUserLoginState(out, u)
	return out
}

// IsEqual compares two user login states for equality.
func (u *UserLoginState) IsEqual(i *UserLoginState) bool {
	return deriveTeleportEqualUserLoginState(u, i)
}

// GetOriginalRoles returns the original roles that the user login state was derived from. It's the
// same as GetRoles() - GetOriginalRoles().
func (u *UserLoginState) GetOriginalRoles() []string {
	return u.Spec.OriginalRoles
}

// GetOriginalTraits returns the original traits that the user login state was derived from. It's
// the same as GetTraits() - GetAccessListTraits().
func (u *UserLoginState) GetOriginalTraits() map[string][]string {
	return u.Spec.OriginalTraits
}

// GetRoles returns the roles attached to the user login state. It's the same as GetOriginalRoles()
// + GetAccessListRoles().
func (u *UserLoginState) GetRoles() []string {
	return u.Spec.Roles
}

// GetTraits returns the traits attached to the user login state. It's the same as
// GetOriginalTraits() + GetAccessListTraits().
func (u *UserLoginState) GetTraits() map[string][]string {
	return u.Spec.Traits
}

// GetAccessListRoles returns roles granted to this user by the Access Lists membership/ownership.
// It's the same as GetRoles() - GetOriginalRoles().
func (u *UserLoginState) GetAccessListRoles() []string {
	return u.Spec.AccessListRoles
}

// GetAccessListTraits returns traits granted to this user by the Access Lists
// membership/ownership. It's the same as GetTraits() - GetOriginalTraits().
func (u *UserLoginState) GetAccessListTraits() map[string][]string {
	return u.Spec.AccessListTraits
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

// GetLabel fetches the given user label, with the same semantics
// as a map read
func (u *UserLoginState) GetLabel(key string) (value string, ok bool) {
	value, ok = u.Metadata.Labels[key]
	return
}

// GetGithubIdentities returns a list of connected Github identities
func (u *UserLoginState) GetGithubIdentities() []types.ExternalIdentity {
	if u.Spec.GitHubIdentity == nil {
		return nil
	}
	return []types.ExternalIdentity{{
		UserID:   u.Spec.GitHubIdentity.UserID,
		Username: u.Spec.GitHubIdentity.Username,
	}}
}

// SetGithubIdentities sets the list of connected GitHub identities.
// Note that currently only one identity is kept in UserLoginState.
func (u *UserLoginState) SetGithubIdentities(identities []types.ExternalIdentity) {
	if len(identities) == 0 {
		u.Spec.GitHubIdentity = nil
	} else {
		u.Spec.GitHubIdentity = &ExternalIdentity{
			UserID:   identities[0].UserID,
			Username: identities[0].Username,
		}
	}
}
