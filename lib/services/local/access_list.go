/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"slices"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	accessListPrefix      = "access_list"
	accessListMaxPageSize = 100

	accessListMemberPrefix      = "access_list_member"
	accessListMemberMaxPageSize = 200

	accessListReviewPrefix      = "access_list_review"
	accessListReviewMaxPageSize = 200

	// This lock is necessary to prevent a race condition between access lists and members and to ensure
	// consistency of the one-to-many relationship between them.
	accessListLockTTL = 5 * time.Second

	// createAccessListLimitLockName is the lock used to prevent simultaneous
	// creation or update of AccessLists in order to enforce the license limit
	// on the number AccessLists in a cluster.
	createAccessListLimitLockName = "createAccessListLimitLock"
	// accessListResourceLockName is the lock used to prevent simultaneous
	// writing to any AccessList resources (AccessLists, AccessListMembers).
	// it shares the same string as createAccessListLimitLockName to ensure
	// backwards compatibility.
	accessListResourceLockName = createAccessListLimitLockName
)

// AccessListService manages Access List resources in the Backend. The AccessListService's
// sole job is to manage and co-ordinate operations on the underlying AccessList,
// AccessListMember, etc resources in the backend in order to provide a
// consistent view to the rest of the Teleport application. It makes no decisions
// about granting or withholding list membership.
type AccessListService struct {
	clock         clockwork.Clock
	service       *generic.Service[*accesslist.AccessList]
	memberService *generic.Service[*accesslist.AccessListMember]
	reviewService *generic.Service[*accesslist.Review]
}

type accessListAndMembersGetter struct {
	service       *generic.Service[*accesslist.AccessList]
	memberService *generic.Service[*accesslist.AccessListMember]
}

func (s *accessListAndMembersGetter) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return s.memberService.WithPrefix(accessListName).ListResources(ctx, pageSize, pageToken)
}
func (s *accessListAndMembersGetter) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return s.service.GetResource(ctx, name)
}

// compile-time assertion that the AccessListService implements the AccessLists
// interface
var _ services.AccessLists = (*AccessListService)(nil)

// NewAccessListService creates a new AccessListService.
func NewAccessListService(b backend.Backend, clock clockwork.Clock, opts ...ServiceOption) (*AccessListService, error) {
	var opt serviceOptions
	for _, o := range opts {
		o(&opt)
	}
	service, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessList]{
		Backend:                     b,
		PageLimit:                   accessListMaxPageSize,
		ResourceKind:                types.KindAccessList,
		BackendPrefix:               backend.NewKey(accessListPrefix),
		MarshalFunc:                 services.MarshalAccessList,
		UnmarshalFunc:               services.UnmarshalAccessList,
		RunWhileLockedRetryInterval: opt.runWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberService, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessListMember]{
		Backend:                     b,
		PageLimit:                   accessListMemberMaxPageSize,
		ResourceKind:                types.KindAccessListMember,
		BackendPrefix:               backend.NewKey(accessListMemberPrefix),
		MarshalFunc:                 services.MarshalAccessListMember,
		UnmarshalFunc:               services.UnmarshalAccessListMember,
		RunWhileLockedRetryInterval: opt.runWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviewService, err := generic.NewService(&generic.ServiceConfig[*accesslist.Review]{
		Backend:                     b,
		PageLimit:                   accessListReviewMaxPageSize,
		ResourceKind:                types.KindAccessListReview,
		BackendPrefix:               backend.NewKey(accessListReviewPrefix),
		MarshalFunc:                 services.MarshalAccessListReview,
		UnmarshalFunc:               services.UnmarshalAccessListReview,
		RunWhileLockedRetryInterval: opt.runWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		clock:         clock,
		service:       service,
		memberService: memberService,
		reviewService: reviewService,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (a *AccessListService) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	accessLists, err := a.service.GetResources(ctx)
	return accessLists, trace.Wrap(err)
}

// GetInheritedGrants returns grants inherited by access list accessListID from parent access lists.
// This is not implemented in the local service.
func (a *AccessListService) GetInheritedGrants(ctx context.Context, accessListID string) (*accesslist.Grants, error) {
	return nil, trace.NotImplemented("GetInheritedGrants should not be called")
}

// ListAccessLists returns a paginated list of access lists.
func (a *AccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	return a.service.ListResources(ctx, pageSize, nextToken)
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	var accessList *accesslist.AccessList
	err := a.service.RunWhileLocked(ctx, lockName(name), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		var err error
		accessList, err = a.service.GetResource(ctx, name)
		return trace.Wrap(err)
	})
	return accessList, trace.Wrap(err)
}

// GetAccessListsToReview returns access lists that the user needs to review. This is not implemented in the local service.
func (a *AccessListService) GetAccessListsToReview(ctx context.Context) ([]*accesslist.AccessList, error) {
	return nil, trace.NotImplemented("GetAccessListsToReview should not be called")
}

// UpsertAccessList creates or updates an access list resource.
func (a *AccessListService) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	return a.runOpWithLock(ctx, accessList, opTypeUpsert)
}

