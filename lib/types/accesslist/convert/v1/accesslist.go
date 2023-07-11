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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/lib/types/accesslist"
	headerv1 "github.com/gravitational/teleport/lib/types/header/convert/v1"
	traitv1 "github.com/gravitational/teleport/lib/types/trait/convert/v1"
)

// FromProto converts a v1 access list into an internal access list object.
func FromProto(msg *accesslistv1.AccessList) (*accesslist.AccessList, error) {
	owners := make([]accesslist.AccessListOwner, len(msg.Spec.Owners))
	for i, owner := range msg.Spec.Owners {
		owners[i] = accesslist.AccessListOwner{
			Name:        owner.Name,
			Description: owner.Description,
		}
	}

	members := make([]accesslist.AccessListMember, len(msg.Spec.Members))
	for i, member := range msg.Spec.Members {
		members[i] = accesslist.AccessListMember{
			Name:    member.Name,
			Joined:  member.Joined.AsTime(),
			Expires: member.Expires.AsTime(),
			Reason:  member.Reason,
			AddedBy: member.AddedBy,
		}
	}

	accessList, err := accesslist.NewAccessList(headerv1.FromMetadataProto(msg.Header.Metadata), accesslist.AccessListSpec{
		Description: msg.Spec.Description,
		Owners:      owners,
		Audit: accesslist.AccessListAudit{
			Frequency: msg.Spec.Audit.Frequency.AsDuration(),
		},
		MembershipRequires: accesslist.AccessListRequires{
			Roles:  msg.Spec.MembershipRequires.Roles,
			Traits: traitv1.FromProto(msg.Spec.MembershipRequires.Traits),
		},
		OwnershipRequires: accesslist.AccessListRequires{
			Roles:  msg.Spec.OwnershipRequires.Roles,
			Traits: traitv1.FromProto(msg.Spec.OwnershipRequires.Traits),
		},
		Grants: accesslist.AccessListGrants{
			Roles:  msg.Spec.Grants.Roles,
			Traits: traitv1.FromProto(msg.Spec.Grants.Traits),
		},
		Members: members,
	})

	return accessList, trace.Wrap(err)
}

// ToProto converts an internal access list into a v1 access list object.
func ToProto(accessList *accesslist.AccessList) *accesslistv1.AccessList {
	owners := make([]*accesslistv1.AccessListOwner, len(accessList.Spec.Owners))
	for i, owner := range accessList.Spec.Owners {
		owners[i] = &accesslistv1.AccessListOwner{
			Name:        owner.Name,
			Description: owner.Description,
		}
	}

	members := make([]*accesslistv1.AccessListMember, len(accessList.Spec.Members))
	for i, member := range accessList.Spec.Members {
		members[i] = &accesslistv1.AccessListMember{
			Name:    member.Name,
			Joined:  timestamppb.New(member.Joined),
			Expires: timestamppb.New(member.Expires),
			Reason:  member.Reason,
			AddedBy: member.AddedBy,
		}
	}

	return &accesslistv1.AccessList{
		Header: headerv1.ToResourceHeaderProto(accessList.ResourceHeader),
		Spec: &accesslistv1.AccessListSpec{
			Description: accessList.Spec.Description,
			Owners:      owners,
			Audit: &accesslistv1.AccessListAudit{
				Frequency: durationpb.New(accessList.Spec.Audit.Frequency),
			},
			MembershipRequires: &accesslistv1.AccessListRequires{
				Roles:  accessList.Spec.MembershipRequires.Roles,
				Traits: traitv1.ToProto(accessList.Spec.MembershipRequires.Traits),
			},
			OwnershipRequires: &accesslistv1.AccessListRequires{
				Roles:  accessList.Spec.OwnershipRequires.Roles,
				Traits: traitv1.ToProto(accessList.Spec.OwnershipRequires.Traits),
			},
			Grants: &accesslistv1.AccessListGrants{
				Roles:  accessList.Spec.Grants.Roles,
				Traits: traitv1.ToProto(accessList.Spec.Grants.Traits),
			},
			Members: members,
		},
	}
}
