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
	spec := msg.GetSpec()
	if spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	metadata := headerv1.FromMetadataProto(msg.GetHeader().GetMetadata())

	accessListSpec := accesslist.Spec{
		Type:               accesslist.Type(msg.GetSpec().GetType()),
		Title:              spec.GetTitle(),
		Description:        spec.GetDescription(),
		Owners:             convertOwnersFromProto(spec.GetOwners()),
		Audit:              convertAuditFromProto(spec.GetAudit()),
		MembershipRequires: convertRequiresFromProto(spec.GetMembershipRequires()),
		OwnershipRequires:  convertRequiresFromProto(spec.GetOwnershipRequires()),
		Grants:             convertGrantsFromProto(spec.GetGrants()),
		OwnerGrants:        convertGrantsFromProto(spec.GetOwnerGrants()),
	}

	accessList, err := accesslist.NewAccessList(metadata, accessListSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessList.SetSubKind(msg.GetHeader().GetSubKind())

	if status := fromStatusProto(msg); status != nil {
		accessList.Status = *status
	}
	for _, opt := range opts {
		opt(accessList)
	}

	return accessList, nil
}

// ToProto converts an internal access list into a v1 access list object.
func ToProto(accessList *accesslist.AccessList) *accesslistv1.AccessList {
	if accessList == nil {
		return nil
	}

	return &accesslistv1.AccessList{
		Header: headerv1.ToResourceHeaderProto(accessList.ResourceHeader),
		Spec: &accesslistv1.AccessListSpec{
			Type:               string(accessList.Spec.Type),
			Title:              accessList.Spec.Title,
			Description:        accessList.Spec.Description,
			Owners:             convertOwnersToProto(accessList.Spec.Owners),
			Audit:              convertAuditToProto(accessList.Spec.Audit),
			MembershipRequires: convertRequiresToProto(accessList.Spec.MembershipRequires),
			OwnershipRequires:  convertRequiresToProto(accessList.Spec.OwnershipRequires),
			Grants:             convertGrantsToProto(accessList.Spec.Grants),
			OwnerGrants:        convertGrantsToProto(accessList.Spec.OwnerGrants),
		},
		Status: convertStatusToProto(&accessList.Status),
	}
}

func convertAuditFromProto(audit *accesslistv1.AccessListAudit) accesslist.Audit {
	if audit == nil {
		return accesslist.Audit{}
	}
	return accesslist.Audit{
		NextAuditDate: convertTimeFromProto(audit.GetNextAuditDate()),
		Recurrence: accesslist.Recurrence{
			Frequency:  accesslist.ReviewFrequency(audit.GetRecurrence().GetFrequency()),
			DayOfMonth: accesslist.ReviewDayOfMonth(audit.GetRecurrence().GetDayOfMonth()),
		},
		Notifications: convertNotificationsFromProto(audit.GetNotifications()),
	}
}

func convertTimeFromProto(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}

func convertNotificationsFromProto(notifications *accesslistv1.Notifications) accesslist.Notifications {
	if notifications.GetStart() == nil {
		return accesslist.Notifications{}
	}
	return accesslist.Notifications{
		Start: notifications.GetStart().AsDuration(),
	}
}

func convertRequiresFromProto(requires *accesslistv1.AccessListRequires) accesslist.Requires {
	if requires == nil {
		return accesslist.Requires{}
	}
	return accesslist.Requires{
		Roles:  requires.GetRoles(),
		Traits: traitv1.FromProto(requires.GetTraits()),
	}
}
func convertOwnersFromProto(protoOwners []*accesslistv1.AccessListOwner) []accesslist.Owner {
	owners := make([]accesslist.Owner, len(protoOwners))
	for i, owner := range protoOwners {
		owners[i] = FromOwnerProto(owner)
		owners[i].IneligibleStatus = "" // default, overridden via option if needed
	}
	return owners
}

func convertGrantsFromProto(protoGrants *accesslistv1.AccessListGrants) accesslist.Grants {
	if protoGrants == nil {
		return accesslist.Grants{}
	}
	return accesslist.Grants{
		Roles:  protoGrants.GetRoles(),
		Traits: traitv1.FromProto(protoGrants.GetTraits()),
	}
}

// ToOwnerProto converts an internal access list owner into a v1 access list owner object.
func ToOwnerProto(owner accesslist.Owner) *accesslistv1.AccessListOwner {
	var ineligibleStatus accesslistv1.IneligibleStatus
	if owner.IneligibleStatus != "" {
		if enumVal, ok := accesslistv1.IneligibleStatus_value[owner.IneligibleStatus]; ok {
			ineligibleStatus = accesslistv1.IneligibleStatus(enumVal)
		}
	} else {
		ineligibleStatus = accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED
	}

	var kind accesslistv1.MembershipKind
	if enumVal, ok := accesslistv1.MembershipKind_value[owner.MembershipKind]; ok {
		kind = accesslistv1.MembershipKind(enumVal)
	}

	return &accesslistv1.AccessListOwner{
		Name:             owner.Name,
		Description:      owner.Description,
		IneligibleStatus: ineligibleStatus,
		MembershipKind:   kind,
	}
}

