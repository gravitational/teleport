package accesslists

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
)

// GetMembersFor returns a flattened list of Members for an Access List, including inherited Members.
//
// Returned Members are not validated for expiration or other requirements – use IsAccessListMember
// to validate a Member's membership status.
// DEPRECATED: use Hierarchy.GetMembersFor instead.
func GetMembersFor(ctx context.Context, accessListName string, g AccessListAndMembersGetter) ([]*accesslist.AccessListMember, error) {
	h, err := NewHierarchy(HierarchyConfig{
		AccessListsService: g,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members, err := h.GetMembersFor(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return members, nil
}

// GetOwnersFor returns a flattened list of Owners for an Access List, including inherited Owners.
//
// Returned Owners are not validated for expiration or other requirements – use IsAccessListOwner
// to validate an Owner's ownership status.
// DEPRECATED: use Hierarchy.GetOwnersFor instead.
func GetOwnersFor(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) ([]*accesslist.Owner, error) {
	h, err := NewHierarchy(HierarchyConfig{
		AccessListsService: g,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	owners, err := h.GetOwnersFor(ctx, accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return owners, nil
}

// IsAccessListOwner checks if the given user is the Access List owner. It returns an error matched
// by [IsUserLocked] if the user is locked.
// DEPRECATED: use Hierarchy.IsAccessListOwner instead.
func IsAccessListOwner(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (accesslistv1.AccessListUserAssignmentType, error) {
	h, err := NewHierarchy(HierarchyConfig{
		AccessListsService: g,
		LockService:        lockGetter,
		Clock:              clock,
	})
	if err != nil {
		return userAssignUnspecified, trace.Wrap(err)
	}
	r, err := h.IsAccessListOwner(ctx, user, accessList)
	if err != nil {
		return userAssignUnspecified, trace.Wrap(err)
	}
	return r, nil
}

// IsAccessListMember checks if the given user is the Access List member. It returns an error
// matched by [IsUserLocked] if the user is locked.
// DEPRECATED: use Hierarchy.IsAccessListMember instead.
func IsAccessListMember(
	ctx context.Context,
	user types.User,
	accessList *accesslist.AccessList,
	g AccessListAndMembersGetter,
	lockGetter services.LockGetter,
	clock clockwork.Clock,
) (accesslistv1.AccessListUserAssignmentType, error) {
	h, err := NewHierarchy(HierarchyConfig{
		AccessListsService: g,
		LockService:        lockGetter,
		Clock:              clock,
	})
	if err != nil {
		return userAssignUnspecified, trace.Wrap(err)
	}
	r, err := h.IsAccessListMember(ctx, user, accessList)
	if err != nil {
		return userAssignUnspecified, trace.Wrap(err)
	}
	return r, nil
}

// GetAncestorsFor calculates and returns the set of Ancestor ACLs depending on
// the supplied relationship criteria. Order of the ancestor list is undefined.
// DEPRECATED: use Hierarchy.GetAncestorsFor instead.
func GetAncestorsFor(ctx context.Context, accessList *accesslist.AccessList, kind RelationshipKind, g AccessListAndMembersGetter) ([]*accesslist.AccessList, error) {
	h, err := NewHierarchy(HierarchyConfig{
		AccessListsService: g,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ancestors, err := h.GetAncestorsFor(ctx, accessList, kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ancestors, nil
}