// UpdateAccessList updates an access list resource.
func (a *AccessListService) UpdateAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	return a.runOpWithLock(ctx, accessList, opTypeUpdate)
}

type opType int

const (
	opTypeUpsert opType = iota
	opTypeUpdate
)

func (a *AccessListService) runOpWithLock(ctx context.Context, accessList *accesslist.AccessList, op opType) (*accesslist.AccessList, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var upserted *accesslist.AccessList
	var existingList *accesslist.AccessList

	opFn := a.service.UpsertResource
	if op == opTypeUpdate {
		opFn = a.service.ConditionalUpdateResource
	}

	validateAccessList := func() error {
		var err error

		if op == opTypeUpdate {
			existingList, err = a.service.GetResource(ctx, accessList.GetName())
			if err != nil {
				return trace.Wrap(err)
			}
			// Set memberOf / ownerOf to the existing values to prevent them from being updated.
			accessList.Status.MemberOf = existingList.Status.MemberOf
			accessList.Status.OwnerOf = existingList.Status.OwnerOf
		} else {
			// In case the MemberOf/OwnerOf fields were manually changed, set to empty.
			accessList.Status.MemberOf = []string{}
			accessList.Status.OwnerOf = []string{}
		}

		listMembers, err := a.memberService.WithPrefix(accessList.GetName()).GetResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		return accesslists.ValidateAccessListWithMembers(ctx, accessList, listMembers, &accessListAndMembersGetter{a.service, a.memberService})
	}

	updateAccessList := func() error {
		var err error
		upserted, err = opFn(ctx, accessList)
		return trace.Wrap(err)
	}

	reconcileOwners := func() error {
		// Create map to store owners for efficient lookup
		originalOwnersMap := make(map[string]struct{})
		if existingList != nil {
			for _, owner := range existingList.Spec.Owners {
				if owner.MembershipKind == accesslist.MembershipKindList {
					originalOwnersMap[owner.Name] = struct{}{}
				}
			}
		}

		currentOwnersMap := make(map[string]struct{})
		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				currentOwnersMap[owner.Name] = struct{}{}
			}
		}

		// update references for new owners
		for ownerName := range currentOwnersMap {
			if _, exists := originalOwnersMap[ownerName]; !exists {
				if err := a.updateAccessListOwnerOf(ctx, accessList.GetName(), ownerName, true); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		// update references for old owners
		for ownerName := range originalOwnersMap {
			if _, exists := currentOwnersMap[ownerName]; !exists {
				if err := a.updateAccessListOwnerOf(ctx, accessList.GetName(), ownerName, false); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		return nil
	}

	var actions []func() error

	// If IGS is not enabled for this cluster we need to wrap the whole
	// operation inside *another* lock so that we can accurately count the
	// access lists in the cluster in order to prevent un-authorized use of
	// the AccessList feature
	if !modules.GetModules().Features().GetEntitlement(entitlements.Identity).Enabled {
		actions = append(actions, func() error { return a.VerifyAccessListCreateLimit(ctx, accessList.GetName()) })
	}

	actions = append(actions, validateAccessList, updateAccessList, reconcileOwners)

	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL,
		func(ctx context.Context, _ backend.Backend) error {
			return a.service.RunWhileLocked(ctx, lockName(accessList.GetName()), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
				for _, action := range actions {
					if err := action(); err != nil {
						return trace.Wrap(err)
					}
				}
				return nil
			})
		})

	return upserted, trace.Wrap(err)
}

