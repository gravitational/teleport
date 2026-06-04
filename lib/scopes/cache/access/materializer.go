// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package access

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"iter"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/utils/set"
)

type repairEvent int

const (
	repairExpiredMembersEvent repairEvent = iota
	repairMissedMembersEvent

	// A century is close enough to forever for scheduling the repair
	// indefinitely in the future when it's not needed.
	century time.Duration = 100 * 365 * 24 * time.Hour
	// ExpiredMembersRepairBackoff is an amount of time the materializer backs
	// of after failing to repair expired member paths.
	ExpiredMembersRepairBackoff = 30 * time.Second
	// MissedMembersRepairBackoff is an amount of time the materializer backs
	// of after failing to repair missed member paths.
	MissedMembersRepairBackoff = 30 * time.Second
)

// AccessListReader provides the upstream source of access list and member resources.
type AccessListReader interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	GetAccessList(ctx context.Context, accessListName string) (*accesslist.AccessList, error)

	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
}

// AssignmentStore is the interface the materializer uses to persist
// materialized assignments.
type AssignmentStore interface {
	Put(assignment *scopedaccessv1.ScopedRoleAssignment) error
	Delete(name, subKind string)
}

// listAccessListMembers is a helper for calling clientutils.Resources to range
// over all members of [listName].
func listAccessListMembers(aclReader AccessListReader, listName string) func(context.Context, int, string) ([]*accesslist.AccessListMember, string, error) {
	return func(ctx context.Context, pageSize int, cursor string) ([]*accesslist.AccessListMember, string, error) {
		return aclReader.ListAccessListMembers(ctx, listName, pageSize, cursor)
	}
}

// Materializer is responsible for "materializing" scoped role assignments into
// the scoped access cache as they are derived from access lists and their
// members and owners. See RFD 243 for design decisions.
//
// The goal is for every (user, list) pair, where user is a valid explicit or
// inherited member or owner of list and where list has scoped role grants, to
// result in 1 materialized scoped role assignment. Each materialized
// assignment for (user, list) will grant exactly the scoped roles defined in
// the spec of that list for members, owners, or both depending on the user's
// relationship with the list.
//
// [Materializer.Init] must always be called before any other methods.
//
// The Materializer is not safe for concurrent use, it is meant to be driven
// from a single event loop, pushing events into [Materializer.ProcessEvent].
//
// If the Materializer is expected to be long-lived, callers should run
// [Materializer.RepairEventLoop] in a goroutine after [Materializer.Init]
// succeeds.
// Then, in the main event loop, events from [Materializer.RepairEvents]
// should be received and pushed into [Materializer.ProcessRepairEvent].
//
// Notably, the Materializer never reads the actual scoped roles it is
// generating assignments for. It does not attempt to validate that scoped role
// exist or that the assigned scope is allowed by the scoped role definition.
// This validation is the responsibility of the backend service. Anything that
// reads scoped role assignments must also validate them before using them for
// access decisions.
type Materializer struct {
	// aclReader is used for upstream reads of access lists and members, it is
	// expected to be an in-memory cache, and all events are expected to be
	// pushed in to ProcessEvent after the state has been persisted to the
	// cache.
	aclReader AccessListReader

	// assignmentStore is where materialized assignments are persisted.
	assignmentStore AssignmentStore

	// ancestorCache holds all possible direct membership and ownership edges,
	// even if they may be expired or invalid based on membership or ownership
	// requirements.
	ancestorCache *ancestorCache

	// materializedAssignments is just internal bookkeeping holding the current
	// set of assignments that have been "materialized" into the cache state.
	materializedAssignments map[MaterializedAssignmentKey]ancestorRelation

	logger *slog.Logger
	tracer oteltrace.Tracer

	repairTimeMu                 sync.Mutex
	repairEventC                 chan repairEvent
	wakeRepairLoop               chan struct{}
	nextExpiredMembersRepairTime time.Time
	nextRepairMissedMembersTime  time.Time
}

// MaterializedAssignmentKey uniquely identifies a materialized scoped role assignment.
type MaterializedAssignmentKey struct {
	// User is the name of the user the assignment is materialized for.
	User string
	// List is the name of the access list the assignment is materialized for.
	List string
}

// AssignmentName returns the canonical name for the materialized scoped role assignment.
func (k MaterializedAssignmentKey) AssignmentName() string {
	h := sha256.New224()
	binary.Write(h, binary.LittleEndian, uint16(len(k.User)))
	h.Write([]byte(k.User))
	h.Write([]byte(k.List))
	return "acl-" + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// NewMaterializer returns a new scoped role assignment materializer.
func NewMaterializer(assignmentStore AssignmentStore, aclReader AccessListReader, tracer oteltrace.Tracer) *Materializer {
	now := time.Now()
	return &Materializer{
		assignmentStore:              assignmentStore,
		aclReader:                    aclReader,
		ancestorCache:                newAncestorCache(),
		materializedAssignments:      make(map[MaterializedAssignmentKey]ancestorRelation),
		logger:                       slog.With(teleport.ComponentKey, "sra_materializer"),
		tracer:                       tracer,
		repairEventC:                 make(chan repairEvent),
		wakeRepairLoop:               make(chan struct{}, 1),
		nextExpiredMembersRepairTime: now.Add(century),
		nextRepairMissedMembersTime:  now.Add(century),
	}
}

// Init materializes all necessary scoped role assignments into the assignment
// store based on the current set of access list memberships.
func (m *Materializer) Init(ctx context.Context) (err error) {
	ctx, span := m.tracer.Start(ctx, "scoped_access_cache/materializer/init")
	defer func() { tracing.EndSpan(span, err) }()
	m.syncMaterializedAssignmentsMetric()

	// First populate the ancestor cache with all list->list memberships and
	// ownerships, it's critical for this to be up to date to process
	// relationships and future changes. The ancestor cache should include even
	// expired or invalid membership edges for defensive handling of delete
	// events.
	//
	// Also track the earliest member expiration that's in the future, so we
	// can react to member expiration.
	now := time.Now()
	nextExpiry := now.Add(century)
	for member, err := range clientutils.Resources(ctx, m.aclReader.ListAllAccessListMembers) {
		if err != nil {
			return trace.Wrap(err, "reading access list members")
		}
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			m.ancestorCache.addMembership(member.Spec.AccessList, member.GetName())
		}
		if member.Spec.Expires.After(now) && member.Spec.Expires.Before(nextExpiry) {
			nextExpiry = member.Spec.Expires
		}
	}
	m.reportFutureMemberExpiry(ctx, nextExpiry)

	for list, err := range clientutils.Resources(ctx, m.aclReader.ListAccessLists) {
		if err != nil {
			return trace.Wrap(err, "reading access lists")
		}
		for _, owner := range list.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				m.ancestorCache.addOwnership(owner.Name, list.GetName())
			}
		}
	}

	// We iterate the access lists again separately so that the ancestor cache
	// is fully initialized before it may be referenced in
	// m.initAccessListMembers.
	for list, err := range clientutils.Resources(ctx, m.aclReader.ListAccessLists) {
		if err != nil {
			return trace.Wrap(err, "reading access lists")
		}
		// Materialize assignments as necessary for all members of every access list.
		m.initAccessListMembers(ctx, list)
		// Materialize assignments as necessary for all owners of every access list.
		m.initAccessListOwners(ctx, list)
	}

	return nil
}

