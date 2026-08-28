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

	uls, err := userloginstate.New(headerv1.FromMetadataProto(msg.GetHeader().GetMetadata()), userloginstate.Spec{
		OriginalRoles:    msg.GetSpec().GetOriginalRoles(),
		OriginalTraits:   traitv1.FromProto(msg.GetSpec().GetOriginalTraits()),
		AccessListRoles:  msg.GetSpec().GetAccessListRoles(),
		AccessListTraits: traitv1.FromProto(msg.GetSpec().GetAccessListTraits()),
		Roles:            msg.GetSpec().GetRoles(),
		Traits:           traitv1.FromProto(msg.GetSpec().GetTraits()),
		UserType:         types.UserType(msg.GetSpec().GetUserType()),
		GitHubIdentity:   externalIdentityFromProto(msg.GetSpec().GetGitHubIdentity()),
		SAMLIdentities:   externalIdentitiesFromProto(msg.GetSpec().GetSamlIdentities()),
	})

	return uls, trace.Wrap(err)
}

// ToProto converts an internal user login state into a v1 user login state object.
func ToProto(uls *userloginstate.UserLoginState) *userloginstatev1.UserLoginState {
	return &userloginstatev1.UserLoginState{
		Header: headerv1.ToResourceHeaderProto(uls.ResourceHeader),
		Spec: &userloginstatev1.Spec{
			OriginalRoles:    uls.GetOriginalRoles(),
			OriginalTraits:   traitv1.ToProto(uls.GetOriginalTraits()),
			AccessListRoles:  uls.GetAccessListRoles(),
			AccessListTraits: traitv1.ToProto(uls.GetAccessListTraits()),
			Roles:            uls.GetRoles(),
			Traits:           traitv1.ToProto(uls.GetTraits()),
			UserType:         string(uls.Spec.UserType),
			GitHubIdentity:   externalIdentityToProto(uls.Spec.GitHubIdentity),
			SamlIdentities:   externalIdentitiesToProto(uls.Spec.SAMLIdentities),
		},
	}
}

func externalIdentityFromProto(identity *userloginstatev1.ExternalIdentity) *userloginstate.ExternalIdentity {
	if identity != nil {
		return &userloginstate.ExternalIdentity{
			ConnectorID:   identity.ConnectorId,
			UserID:        identity.UserId,
			Username:      identity.Username,
			GrantedRoles:  identity.GrantedRoles,
			GrantedTraits: traitv1.FromProto(identity.GrantedTraits),
		}
	}
	return nil
}

func externalIdentitiesFromProto(identities []*userloginstatev1.ExternalIdentity) []userloginstate.ExternalIdentity {
	var res []userloginstate.ExternalIdentity
	for _, identity := range identities {
		res = append(res, *externalIdentityFromProto(identity))
	}
	return res
}

func externalIdentityToProto(identity *userloginstate.ExternalIdentity) *userloginstatev1.ExternalIdentity {
	if identity != nil {
		return &userloginstatev1.ExternalIdentity{
			ConnectorId:   identity.ConnectorID,
			UserId:        identity.UserID,
			Username:      identity.Username,
			GrantedRoles:  identity.GrantedRoles,
			GrantedTraits: traitv1.ToProto(identity.GrantedTraits),
		}
	}
	return nil
}

func externalIdentitiesToProto(identities []userloginstate.ExternalIdentity) []*userloginstatev1.ExternalIdentity {
	var res []*userloginstatev1.ExternalIdentity
	for _, identity := range identities {
		res = append(res, externalIdentityToProto(&identity))
	}
	return res
}