// DeleteAccessList removes the specified access list resource.
func (a *AccessListService) DeleteAccessList(ctx context.Context, name string) error {
	action := func(ctx context.Context, _ backend.Backend) error {
		// Get list resource.
		accessList, err := a.service.GetResource(ctx, name)
		if err != nil {
			return trace.Wrap(err)
		}

		// Check if the access list is a member or owner of any other access lists.
		if len(accessList.Status.MemberOf) > 0 {
			for _, memberOf := range accessList.Status.MemberOf {
				if _, err := a.service.GetResource(ctx, memberOf); err == nil {
					return trace.AccessDenied("Cannot delete '%s', as it is a member of one or more other Access Lists", accessList.Spec.Title)
				}
			}
		}
		if len(accessList.Status.OwnerOf) > 0 {
			for _, ownerOf := range accessList.Status.OwnerOf {
				if _, err := a.service.GetResource(ctx, ownerOf); err == nil {
					return trace.AccessDenied("Cannot delete '%s', as it is an owner of one or more other Access Lists", accessList.Spec.Title)
				}
			}
		}

		// Delete all associated members.
		members, err := a.memberService.WithPrefix(name).GetResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := a.memberService.WithPrefix(name).DeleteAllResources(ctx); err != nil {
			return trace.Wrap(err)
		}

		// Update memberOf refs.
		for _, member := range members {
			if member.Spec.MembershipKind != accesslist.MembershipKindList {
				continue
			}
			if err := a.updateAccessListMemberOf(ctx, name, member.GetName(), false); err != nil {
				return trace.Wrap(err)
			}
		}

		// Delete list itself.
		if err := a.service.DeleteResource(ctx, name); err != nil {
			return trace.Wrap(err)
		}

		// Update ownerOf refs.
		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind != accesslist.MembershipKindList {
				continue
			}
			if err := a.updateAccessListOwnerOf(ctx, name, owner.Name, false); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}

	return trace.Wrap(a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(name), accessListLockTTL, action)
	}))
}

// DeleteAllAccessLists removes all access lists.
func (a *AccessListService) DeleteAllAccessLists(ctx context.Context) error {
	// Locks are not used here as these operations are more likely to be used by the cache.
	// Delete all members for all access lists.
	err := a.memberService.DeleteAllResources(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.service.DeleteAllResources(ctx))
}

// GetSuggestedAccessLists returns a list of access lists that are suggested for a given request. This is not implemented in the local service.
func (a *AccessListService) GetSuggestedAccessLists(ctx context.Context, accessRequestID string) ([]*accesslist.AccessList, error) {
	return nil, trace.NotImplemented("GetSuggestedAccessLists should not be called")
}

// GetSuggestedAccessListsForResources returns a list of access lists that are suggested for a set of resources.
func (a *AccessListService) GetSuggestedAccessListsForResources(ctx context.Context, resourceIDs []string) (*accesslistv1.GetSuggestedAccessListsResponse, error) {
	// Get all access lists
	_, err := a.GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return empty response with properly initialized fields
	return &accesslistv1.GetSuggestedAccessListsResponse{
		AccessLists:               []*accesslistv1.AccessList{},
		ResourcesNotInAccessList:  resourceIDs, // All resources are considered "not in list" in OSS
		ResourcesInDifferentLists: []string{},
	}, nil
}