// ProcessEvent is the entry point for all event-driven changes to materializer
// state, driven by access list and access list member events.
func (m *Materializer) ProcessEvent(ctx context.Context, event types.Event) (err error) {
	ctx, span := m.tracer.Start(ctx, "scoped_access_cache/materializer/process_event",
		oteltrace.WithAttributes(materializerEventAttributes(event)...),
	)
	defer func() { tracing.EndSpan(span, err) }()

	switch event.Type {
	case types.OpPut:
		switch item := event.Resource.(type) {
		case *accesslist.AccessList:
			m.handleAccessListPut(ctx, item)
		case *accesslist.AccessListMember:
			m.handleAccessListMemberPut(ctx, item)
		}
	case types.OpDelete:
		switch event.Resource.GetKind() {
		case types.KindAccessList:
			m.handleAccessListDelete(ctx, event.Resource.GetName())
		case types.KindAccessListMember:
			listName := event.Resource.GetMetadata().Description
			if listName == "" {
				// This is a bug, return a hard failure.
				return trace.Errorf("missing access list name in access list member delete event description")
			}
			m.handleAccessListMemberDelete(ctx, listName, event.Resource.GetName())
		}
	}
	return nil
}

func (m *Materializer) handleAccessListMemberPut(ctx context.Context, member *accesslist.AccessListMember) {
	if member.IsUser() {
		m.handleUserMemberPut(ctx, member)
	}
	if member.Spec.MembershipKind == accesslist.MembershipKindList {
		m.handleListMemberPut(ctx, member)
	}
}

// handleUserMemberPut materializes an assignment for user in list and all
// ancestors of list.
//
// If the member resource is expired, it re-checks all materialized assignments
// for the user in case they were granted via this membership.
func (m *Materializer) handleUserMemberPut(ctx context.Context, member *accesslist.AccessListMember) {
	listName := member.Spec.AccessList
	userName := member.GetName()

	if m.ancestorCache.children.Get(listName).Contains(userName) {
		// Due to presence in the ancestor cache, this access list member must
		// have been observed with membership kind list before, and now has
		// membership kind user. Process a list member delete first, and then
		// carry on to handle the user member put.
		m.handleListMemberDelete(ctx, listName, userName)
	}

	if member.IsExpired(time.Now()) {
		m.handleUserMemberDeleteOrExpired(ctx, listName, userName)
		return
	}
	m.reportFutureMemberExpiry(ctx, member.Spec.Expires)

	list, err := m.aclReader.GetAccessList(ctx, listName)
	if err != nil {
		m.logger.InfoContext(ctx, "Failed to get access list while handling member put",
			"error", err,
			"list", listName,
			"user", userName)
		m.scheduleMissedMembersRepair(ctx)
		return
	}

	if hasMembershipRequires(list) {
		// Memberships paths through access lists that have any member
		// requirements are not considered for scoped role assignment
		// materialization. Including member requirements in any access list
		// that may transitively grant a scoped role is considered invalid.
		return
	}

	if err := m.addMemberRelationshipAndMaterialize(ctx, list, userName); err != nil {
		m.logger.WarnContext(ctx, "Failed to materialize assignment",
			"error", err,
			"user", userName,
			"list", list.GetName())
	}

	ancestors, validationErrors := m.collectAncestors(ctx, list.GetName())
	for _, validationError := range validationErrors {
		m.logger.InfoContext(ctx, "Error while validating access list ancestors, some scoped role assignments may not be materialized",
			"error", validationError)
	}
	if len(validationErrors) > 0 {
		m.scheduleMissedMembersRepair(ctx)
	}

	// As a member of this list, the user shares the list's relationship with
	// all of its ancestors, make sure assignments are materialized.
	for _, ancestor := range ancestors {
		if err := m.updateRelationshipAndMaterialize(ctx, ancestor.list, ancestor.relation, userName); err != nil {
			m.logger.WarnContext(ctx, "Failed to materialize assignment",
				"error", err,
				"user", userName,
				"list", ancestor.list.GetName())
		}
	}
}

// handleListMemberPut adds the direct membership to the ancestor cache,
// and then makes sure that all nested members of the member list have
// materialized assignments for the parent list and all ancestors of the
// parent list.
//
// If the member resource is expired, it re-checks all materialized assignments
// for the parent list and all of its ancestors, in case they were granted via this membership.
func (m *Materializer) handleListMemberPut(ctx context.Context, member *accesslist.AccessListMember) {
	parentListName, memberListName := member.Spec.AccessList, member.Spec.Name

	// It's possible this member resource used to have membership kind user and
	// was updated to have membership kind list, so we must clear anything
	// related to a previous user membership.
	m.handleUserMemberDeleteOrExpired(ctx, parentListName, memberListName)

	m.ancestorCache.addMembership(parentListName, memberListName)

	if member.IsExpired(time.Now()) {
		m.handleListMemberExpired(ctx, parentListName)
		return
	}
	m.reportFutureMemberExpiry(ctx, member.Spec.Expires)

	// Every user that is a nested member of memberList may have just become a
	// member of parentList and all lists it is a nested member of. They also
	// may have become an owner of every list parentList is an owner of.
	parentList, err := m.aclReader.GetAccessList(ctx, parentListName)
	if err != nil {
		m.logger.InfoContext(ctx, "Failed to get access list while handling list member put, some scoped role assignments may not be materialized",
			"error", err,
			"list", parentListName,
			"member_list", memberListName)
		m.scheduleMissedMembersRepair(ctx)
		return
	}
	memberList, err := m.aclReader.GetAccessList(ctx, memberListName)
	if err != nil {
		m.logger.InfoContext(ctx, "Failed to get access list while handling list member put, some scoped role assignments may not be materialized",
			"error", err,
			"list", memberListName,
			"parent_list", parentListName)
		m.scheduleMissedMembersRepair(ctx)
		return
	}

	if hasMembershipRequires(parentList) {
		// Membership paths through access lists that have any member
		// requirements are not considered for scoped role assignment
		// materialization. Including member requirements in any role that may
		// transitively grant a scoped role is considered invalid.
		return
	}

	// Collect all ancestors of the parent list, in case any (nested) members
	// of the new member list just became members or owners of the ancestor.
	ancestors, validationErrors := m.collectAncestors(ctx, parentListName)
	for _, validationError := range validationErrors {
		m.logger.InfoContext(ctx, "Error while validating access list ancestors, some scoped role assignments may not be materialized",
			"error", validationError)
	}
	if len(validationErrors) > 0 {
		m.scheduleMissedMembersRepair(ctx)
	}

	for member, err := range m.walkUserMembers(ctx, memberList) {
		if err != nil {
			m.logger.WarnContext(ctx, "Error while walking members of access list, some scoped role assignments may not be materialized",
				"error", err,
				"list", memberListName)
			m.scheduleMissedMembersRepair(ctx)
			// walkUserMembers may yield errors from walking members of any
			// member lists, but it may not be done, so continue the loop.
			continue
		}

		// User is now a member of the parent list, materialize an assignment.
		if err := m.addMemberRelationshipAndMaterialize(ctx, parentList, member.GetName()); err != nil {
			m.logger.WarnContext(ctx, "Failed to materialize assignment",
				"error", err,
				"user", member.GetName(),
				"list", parentListName)
		}

		// As a member of the parent list, the user now shares the parent
		// list's relationship with all of its ancestors, make sure assignments
		// are materialized.
		for _, ancestor := range ancestors {
			if err := m.updateRelationshipAndMaterialize(ctx, ancestor.list, ancestor.relation, member.GetName()); err != nil {
				m.logger.WarnContext(ctx, "Failed to materialize assignment",
					"error", err,
					"user", member.GetName(),
					"list", ancestor.list.GetName())
			}
		}
	}
}

