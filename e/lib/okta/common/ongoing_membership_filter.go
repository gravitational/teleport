package common

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

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

// applyFilter removes members from the input maps in place based on ongoing access request assignments.
// The inOkta and inTeleport parameters are modified directly rather than creating filtered copies.
func (c *ongoingAccessRequestAssignments) applyFilter(inOkta, inTeleport map[string]*accesslist.AccessListMember) {
	// If an assignment is created between the call to collecting ongoing assignments
	// and the len(ongoing) check, the affected membership may not appear in oktaMembers
	// fetched from the Okta API.
	// This is intended because the check doesn't need to be atomic
	// to exclude Okta group assignments that have already been provisioned to Okta because oktaMembers
	// contains assignment fetch from Okta API before the ongoing assignment was listed.
	// so if the assignment was created between the two calls, it will be excluded from the oktaMembers.
	if len(c.accessRequestAssignments) == 0 {
		return
	}
	for k := range assignmentMapKeys(c.accessRequestAssignments) {
		if _, ok := inTeleport[k]; !ok {
			// This Okta member was added as a result of a just-in-time short-term access request to an Okta resource.
			// Since the assignment to the Okta group was temporary and initiated by an access request,
			// it should not be synced back as a persistent Access List member.
			// For that reason, we remove it from the Okta members
			// till the corresponding access request assignment is removed.
			delete(inOkta, k)
		}
	}
}

type OngoingAssignmentsMembershipFilter struct {
	// AssignmentsService is used to list Okta assignments.
	// It should interact directly with the backend (not a cache) to avoid
	// propagation delays that could result in missing filtered members.
	AssignmentsService OktaAssignmentService
}

// Filter excludes members from the access list sync whose Okta assignments are
// still being processed or are in a transitional state. It applies the following
// filter stages:
//
//  1. ongoingAccessRequestAssignments – removes members added via JIT access
//     requests so that temporary Okta group memberships are not imported as
//     permanent access list members.
//     See: https://github.com/gravitational/teleport-private/issues/1944
//  2. pendingAssignmentFilter – removes members whose assignments are pending
//     (not yet provisioned to Okta) to avoid the sync from prematurely deleting
//     a membership that the assignment processor is about to create.
//  3. cleanupAssignmentFilter – removes members whose assignments are scheduled
//     for cleanup (cleanup_time set, not yet finalized) to prevent stale Okta
//     group data from causing re-addition of a member that is being removed.
//     See: https://github.com/gravitational/teleport.e/issues/6558
//
// Note: This function modifies the input parameters in place. Both inOktaMembers and inTeleportMembers
// maps are filtered on the fly, removing entries that match filter.
func (a *OngoingAssignmentsMembershipFilter) Filter(ctx context.Context, inOktaMembers, inTeleportMembers map[string]*accesslist.AccessListMember) error {
	filter := []assignmentFilterStage{
		&ongoingAccessRequestAssignments{},
		&pendingAssignmentFilter{},
		&cleanupAssignmentFilter{},
	}
	if err := a.collectAssignments(ctx, filter); err != nil {
		return trace.Wrap(err)
	}
	for _, v := range filter {
		v.applyFilter(inOktaMembers, inTeleportMembers)
	}
	return nil
}

type OktaAssignmentService interface {
	// ListOktaAssignments lists Okta assignments.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
}

type assignmentFilterStage interface {
	collect(types.OktaAssignment)
	applyFilter(inOkta, inTeleport map[string]*accesslist.AccessListMember)
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

func assignmentMapKeys(assignments []types.OktaAssignment) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, assignment := range assignments {
			for _, target := range assignment.GetTargets() {
				if !yield(assigmentMapKey(target.GetID(), assignment.GetUser())) {
					return
				}
			}
		}
	}
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

// pendingAssignmentFilter is a filter stage that prevents the access list sync from processing
// members with pending Okta assignments.
//
// Background:
// When an access list membership is added or updated in Teleport, an OktaAssignment resource
// is created with a "pending" status. The assignment processor then picks up this assignment
// and provisions it to Okta (e.g., adds the user to an Okta group).
//
// Problem:
// A race condition can occur between the assignment processor and the access list sync:
// 1. Access list membership is added in Teleport
// 2. OktaAssignment is created with status "pending"
// 3. Access list sync runs before the assignment processor completes
// 4. Sync queries Okta and doesn't find the user in the group yet
// 5. Sync incorrectly treats this as a membership to remove from Teleport
// 6. This conflicts with the assignment processor trying to add the same membership
//
// Solution:
// This filter collects all pending assignments and removes their corresponding members
// from both the Okta and Teleport member maps. By excluding these members from the sync,
// we prevent the race condition and allow the assignment processor to complete its work
// without interference.
type pendingAssignmentFilter struct {
	assignments []types.OktaAssignment
}