// CountAccessListMembers will count all access list members.
func (a *AccessListService) CountAccessListMembers(ctx context.Context, accessListName string) (users uint32, lists uint32, err error) {
	count := uint(0)
	listCount := uint(0)
	err = a.service.RunWhileLocked(ctx, lockName(accessListName), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		var err error
		members, err := a.memberService.WithPrefix(accessListName).GetResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, member := range members {
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				listCount++
			} else {
				count++
			}
		}
		return nil
	})

	return uint32(count), uint32(listCount), trace.Wrap(err)
}

// ListAccessListMembers returns a paginated list of all access list members.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	var members []*accesslist.AccessListMember
	err := a.service.RunWhileLocked(ctx, lockName(accessListName), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}
		members, nextToken, err = a.memberService.WithPrefix(accessListName).ListResources(ctx, pageSize, nextToken)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return members, nextToken, nil
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (a *AccessListService) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	members, next, err := a.memberService.ListResourcesReturnNextResource(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	var nextKey string
	if next != nil {
		nextKey = (*next).Spec.AccessList + string(backend.Separator) + (*next).Metadata.Name
	}
	return members, nextKey, nil
}

// GetAccessListMember returns the specified access list member resource.
func (a *AccessListService) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	var member *accesslist.AccessListMember
	err := a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		member, err = a.memberService.WithPrefix(accessList).GetResource(ctx, memberName)
		return trace.Wrap(err)
	})
	return member, trace.Wrap(err)
}

// updateAccessListRefField is a helper that updates the specified field (memberOf or ownerOf) of an Access List,
// adding or removing the specified accessListName to the field of targetName.
func (a *AccessListService) updateAccessListRefField(
	ctx context.Context,
	accessListName string,
	targetName string,
	new bool,
	fieldSelector func(status *accesslist.Status) *[]string,
) error {
	targetAccessList, err := a.service.GetResource(ctx, targetName)
	if err != nil {
		if trace.IsNotFound(err) {
			// If list is not found, it's possible that it was deleted. Regardless, there's nothing to update.
			return nil
		}
		return trace.Wrap(err)
	}

	field := fieldSelector(&targetAccessList.Status)

	// If the field already contains the Access List, and we're adding,
	// or doesn't contain it, and we're removing, there's nothing to do.
	if slices.Contains(*field, accessListName) == new {
		return nil
	}

	if new {
		*field = append(*field, accessListName)
	} else {
		*field = slices.DeleteFunc(*field, func(e string) bool {
			return e == accessListName
		})
	}

	if _, err := a.service.UpdateResource(ctx, targetAccessList); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// updateAccessListMemberOf updates the memberOf field for the specified memberName and accessListName.
// Should only be called after the relevant member has been successfully upserted or deleted.
func (a *AccessListService) updateAccessListMemberOf(ctx context.Context, accessListName, memberName string, new bool) error {
	return a.updateAccessListRefField(ctx, accessListName, memberName, new, func(status *accesslist.Status) *[]string {
		return &status.MemberOf
	})
}

// updateAccessListOwnerOf updates the ownerOf field for the specified ownerName and accessListName.
// Should only be called after the relevant owner has been successfully upserted or deleted.
func (a *AccessListService) updateAccessListOwnerOf(ctx context.Context, accessListName, ownerName string, new bool) error {
	return a.updateAccessListRefField(ctx, accessListName, ownerName, new, func(status *accesslist.Status) *[]string {
		return &status.OwnerOf
	})
}

// GetAccessListOwners returns a list of all owners in an Access List, including those inherited from nested Access Lists.
//
// Returned Owners are not validated for ownership requirements – use `IsAccessListOwner` for validation.
func (a *AccessListService) GetAccessListOwners(ctx context.Context, accessListName string) ([]*accesslist.Owner, error) {
	var owners []*accesslist.Owner
	err := a.service.RunWhileLocked(ctx, lockName(accessListName), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		accessList, err := a.service.GetResource(ctx, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}
		owners, err = accesslists.GetOwnersFor(ctx, accessList, &accessListAndMembersGetter{a.service, a.memberService})
		return trace.Wrap(err)
	})
	return owners, trace.Wrap(err)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (a *AccessListService) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	var upserted *accesslist.AccessListMember
	action := func(ctx context.Context, _ backend.Backend) error {
		memberList, err := a.service.GetResource(ctx, member.Spec.AccessList)
		if err != nil {
			return trace.Wrap(err)
		}
		existingMember, err := a.memberService.GetResource(ctx, member.GetName())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		keepAWSIdentityCenterLabels(existingMember, member)

		if err := accesslists.ValidateAccessListMember(ctx, memberList, member, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
			return trace.Wrap(err)
		}

		upserted, err = a.memberService.WithPrefix(member.Spec.AccessList).UpsertResource(ctx, member)

		if err == nil && member.Spec.MembershipKind == accesslist.MembershipKindList {
			if err := a.updateAccessListMemberOf(ctx, member.Spec.AccessList, member.Spec.Name, true); err != nil {
				return trace.Wrap(err)
			}
		}

		return trace.Wrap(err)
	}

	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(member.Spec.AccessList), accessListLockTTL, action)
	})
	return upserted, trace.Wrap(err)
}