func (m *Materializer) handleAccessListMemberDelete(ctx context.Context, listName, memberName string) {
	// We don't get enough info from the event to know if the deleted member
	// was a user or a list. Luckily member names are unique, so we know that
	// if this membership is present in the ancestor cache as a list, then this
	// member was last observed as a list. If it is not present in the ancestor
	// cache, then it was last observed as a user (or not observed at all) and
	// we handle it as a user member delete.
	if m.ancestorCache.children.Get(listName).Contains(memberName) {
		m.handleListMemberDelete(ctx, listName, memberName)
		return
	}
	m.handleUserMemberDeleteOrExpired(ctx, listName, memberName)
}

// handleListMemberDelete handles delete events for nested access list memberships.
func (m *Materializer) handleListMemberDelete(ctx context.Context, parentListName, memberListName string) {
	// First and foremost, always keep the ancestor cache up to date with all
	// direct list->list memberships.
	m.ancestorCache.removeMembership(parentListName, memberListName)

	// The membership being deleted is equivalent to it being expired. Expired
	// memberships remain in the ancestor cache.
	m.handleListMemberExpired(ctx, parentListName)
}

// handleListMemberExpired handles the event where an access list membership
// has been deleted or has expired. Any nested members of the member list may
// no longer be valid members or owners of the parent list or any of its
// ancestors. At risk of being overly pessimistic, we re-check every
// materialized assignment for the parent list and all of its ancestors.
func (m *Materializer) handleListMemberExpired(ctx context.Context, parentListName string) {
	// We must iterate all ancestors without relying on paging through
	// collections in the cache to make sure we don't miss any assignments that
	// need to be invalidated due to a paging error or any other transient
	// error. The ancestor cache maintains all known direct list->list
	// memberships and ownerships, which is sufficient.
	ancestors := m.collectAncestorListsWithoutValidation(parentListName)

	// Iterate all currently materialized assignments.
	for key := range m.materializedAssignments {
		_, isAncestor := ancestors[key.List]
		if key.List != parentListName && !isAncestor {
			// We don't need to validate any assignments that are not for the
			// parent list or any of its ancestors.
			continue
		}

		if err := m.recheckAssignment(ctx, key); err != nil {
			// Must pessimistically assume any assignment is invalid if we
			// encountered an error trying to validate it.
			m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
				"error", err)
			m.deleteMaterializedAssignment(ctx, key)
			m.scheduleMissedMembersRepair(ctx)
		}
	}
}

// User is no longer a direct member of list but could still be a nested member.
// It's possible they are no longer a valid member of the parent list or any of
// its ancestors. We need to re-check all current materialized assignments for
// this user in the initial list or any of its ancestors.
func (m *Materializer) handleUserMemberDeleteOrExpired(ctx context.Context, parentListName, userName string) {
	// We must iterate all ancestors without relying on paging through
	// collections in the cache to make sure we don't miss any assignments that
	// need to be invalidated due to a paging error or any other transient
	// error. The ancestor cache maintains all known direct list->list
	// memberships and ownerships, which is sufficient.
	ancestors := m.collectAncestorListsWithoutValidation(parentListName)
	for key := range m.materializedAssignmentsForUser(userName) {
		_, isAncestor := ancestors[key.List]
		if key.List != parentListName && !isAncestor {
			// We don't need to validate any assignments that are not for the
			// parent list or any of its ancestors.
			continue
		}
		if err := m.recheckAssignment(ctx, key); err != nil {
			// Must pessimistically assume any assignment is invalid if we
			// encountered an error trying to validate it.
			m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
				"error", err)
			m.deleteMaterializedAssignment(ctx, key)
			m.scheduleMissedMembersRepair(ctx)
		}
	}
}

// handleAccessListPut handles put events for access lists.
//
// It must first update any owner list edges in the ancestor cache, as the
// source of truth for owners is the access list resource.
//
// In case the scoped role grants or owners were modified, it must re-check all
// materialized assignments for the list.
// As a slight optimization, it can just delete any extant assignments of the
// new version of the role does not grant any scoped roles.
//
// If the list has any direct user owners they may be new, make sure there's a
// materialized assignment for them.
//
// If the list has any scoped owner grants and any owner lists were added,
// materialize an assignment for them.
func (m *Materializer) handleAccessListPut(ctx context.Context, list *accesslist.AccessList) {
	// Update any owner list edges in the ancestor cache.
	m.ancestorCache.clearOwnersOf(list.GetName())
	for _, owner := range list.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		m.ancestorCache.addOwnership(owner.Name, list.GetName())
	}

	// Re-check all extant assignments for this list in case any grants were added
	// or removed or ownership status changed.
	hasScopedGrants := hasMemberGrants(list) || hasOwnerGrants(list)
	for key := range m.materializedAssignmentsForList(list.GetName()) {
		if hasScopedGrants {
			// The list has some grants we should re-check. This may delete
			// assignments as necessary.
			if err := m.recheckAssignment(ctx, key); err != nil {
				// Must pessimistically assume any assignment is invalid if we
				// encountered an error trying to validate it.
				m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
					"error", err)
				m.deleteMaterializedAssignment(ctx, key)
				m.scheduleMissedMembersRepair(ctx)
			}
		} else {
			// If the list doesn't grant any scoped roles, we can just delete
			// all extant assignments for it without actually checking
			// anything.
			m.deleteMaterializedAssignment(ctx, key)
		}
	}

	// If the list now has membership requirements, it's possible that this
	// update has broken membership paths for any ancestor lists. Otherwise it
	// is not possible for an access list update to invalidate membership in
	// any other lists.
	if hasMembershipRequires(list) {
		ancestors := m.collectAncestorListsWithoutValidation(list.GetName())
		for ancestorListName := range ancestors {
			for key := range m.materializedAssignmentsForList(ancestorListName) {
				if err := m.recheckAssignment(ctx, key); err != nil {
					// Must pessimistically assume any assignment is invalid if we
					// encountered an error trying to validate it.
					m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
						"error", err)
					m.deleteMaterializedAssignment(ctx, key)
					m.scheduleMissedMembersRepair(ctx)
				}
			}
		}
	}

	// Now that any invalidated assignments have been deleted, we can call
	// initAccessListMembers and initAccessListOwners to materialize any
	// necessary new assignments.
	m.initAccessListMembers(ctx, list)
	m.initAccessListOwners(ctx, list)
}