// FromOwnerProto converts a v1 access list owner into an internal access list owner object.
func FromOwnerProto(protoOwner *accesslistv1.AccessListOwner) accesslist.Owner {
	if protoOwner == nil {
		return accesslist.Owner{}
	}
	ineligibleStatus := ""
	if protoOwner.GetIneligibleStatus() != accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED {
		ineligibleStatus = protoOwner.GetIneligibleStatus().String()
	}

	return accesslist.Owner{
		Name:             protoOwner.GetName(),
		Description:      protoOwner.GetDescription(),
		IneligibleStatus: ineligibleStatus,
		MembershipKind:   protoOwner.GetMembershipKind().String(),
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

func convertStatusToProto(status *accesslist.Status) *accesslistv1.AccessListStatus {
	if status == nil {
		return nil
	}

	return &accesslistv1.AccessListStatus{
		MemberCount:            copyPointer(status.MemberCount),
		MemberListCount:        copyPointer(status.MemberListCount),
		OwnerOf:                status.OwnerOf,
		MemberOf:               status.MemberOf,
		CurrentUserAssignments: toCurrentUserAssignmentsProto(status.CurrentUserAssignments),
	}
}

func fromStatusProto(msg *accesslistv1.AccessList) *accesslist.Status {
	protoStatus := msg.GetStatus()
	if protoStatus == nil {
		return nil
	}

	return &accesslist.Status{
		MemberCount:            copyPointer(protoStatus.MemberCount),
		MemberListCount:        copyPointer(protoStatus.MemberListCount),
		OwnerOf:                protoStatus.GetOwnerOf(),
		MemberOf:               protoStatus.GetMemberOf(),
		CurrentUserAssignments: fromCurrentUserAssignmentsProto(protoStatus.GetCurrentUserAssignments()),
	}
}

func copyPointer[T any](val *T) *T {
	if val == nil {
		return nil
	}
	out := *val
	return &out
}

func toCurrentUserAssignmentsProto(assignments *accesslist.CurrentUserAssignments) *accesslistv1.CurrentUserAssignments {
	if assignments == nil {
		return nil
	}
	return &accesslistv1.CurrentUserAssignments{
		OwnershipType:  assignments.OwnershipType,
		MembershipType: assignments.MembershipType,
	}
}

func fromCurrentUserAssignmentsProto(assignments *accesslistv1.CurrentUserAssignments) *accesslist.CurrentUserAssignments {
	if assignments == nil {
		return nil
	}
	return &accesslist.CurrentUserAssignments{
		OwnershipType:  assignments.GetOwnershipType(),
		MembershipType: assignments.GetMembershipType(),
	}
}
func convertOwnersToProto(owners []accesslist.Owner) []*accesslistv1.AccessListOwner {
	protoOwners := make([]*accesslistv1.AccessListOwner, len(owners))
	for i, owner := range owners {
		protoOwners[i] = ToOwnerProto(owner)
	}
	return protoOwners
}

func convertGrantsToProto(grants accesslist.Grants) *accesslistv1.AccessListGrants {
	if len(grants.Roles) == 0 && len(grants.Traits) == 0 {
		return nil
	}

	return &accesslistv1.AccessListGrants{
		Roles:  grants.Roles,
		Traits: traitv1.ToProto(grants.Traits),
	}
}

func convertAuditToProto(audit accesslist.Audit) *accesslistv1.AccessListAudit {
	if audit == (accesslist.Audit{}) {
		return nil
	}

	var nextAuditDate *timestamppb.Timestamp
	if !audit.NextAuditDate.IsZero() {
		nextAuditDate = timestamppb.New(audit.NextAuditDate)
	}

	return &accesslistv1.AccessListAudit{
		NextAuditDate: nextAuditDate,
		Recurrence: &accesslistv1.Recurrence{
			Frequency:  accesslistv1.ReviewFrequency(audit.Recurrence.Frequency),
			DayOfMonth: accesslistv1.ReviewDayOfMonth(audit.Recurrence.DayOfMonth),
		},
		Notifications: &accesslistv1.Notifications{
			Start: durationpb.New(audit.Notifications.Start),
		},
	}
}

func convertRequiresToProto(requires accesslist.Requires) *accesslistv1.AccessListRequires {
	return &accesslistv1.AccessListRequires{
		Roles:  requires.Roles,
		Traits: traitv1.ToProto(requires.Traits),
	}
}