// UpdateAccessListMember conditionally updates an access list member resource.
func (a *AccessListService) UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	var updated *accesslist.AccessListMember
	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(member.Spec.AccessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			memberList, err := a.service.GetResource(ctx, member.Spec.AccessList)
			if err != nil {
				return trace.Wrap(err)
			}
			existingMember, err := a.memberService.GetResource(ctx, member.GetName())
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			keepAWSIdentityCenterLabels(existingMember, member)

			if err := accesslists.ValidateAccessListMember(ctx, memberList, member, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
				return trace.Wrap(err)
			}

			updated, err = a.memberService.WithPrefix(member.Spec.AccessList).ConditionalUpdateResource(ctx, member)
			return trace.Wrap(err)
		})
	})
	return updated, trace.Wrap(err)
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (a *AccessListService) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			_, err := a.service.GetResource(ctx, accessList)
			if err != nil {
				return trace.Wrap(err)
			}

			member, err := a.memberService.WithPrefix(accessList).GetResource(ctx, memberName)
			if err != nil {
				return trace.Wrap(err)
			}

			if err := a.memberService.WithPrefix(accessList).DeleteResource(ctx, memberName); err != nil {
				return trace.Wrap(err)
			}

			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListMemberOf(ctx, accessList, memberName, false); err != nil {
					return trace.Wrap(err)
				}
			}

			return nil
		})
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members
// for an access list. Note that deleting all members is the only member operation
// allowed on a list with implicit membership, as it provides a mechanism for
// cleaning out the user list if a list is converted from explicit to implicit.
func (a *AccessListService) DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error {
	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			_, err := a.service.GetResource(ctx, accessList)
			if err != nil {
				return trace.Wrap(err)
			}

			allMembers, err := a.memberService.WithPrefix(accessList).GetResources(ctx)
			if err != nil {
				return trace.Wrap(err)
			}

			if err := a.memberService.WithPrefix(accessList).DeleteAllResources(ctx); err != nil {
				return trace.Wrap(err)
			}

			for _, member := range allMembers {
				if member.Spec.MembershipKind != accesslist.MembershipKindList {
					continue
				}
				if err := a.updateAccessListMemberOf(ctx, accessList, member.Spec.Name, false); err != nil {
					return trace.Wrap(err)
				}
			}

			return nil
		})
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembers hard deletes all access list members.
func (a *AccessListService) DeleteAllAccessListMembers(ctx context.Context) error {
	// Locks are not used here as this operation is more likely to be used by the cache.
	return trace.Wrap(a.memberService.DeleteAllResources(ctx))
}

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (a *AccessListService) UpsertAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for _, m := range membersIn {
		if err := m.CheckAndSetDefaults(); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	validateAccessList := func() error {
		existingList, err := a.service.GetResource(ctx, accessList.GetName())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if existingList != nil {
			accessList.Status.MemberOf = existingList.Status.MemberOf
			accessList.Status.OwnerOf = existingList.Status.OwnerOf
		} else {
			// In case the MemberOf/OwnerOf fields were manually changed, set to empty.
			accessList.Status.MemberOf = []string{}
			accessList.Status.OwnerOf = []string{}
		}

		if err := accesslists.ValidateAccessListWithMembers(ctx, accessList, membersIn, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	reconcileMembers := func() error {
		// Convert the members slice to a map for easier lookup.
		membersMap := utils.FromSlice(membersIn, types.GetName)

		var (
			members      []*accesslist.AccessListMember
			membersToken string
		)

		for {
			// List all members for the access list.
			var err error
			members, membersToken, err = a.memberService.WithPrefix(accessList.GetName()).ListResources(ctx, 0 /* default size */, membersToken)
			if err != nil {
				return trace.Wrap(err)
			}

			for _, existingMember := range members {
				// If the member is not in the new members map (request), delete it.
				if newMember, ok := membersMap[existingMember.GetName()]; !ok {
					err = a.memberService.WithPrefix(accessList.GetName()).DeleteResource(ctx, existingMember.GetName())
					if err != nil {
						return trace.Wrap(err)
					}
					// Update memberOf field if nested list.
					if existingMember.Spec.MembershipKind == accesslist.MembershipKindList {
						if err := a.updateAccessListMemberOf(ctx, accessList.GetName(), existingMember.GetName(), false); err != nil {
							return trace.Wrap(err)
						}
					}
				} else {
					// Preserve the membership metadata for any existing members
					// to suppress member records flipping back and forth due
					// due SCIM pushes or Sync Service updates.
					if !existingMember.Spec.Expires.IsZero() {
						newMember.Spec.Expires = existingMember.Spec.Expires
					}
					if existingMember.Spec.Reason != "" {
						newMember.Spec.Reason = existingMember.Spec.Reason
					}
					keepAWSIdentityCenterLabels(existingMember, newMember)
					newMember.Spec.AddedBy = existingMember.Spec.AddedBy

					// Compare members and update if necessary.
					if !cmp.Equal(newMember, existingMember) {
						// Update the member.
						upserted, err := a.memberService.WithPrefix(accessList.GetName()).UpsertResource(ctx, newMember)
						if err != nil {
							return trace.Wrap(err)
						}

						existingMember.SetRevision(upserted.GetRevision())
					}
				}

				// Remove the member from the map.
				delete(membersMap, existingMember.GetName())
			}

			if membersToken == "" {
				break
			}
		}

		// Add any remaining members to the access list.
		for _, member := range membersMap {
			upserted, err := a.memberService.WithPrefix(accessList.GetName()).UpsertResource(ctx, member)
			if err != nil {
				return trace.Wrap(err)
			}
			// Update memberOf field if nested list.
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListMemberOf(ctx, accessList.GetName(), member.Spec.Name, true); err != nil {
					return trace.Wrap(err)
				}
			}
			member.SetRevision(upserted.GetRevision())
		}

		return nil
	}

	reconcileOwners := func() error {
		// update references for new owners
		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListOwnerOf(ctx, accessList.GetName(), owner.Name, true); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		return nil
	}

	updateAccessList := func() error {
		var err error
		accessList, err = a.service.UpsertResource(ctx, accessList)
		return trace.Wrap(err)
	}

	var actions []func() error

	// If IGS is not enabled for this cluster we need to wrap the whole update and
	// member reconciliation in *another* lock so that we can accurately count the
	// access lists in the cluster in order to  prevent un-authorized use of the
	// AccessList feature
	if !modules.GetModules().Features().GetEntitlement(entitlements.Identity).Enabled {
		actions = append(actions, func() error { return a.VerifyAccessListCreateLimit(ctx, accessList.GetName()) })
	}

	actions = append(actions, validateAccessList, reconcileMembers, updateAccessList, reconcileOwners)

	if err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(accessList.GetName()), 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			for _, action := range actions {
				if err := action(); err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		})
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return accessList, membersIn, nil
}