// initAccessListMembers additively materializes scoped role assignments for
// all members of the given list.
func (m *Materializer) initAccessListMembers(ctx context.Context, list *accesslist.AccessList) {
	if hasMembershipRequires(list) {
		// membership requires are not supported, do not follow any membership
		// edges if this list has membership requires.
		return
	}

	ancestors, validationErrors := m.collectAncestors(ctx, list.GetName())
	for _, validationError := range validationErrors {
		m.logger.InfoContext(ctx, "Error while validating access list ancestors, some scoped role assignments may not be materialized",
			"error", validationError)
	}
	if len(validationErrors) > 0 {
		m.scheduleMissedMembersRepair(ctx)
	}

	hasMemberGrants := hasMemberGrants(list)
	if !hasMemberGrants && len(ancestors) == 0 {
		// There will be no scoped role grants to materialize assignments for
		// in this list or any of its ancestors, return early.
		return
	}

	// Walk all user members of this list and materialize necessary scoped role
	// assignments for their membership in list and/or membership and/or
	// ownership in any ancestor list.
	for member, err := range m.walkUserMembers(ctx, list) {
		if err != nil {
			m.logger.InfoContext(ctx, "Error while walking members of access list, some scoped role assignments may not be materialized",
				"error", err,
				"list", list.GetName())
			m.scheduleMissedMembersRepair(ctx)
			// walkUserMembers may yield errors from walking members of any
			// member lists, but it may not be done, so continue the loop.
			continue
		}

		if hasMemberGrants {
			// This user is a confirmed member, make sure a member assignment is materialized.
			if err := m.addMemberRelationshipAndMaterialize(ctx, list, member.GetName()); err != nil {
				m.logger.WarnContext(ctx, "Failed to materialize assignment",
					"error", err,
					"user", member.GetName(),
					"list", list.GetName())
			}
		}

		// As a member of this list, the user shares the list's relationship
		// with all of its ancestors, make sure assignments are materialized.
		for _, ancestor := range ancestors {
			if err := m.updateRelationshipAndMaterialize(ctx, ancestor.list, ancestor.relation, member.GetName()); err != nil {
				m.logger.WarnContext(ctx, "Failed to materialize assignment",
					"error", err,
					"user", member.GetName(),
					"list", ancestor.list.GetName())
			}

		}
	}
}

// initAccessListOwners additively materializes scoped role assignments for
// all owners of the given list. That includes direct owers and (nested)
// members of owner lists.
func (m *Materializer) initAccessListOwners(ctx context.Context, list *accesslist.AccessList) {
	if hasOwnershipRequires(list) || !hasOwnerGrants(list) {
		// There is nothing to materialize for owners of this list.
		return
	}

	// Keep track of users and lists that have already been seen across member
	// iterations to avoid duplicating work if the same user or list is a
	// member of multiple owner lists.
	seenLists := set.New[string]()
	seenUsers := set.New[string]()

	for _, owner := range list.Spec.Owners {
		if owner.IsMembershipKindUser() {
			// This user is a confirmed direct owner, make sure an owner
			// assignment is materialized.
			seenUsers.Add(owner.Name)
			if err := m.addOwnerRelationshipAndMaterialize(ctx, list, owner.Name); err != nil {
				m.logger.WarnContext(ctx, "Failed to materialize assignment",
					"error", err,
					"user", owner.Name,
					"list", list.GetName())
			}
			continue
		}

		if owner.MembershipKind != accesslist.MembershipKindList {
			continue
		}

		ownerList, err := m.aclReader.GetAccessList(ctx, owner.Name)
		if err != nil {
			m.logger.InfoContext(ctx, "Failed to get owner list, some scoped role assignments may not be materialized for owners",
				"error", err)
			m.scheduleMissedMembersRepair(ctx)
			continue

		}

		// All nested members of owners lists are owners of this list
		// and a scoped role assignment should be materialized.
		for member, err := range m.walkUserMembersRecursive(ctx, ownerList, seenLists, seenUsers) {
			if err != nil {
				m.logger.WarnContext(ctx, "Error while walking members of access list, some scoped role assignments may not be materialized",
					"error", err,
					"list", list.GetName())
				m.scheduleMissedMembersRepair(ctx)
				// walkUserMembersRecursive may yield errors from walking members of any
				// member lists, but it may not be done, so continue the loop.
				continue
			}
			if err := m.addOwnerRelationshipAndMaterialize(ctx, list, member.GetName()); err != nil {
				m.logger.WarnContext(ctx, "Failed to materialize assignment",
					"error", err,
					"user", member.GetName(),
					"list", list.GetName())
			}
		}
	}
}

// handleAccessListDelete handles delete events for access lists.
// Access lists are unlikely to be deleted if they are a member or owner of
// another list (the access list service attempts to prevent it), but to be
// defensive we re-check assignments for all ancestors anyway in case a
// membership path through this list has been broken.
func (m *Materializer) handleAccessListDelete(ctx context.Context, listName string) {
	// First of all, keep the ancestor cache up to date with respect to owners.
	// Access lists are the source of truth for direct owner relationships.
	// While the deletion of the access list may invalidate a direct membership
	// involving this list, the source of truth is the member resource, and the
	// ancestor cache for members will be reconciled in response to member
	// events.
	for owner := range m.ancestorCache.ownersOf.Get(listName).Items() {
		m.ancestorCache.removeOwnership(owner, listName)
	}

	// Assignments for any ancestor list may be affected if a membership path
	// through this deleted list is now broken. Collect all ancestor lists
	// without validation to avoid missing any due to read/paging errors.
	ancestors := m.collectAncestorListsWithoutValidation(listName)

	// Iterate all currently materialized assignments.
	for key := range m.materializedAssignments {
		_, isAncestor := ancestors[key.List]
		if key.List != listName && !isAncestor {
			// We don't need to validate any assignments that are not for the
			// deleted list or any of its ancestors.
			continue
		}

		if err := m.recheckAssignment(ctx, key); err != nil {
			// Must pessimistically assume any assignment is invalid if we
			// encountered an error trying to validate it.
			m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
				"error", err)
			m.deleteMaterializedAssignment(ctx, key)
			m.scheduleMissedMembersRepair(ctx)
		}
	}
}

// addMemberRelationshipAndMaterialize adds the given user as a member of list,
// additively merges with any existing materialized assignment for the user in
// case the user is already a known owner of the list, and materializes an
// assignment.
func (m *Materializer) addMemberRelationshipAndMaterialize(ctx context.Context, list *accesslist.AccessList, userName string) error {
	relation := ancestorRelation{isMember: true}
	return m.updateRelationshipAndMaterialize(ctx, list, relation, userName)
}

// addOwnerRelationshipAndMaterialize adds the given user as an owner of list,
// additively merges with any existing materialized assignment for the user in
// case the user is already a known member of the list, and materializes an
// assignment.
func (m *Materializer) addOwnerRelationshipAndMaterialize(ctx context.Context, list *accesslist.AccessList, userName string) error {
	relation := ancestorRelation{isOwner: true}
	return m.updateRelationshipAndMaterialize(ctx, list, relation, userName)
}

// updateRelationshipAndMaterialize additively updates our knowledge about a
// given user's relationship (owner and/or member) to a given list, without
// clearing previously set relationships. Ensures the corresponding materialized
// assignment is created or updated as-needed. This function is called both for
// direct relationships, and to record the transitive relationships implied by
// nested lists.
func (m *Materializer) updateRelationshipAndMaterialize(
	ctx context.Context,
	list *accesslist.AccessList,
	relation ancestorRelation,
	userName string,
) error {
	currentRelation := m.materializedAssignments[MaterializedAssignmentKey{
		List: list.GetName(),
		User: userName,
	}]
	relation.isMember = relation.isMember || currentRelation.isMember
	relation.isOwner = relation.isOwner || currentRelation.isOwner
	return m.materializeAssignment(ctx, list, relation, userName)
}

