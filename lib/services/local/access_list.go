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
	"iter"
	"maps"
	"slices"
	"strings"
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
	"github.com/gravitational/teleport/lib/itertools/stream"
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
	backend       backend.Backend
	modules       modules.Modules
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

// GetAccessListMember returns the specified access list member resource.
// If a user is not directly a member of the access list the NotFound error is returned.
func (s *accessListAndMembersGetter) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	return s.memberService.WithPrefix(accessListName).GetResource(ctx, memberName)
}

// compile-time assertion that the AccessListService implements the AccessLists
// interface
var _ services.AccessLists = (*AccessListService)(nil)

// AccessListServiceConfig contains dependencies required to construct
// an AccessListService.
type AccessListServiceConfig struct {
	// Backend is the persistent storage mechanism.
	Backend backend.Backend
	// Modules specifies which AccessList features are enabled.
	Modules modules.Modules
	// RunWhileLockedRetryInterval alters locking behavior when interacting with the backend.
	// This allows tests to run faster.
	RunWhileLockedRetryInterval time.Duration
}

// NewAccessListServiceV2 creates a new AccessListService.
func NewAccessListServiceV2(cfg AccessListServiceConfig) (*AccessListService, error) {
	if cfg.Modules == nil {
		return nil, trace.BadParameter("Modules are a required parameter for the AccessListService")
	}

	service, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessList]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListMaxPageSize,
		ResourceKind:                types.KindAccessList,
		BackendPrefix:               backend.NewKey(accessListPrefix),
		MarshalFunc:                 services.MarshalAccessList,
		UnmarshalFunc:               services.UnmarshalAccessList,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberService, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessListMember]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListMemberMaxPageSize,
		ResourceKind:                types.KindAccessListMember,
		BackendPrefix:               backend.NewKey(accessListMemberPrefix),
		MarshalFunc:                 services.MarshalAccessListMember,
		UnmarshalFunc:               services.UnmarshalAccessListMember,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviewService, err := generic.NewService(&generic.ServiceConfig[*accesslist.Review]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListReviewMaxPageSize,
		ResourceKind:                types.KindAccessListReview,
		BackendPrefix:               backend.NewKey(accessListReviewPrefix),
		MarshalFunc:                 services.MarshalAccessListReview,
		UnmarshalFunc:               services.UnmarshalAccessListReview,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		backend:       cfg.Backend,
		modules:       cfg.Modules,
		service:       service,
		memberService: memberService,
		reviewService: reviewService,
	}, nil
}

