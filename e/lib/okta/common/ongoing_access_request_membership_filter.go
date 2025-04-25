package common

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// OngoingAccessRequestMembershipFilter excludes Okta members that are part of an ongoing access request assignment.
type OngoingAccessRequestMembershipFilter struct {
	// AssignmentsService is used to list Okta assignments.
	// It should interact directly with the backend (not a cache) to avoid
	// propagation delays that could result in missing filtered members.
	AssignmentsService OktaAssignmentService
}

// Filter filters out Okta members that are part of an ongoing access request assignment.
// These members are added as a result of a just-in-time short-term access request to an Okta resource.
// Since the assignment to the Okta group was temporary and initiated by an access request,
// When Okta integration is enabled:
// - Access Requests can grant short-term access to resources via Access Lists.
// - Approving a request creates an `okta_assignment`, temporarily assigning the user to a group/app.
// - This assignment is processed by the Okta Assignment Processor.
//
// Issue:
// - Okta Access List Sync periodically fetches group memberships from Okta.
// - These fetched temporary assignments appear as external changes.
// - Teleport then incorrectly treats them as permanent and updates the Access List accordingly.
//
// To prevent this, we filter out these temporary assignments from the Okta members list.
// See: https://github.com/gravitational/teleport-private/issues/1944 For more details.
func (a *OngoingAccessRequestMembershipFilter) Filter(ctx context.Context, oktaMembers, teleportMembers map[string]*accesslist.AccessListMember) (map[string]*accesslist.AccessListMember, error) {
	ongoing, err := a.getACLOngoingAccessRequestAssignments(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := maps.Clone(oktaMembers)
	// If an assignment is created between the call to getACLOngoingAccessRequestAssignments
	// and the len(ongoing) check, the affected membership may not appear in oktaMembers
	// fetched from the Okta API.
	// This is intended because the check doesn't need to be atomic
	// to exclude Okta group assignments that have already been provisioned to Okta because oktaMembers
	// contains assignment fetch from Okta API before the ongoing assignment was listed.
	// so if the assigment was created between the two calls, it will be excluded from the oktaMembers.
	if len(ongoing) == 0 {
		return out, nil
	}
	for k := range ongoing {
		if _, ok := teleportMembers[k]; !ok {
			// This Okta member was added as a result of a just-in-time short-term access request to an Okta resource.
			// Since the assignment to the Okta group was temporary and initiated by an access request,
			// it should not be synced back as a persistent Access List member.
			// For that reason, we remove it from the Okta members
			// till the corresponding access request assignment is removed.
			delete(out, k)
		}
	}
	return out, nil
}

type OktaAssignmentService interface {
	// ListOktaAssignments lists Okta assignments.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
}

func (a *OngoingAccessRequestMembershipFilter) fetchAccessRequestOngoingAssignments(ctx context.Context) ([]types.OktaAssignment, error) {
	var out []types.OktaAssignment
	err := utils.ForEachResource(ctx, a.AssignmentsService.ListOktaAssignments, func(assigment types.OktaAssignment) error {
		if !isAccessRequestAssignment(assigment) {
			return nil
		}
		switch assigment.GetStatus() {
		case constants.OktaAssignmentStatusSuccessful, constants.OktaAssignmentStatusProcessing:
			out = append(out, assigment)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (a *OngoingAccessRequestMembershipFilter) getACLOngoingAccessRequestAssignments(ctx context.Context) (map[string]struct{}, error) {
	assignments, err := a.fetchAccessRequestOngoingAssignments(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ongoing := make(map[string]struct{})
	for _, v := range assignments {
		for _, target := range v.GetTargets() {
			ongoing[assigmentMapKey(target.GetID(), v.GetUser())] = struct{}{}
		}
	}
	return ongoing, nil
}

func isAccessRequestAssignment(assignment types.OktaAssignment) bool {
	const accessRequestSourceFormat = "access-request/"
	v, ok := assignment.GetLabel(eteleport.OktaAssignmentSourceLabel)
	return ok && strings.HasPrefix(v, accessRequestSourceFormat)
}

func assigmentMapKey(prefix, suffix string) string {
	return fmt.Sprintf("%s/%s", prefix, suffix)
}

// MemberKey returns unique key for the AccessListMember.
func MemberKey(m *accesslist.AccessListMember) string {
	return assigmentMapKey(m.Spec.AccessList, m.GetName())
}