// collect gathers Okta assignments that are in pending status.
// Pending assignments indicate that the assignment processor has not yet completed
// provisioning the membership to Okta. Only non-access-request assignments are collected,
// as access request assignments are handled by a separate filter stage.
func (c *pendingAssignmentFilter) collect(in types.OktaAssignment) {
	if in.GetStatus() == constants.OktaAssignmentStatusPending {
		c.assignments = append(c.assignments, in)
	}
}

// applyFilter removes members with pending assignments from both input maps.
// This prevents the access list sync from processing these members while their assignments
// are still being evaluated by the assignment processor, thereby eliminating the race condition
// between the sync and provisioning operations.
//
// The function modifies both inOkta and inTeleport in place, removing entries that correspond
// to pending assignments. This causes the sync operation to skip these members entirely until
// their assignment status changes from pending to successful or failed.
func (c *pendingAssignmentFilter) applyFilter(inOkta, inTeleport map[string]*accesslist.AccessListMember) {
	// Remove pending assignments from both maps to prevent the access list sync from processing them.
	// Pending assignments are those scheduled for evaluation.
	// By removing them from both inOkta and inTeleport, we ensure that the sync operation
	// skips these members entirely, That helps for reducing the race between assigment processor evaluation
	// and from Okta to Teleport sync. The excluded members change will be picked in next access list sync cycle.
	for k := range assignmentMapKeys(c.assignments) {
		delete(inOkta, k)
		delete(inTeleport, k)
	}
}

// cleanupAssignmentFilter is a filter prevents the access list sync from
// re-adding members whose Okta assignments are scheduled for cleanup -  removal from Okta upstream.
//
// When a user is removed from an access list, the user monitor sets cleanup_time on the
// corresponding OktaAssignment. The assignment processor then picks up this assignment and
// removes the user from the Okta group or app . Once successfully processed, the assignment is
// marked as finalized and eventually deleted.
//
// Problem (see https://github.com/gravitational/teleport.e/issues/6558):
// A race condition occurs between the assignment processor and the access list sync:
//  1. User is removed from an access list in Teleport
//  2. OktaAssignment cleanup_time is set (cleanup is pending)
//  3. The assignment processor may be delayed in executing the cleanup
//     (e.g., due to Okta rate limiting)
//  4. Access list sync fetches group memberships from Okta and sees the user
//     still in the group (stale data from before the cleanup was executed)
//  5. Sync incorrectly re-adds the user to the access list in Teleport
//
// This filter collects all non-finalized assignments with cleanup_time set and removes
// their corresponding members from both the Okta and Teleport member maps. By excluding
// these members from the sync, we prevent the stale Okta data from causing re-additions
// while the assignment processor completes the removal.
type cleanupAssignmentFilter struct {
	assignments []types.OktaAssignment
}

// collect gathers Okta assignments that are scheduled for cleanup but not yet finalized.
// These represent users being removed from Okta groups where the removal hasn't completed yet.
//
// Note: This also matches JIT access request assignments with a future cleanup time
// (i.e. ones that will expire later). That overlap is harmless because the
// ongoingAccessRequestAssignments filter already excludes those members from
// being imported as persistent access list members. Both filters deleting the
// same map key is a no-op.
func (c *cleanupAssignmentFilter) collect(in types.OktaAssignment) {
	if !in.GetCleanupTime().IsZero() && !in.IsFinalized() {
		c.assignments = append(c.assignments, in)
	}
}

// applyFilter removes members with in-progress cleanup assignments from both input maps.
// This prevents the access list sync from processing these members while the assignment
// processor is still removing them from Okta, avoiding the race where stale Okta membership
// data causes a removed user to be spuriously re-added.
func (c *cleanupAssignmentFilter) applyFilter(inOkta, inTeleport map[string]*accesslist.AccessListMember) {
	for k := range assignmentMapKeys(c.assignments) {
		delete(inOkta, k)
		delete(inTeleport, k)
	}
}