// materializeAssignment materializes a scoped role assignment for the given
// (user, list) pair with the given relation between the user and list.
//
// The assignment will be injected into the assignment store, unless it contains
// no grants, in which case any extant assignment in the store will be deleted.
func (m *Materializer) materializeAssignment(
	ctx context.Context,
	list *accesslist.AccessList,
	relation ancestorRelation,
	userName string,
) error {
	key := MaterializedAssignmentKey{
		User: userName,
		List: list.GetName(),
	}

	if (!relation.isMember || !hasMemberGrants(list)) && (!relation.isOwner || !hasOwnerGrants(list)) {
		// This access list does not grant any scoped roles to this user,
		// delete any existing materialized assignment if present.
		m.deleteMaterializedAssignment(ctx, key)
		return nil
	}

	assignment := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		SubKind: scopedaccess.SubKindMaterialized,
		Version: types.V1,
		Scope:   "/",
		Metadata: &headerv1.Metadata{
			Name: key.AssignmentName(),
		},
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: userName,
		},
		Status: &scopedaccessv1.ScopedRoleAssignmentStatus{
			Origin: &scopedaccessv1.ScopedRoleAssignmentStatus_Origin{
				CreatorKind: scopedaccess.CreatorKindAccessList,
				CreatorName: list.GetName(),
			},
		},
	}

	if relation.isMember {
		for _, grant := range list.Spec.Grants.ScopedRoles {
			assignment.Spec.Assignments = append(assignment.Spec.Assignments, &scopedaccessv1.Assignment{
				Role:  grant.Role,
				Scope: grant.Scope,
			})
		}
	}
	if relation.isOwner {
		for _, grant := range list.Spec.OwnerGrants.ScopedRoles {
			assignment.Spec.Assignments = append(assignment.Spec.Assignments, &scopedaccessv1.Assignment{
				Role:  grant.Role,
				Scope: grant.Scope,
			})
		}
	}

	m.logger.DebugContext(ctx, "Materializing scoped role assignment",
		"user", key.User,
		"list", key.List)
	if err := m.assignmentStore.Put(assignment); err != nil {
		return trace.Wrap(err, "putting materialized assignment into cache")
	}
	m.materializedAssignments[key] = relation
	m.syncMaterializedAssignmentsMetric()
	return nil
}

func (m *Materializer) deleteMaterializedAssignment(ctx context.Context, key MaterializedAssignmentKey) {
	if _, ok := m.materializedAssignments[key]; ok {
		m.logger.DebugContext(ctx, "Deleting materialized scoped role assignment",
			"user", key.User,
			"list", key.List)
		m.assignmentStore.Delete(key.AssignmentName(), scopedaccess.SubKindMaterialized)
		delete(m.materializedAssignments, key)
		m.syncMaterializedAssignmentsMetric()

	}
}

func (m *Materializer) syncMaterializedAssignmentsMetric() {
	materializedAssignmentsMetric.Set(float64(len(m.materializedAssignments)))
}

func (m *Materializer) recheckAssignment(ctx context.Context, key MaterializedAssignmentKey) error {
	list, err := m.aclReader.GetAccessList(ctx, key.List)
	if err != nil {
		return trace.Wrap(err, "getting access list %v", key.List)
	}

	relation := ancestorRelation{
		isOwner:  m.checkNestedOwnership(ctx, list, key.User),
		isMember: m.checkNestedMembership(ctx, list, key.User),
	}

	if err := m.materializeAssignment(ctx, list, relation, key.User); err != nil {
		return trace.Wrap(err, "materializing assignment")
	}
	return nil
}

func (m *Materializer) checkNestedOwnership(ctx context.Context, list *accesslist.AccessList, userName string) bool {
	if hasOwnershipRequires(list) {
		// Ownerships can not be valid, for the puposes of granting scoped
		// roles, if the list has any ownership requires.
		return false
	}

	// First check if the user is directly an owner.
	for _, owner := range list.Spec.Owners {
		if !owner.IsMembershipKindUser() {
			continue
		}
		if owner.Name == userName {
			return true
		}
	}

	// Then check if user is a member or nested member of any owner lists.
	for _, owner := range list.Spec.Owners {
		if owner.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		ownerList, err := m.aclReader.GetAccessList(ctx, owner.Name)
		if err != nil {
			// Must assume any membership is invalid if we can't fetch the access list.
			continue
		}
		if m.checkNestedMembership(ctx, ownerList, userName) {
			return true
		}
	}

	return false
}

func (m *Materializer) checkNestedMembership(ctx context.Context, list *accesslist.AccessList, userName string) bool {
	seen := set.New[string]()

	var checkNestedMembershipRecursive func(*accesslist.AccessList) bool
	checkNestedMembershipRecursive = func(list *accesslist.AccessList) bool {
		if seen.Contains(list.GetName()) {
			return false
		}
		seen.Add(list.GetName())

		if hasMembershipRequires(list) {
			// Memberships can not be valid, for the purposes of granting scoped
			// roles, if the list has any ownership requires.
			return false
		}

		// Check if the user is a non-expired direct member of this list.
		member, err := m.aclReader.GetAccessListMember(ctx, list.GetName(), userName)
		if err == nil {
			if member.IsUser() && !member.IsExpired(time.Now()) {
				// This user is a valid member.
				return true
			}
		}

		// Recursively check if the user is a valid member of any child lists.
		for childListName := range m.ancestorCache.children.Get(list.GetName()).Items() {
			listMember, err := m.aclReader.GetAccessListMember(ctx, list.GetName(), childListName)
			if err != nil || listMember.IsExpired(time.Now()) {
				// Couldn't fetch child list member or its membership is
				// expired, don't walk this membership path.
				continue
			}
			childList, err := m.aclReader.GetAccessList(ctx, childListName)
			if err != nil {
				// Must assume any membership is invalid if we can't fetch the access list.
				continue
			}
			if checkNestedMembershipRecursive(childList) {
				return true
			}
		}

		// User was not a member of this list or any of its children.
		return false
	}

	return checkNestedMembershipRecursive(list)
}

func (m *Materializer) materializedAssignmentsForList(listName string) iter.Seq2[MaterializedAssignmentKey, ancestorRelation] {
	return func(yield func(MaterializedAssignmentKey, ancestorRelation) bool) {
		for key, relation := range m.materializedAssignments {
			if key.List != listName {
				continue
			}
			if !yield(key, relation) {
				return
			}
		}
	}
}

func (m *Materializer) materializedAssignmentsForUser(userName string) iter.Seq2[MaterializedAssignmentKey, ancestorRelation] {
	return func(yield func(MaterializedAssignmentKey, ancestorRelation) bool) {
		for key, relation := range m.materializedAssignments {
			if key.User != userName {
				continue
			}
			if !yield(key, relation) {
				return
			}
		}
	}
}

// walkUserMembers returns an iterator that yields all nested user members of
// [list], i.e. all its direct user members and all members of all its member
// lists, recursively. Each user member will be yielded at most once.
//
// Each list is checked that it does not contain membership requires and that
// the member resources exists and is not expired -> invalid edges are not
// followed.
//
// It will yield any errors encountered while fetching list or member resources
// but may continue iterating over other lists/members.
func (m *Materializer) walkUserMembers(ctx context.Context, list *accesslist.AccessList) iter.Seq2[*accesslist.AccessListMember, error] {
	seenLists := set.New[string]()
	seenUsers := set.New[string]()
	return m.walkUserMembersRecursive(ctx, list, seenLists, seenUsers)
}

