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
	"github.com/gravitational/teleport/api/utils/clientutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/utils/set"
)

type MembersMapType map[string]*accesslist.AccessListMember

type ongoingAccessRequestAssignments struct {
	accessRequestAssignments []types.OktaAssignment
}

func (c *ongoingAccessRequestAssignments) collect(in types.OktaAssignment) {
	if !isAccessRequestAssignment(in) {
		return
	}
	switch in.GetStatus() {
	case constants.OktaAssignmentStatusSuccessful, constants.OktaAssignmentStatusProcessing:
	default:
		return
	}
	c.accessRequestAssignments = append(c.accessRequestAssignments, in)
}

func (c *ongoingAccessRequestAssignments) applyFilter(inOkta, inTeleport MembersMapType) (outOkta, outTeleport MembersMapType) {
	outOkta = maps.Clone(inOkta)
	outTeleport = maps.Clone(inTeleport)
	// If an assignment is created between the call to collecting ongoing assignments
	// and the len(ongoing) check, the affected membership may not appear in oktaMembers
	// fetched from the Okta API.
	// This is intended because the check doesn't need to be atomic
	// to exclude Okta group assignments that have already been provisioned to Okta because oktaMembers
	// contains assignment fetch from Okta API before the ongoing assignment was listed.
	// so if the assignment was created between the two calls, it will be excluded from the oktaMembers.
	if len(c.accessRequestAssignments) == 0 {
		return outOkta, outTeleport
	}
	for k := range toSet(c.accessRequestAssignments) {
		if _, ok := outTeleport[k]; !ok {
			// This Okta member was added as a result of a just-in-time short-term access request to an Okta resource.
			// Since the assignment to the Okta group was temporary and initiated by an access request,
			// it should not be synced back as a persistent Access List member.
			// For that reason, we remove it from the Okta members
			// till the corresponding access request assignment is removed.
			delete(outOkta, k)
		}
	}
	return outOkta, outTeleport
}

type OngoingAssignmentsMembershipFilter struct {
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
func (a *OngoingAssignmentsMembershipFilter) Filter(ctx context.Context, inOktaMembers, inTeleportMembers map[string]*accesslist.AccessListMember) (outOktaMembers MembersMapType, outTeleportMembers MembersMapType, err error) {
	cleanupAssignmentsColl := &ongoingAccessRequestAssignments{}
	filter := []assignmentFilterStage{
		cleanupAssignmentsColl,
	}
	if err := a.collectAssignments(ctx, filter); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	outOktaMembers = inOktaMembers
	outTeleportMembers = inTeleportMembers
	for _, v := range filter {
		outOktaMembers, outTeleportMembers = v.applyFilter(outOktaMembers, outTeleportMembers)
	}
	return outOktaMembers, outTeleportMembers, nil
}

type OktaAssignmentService interface {
	// ListOktaAssignments lists Okta assignments.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
}

type assignmentFilterStage interface {
	collect(types.OktaAssignment)
	applyFilter(inOkta, inTeleport MembersMapType) (MembersMapType, MembersMapType)
}

func (a *OngoingAssignmentsMembershipFilter) collectAssignments(ctx context.Context, collectors []assignmentFilterStage) error {
	for assignment, err := range clientutils.Resources(ctx, a.AssignmentsService.ListOktaAssignments) {
		if err != nil {
			return trace.Wrap(err)
		}

		for _, collector := range collectors {
			collector.collect(assignment)
		}
	}

	return nil
}

func toSet(assignments []types.OktaAssignment) set.Set[string] {
	out := set.NewWithCapacity[string](len(assignments))
	for _, assignment := range assignments {
		for _, target := range assignment.GetTargets() {
			out.Add(assigmentMapKey(target.GetID(), assignment.GetUser()))
		}
	}
	return out
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
