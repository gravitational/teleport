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
	"time"

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

	var recurrence accesslist.Recurrence
	if msg.Spec.Audit.Recurrence != nil {
		recurrence.Frequency = accesslist.ReviewFrequency(msg.Spec.Audit.Recurrence.Frequency)
		recurrence.DayOfMonth = accesslist.ReviewDayOfMonth(msg.Spec.Audit.Recurrence.DayOfMonth)
	}

	var notifications accesslist.Notifications
	if msg.Spec.Audit.Notifications != nil {
		if msg.Spec.Audit.Notifications.Start != nil {
			notifications.Start = msg.Spec.Audit.Notifications.Start.AsDuration()
		}
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

	var ownerGrants accesslist.Grants
	if msg.Spec.OwnerGrants != nil {
		ownerGrants.Roles = msg.Spec.OwnerGrants.Roles
		if msg.Spec.OwnerGrants.Traits != nil {
			ownerGrants.Traits = traitv1.FromProto(msg.Spec.OwnerGrants.Traits)
		}
	}

	// We map the zero protobuf time (nil) to the zero go time.
	// NewAccessList will handle this properly and set a time in the future
	// based on the recurrence rules.
	var nextAuditDate time.Time
	if msg.Spec.Audit.NextAuditDate != nil {
		nextAuditDate = msg.Spec.Audit.NextAuditDate.AsTime()
	}

	var memberCount *uint32
	if msg.Status != nil && msg.Status.MemberCount != nil {
		memberCount = new(uint32)
		*memberCount = *msg.Status.MemberCount
	}

	accessList, err := accesslist.NewAccessList(headerv1.FromMetadataProto(msg.Header.Metadata), accesslist.Spec{
		Title:       msg.Spec.Title,
		Description: msg.Spec.Description,
		Owners:      owners,
		Audit: accesslist.Audit{
			NextAuditDate: nextAuditDate,
			Recurrence:    recurrence,
			Notifications: notifications,
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
		OwnerGrants: ownerGrants,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessList.Status = accesslist.Status{
		MemberCount: memberCount,
	}

	for _, opt := range opts {
		opt(accessList)
	}

	return accessList, nil
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

	var ownerGrants *accesslistv1.AccessListGrants
	if len(accessList.Spec.OwnerGrants.Roles) > 0 {
		ownerGrants = &accesslistv1.AccessListGrants{
			Roles: accessList.Spec.OwnerGrants.Roles,
		}
	}

	if len(accessList.Spec.OwnerGrants.Traits) > 0 {
		if ownerGrants == nil {
			ownerGrants = &accesslistv1.AccessListGrants{}
		}

		ownerGrants.Traits = traitv1.ToProto(accessList.Spec.OwnerGrants.Traits)
	}

	// We map the zero go time to the zero protobuf time (nil).
	var nextAuditDate *timestamppb.Timestamp
	if !accessList.Spec.Audit.NextAuditDate.IsZero() {
		nextAuditDate = timestamppb.New(accessList.Spec.Audit.NextAuditDate)
	}

	var memberCount *uint32
	if accessList.Status.MemberCount != nil {
		memberCount = new(uint32)
		*memberCount = *accessList.Status.MemberCount
	}

	return &accesslistv1.AccessList{
		Header: headerv1.ToResourceHeaderProto(accessList.ResourceHeader),
		Spec: &accesslistv1.AccessListSpec{
			Title:       accessList.Spec.Title,
			Description: accessList.Spec.Description,
			Owners:      owners,
			Audit: &accesslistv1.AccessListAudit{
				NextAuditDate: nextAuditDate,
				Recurrence: &accesslistv1.Recurrence{
					Frequency:  accesslistv1.ReviewFrequency(accessList.Spec.Audit.Recurrence.Frequency),
					DayOfMonth: accesslistv1.ReviewDayOfMonth(accessList.Spec.Audit.Recurrence.DayOfMonth),
				},
				Notifications: &accesslistv1.Notifications{
					Start: durationpb.New(accessList.Spec.Audit.Notifications.Start),
				},
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
			OwnerGrants: ownerGrants,
		},
		Status: &accesslistv1.AccessListStatus{
			MemberCount: memberCount,
		},
	}
}

// WithOwnersIneligibleStatusField sets the "ineligibleStatus" field to the provided proto value.
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