// walkUserMembers returns an iterator that yields all nested user members of
// [list], i.e. all its direct user members and all members of all its member
// lists, recursively.
//
// It will not walk members of any lists already present in [seenLists], and
// each list will be added as it is walked.
// It will not yield any users already present in [seenUsers], and each user
// will be added as it is yielded, so that it is yielded at most once.
//
// This can be called multiple times with the same seenLists/seenUsers to walk
// multiple list subtrees without repeating lists/users multiple times.
//
// Each list is checked that it does not contain membership requires and that
// the member resources exists and is not expired -> invalid edges are not
// followed.
//
// It will yield any errors encountered while fetching list or member resources
// but may continue iterating over other lists/members.
func (m *Materializer) walkUserMembersRecursive(ctx context.Context, list *accesslist.AccessList, seenLists set.Set[string], seenUsers set.Set[string]) iter.Seq2[*accesslist.AccessListMember, error] {
	return func(yield func(*accesslist.AccessListMember, error) bool) {
		seenLists.Add(list.GetName())
		if hasMembershipRequires(list) {
			// membership requires are not supported in lists that transitively
			// grant scoped roles, do not walk members of this list.
			return
		}

		for member, err := range clientutils.Resources(ctx, listAccessListMembers(m.aclReader, list.GetName())) {
			if err != nil {
				if !yield(nil, trace.Wrap(err)) {
					return
				}
				continue
			}

			if member.IsExpired(time.Now()) {
				// Do not follow expired memberships.
				continue
			}

			if member.IsUser() {
				// Found a legitimate user member, yield it if it has not
				// already been yielded.
				if seenUsers.Contains(member.GetName()) {
					continue
				}
				seenUsers.Add(member.GetName())
				if !yield(member, nil) {
					return
				}
			}

			if member.Spec.MembershipKind != accesslist.MembershipKindList {
				// Currently members can only be users or lists, may need to
				// handle bots in the future.
				continue
			}

			// Don't walk through this list if has already been seen.
			if seenLists.Contains(member.GetName()) {
				continue
			}

			// Fetch the member list resource so the recursive call can check
			// if it has any membership requires.
			memberList, err := m.aclReader.GetAccessList(ctx, member.GetName())
			if err != nil {
				// Yield the error so the caller can handle it how it wants,
				// but continue iterating other nested members.
				if !yield(nil, trace.Wrap(err, "fetching member list")) {
					return
				}
				continue
			}

			// Walk and yield all nested members of this member list.
			m.walkUserMembersRecursive(ctx, memberList, seenLists, seenUsers)(yield)
		}
	}
}

type mapStringSet struct {
	m map[string]set.Set[string]
}

func newMapStringSet() mapStringSet {
	return mapStringSet{
		m: make(map[string]set.Set[string]),
	}
}

// readOnlySet implements a read-only view of a set.Set[string] that only
// contains methods guaranteed to work even if the underlying map is nil.
type readOnlySet struct {
	s set.Set[string]
}

// Contains implements a membership test for the readOnlySet.
func (s readOnlySet) Contains(key string) bool {
	return s.s.Contains(key)
}

// Items returns an iterator over all items in the set.
func (s readOnlySet) Items() iter.Seq[string] {
	return maps.Keys(s.s)
}

// Get returns a read-only view of the set for the given key, it does not
// allocate a set if the key is not present.
func (m *mapStringSet) Get(key string) readOnlySet {
	return readOnlySet{m.m[key]}
}

// Ensure returns a set for the given key, creating an empty set if one is not
// currently present. Prefer [mapStringSet.Get] if the retuned set does not
// need to be mutated.
func (m *mapStringSet) Ensure(key string) set.Set[string] {
	if s, ok := m.m[key]; ok {
		return s
	}
	s := set.New[string]()
	m.m[key] = s
	return s
}

type ancestorCache struct {
	parents  mapStringSet
	children mapStringSet
	ownedBy  mapStringSet
	ownersOf mapStringSet
}

func newAncestorCache() *ancestorCache {
	return &ancestorCache{
		parents:  newMapStringSet(),
		children: newMapStringSet(),
		ownedBy:  newMapStringSet(),
		ownersOf: newMapStringSet(),
	}
}

func (c *ancestorCache) addMembership(parent, member string) {
	c.parents.Ensure(member).Add(parent)
	c.children.Ensure(parent).Add(member)
}

func (c *ancestorCache) removeMembership(parent, member string) {
	c.parents.Ensure(member).Remove(parent)
	c.children.Ensure(parent).Remove(member)
}

func (c *ancestorCache) addOwnership(owner, owned string) {
	c.ownedBy.Ensure(owner).Add(owned)
	c.ownersOf.Ensure(owned).Add(owner)
}

func (c *ancestorCache) removeOwnership(owner, owned string) {
	c.ownedBy.Ensure(owner).Remove(owned)
	c.ownersOf.Ensure(owned).Remove(owner)
}

func (c *ancestorCache) clearOwnersOf(owned string) {
	for owner := range c.ownersOf.Get(owned).Items() {
		c.removeOwnership(owner, owned)
	}
}

type collectAncestorListsParams struct {
	startListName      string
	validateMembership func(parentListName, memberListName string) bool
	validateOwnership  func(ownerListName, ownedListName string) bool
}

type ancestorRelation struct {
	isOwner  bool
	isMember bool
}

// collectAncestorLists returns a collection of all ancestor lists of the given
// list, that is all lists where the given list is a (nested) member or owner.
// This may be useful for calculating all related lists that may require an
// assignment to be materialized for members of the starting list.
func (c *ancestorCache) collectAncestorLists(params collectAncestorListsParams) map[string]ancestorRelation {
	result := make(map[string]ancestorRelation)
	markOwned := func(ownedListName string) {
		curr := result[ownedListName]
		curr.isOwner = true
		result[ownedListName] = curr
	}
	markMember := func(parentListName string) {
		curr := result[parentListName]
		curr.isMember = true
		result[parentListName] = curr
	}

	seen := set.New[string]()

	var collectAncestorsRecursive func(currListName string)
	collectAncestorsRecursive = func(currListName string) {
		if seen.Contains(currListName) {
			return
		}
		seen.Add(currListName)

		// User is a member of currList, which implies they are an owner of all
		// lists owned by currList.
		for ownedListName := range c.ownedBy.Get(currListName).Items() {
			if !params.validateOwnership(currListName, ownedListName) {
				continue
			}
			markOwned(ownedListName)
		}

		// User is a member of currList, which implies they are a member of all
		// lists where currList is a member.
		for parentListName := range c.parents.Get(currListName).Items() {
			if !params.validateMembership(parentListName, currListName) {
				continue
			}
			markMember(parentListName)
			collectAncestorsRecursive(parentListName)
		}
	}

	collectAncestorsRecursive(params.startListName)
	return result
}

// collectAncestorListsWithoutValidation returns a collection of all ancestor
// lists of the given list, that is all lists where the given list is a
// (nested) member or owner.
//
// This variant does no validation on membership or ownerships and is infallible:
// if m.ancestorCache is in a correct state, this is guaranteed to return all
// potential ancestors of the starting list, even if no access list or member
// resources can be fetched.
//
// This is useful when handling membership deletions, which requires
// pessimistic validation of every possible already-materialized assignment
// that may need to be invalidated.
func (m *Materializer) collectAncestorListsWithoutValidation(startListName string) map[string]ancestorRelation {
	return m.ancestorCache.collectAncestorLists(collectAncestorListsParams{
		startListName:      startListName,
		validateMembership: func(string, string) bool { return true },
		validateOwnership:  func(string, string) bool { return true },
	})
}

// ancestor represents an ancestor access list and its relation to a starting
// access list.
type ancestor struct {
	relation ancestorRelation
	list     *accesslist.AccessList
}