func (a *AccessListService) AccessRequestPromote(_ context.Context, _ *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error) {
	return nil, trace.NotImplemented("AccessRequestPromote should not be called")
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (a *AccessListService) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	err = a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		reviews, nextToken, err = a.reviewService.WithPrefix(accessList).ListResources(ctx, pageSize, pageToken)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return reviews, nextToken, nil
}

// ListAllAccessListReviews will list access list reviews for all access lists.
func (a *AccessListService) ListAllAccessListReviews(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
	reviews, next, err := a.reviewService.ListResourcesReturnNextResource(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	var nextKey string
	if next != nil {
		nextKey = (*next).Spec.AccessList + string(backend.Separator) + (*next).Metadata.Name
	}
	return reviews, nextKey, nil
}

// CreateAccessListReview will create a new review for an access list.
func (a *AccessListService) CreateAccessListReview(ctx context.Context, review *accesslist.Review) (*accesslist.Review, time.Time, error) {
	reviewName := uuid.New().String()
	createdReview, err := accesslist.NewReview(header.Metadata{
		Name:        reviewName,
		Labels:      review.GetAllLabels(),
		Description: review.Metadata.Description,
		Expires:     review.Expiry(),
	}, accesslist.ReviewSpec{
		AccessList: review.Spec.AccessList,
		Reviewers:  review.Spec.Reviewers,
		ReviewDate: review.Spec.ReviewDate,
		Notes:      review.Spec.Notes,
		Changes:    review.Spec.Changes,
	})
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	var nextAuditDate time.Time

	err = a.service.RunWhileLocked(ctx, lockName(review.Spec.AccessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		accessList, err := a.service.GetResource(ctx, review.Spec.AccessList)
		if err != nil {
			return trace.Wrap(err)
		}

		if createdReview.Spec.Changes.MembershipRequirementsChanged != nil {
			if accessListRequiresEqual(*createdReview.Spec.Changes.MembershipRequirementsChanged, accessList.Spec.MembershipRequires) {
				createdReview.Spec.Changes.MembershipRequirementsChanged = nil
			} else {
				accessList.Spec.MembershipRequires = *review.Spec.Changes.MembershipRequirementsChanged
			}
		}

		if createdReview.Spec.Changes.ReviewFrequencyChanged != 0 {
			if createdReview.Spec.Changes.ReviewFrequencyChanged == accessList.Spec.Audit.Recurrence.Frequency {
				createdReview.Spec.Changes.ReviewFrequencyChanged = 0
			} else {
				accessList.Spec.Audit.Recurrence.Frequency = review.Spec.Changes.ReviewFrequencyChanged
			}
		}

		if createdReview.Spec.Changes.ReviewDayOfMonthChanged != 0 {
			if createdReview.Spec.Changes.ReviewDayOfMonthChanged == accessList.Spec.Audit.Recurrence.DayOfMonth {
				createdReview.Spec.Changes.ReviewDayOfMonthChanged = 0
			} else {
				accessList.Spec.Audit.Recurrence.DayOfMonth = review.Spec.Changes.ReviewDayOfMonthChanged
			}
		}

		createdReview, err = a.reviewService.WithPrefix(review.Spec.AccessList).CreateResource(ctx, createdReview)
		if err != nil {
			return trace.Wrap(err)
		}

		nextAuditDate = accessList.SelectNextReviewDate()
		accessList.Spec.Audit.NextAuditDate = nextAuditDate

		for _, removedMember := range review.Spec.Changes.RemovedMembers {
			if err := a.memberService.WithPrefix(review.Spec.AccessList).DeleteResource(ctx, removedMember); err != nil {
				return trace.Wrap(err)
			}
		}

		if _, err := a.service.UpdateResource(ctx, accessList); err != nil {
			return trace.Wrap(err, "updating audit date in access list")
		}

		return nil
	})
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	return createdReview, nextAuditDate, nil
}

// accessListRequiresEqual returns true if two access lists are equal.
func accessListRequiresEqual(a, b accesslist.Requires) bool {
	// Check roles and traits length.
	if len(a.Roles) != len(b.Roles) {
		return false
	}
	if len(a.Traits) != len(b.Traits) {
		return false
	}

	// Make sure roles are equal.
	for i, role := range a.Roles {
		if b.Roles[i] != role {
			return false
		}
	}

	// Make sure traits are equal.
	for key, vals := range a.Traits {
		bVals, ok := b.Traits[key]
		if !ok {
			return false
		}

		if len(bVals) != len(vals) {
			return false
		}

		for i, val := range vals {
			if bVals[i] != val {
				return false
			}
		}
	}

	return true
}

// DeleteAccessListReview will delete an access list review from the backend.
func (a *AccessListService) DeleteAccessListReview(ctx context.Context, accessListName, reviewName string) error {
	err := a.service.RunWhileLocked(ctx, lockName(accessListName), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(a.reviewService.WithPrefix(accessListName).DeleteResource(ctx, reviewName))
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListReviews will delete all access list reviews from all access lists.
func (a *AccessListService) DeleteAllAccessListReviews(ctx context.Context) error {
	// Locks are not used here as these operations are more likely to be used by the cache.
	// Delete all members for all access lists.
	return trace.Wrap(a.reviewService.DeleteAllResources(ctx))
}

func lockName(accessListName string) []string {
	return []string{accessListPrefix, accessListName}
}

// VerifyAccessListCreateLimit ensures creating access list is limited to no more than 1 (updating is allowed).
// It differentiates request for `creating` and `updating` by checking to see if the request
// access list name matches the ones we retrieved.
// Returns error if limit has been reached.
func (a *AccessListService) VerifyAccessListCreateLimit(ctx context.Context, targetAccessListName string) error {
	f := modules.GetModules().Features()
	if f.GetEntitlement(entitlements.Identity).Enabled {
		return nil // unlimited
	}

	lists, err := a.service.GetResources(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// We are *always* allowed to create at least one AccessLists in order to
	// demonstrate the functionality.
	// TODO(tcsc): replace with a default OSS entitlement of 1
	if len(lists) == 0 {
		return nil
	}

	// Iterate through fetched lists, to check if the request was
	// an update, which is allowed.
	for _, list := range lists {
		if list.GetName() == targetAccessListName {
			return nil
		}
	}

	accessListEntitlement := f.GetEntitlement(entitlements.AccessLists)
	if accessListEntitlement.UnderLimit(len(lists)) {
		return nil
	}

	const limitReachedMessage = "cluster has reached its limit for creating access lists, please contact the cluster administrator"
	return trace.AccessDenied("%s", limitReachedMessage)
}

// keepAWSIdentityCenterLabels preserves member labels if
// it originated from AWS Identity Center plugin.
// The Web UI does not currently preserve metadata labels so this function should be called
// in every update/upsert member calls.
// Remove this function once https://github.com/gravitational/teleport.e/issues/5415 is addressed.
func keepAWSIdentityCenterLabels(old, new *accesslist.AccessListMember) {
	if old == nil || new == nil {
		return
	}
	if old.Origin() == common.OriginAWSIdentityCenter {
		new.Metadata.Labels = old.GetAllLabels()
	}
}
