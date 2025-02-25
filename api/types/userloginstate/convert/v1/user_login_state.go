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

package v1

import (
	"github.com/gravitational/trace"

	userloginstatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	"github.com/gravitational/teleport/api/types"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	traitv1 "github.com/gravitational/teleport/api/types/trait/convert/v1"
	"github.com/gravitational/teleport/api/types/userloginstate"
)

// FromProto converts a v1 user login state into an internal user login state object.
func FromProto(msg *userloginstatev1.UserLoginState) (*userloginstate.UserLoginState, error) {
	if msg.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	uls, err := userloginstate.New(headerv1.FromMetadataProto(msg.Header.Metadata), userloginstate.Spec{
		OriginalRoles:  msg.Spec.GetOriginalRoles(),
		OriginalTraits: traitv1.FromProto(msg.Spec.OriginalTraits),
		Roles:          msg.Spec.Roles,
		Traits:         traitv1.FromProto(msg.Spec.Traits),
		UserType:       types.UserType(msg.Spec.UserType),
		GitHubIdentity: externalIdentityFromProto(msg.Spec.GitHubIdentity),
	})

	return uls, trace.Wrap(err)
}

// ToProto converts an internal user login state into a v1 user login state object.
func ToProto(uls *userloginstate.UserLoginState) *userloginstatev1.UserLoginState {
	return &userloginstatev1.UserLoginState{
		Header: headerv1.ToResourceHeaderProto(uls.ResourceHeader),
		Spec: &userloginstatev1.Spec{
			OriginalRoles:  uls.GetOriginalRoles(),
			OriginalTraits: traitv1.ToProto(uls.GetOriginalTraits()),
			Roles:          uls.GetRoles(),
			Traits:         traitv1.ToProto(uls.GetTraits()),
			UserType:       string(uls.Spec.UserType),
			GitHubIdentity: externalIdentityToProto(uls.Spec.GitHubIdentity),
		},
	}
}

func externalIdentityFromProto(identity *userloginstatev1.ExternalIdentity) *userloginstate.ExternalIdentity {
	if identity != nil {
		return &userloginstate.ExternalIdentity{
			UserID:   identity.UserId,
			Username: identity.Username,
		}
	}
	return nil
}

func externalIdentityToProto(identity *userloginstate.ExternalIdentity) *userloginstatev1.ExternalIdentity {
	if identity != nil {
		return &userloginstatev1.ExternalIdentity{
			UserId:   identity.UserID,
			Username: identity.Username,
		}
	}
	return nil
}