// collectAncestors returns a filtered and validated view of all ancestor
// lists of the given starting list. This is the set of all access lists that
// should grant scoped roles to a user based on membership in the starting
// list.
//
// Initially, all lists where the starting list is a (nested) member OR an
// owner are considered an ancestor.
//
// Every member relationship is validated to assert that it is not expired and
// the parent list does not contain any membership_requires. Membership paths
// through invalid member resources or access lists are now followed.
//
// Every owner relationship is validated to assert that the owned list does not
// contain any ownership requires.
//
// Finally, any lists that do not contain any effective scoped role grants are
// filted out. It is safe to materialize an assignment for all returned lists
// without any further validation.
func (m *Materializer) collectAncestors(ctx context.Context, startListName string) ([]ancestor, []error) {
	var validationErrors []error
	fetchedLists := make(map[string]*accesslist.AccessList)
	ancestorRelations := m.ancestorCache.collectAncestorLists(collectAncestorListsParams{
		startListName: startListName,
		validateMembership: func(parentListName, memberListName string) bool {
			memberResource, err := m.aclReader.GetAccessListMember(ctx, parentListName, memberListName)
			if err != nil {
				validationErrors = append(validationErrors,
					trace.Wrap(err, "getting access list member %q in list %q", memberListName, parentListName))
				return false
			}
			if memberResource.IsExpired(time.Now()) {
				// This is not a validation error, expired member resources are
				// expected but should not be followed.
				return false
			}
			parentList, err := m.aclReader.GetAccessList(ctx, parentListName)
			if err != nil {
				validationErrors = append(validationErrors,
					trace.Wrap(err, "getting access list %q", parentListName))
				return false
			}
			fetchedLists[parentListName] = parentList
			if hasMembershipRequires(parentList) {
				// This is not necessarily a validation error, many lists may
				// have membership requires, which is fine as long as they
				// don't grant scoped roles, we just don't need to follow the
				// edge.
				return false
			}
			return true
		},
		validateOwnership: func(parentListName, ownedListName string) bool {
			ownedList, err := m.aclReader.GetAccessList(ctx, ownedListName)
			if err != nil {
				validationErrors = append(validationErrors,
					trace.Wrap(err, "getting access list %q", ownedListName))
				return false
			}
			fetchedLists[ownedListName] = ownedList
			if hasOwnershipRequires(ownedList) {
				// This is not necessarily a validation error, many lists may
				// have ownership requires, which is fine as long as they
				// don't grant scoped roles, we just don't need to follow the
				// edge.
				return false
			}
			return true
		},
	})

	var filteredAncestors []ancestor
	for ancestorListName, ancestorRelation := range ancestorRelations {
		ancestorList, ok := fetchedLists[ancestorListName]
		if !ok {
			validationErrors = append(validationErrors, trace.Errorf("ancestor list was not fetched, this is a bug"))
			continue
		}
		if hasMemberGrants(ancestorList) && ancestorRelation.isMember ||
			hasOwnerGrants(ancestorList) && ancestorRelation.isOwner {
			filteredAncestors = append(filteredAncestors, ancestor{
				list:     ancestorList,
				relation: ancestorRelation,
			})
		}
	}
	return filteredAncestors, validationErrors
}

// RepairEventLoop should be called in a goroutine after materializer init
// if the materializer is expected to be long-lived. It runs continuously until
// ctx is done. It will send an event on [materializer.RepairEvents] at the
// time of each scheduled repair event.
func (m *Materializer) RepairEventLoop(ctx context.Context) {
	for {
		nextEvent, nextEventTime := m.nextRepairEvent()

		// If the next scheduled repair event is in the future, wait until
		// that time or we get woken up because the time has been moved
		// earlier.
		waitFor := time.Until(nextEventTime)
		if waitFor > 0 {
			select {
			case <-time.After(waitFor):
			case <-ctx.Done():
				return
			case <-m.wakeRepairLoop:
				continue
			}
		}

		// We have a repair event scheduled for now, send it on repairEventC.
		// Reset the repair time before sending it in case it gets set again
		// while handling the repair event.
		m.resetRepairTime(nextEvent)
		select {
		case m.repairEventC <- nextEvent:
		case <-ctx.Done():
			return
		}
	}
}

// RepairEvents returns a channel from which events should be received and
// passed to [materializer.ProcessRepairEvent]. This facilitates
// single-threaded processing of cache events and repair events.
func (m *Materializer) RepairEvents() <-chan repairEvent {
	return m.repairEventC
}

// ProcessRepairEvent should be called with repair events read from
// [materializer.RepairEvents]. It must not be called concurrently with
// [materializer.ProcessEvent].
func (m *Materializer) ProcessRepairEvent(ctx context.Context, event repairEvent) {
	ctx, span := m.tracer.Start(ctx, "scoped_access_cache/materializer/process_repair_event",
		oteltrace.WithAttributes(attribute.String("repair.type", event.String())),
	)
	defer span.End()

	switch event {
	case repairExpiredMembersEvent:
		m.repairExpiredMembers(ctx)
	case repairMissedMembersEvent:
		m.repairMissedMembers(ctx)
	}
}

func (m *Materializer) scheduleRepair(ctx context.Context, event repairEvent, t time.Time) {
	m.repairTimeMu.Lock()
	defer m.repairTimeMu.Unlock()

	switch event {
	case repairExpiredMembersEvent:
		if t.Before(m.nextExpiredMembersRepairTime) {
			m.nextExpiredMembersRepairTime = t
			m.logger.DebugContext(ctx, "Scheduled next expired membership repair", "repair_time", t)
			select {
			case m.wakeRepairLoop <- struct{}{}:
			default:
			}
		}
	case repairMissedMembersEvent:
		if t.Before(m.nextRepairMissedMembersTime) {
			m.nextRepairMissedMembersTime = t
			m.logger.DebugContext(ctx, "Scheduled next missed membership repair", "repair_time", t)
			select {
			case m.wakeRepairLoop <- struct{}{}:
			default:
			}
		}
	}
}

func (m *Materializer) resetRepairTime(event repairEvent) {
	m.repairTimeMu.Lock()
	defer m.repairTimeMu.Unlock()
	switch event {
	case repairExpiredMembersEvent:
		m.nextExpiredMembersRepairTime = time.Now().Add(century)
	case repairMissedMembersEvent:
		m.nextRepairMissedMembersTime = time.Now().Add(century)
	}
}

// reportFutureMemberExpiry schedules repairExpiredMembers for the expiry time,
// if one is not already scheduled for earlier.
func (m *Materializer) reportFutureMemberExpiry(ctx context.Context, expires time.Time) {
	if expires.IsZero() {
		return
	}
	m.scheduleRepair(ctx, repairExpiredMembersEvent, expires)
}

// scheduleMissedMembersRepair schedules repairMissedMembers for some time in
// the future.
func (m *Materializer) scheduleMissedMembersRepair(ctx context.Context) {
	m.scheduleRepair(ctx, repairMissedMembersEvent, time.Now().Add(MissedMembersRepairBackoff))
}

func (m *Materializer) nextRepairEvent() (repairEvent, time.Time) {
	m.repairTimeMu.Lock()
	defer m.repairTimeMu.Unlock()
	if m.nextRepairMissedMembersTime.Before(m.nextExpiredMembersRepairTime) {
		return repairMissedMembersEvent, m.nextRepairMissedMembersTime
	}
	return repairExpiredMembersEvent, m.nextExpiredMembersRepairTime
}

// repairExpiredMembers processes all expired member resources to make sure
// that any invalid materialized assignments are cleared.
func (m *Materializer) repairExpiredMembers(ctx context.Context) {
	m.logger.InfoContext(ctx, "Running expired membership repair")

	expiredMembersOf, nextExpiry := m.collectExpiredMembers(ctx)
	m.reportFutureMemberExpiry(ctx, nextExpiry)

	affected := 0
	for key := range m.affectedAssignmentsForExpiredMembers(expiredMembersOf) {
		affected++
		if err := m.recheckAssignment(ctx, key); err != nil {
			// Must pessimistically assume any assignment is invalid if we
			// encountered an error trying to validate it.
			m.logger.InfoContext(ctx, "Encountered an error validating materialized assignment, will delete the assignment",
				"error", err)
			m.deleteMaterializedAssignment(ctx, key)
			m.scheduleMissedMembersRepair(ctx)
		}
	}
	oteltrace.SpanFromContext(ctx).SetAttributes(attribute.Int("assignments.affected", affected))
}

