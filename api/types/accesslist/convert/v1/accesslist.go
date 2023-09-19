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
	"github.com/gravitational/teleport/api/types/accesslist"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	traitv1 "github.com/gravitational/teleport/api/types/trait/convert/v1"
)

type AccessListOption func(*accesslist.AccessList)

// FromProto converts a v1 access list into an internal access list object.
func FromProto(msg *accesslistv1.AccessList, opts ...AccessListOption) (*accesslist.AccessList, error) {
	if msg == nil {
		return nil, trace.BadParameter("access list message is nil")
	}

	if msg.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}
	if msg.Spec.Audit == nil {
		return nil, trace.BadParameter("audit is missing")
	}
	if msg.Spec.MembershipRequires == nil {
		return nil, trace.BadParameter("membershipRequires is missing")
	}
	if msg.Spec.OwnershipRequires == nil {
		return nil, trace.BadParameter("ownershipRequires is missing")
	}
	if msg.Spec.Grants == nil {
		return nil, trace.BadParameter("grants is missing")
	}

	owners := make([]accesslist.Owner, len(msg.Spec.Owners))
	for i, owner := range msg.Spec.Owners {
		owners[i] = accesslist.Owner{
			Name:        owner.Name,
			Description: owner.Description,
			// Set it to empty as default.
			// Must provide as options to set it with the provided value.
			IneligibleStatus: "",
		}
	}

	accessList, err := accesslist.NewAccessList(headerv1.FromMetadataProto(msg.Header.Metadata), accesslist.Spec{
		Title:       msg.Spec.Title,
		Description: msg.Spec.Description,
		Owners:      owners,
		Audit: accesslist.Audit{
			Frequency:     msg.Spec.Audit.Frequency.AsDuration(),
			NextAuditDate: msg.Spec.Audit.NextAuditDate.AsTime(),
		},
		MembershipRequires: accesslist.Requires{
			Roles:  msg.Spec.MembershipRequires.Roles,
			Traits: traitv1.FromProto(msg.Spec.MembershipRequires.Traits),
		},
		OwnershipRequires: accesslist.Requires{
			Roles:  msg.Spec.OwnershipRequires.Roles,
			Traits: traitv1.FromProto(msg.Spec.OwnershipRequires.Traits),
		},
		Grants: accesslist.Grants{
			Roles:  msg.Spec.Grants.Roles,
			Traits: traitv1.FromProto(msg.Spec.Grants.Traits),
		},
	})

	for _, opt := range opts {
		opt(accessList)
	}

	return accessList, trace.Wrap(err)
}

// ToProto converts an internal access list into a v1 access list object.
func ToProto(accessList *accesslist.AccessList) *accesslistv1.AccessList {
	owners := make([]*accesslistv1.AccessListOwner, len(accessList.Spec.Owners))
	for i, owner := range accessList.Spec.Owners {
		var ineligibleStatus accesslistv1.IneligibleStatus
		if enumVal, ok := accesslistv1.IneligibleStatus_value[owner.IneligibleStatus]; ok {
			ineligibleStatus = accesslistv1.IneligibleStatus(enumVal)
		}
		owners[i] = &accesslistv1.AccessListOwner{
			Name:             owner.Name,
			Description:      owner.Description,
			IneligibleStatus: ineligibleStatus,
		}
	}

	return &accesslistv1.AccessList{
		Header: headerv1.ToResourceHeaderProto(accessList.ResourceHeader),
		Spec: &accesslistv1.AccessListSpec{
			Title:       accessList.Spec.Title,
			Description: accessList.Spec.Description,
			Owners:      owners,
			Audit: &accesslistv1.AccessListAudit{
				Frequency:     durationpb.New(accessList.Spec.Audit.Frequency),
				NextAuditDate: timestamppb.New(accessList.Spec.Audit.NextAuditDate),
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
		},
	}
}

func WithOwnersIneligibleStatusField(protoOwners []*accesslistv1.AccessListOwner) AccessListOption {
	return func(a *accesslist.AccessList) {
		updatedOwners := make([]accesslist.Owner, len(a.GetOwners()))
		for i, owner := range a.GetOwners() {
			protoIneligibleStatus := protoOwners[i].GetIneligibleStatus()
			ineligibleStatus := ""
			if protoIneligibleStatus != accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED {
				ineligibleStatus = protoIneligibleStatus.String()
			}
			owner.IneligibleStatus = ineligibleStatus
			updatedOwners[i] = owner
		}
		a.SetOwners(updatedOwners)
	}
}