// NewAccessListService creates a new AccessListService.
// Deprecated: Prefer using NewAccessListServiceV2
// TODO(tross): Delete when everything is using V2.
func NewAccessListService(b backend.Backend, clock clockwork.Clock, opts ...ServiceOption) (*AccessListService, error) {
	var opt serviceOptions
	for _, o := range opts {
		o(&opt)
	}

	return NewAccessListServiceV2(AccessListServiceConfig{
		Backend:                     b,
		Modules:                     modules.GetModules(),
		RunWhileLockedRetryInterval: opt.runWhileLockedRetryInterval,
	})
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

// ListAccessListsV2 returns a filtered and sorted paginated list of access lists.
func (a *AccessListService) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	// Currently, the backend only sorts on lexicographical keys and not
	// based on fields within a resource
	if req.SortBy != nil && (req.GetSortBy().Field != "name" || req.GetSortBy().IsDesc != false) {
		return nil, "", trace.CompareFailed("unsupported sort, only name:asc is supported, but got %q (desc = %t)", req.GetSortBy().Field, req.GetSortBy().IsDesc)
	}

	return a.service.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), func(item *accesslist.AccessList) bool {
		return services.MatchAccessList(item, req.GetFilter())
	})
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return a.service.GetResource(ctx, name)
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
	var existingAccessList *accesslist.AccessList

	opFn := a.service.UpsertResource
	if op == opTypeUpdate {
		opFn = a.service.ConditionalUpdateResource
	}

	validateAccessList := func() error {
		var err error

		existingAccessList, err = a.service.GetResource(ctx, accessList.GetName())
		if op == opTypeUpsert && trace.IsNotFound(err) {
			// Not having already existing access_list in the backend is ok in case of
			// upsert.
		} else if err != nil {
			return trace.Wrap(err)
		}
		preserveAccessListFields(existingAccessList, accessList)
		listMembers, err := a.memberService.WithPrefix(accessList.GetName()).GetResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		return accesslists.ValidateAccessListWithMembers(ctx, existingAccessList, accessList, listMembers, &accessListAndMembersGetter{a.service, a.memberService})
	}

	updateAccessList := func() error {
		var err error
		upserted, err = opFn(ctx, accessList)
		return trace.Wrap(err)
	}

	reconcileOwners := func() error {
		currentOwnersMap := make(map[string]struct{})
		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				currentOwnersMap[owner.Name] = struct{}{}
			}
		}

		// update references for new owners
		for ownerName := range currentOwnersMap {
			if err := a.updateAccessListOwnerOf(ctx, accessList.GetName(), ownerName, true); err != nil {
				return trace.Wrap(err)
			}
		}

		// update references for old owners
		if existingAccessList != nil {
			for _, owner := range existingAccessList.Spec.Owners {
				if owner.MembershipKind != accesslist.MembershipKindList {
					continue
				}
				// If this owner access list is not an owner anymore after the
				// update/upsert, its status.owner_of has to be updated.
				if _, exists := currentOwnersMap[owner.Name]; !exists {
					if err := a.updateAccessListOwnerOf(ctx, accessList.GetName(), owner.Name, false); err != nil {
						return trace.Wrap(err)
					}
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
	if !a.modules.Features().GetEntitlement(entitlements.Identity).Enabled {
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

		// Check if the Access List has any blocking relationships.
		if err := a.checkDeletionBlockingRelationships(ctx, accessList); err != nil {
			return trace.Wrap(err)
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

// CountAccessListMembers will count all access list members.
func (a *AccessListService) CountAccessListMembers(ctx context.Context, accessListName string) (users uint32, lists uint32, err error) {
	count := uint(0)
	listCount := uint(0)
	members, err := a.memberService.WithPrefix(accessListName).GetResources(ctx)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	for _, member := range members {
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			listCount++
		} else {
			count++
		}
	}

	return uint32(count), uint32(listCount), trace.Wrap(err)
}

// ListAccessListMembers returns a paginated list of all access list members.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	_, err := a.service.GetResource(ctx, accessListName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	members, nextToken, err := a.memberService.WithPrefix(accessListName).ListResources(ctx, pageSize, nextToken)

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
	_, err := a.service.GetResource(ctx, accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	member, err := a.memberService.WithPrefix(accessList).GetResource(ctx, memberName)
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
	accessList, err := a.service.GetResource(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	owners, err = accesslists.GetOwnersFor(ctx, accessList, &accessListAndMembersGetter{a.service, a.memberService})
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
		if err != nil {
			return trace.Wrap(err)
		}

		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			if err := a.updateAccessListMemberOf(ctx, member.Spec.AccessList, member.Spec.Name, true); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}

	// without this check creating the lock may fail with "special characters are not allowed in resource names"
	if member.Spec.AccessList == "" {
		return nil, trace.BadParameter("access_list_member %s: spec.access_list field empty", member.GetName())
	}
	err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(member.Spec.AccessList), accessListLockTTL, action)
	})
	return upserted, trace.Wrap(err)
}

// UpdateAccessListMember conditionally updates an access list member resource.
func (a *AccessListService) UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	var updated *accesslist.AccessListMember
	// without this check creating the lock may fail with "special characters are not allowed in resource names"
	if member.Spec.AccessList == "" {
		return nil, trace.BadParameter("access_list_member %s: spec.access_list field empty", member.GetName())
	}
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
			if err != nil {
				return trace.Wrap(err)
			}

			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListMemberOf(ctx, member.Spec.AccessList, member.Spec.Name, true); err != nil {
					return trace.Wrap(err)
				}
			}

			return nil
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