// collectExpiredMembers collects all expired access list members by parent
// list, and returns the earliest seen future expiry.
func (m *Materializer) collectExpiredMembers(ctx context.Context) (expiredMembersOf map[string]expiredMembers, nextExpiry time.Time) {
	// Use a consistent time for "now" so that this will report all members
	// expired before this time, and schedule another repair for any members
	// expiring after this time.
	now := time.Now()

	// Keep track of the nearest seen member expiry time that's in the future.
	nextExpiry = now.Add(century)

	expiredMembersOf = make(map[string]expiredMembers)
	expiredCount := 0

	// Iterate all access list members to find any that are expired.
	for member, err := range clientutils.Resources(ctx, m.aclReader.ListAllAccessListMembers) {
		if err != nil {
			// Failed to iterate all access list member resources, we may have
			// missed an expired member or one that will be expiring soon.
			m.logger.WarnContext(ctx, "Failed to iterate access list members, some stale scoped role assignments may remain despite expired access list membership",
				"error", err)
			nextRepairAt := time.Now().Add(ExpiredMembersRepairBackoff)
			if nextRepairAt.Before(nextExpiry) {
				nextExpiry = nextRepairAt
			}
			continue
		}

		if !member.IsExpired(now) {
			if !member.Spec.Expires.IsZero() && member.Spec.Expires.Before(nextExpiry) {
				nextExpiry = member.Spec.Expires
			}
			continue
		}

		m.logger.DebugContext(ctx, "Found expired member resource",
			"list", member.Spec.AccessList,
			"member", member.GetName(),
			"membership_kind", member.Spec.MembershipKind,
			"expired", member.Spec.Expires)

		// Collect the expired membership.
		knownExpiredMembers := expiredMembersOf[member.Spec.AccessList]
		knownExpiredMembers.insert(member)
		expiredMembersOf[member.Spec.AccessList] = knownExpiredMembers
		expiredCount++
	}
	oteltrace.SpanFromContext(ctx).SetAttributes(attribute.Int("members.expired", expiredCount))

	return expiredMembersOf, nextExpiry
}

// affectedAssignmentsForExpiredMembers returns an iterator of all current
// materialized assignments that may be affected by an expired membership.
func (m *Materializer) affectedAssignmentsForExpiredMembers(expiredMembersOf map[string]expiredMembers) iter.Seq[MaterializedAssignmentKey] {
	assignmentFilters := make(map[string]assignmentFilter)
	for parentListName, knownExpiredMembers := range expiredMembersOf {
		currFilter := assignmentFilters[parentListName]
		currFilter.merge(assignmentFilter{
			// If this list has an expired list member, assignments for any
			// user in this list may be affected.
			anyUser: knownExpiredMembers.hasExpiredListMember,
			// If this list has expired user members, assignments for those
			// users in this list may be affected.
			users: knownExpiredMembers.users,
		})
		assignmentFilters[parentListName] = currFilter

		// Assignments for any ancestor list may be affected. Collect all
		// ancestor lists without validation to avoid missing any due to
		// read/paging errors.
		ancestors := m.collectAncestorListsWithoutValidation(parentListName)
		for ancestorListName := range ancestors {
			currFilter := assignmentFilters[ancestorListName]
			currFilter.merge(assignmentFilter{
				// If the original list has an expired list member, assignments
				// for any user in this ancestor list may be affected.
				anyUser: knownExpiredMembers.hasExpiredListMember,
				// If the original list has expired user members, assignments
				// for those users in this ancestor list may be affected.
				users: knownExpiredMembers.users,
			})
			assignmentFilters[ancestorListName] = currFilter
		}
	}

	return func(yield func(MaterializedAssignmentKey) bool) {
		for key := range m.materializedAssignments {
			if !assignmentFilters[key.List].match(key.User) {
				continue
			}
			if !yield(key) {
				return
			}
		}
	}
}

type expiredMembers struct {
	hasExpiredListMember bool
	users                set.Set[string]
}

func (m *expiredMembers) insert(member *accesslist.AccessListMember) {
	m.hasExpiredListMember = m.hasExpiredListMember || member.Spec.MembershipKind == accesslist.MembershipKindList
	if m.hasExpiredListMember {
		// users set is unneeded if there is an expired list member.
		m.users = nil
		return
	}
	if member.IsUser() {
		if m.users == nil {
			m.users = set.NewWithCapacity[string](1)
		}
		m.users.Add(member.GetName())
	}
}

type assignmentFilter struct {
	anyUser bool
	users   set.Set[string]
}

func (f assignmentFilter) match(user string) bool {
	if f.anyUser {
		return true
	}
	return f.users.Contains(user)
}

func (f *assignmentFilter) merge(other assignmentFilter) {
	f.anyUser = f.anyUser || other.anyUser
	if f.anyUser {
		// users set is unneeded if matching any user.
		f.users = nil
		return
	}
	if other.users.Len() == 0 {
		return
	}
	if f.users == nil {
		f.users = set.NewWithCapacity[string](other.users.Len())
	}
	f.users.Union(other.users)
}

// repairMissedMembers additively materializes assignments for all members and
// owners in all lists.
func (m *Materializer) repairMissedMembers(ctx context.Context) {
	m.logger.InfoContext(ctx, "Running missed membership repair")

	// Iterate all access lists to additively materialize assignments for all
	// their members and owners.
	repairedLists := 0
	for list, err := range clientutils.Resources(ctx, m.aclReader.ListAccessLists) {
		if err != nil {
			m.logger.InfoContext(ctx, "Missed membership repair failed to read all access lists, scheduling another repair",
				"error", err)
			oteltrace.SpanFromContext(ctx).RecordError(err)
			m.scheduleMissedMembersRepair(ctx)
			return
		}
		repairedLists++
		m.initAccessListMembers(ctx, list)
		m.initAccessListOwners(ctx, list)
	}
	oteltrace.SpanFromContext(ctx).SetAttributes(attribute.Int("lists.repaired", repairedLists))
}

func hasMemberGrants(list *accesslist.AccessList) bool {
	return len(list.Spec.Grants.ScopedRoles) > 0
}

func hasOwnerGrants(list *accesslist.AccessList) bool {
	return len(list.Spec.OwnerGrants.ScopedRoles) > 0
}

func hasMembershipRequires(list *accesslist.AccessList) bool {
	return !list.Spec.MembershipRequires.IsEmpty()
}

func hasOwnershipRequires(list *accesslist.AccessList) bool {
	return !list.Spec.OwnershipRequires.IsEmpty()
}

func materializerEventAttributes(event types.Event) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("event.type", event.Type.String()),
	}
	if event.Resource == nil {
		return attrs
	}
	attrs = append(attrs,
		attribute.String("resource.kind", event.Resource.GetKind()),
		attribute.String("resource.name", event.Resource.GetName()),
	)
	return attrs
}

func (e repairEvent) String() string {
	switch e {
	case repairExpiredMembersEvent:
		return "expired_members"
	case repairMissedMembersEvent:
		return "missed_members"
	default:
		return "unknown"
	}
}