type writeFn func(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)

func (a *AccessListService) selectWriteFn(op opType) (writeFn, error) {
	switch op {
	case opTypeUpdate:
		return a.service.ConditionalUpdateResource, nil

	case opTypeUpsert:
		return a.service.UpsertResource, nil
	}

	return nil, trace.BadParameter("Unknown Access List write operation: %d", op)
}

// writeAccessListWithMembers holds all of the common logic for updating and
// upserting an access list and it's collection of members.
func (a *AccessListService) writeAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember, op opType) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	writeFn, err := a.selectWriteFn(op)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for _, m := range membersIn {
		if err := m.CheckAndSetDefaults(); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	validateAccessList := func() error {
		existingAccessList, err := a.service.GetResource(ctx, accessList.GetName())
		if err != nil {
			// a not found error is totally legal for an upsert operation, but
			// fatal for an update.
			if op == opTypeUpdate || !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		if op == opTypeUpdate {
			if accessList.Metadata.Revision != existingAccessList.Metadata.Revision {
				return trace.CompareFailed("access list revision does not match. it may have been concurrently modified")
			}
		}

		preserveAccessListFields(existingAccessList, accessList)

		if err := accesslists.ValidateAccessListWithMembers(ctx, existingAccessList, accessList, membersIn, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
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

		if err := a.insertMembersAndUpdateNestedRelationships(ctx, slices.Collect(maps.Values(membersMap))); err != nil {
			return trace.Wrap(err)
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

	writeAccessList := func() error {
		var err error
		accessList, err = writeFn(ctx, accessList)
		return trace.Wrap(err)
	}

	var actions []func() error

	// If IGS is not enabled for this cluster we need to wrap the whole update and
	// member reconciliation in *another* lock so that we can accurately count the
	// access lists in the cluster in order to  prevent un-authorized use of the
	// AccessList feature
	if !a.modules.Features().GetEntitlement(entitlements.Identity).Enabled {
		actions = append(actions, func() error { return a.VerifyAccessListCreateLimit(ctx, accessList.GetName()) })
	}

	actions = append(actions, validateAccessList, reconcileMembers, writeAccessList, reconcileOwners)

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

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (a *AccessListService) UpsertAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	upsertedACL, upsertedMembers, err := a.writeAccessListWithMembers(ctx, accessList, membersIn, opTypeUpsert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return upsertedACL, upsertedMembers, nil
}

// UpdateAccessListAndOverwriteMembers does a conditional update on an AccessList and
// all its members. For the purposes of this update, the Access List's member
// records  are covered under the enclosing Access List's revision.
func (a *AccessListService) UpdateAccessListAndOverwriteMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	updatedACL, udatedMembers, err := a.writeAccessListWithMembers(ctx, accessList, membersIn, opTypeUpdate)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return updatedACL, udatedMembers, nil
}

func (a *AccessListService) AccessRequestPromote(_ context.Context, _ *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error) {
	return nil, trace.NotImplemented("AccessRequestPromote should not be called")
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (a *AccessListService) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	_, err = a.service.GetResource(ctx, accessList)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	reviews, nextToken, err = a.reviewService.WithPrefix(accessList).ListResources(ctx, pageSize, pageToken)
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

		if !accessList.IsReviewable() {
			return trace.BadParameter("access_list %q is not reviewable", accessList.GetName())
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

		nextAuditDate, err = accessList.SelectNextReviewDate()
		if err != nil {
			return trace.Wrap(err, "selecting next review date")
		}
		accessList.Spec.Audit.NextAuditDate = nextAuditDate

		for _, removedMember := range review.Spec.Changes.RemovedMembers {
			_, err := a.memberService.WithPrefix(review.Spec.AccessList).GetResource(ctx, removedMember)
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			isAccessListMember := err == nil

			if isAccessListMember {
				if err := a.updateAccessListMemberOf(ctx, review.Spec.AccessList, removedMember, false); err != nil {
					return trace.Wrap(err)
				}
			}
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
	_, err := a.service.GetResource(ctx, accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.reviewService.WithPrefix(accessListName).DeleteResource(ctx, reviewName))
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
	f := a.modules.Features()
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

func preserveAccessListFields(existingAccessList, accessList *accesslist.AccessList) {
	if existingAccessList != nil {
		// Set MemberOf/OwnerOf to the existing values to prevent them from being updated.
		accessList.Status.MemberOf = existingAccessList.Status.MemberOf
		accessList.Status.OwnerOf = existingAccessList.Status.OwnerOf
	} else {
		// For newly created AccessList make sure MemberOf/OwnerOf are empty.
		accessList.Status.MemberOf = []string{}
		accessList.Status.OwnerOf = []string{}
	}
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

// ListUserAccessLists is not implemented in the local service.
func (a *AccessListService) ListUserAccessLists(ctx context.Context, req *accesslistv1.ListUserAccessListsRequest) ([]*accesslist.AccessList, string, error) {
	return nil, "", trace.NotImplemented("ListUserAccessLists should not be called on local service")
}

func (a *AccessListService) insertMembersAndUpdateNestedRelationships(ctx context.Context, members []*accesslist.AccessListMember) error {
	if err := a.insertMembers(ctx, members); err != nil {
		return trace.Wrap(err)
	}
	// In case of nested access list members.
	if err := a.updatedMembersNestedRelationships(ctx, members); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *AccessListService) insertMembers(ctx context.Context, members []*accesslist.AccessListMember) error {
	items, err := a.membersToBackendItems(members)
	if err != nil {
		return trace.Wrap(err)
	}

	revs, err := backend.PutBatch(ctx, a.backend, items)
	if err != nil {
		return trace.Wrap(err)
	}
	for i, rev := range revs {
		members[i].SetRevision(rev)
	}
	return nil
}

func (a *AccessListService) membersToBackendItems(members []*accesslist.AccessListMember) ([]backend.Item, error) {
	out := make([]backend.Item, 0, len(members))
	for _, member := range members {
		item, err := a.memberService.WithPrefix(member.Spec.AccessList).MakeBackendItem(member)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *AccessListService) updatedMembersNestedRelationships(ctx context.Context, members []*accesslist.AccessListMember) error {
	for _, member := range members {
		if member.Spec.MembershipKind != accesslist.MembershipKindList {
			continue
		}
		// Update memberOf field if nested list.
		if err := a.updateAccessListMemberOf(ctx, member.Spec.AccessList, member.Spec.Name, true); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CleanupAccessListStatus removes invalid Status.OwnerOf and Status.MemberOf references.
func (a *AccessListService) CleanupAccessListStatus(ctx context.Context, accessListName string) (*accesslist.AccessList, error) {
	return a.runWithGlobalLockAccessList(ctx, accessListName, func() (*accesslist.AccessList, error) {
		accessList, err := a.service.GetResource(ctx, accessListName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var ownerRefreshErr error
		accessList.Status.OwnerOf = slices.DeleteFunc(accessList.Status.OwnerOf, func(ownerOf string) bool {
			ownedList, err := a.service.GetResource(ctx, ownerOf)
			if err != nil {
				if trace.IsNotFound(err) {
					return true
				}
				ownerRefreshErr = err
				return false
			}
			isActualOwner := slices.ContainsFunc(ownedList.Spec.Owners, func(ownedListOwner accesslist.Owner) bool {
				return ownedListOwner.MembershipKind == accesslist.MembershipKindList && ownedListOwner.Name == accessList.GetName()
			})
			return !isActualOwner
		})
		if ownerRefreshErr != nil {
			return nil, trace.Wrap(ownerRefreshErr)
		}

		var memberRefreshErr error
		accessList.Status.MemberOf = slices.DeleteFunc(accessList.Status.MemberOf, func(memberOf string) bool {
			if _, err := a.memberService.WithPrefix(memberOf).GetResource(ctx, accessList.GetName()); err != nil {
				if trace.IsNotFound(err) {
					return true
				}
				memberRefreshErr = err
			}
			return false
		})
		if memberRefreshErr != nil {
			return nil, trace.Wrap(memberRefreshErr)
		}

		accessList, err = a.service.UpdateResource(ctx, accessList)
		return accessList, trace.Wrap(err)
	})
}

// EnsureNestedAccessListStatuses goes over all nested owners and nested members of the named
// access list and ensures nested lists' statuses owner_of/member_of contain the access list name.
func (a *AccessListService) EnsureNestedAccessListStatuses(ctx context.Context, accessListName string) error {
	return a.runWithGlobalLock(ctx, accessListName, func() error {
		accessList, err := a.service.GetResource(ctx, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListOwnerOf(ctx, accessListName, owner.Name, true); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		members, err := a.memberService.WithPrefix(accessListName).GetResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, member := range members {
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				if err := a.updateAccessListMemberOf(ctx, accessListName, member.GetName(), true); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		return nil
	})
}

// InsertAccessListCollection inserts a complete collection of access lists and their members from a single
// upstream source (e.g. EntraID) using a batch operation for improved performance.
//
// This method is designed for bulk import scenarios where an entire access list collection needs to be
// synchronized from an external source. All access lists and members in the collection are
// inserted using chunked batch operations, minimizing memory allocation while still reducing
// the number of write operations. Due to the batch nature of this operation (access list hierarchy
// is known upfront), we can avoid per-access-list locking and global locks to improve performance.
//
// Important: This method assumes the collection is self-contained. Access lists in the collection
// cannot reference access lists outside the collection as members or owners. This is intentional for
// collections representing a complete snapshot from a single upstream source.
// The function should be used only once during initial import where
// we are sure that Teleport doesn't have any pre-existing access lists from the upstream and the
// internal relation between upstream access lists and internal access lists doesn't exist yet.
//
// Operation can fail due to backend shutdown. In that case, if partial state was created,
// use UpsertAccessListWithMembers/DeleteAccessListMember to reconcile to the desired state.
func (a *AccessListService) InsertAccessListCollection(ctx context.Context, collection *accesslists.Collection) error {
	if err := collection.Validate(ctx); err != nil {
		return trace.Wrap(err)
	}
	// Collect backend items in chunks of 800 items each to avoid high memory consumption
	// from constructing a large slice of all backend items at once. PutBatch will then
	// leverage its own internal chunking to write items to the backend in smaller batches.
	// TODO(smallinsky) align the chunk size with the one used in backend.PutBatch
	for chunk, err := range stream.Chunks(a.collectionToBackendItemsIter(collection), 800) {
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := backend.PutBatch(ctx, a.backend, chunk); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// collectionToBackendItemsIter converts access list collection to an iterator of backend items.
// The iterator yields access list members first, followed by their parent access list.
func (a *AccessListService) collectionToBackendItemsIter(collection *accesslists.Collection) iter.Seq2[backend.Item, error] {
	return func(yield func(backend.Item, error) bool) {
		for aclName, members := range collection.MembersByAccessList {
			acl, ok := collection.AccessListsByName[aclName]
			if !ok {
				yield(backend.Item{}, trace.NotFound("access list %q not found", aclName))
				return
			}
			for _, member := range members {
				item, err := a.memberService.WithPrefix(member.Spec.AccessList).MakeBackendItem(member)
				if err != nil {
					yield(backend.Item{}, trace.Wrap(err))
					return
				}
				if !yield(item, nil) {
					return
				}
			}
			item, err := a.service.MakeBackendItem(acl)
			if err != nil {
				yield(backend.Item{}, trace.Wrap(err))
				return
			}
			if !yield(item, nil) {
				return
			}
		}
	}
}

func (a *AccessListService) runWithGlobalLock(ctx context.Context, accessListName string, fn func() error) error {
	return a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName(accessListName), 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			return trace.Wrap(fn())
		})
	})
}

func (a *AccessListService) runWithGlobalLockAccessList(ctx context.Context, accessListName string, fn func() (*accesslist.AccessList, error)) (*accesslist.AccessList, error) {
	var res *accesslist.AccessList
	err := a.runWithGlobalLock(ctx, accessListName, func() error {
		var err error
		res, err = fn()
		return trace.Wrap(err)
	})
	return res, err
}

// checkDeletionBlockingRelationships checks if the access list has any relationships that would block deletion
func (a *AccessListService) checkDeletionBlockingRelationships(ctx context.Context, accessList *accesslist.AccessList) error {
	if err := a.checkDeletionBlockingMemberRelationships(ctx, accessList); err != nil {
		return trace.Wrap(err)
	}
	if err := a.checkDeletionBlockingOwnerRelationships(ctx, accessList); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkDeletionBlockingMemberRelationships checks if the access list is a member of any other access lists that would block deletion
func (a *AccessListService) checkDeletionBlockingMemberRelationships(ctx context.Context, accessList *accesslist.AccessList) error {
	memberOf := accessList.GetStatus().MemberOf
	if len(memberOf) == 0 {
		return nil
	}

	memberOfTitles := make([]string, 0, len(memberOf))
	for _, memberOf := range memberOf {
		parentList, err := a.service.GetResource(ctx, memberOf)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching parent list "%s"`, memberOf)
		}
		member, err := a.memberService.WithPrefix(parentList.GetName()).GetResource(ctx, accessList.GetName())
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching access list member for "%s"`, memberOf)
		}
		if member.Spec.MembershipKind == accesslist.MembershipKindList {
			memberOfTitles = append(memberOfTitles, parentList.Spec.Title)
		}
	}

	if len(memberOfTitles) > 0 {
		return trace.AccessDenied(`Cannot delete "%s", as it is a member of Access Lists: %s`,
			accessList.Spec.Title, quoteAndJoin(memberOfTitles))
	}

	return nil
}

// checkDeletionBlockingOwnerRelationships checks if the access list owns any other access lists that would block deletion
func (a *AccessListService) checkDeletionBlockingOwnerRelationships(ctx context.Context, accessList *accesslist.AccessList) error {
	ownerOf := accessList.GetStatus().OwnerOf
	if len(ownerOf) == 0 {
		return nil
	}

	ownerOfTitles := make([]string, 0, len(ownerOf))
	for _, ownerOf := range ownerOf {
		ownedList, err := a.service.GetResource(ctx, ownerOf)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching owned list "%s"`, ownerOf)
		}
		isActualOwner := false
		for _, owner := range ownedList.Spec.Owners {
			if owner.Name == accessList.GetName() && owner.MembershipKind == accesslist.MembershipKindList {
				isActualOwner = true
				break
			}
		}
		if isActualOwner {
			ownerOfTitles = append(ownerOfTitles, ownedList.Spec.Title)
		}
	}

	if len(ownerOfTitles) > 0 {
		return trace.AccessDenied(`Cannot delete "%s", as it is an owner of Access Lists: %s`,
			accessList.Spec.Title, quoteAndJoin(ownerOfTitles))
	}

	return nil
}

// quoteAndJoin takes a slice of strings and returns them quoted and comma-separated
func quoteAndJoin(items []string) string {
	if len(items) == 0 {
		return ""
	}
	quotedItems := make([]string, len(items))
	for i, item := range items {
		quotedItems[i] = `"` + item + `"`
	}
	return strings.Join(quotedItems, ", ")
}
