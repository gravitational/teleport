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
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

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
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (
	scopedPrefix = "scoped"

	// Access lists are stored under one of the following key ranges, depending
	// on their scope:
	// - /access_list/<list-name>                             (for unscoped lists)
	// - /scoped/access_list/<encoded-list-scope>/<list-name> (for scoped lists)
	accessListPrefix      = "access_list"
	accessListMaxPageSize = 100

	// Access list members are stored under one of the following key ranges,
	// depending on the scope of the parent list:
	// - /access_list_member/<list-name>/<member-name>                                                    (for unscoped lists)
	// - /scoped/access_list_member/<encoded-list-scope>/<list-name>/<encoded-member-scope>/<member-name> (for scoped lists)
	accessListMemberPrefix      = "access_list_member"
	accessListMemberMaxPageSize = 200

	// Access list reviews are stored under one of the following key ranges,
	// depending on the scope of the reviewed list:
	// - /access_list_review/<list-name>/<review-id>                             (for unscoped lists)
	// - /scoped/access_list_review/<encoded-list-scope>/<list-name>/<review-id> (for scoped lists)
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
	backend backend.Backend
	modules modules.Modules
	// scopesFeatures dictates whether scoped role grants are enabled.
	scopesFeatures scopes.Features
	service        *generic.ScopeAwareService[*accesslist.AccessList]
	memberService  *generic.ScopeAwareService[*accesslist.AccessListMember]
	reviewService  *generic.ScopeAwareService[*accesslist.Review]
}

type accessListAndMembersGetter struct {
	service       *generic.ScopeAwareService[*accesslist.AccessList]
	memberService *generic.ScopeAwareService[*accesslist.AccessListMember]
}

func (s *accessListAndMembersGetter) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	membersService, err := membersServiceForAccessList(s.memberService, accesslists.NormalizedSQN{
		Name: accessListName,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return membersService.listResources(ctx, pageSize, pageToken)
}

func (s *accessListAndMembersGetter) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	listName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	membersService, err := membersServiceForAccessList(s.memberService, listName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return membersService.listResources(ctx, int(req.GetPageSize()), req.GetPageToken())
}

func (s *accessListAndMembersGetter) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return s.getAccessList(ctx, accesslists.NormalizedSQN{Name: name})
}

func (s *accessListAndMembersGetter) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	return s.getAccessList(ctx, accesslists.NormalizedSQN(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}))
}

func (s *accessListAndMembersGetter) getAccessList(ctx context.Context, listName accesslists.NormalizedSQN) (*accesslist.AccessList, error) {
	return s.service.GetResource(ctx, listName.ToScopesQualifiedName())
}

// GetAccessListMember returns the specified access list member resource.
// If a user is not directly a member of the access list the NotFound error is returned.
func (s *accessListAndMembersGetter) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	memberService, err := memberServiceForNamedMember(s.memberService,
		accesslists.NormalizedSQN{Name: accessListName},
		accesslists.NormalizedSQN{Name: memberName},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return memberService.get(ctx)
}

// GetAccessListMemberV2 returns the specified access list member resource.
// If a user is not directly a member of the access list the NotFound error is returned.
func (s *accessListAndMembersGetter) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	memberName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	})
	memberService, err := memberServiceForNamedMember(s.memberService, accessListName, memberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return memberService.get(ctx)
}

// membersServiceForAccessList returns a [*membersService] valid only for
// operations on _all_ members of the given access list, it cannot be used to
// get, create, or delete any single member, use [memberServiceForNamedMember]
// in those cases.
func membersServiceForAccessList(
	genericService *generic.ScopeAwareService[*accesslist.AccessListMember],
	listName accesslists.NormalizedSQN,
) (*membersService, error) {
	service, err := genericService.WithScopedResourcePrefix(listName.ToScopesQualifiedName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &membersService{service}, nil
}

type membersService struct {
	service *generic.Service[*accesslist.AccessListMember]
}

func (s *membersService) resources(ctx context.Context, startKey, endKey string) iter.Seq2[*accesslist.AccessListMember, error] {
	return s.service.Resources(ctx, startKey, endKey)
}

func (s *membersService) listResources(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return s.service.ListResources(ctx, pageSize, pageToken)
}

func (s *membersService) getResources(ctx context.Context) ([]*accesslist.AccessListMember, error) {
	return s.service.GetResources(ctx)
}

func (s *membersService) deleteAllResources(ctx context.Context) error {
	return s.service.DeleteAllResources(ctx)
}

// memberServiceForNamedMember returns a [*memberService] valid for only the
// exact named member, with an appropriate backend prefix and NameKeyFunc configured.
func memberServiceForNamedMember(
	membersService *generic.ScopeAwareService[*accesslist.AccessListMember],
	listName, memberName accesslists.NormalizedSQN,
) (*memberService, error) {
	service, err := membersService.WithScopedResourcePrefix(listName.ToScopesQualifiedName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if listName.Scope == "" {
		// For unscoped parent lists we need only the following backend prefix,
		// which we will already have after calling WithScopedResourcePrefix.
		// We just need to assert that the member is unscoped.
		// - /access_list_member/<list-name>
		if memberName.Scope != "" {
			return nil, trace.BadParameter("unscoped access lists cannot have scoped members")
		}
	} else {
		// For scoped parent lists we need the following backend prefix, after
		// calling WithScopedResourcePrefix we need only to append the encoded
		// member scope. Even if the member has the empty scope, it must be
		// encoded and included in the key.
		// - /scoped/access_list_member/<encoded-list-scope>/<list-name>/<encoded-member-scope>
		encodedMemberScope, err := scopes.EncodeForKey(memberName.Scope)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		service = service.WithPrefix(encodedMemberScope)
	}

	// For scoped members, the metadata.name is a scope-qualified name.
	// The scope is already encoded in the backend prefix, so we must
	// override the NameKeyFunc to use only the plain name.
	// We do this unconditionally even for unscoped members, so that we can
	// return a [memberService] that operates on a single member without
	// requiring a name parameter that may be inaccurate or misleading.
	service = service.WithNameKeyFunc(func() backend.Key {
		return backend.NewKey(memberName.Name)
	})

	return &memberService{
		service:    service,
		memberName: memberName.String(),
	}, nil
}

// memberService wraps a generic service with an overridden NameKeyFunc so that
// is valid for exactly one access list member. The get and delete methods
// don't accept a name parameter to make it clear that it will not be used.
type memberService struct {
	service    *generic.Service[*accesslist.AccessListMember]
	memberName string
}

func (s *memberService) get(ctx context.Context) (*accesslist.AccessListMember, error) {
	// NameKeyFunc has been set, so the name param does not matter except that
	// it will be shown in error messages.
	return s.service.GetResource(ctx, s.memberName)
}

func (s *memberService) delete(ctx context.Context) error {
	// NameKeyFunc has been set, so the name param does not matter except that
	// it will be shown in error messages.
	return s.service.DeleteResource(ctx, s.memberName)
}

func (s *memberService) makeBackendItem(member *accesslist.AccessListMember) (backend.Item, error) {
	// service.MakeBackendItem respects the overridden NameKeyFunc.
	return s.service.MakeBackendItem(member)
}

func (s *memberService) upsert(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	// service.UpsertResource respects the overridden NameKeyFunc.
	return s.service.UpsertResource(ctx, member)
}

func (s *memberService) conditionalUpdateResource(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	// service.ConditionalUpdateResource respects the overridden NameKeyFunc.
	updated, err := s.service.ConditionalUpdateResource(ctx, member)
	if trace.IsNotFound(err) {
		// Re-write NotFound errors to include the member scope.
		return nil, trace.NotFound("%s %q doesn't exist", types.KindAccessListMember, s.memberName)
	}
	return updated, trace.Wrap(err)
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
	// ScopesFeatures specifies which scopes features are enabled.
	ScopesFeatures scopes.Features
}

// NewAccessListServiceV2 creates a new AccessListService.
func NewAccessListServiceV2(cfg AccessListServiceConfig) (*AccessListService, error) {
	if cfg.Modules == nil {
		return nil, trace.BadParameter("Modules are a required parameter for the AccessListService")
	}

	service, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[*accesslist.AccessList]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListMaxPageSize,
		ResourceKind:                types.KindAccessList,
		UnscopedBackendPrefix:       backend.NewKey(accessListPrefix),
		ScopedBackendPrefix:         backend.NewKey(scopedPrefix, accessListPrefix),
		MarshalFunc:                 services.MarshalAccessList,
		UnmarshalFunc:               services.UnmarshalAccessList,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberService, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[*accesslist.AccessListMember]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListMemberMaxPageSize,
		ResourceKind:                types.KindAccessListMember,
		UnscopedBackendPrefix:       backend.NewKey(accessListMemberPrefix),
		ScopedBackendPrefix:         backend.NewKey(scopedPrefix, accessListMemberPrefix),
		MarshalFunc:                 services.MarshalAccessListMember,
		UnmarshalFunc:               services.UnmarshalAccessListMember,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviewService, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[*accesslist.Review]{
		Backend:                     cfg.Backend,
		PageLimit:                   accessListReviewMaxPageSize,
		ResourceKind:                types.KindAccessListReview,
		UnscopedBackendPrefix:       backend.NewKey(accessListReviewPrefix),
		ScopedBackendPrefix:         backend.NewKey(scopedPrefix, accessListReviewPrefix),
		MarshalFunc:                 services.MarshalAccessListReview,
		UnmarshalFunc:               services.UnmarshalAccessListReview,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		backend:        cfg.Backend,
		modules:        cfg.Modules,
		scopesFeatures: cfg.ScopesFeatures,
		service:        service,
		memberService:  memberService,
		reviewService:  reviewService,
	}, nil
}

// GetAccessLists returns a list of all unscoped access lists.
func (a *AccessListService) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	return a.service.UnscopedService.GetResources(ctx)
}

// GetInheritedGrants returns grants inherited by access list accessListID from parent access lists.
// This is not implemented in the local service.
func (a *AccessListService) GetInheritedGrants(ctx context.Context, accessListID string) (*accesslist.Grants, error) {
	return nil, trace.NotImplemented("GetInheritedGrants should not be called")
}

// ListAccessLists returns a paginated list of unscoped access lists.
func (a *AccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	return a.service.UnscopedService.ListResources(ctx, pageSize, nextToken)
}

// ListAccessListsV2 returns a filtered and sorted paginated list of access lists.
func (a *AccessListService) ListAccessListsV2(ctx context.Context, req *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error) {
	return a.ListMatchingAccessLists(ctx, req)
}

// ListMatchingAccessLists returns a filtered and sorted paginated list of access lists.
// Search term matchers are evaluated in order for terms that do not match stored access list fields.
func (a *AccessListService) ListMatchingAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsV2Request, searchTermMatchers ...services.AccessListSearchTermMatcherFunc) ([]*accesslist.AccessList, string, error) {
	// Currently, the backend only sorts on lexicographical keys and not
	// based on fields within a resource
	if req.HasSortBy() && (req.GetSortBy().Field != "name" || req.GetSortBy().IsDesc) {
		return nil, "", trace.CompareFailed("unsupported sort, only name:asc is supported, but got %q (desc = %t)", req.GetSortBy().Field, req.GetSortBy().IsDesc)
	}

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}

	return a.service.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), func(item *accesslist.AccessList) bool {
		return services.MatchAccessList(item, req.GetFilter(), searchTermMatchers...) && scopes.MatchScope(scopeFilter, item.GetScope())
	})
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return a.getAccessList(ctx, accesslists.NormalizedSQN{Name: name})
}

// GetAccessListV2 returns the specified access list resource, supporting
// scoped or unscoped access lists.
func (a *AccessListService) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	return a.getAccessList(ctx, accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}))
}

func (a *AccessListService) getAccessList(ctx context.Context, listName accesslists.NormalizedSQN) (*accesslist.AccessList, error) {
	return a.service.GetResource(ctx, listName.ToScopesQualifiedName())
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

	membersService, err := a.membersServiceForAccessList(accesslists.ScopeQualifiedName(accessList))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateAccessList := func() error {
		var err error

		existingAccessList, err = a.getAccessList(ctx, accesslists.ScopeQualifiedName(accessList))
		if op == opTypeUpsert && trace.IsNotFound(err) {
			// Not having already existing access_list in the backend is ok in case of
			// upsert.
		} else if err != nil {
			return trace.Wrap(err)
		}
		preserveAccessListFields(existingAccessList, accessList)

		listMembers, err := membersService.getResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := accesslists.ValidateAccessListWithMembers(ctx, existingAccessList, accessList, listMembers, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
			return trace.Wrap(err)
		}

		if err := a.checkScopesFeatures(existingAccessList, accessList); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	reconcileOldOwners := func() error {
		if existingAccessList == nil {
			return nil
		}

		currentOwnersMap := make(map[accesslists.NormalizedSQN]struct{})
		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}

			ownerSQN, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err, "getting scope-qualified name of access list owner")
			}
			currentOwnersMap[ownerSQN] = struct{}{}
		}

		// update references for old owners
		for _, owner := range existingAccessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}

			ownerSQN, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err, "getting scope-qualified name of existing access list owner")
			}

			// If this owner access list is not an owner anymore after the
			// update/upsert, its status.owner_of has to be updated.
			if _, exists := currentOwnersMap[ownerSQN]; !exists {
				if err := a.updateAccessListOwnerOf(ctx, accesslists.ScopeQualifiedName(accessList), ownerSQN, false); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		return nil
	}

	updateAccessList := func() (err error) {
		upserted, err = a.writeAccessList(ctx, accessList, op)
		return trace.Wrap(err)
	}

	reconcileNewOwners := func() error {
		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}
			ownerScopedName, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListOwnerOf(ctx, accesslists.ScopeQualifiedName(accessList), ownerScopedName, true); err != nil {
				return trace.Wrap(err)
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
		actions = append(actions, func() error { return a.verifyAccessListCreateLimit(ctx, accesslists.ScopeQualifiedName(accessList)) })
	}

	// Note we need to reconcile the old owners (clean status.owner_of for the owner lists
	// which are removed with this request) first, then update the access list and then
	// reconcile the new owners (set status.owner_of of the owner lists that are added with
	// this request). This is to make sure the operation doesn't escalate privileges if
	// interrupted as we user status.owner_of to calculate hierarchy.
	actions = append(actions, validateAccessList, reconcileOldOwners, updateAccessList, reconcileNewOwners)

	resourceLockName, err := scopeAwareLockName(accesslists.ScopeQualifiedName(accessList))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL,
		func(ctx context.Context, _ backend.Backend) error {
			return a.service.RunWhileLocked(ctx, resourceLockName, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
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
	return a.DeleteAccessListV2(ctx, accesslistv1.DeleteAccessListRequest_builder{
		Name: name,
	}.Build())
}

// DeleteAccessListV2 removes the specified access list resource, supporting
// scoped or unscoped access lists.
func (a *AccessListService) DeleteAccessListV2(ctx context.Context, req *accesslistv1.DeleteAccessListRequest) error {
	name := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
	membersService, err := a.membersServiceForAccessList(name)
	if err != nil {
		return trace.Wrap(err)
	}

	action := func(ctx context.Context, _ backend.Backend) error {
		// Get list resource.
		accessList, err := a.getAccessList(ctx, name)
		if err != nil {
			return trace.Wrap(err)
		}

		// Check if the Access List has any blocking relationships.
		if err := a.checkDeletionBlockingRelationships(ctx, accessList); err != nil {
			return trace.Wrap(err)
		}

		// Delete all associated members.
		members, err := membersService.getResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := membersService.deleteAllResources(ctx); err != nil {
			return trace.Wrap(err)
		}

		// Update memberOf refs.
		for _, member := range members {
			if !member.IsList() {
				continue
			}
			memberName, err := accesslists.MemberScopeQualifiedName(member)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListMemberOf(ctx, name, memberName, false); err != nil {
				return trace.Wrap(err)
			}
		}

		// Delete list itself.
		if err := a.service.DeleteResource(ctx, name.ToScopesQualifiedName()); err != nil {
			return trace.Wrap(err)
		}

		// Update ownerOf refs.
		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}
			ownerSQN, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListOwnerOf(ctx, name, ownerSQN, false); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}

	resourceLockName, err := scopeAwareLockName(name)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, resourceLockName, accessListLockTTL, action)
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
	return a.CountAccessListMembersV2(ctx, accesslistv1.CountAccessListMembersRequest_builder{
		AccessListName: accessListName,
	}.Build())
}

// CountAccessListMembersV2 will count all access list members.
func (a *AccessListService) CountAccessListMembersV2(ctx context.Context, req *accesslistv1.CountAccessListMembersRequest) (users uint32, lists uint32, err error) {
	count := uint(0)
	listCount := uint(0)
	membersService, err := a.membersServiceForAccessList(accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessListName(),
	}))
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	for member, err := range membersService.resources(ctx, "", "") {
		if err != nil {
			return 0, 0, trace.Wrap(err)
		}

		if member.IsList() {
			listCount++
		} else {
			count++
		}
	}

	return uint32(count), uint32(listCount), trace.Wrap(err)
}

// ListAccessListMembers returns a paginated list of all members of the given list.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	return a.ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessList: accessListName,
		PageSize:   int32(pageSize),
		PageToken:  nextToken,
	}.Build())
}

// ListAccessListMembersV2 returns a paginated list of all members of the given list.
func (a *AccessListService) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	_, err := a.getAccessList(ctx, accessListName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	membersService, err := a.membersServiceForAccessList(accessListName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return membersService.listResources(ctx, int(req.GetPageSize()), req.GetPageToken())
}

// ListAllAccessListMembers returns a paginated list of all access list members for all unscoped access lists.
func (a *AccessListService) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return a.memberService.UnscopedService.ListResources(ctx, pageSize, pageToken)
}

// ListAllAccessListMembersV2 returns a paginated list of all access list members for all access lists.
func (a *AccessListService) ListAllAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAllAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	return a.memberService.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), func(member *accesslist.AccessListMember) bool {
		return scopes.MatchScope(scopeFilter, member.GetScope())
	})
}

// GetAccessListMember returns the specified access list member resource.
func (a *AccessListService) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	return a.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessList: accessList,
		MemberName: memberName,
	}.Build())
}

// GetAccessListMemberV2 returns the specified access list member resource.
func (a *AccessListService) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	memberName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	})
	_, err := a.getAccessList(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	memberService, err := a.memberServiceForNamedMember(accessListName, memberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	member, err := memberService.get(ctx)
	return member, trace.Wrap(err)
}

// memberServiceForNamedMember returns a [*memberService] valid for only the
// exact named member, with an appropriate backend prefix and NameKeyFunc configured.
func (a *AccessListService) memberServiceForNamedMember(listName, memberName accesslists.NormalizedSQN) (*memberService, error) {
	return memberServiceForNamedMember(a.memberService, listName, memberName)
}

// membersServiceForAccessList returns a [*membersService] valid only for
// operations on _all_ members of the given access list, it cannot be used to
// get, create, or delete any single member, use [memberServiceForNamedMember]
// in those cases.
func (a *AccessListService) membersServiceForAccessList(listName accesslists.NormalizedSQN) (*membersService, error) {
	return membersServiceForAccessList(a.memberService, listName)
}

// updateAccessListRefField is a helper that updates the specified field (memberOf or ownerOf) of an Access List,
// adding or removing the specified accessListName to the field of targetName.
func (a *AccessListService) updateAccessListRefField(
	ctx context.Context,
	ref accesslists.NormalizedSQN,
	target accesslists.NormalizedSQN,
	new bool,
	fieldSelector func(status *accesslist.Status) *[]string,
) error {
	targetAccessList, err := a.getAccessList(ctx, target)
	if err != nil {
		if trace.IsNotFound(err) {
			// If list is not found, it's possible that it was deleted. Regardless, there's nothing to update.
			return nil
		}
		return trace.Wrap(err)
	}

	field := fieldSelector(&targetAccessList.Status)

	value := ref.String()

	// If the field already contains the Access List, and we're adding,
	// or doesn't contain it, and we're removing, there's nothing to do.
	if slices.Contains(*field, value) == new {
		return nil
	}

	if new {
		*field = append(*field, value)
	} else {
		*field = slices.DeleteFunc(*field, func(e string) bool {
			return e == value
		})
	}

	if _, err := a.service.UpdateResource(ctx, targetAccessList); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// updateAccessListMemberOf updates the memberOf field for the specified memberName and accessListName.
// Should only be called after the relevant member has been successfully upserted or deleted.
func (a *AccessListService) updateAccessListMemberOf(ctx context.Context, accessListName, memberName accesslists.NormalizedSQN, new bool) error {
	return a.updateAccessListRefField(ctx, accessListName, memberName, new, func(status *accesslist.Status) *[]string {
		if accessListName.Scope == "" {
			return &status.MemberOf
		}
		return &status.ScopedMemberOf
	})
}

// updateAccessListOwnerOf updates the ownerOf field for the specified ownerName and accessListName.
// Should only be called after the relevant owner has been successfully upserted or deleted.
func (a *AccessListService) updateAccessListOwnerOf(ctx context.Context, accessListName, ownerName accesslists.NormalizedSQN, new bool) error {
	return a.updateAccessListRefField(ctx, accessListName, ownerName, new, func(status *accesslist.Status) *[]string {
		if accessListName.Scope == "" {
			return &status.OwnerOf
		}
		return &status.ScopedOwnerOf
	})
}

// GetAccessListOwners returns a list of all owners in an Access List, including those inherited from nested Access Lists.
//
// Returned Owners are not validated for ownership requirements – use `IsAccessListOwner` for validation.
func (a *AccessListService) GetAccessListOwners(ctx context.Context, accessListName string) ([]*accesslist.Owner, error) {
	return a.GetAccessListOwnersV2(ctx, accesslistv1.GetAccessListOwnersRequest_builder{
		AccessList: accessListName,
	}.Build())
}

// GetAccessListOwnersV2 returns a list of all owners in an Access List, including those inherited from nested Access Lists.
//
// Returned Owners are not validated for ownership requirements – use `IsAccessListOwner` for validation.
func (a *AccessListService) GetAccessListOwnersV2(ctx context.Context, req *accesslistv1.GetAccessListOwnersRequest) ([]*accesslist.Owner, error) {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	var owners []*accesslist.Owner
	accessList, err := a.getAccessList(ctx, accessListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	owners, err = accesslists.GetOwnersFor(ctx, accessList, &accessListAndMembersGetter{a.service, a.memberService})
	return owners, trace.Wrap(err)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (a *AccessListService) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	parentListName, err := accesslists.ParentListOf(member)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	memberName, err := accesslists.MemberScopeQualifiedName(member)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	memberService, err := a.memberServiceForNamedMember(parentListName, memberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var upserted *accesslist.AccessListMember
	action := func(ctx context.Context, _ backend.Backend) error {
		parentList, err := a.getAccessList(ctx, parentListName)
		if err != nil {
			return trace.Wrap(err)
		}
		existingMember, err := memberService.get(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		keepAWSIdentityCenterLabels(existingMember, member)

		if err := accesslists.ValidateAccessListMember(ctx, parentList, member, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
			return trace.Wrap(err)
		}

		upserted, err = memberService.upsert(ctx, member)
		if err != nil {
			return trace.Wrap(err)
		}

		// Remove stale member_of refs if the membership kind has changed.
		// Switching between MembershipKindList and MembershipKindScopedList is
		// impossible, by definition scoped lists have a non-empty scope, which
		// would mean the member resource would have a different name and key.
		if existingMember != nil && existingMember.IsList() && !member.IsList() {
			if err := a.updateAccessListMemberOf(ctx, parentListName, memberName, false); err != nil {
				return trace.Wrap(err)
			}
		}

		if member.IsList() {
			if err := a.updateAccessListMemberOf(ctx, parentListName, memberName, true); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}

	lockName, err := scopeAwareLockName(parentListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, accessListLockTTL, action)
	})
	return upserted, trace.Wrap(err)
}

// UpdateAccessListMember conditionally updates an access list member resource.
func (a *AccessListService) UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	parentListName, err := accesslists.ParentListOf(member)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	memberName, err := accesslists.MemberScopeQualifiedName(member)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	memberService, err := a.memberServiceForNamedMember(parentListName, memberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var updated *accesslist.AccessListMember
	lockName, err := scopeAwareLockName(parentListName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			memberList, err := a.getAccessList(ctx, parentListName)
			if err != nil {
				return trace.Wrap(err)
			}
			existingMember, err := memberService.get(ctx)
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			keepAWSIdentityCenterLabels(existingMember, member)

			if err := accesslists.ValidateAccessListMember(ctx, memberList, member, &accessListAndMembersGetter{a.service, a.memberService}); err != nil {
				return trace.Wrap(err)
			}

			updated, err = memberService.conditionalUpdateResource(ctx, member)
			if err != nil {
				return trace.Wrap(err)
			}

			// Remove stale member_of refs if the membership kind has changed.
			if existingMember != nil && existingMember.IsList() && !member.IsList() {
				if err := a.updateAccessListMemberOf(ctx, parentListName, memberName, false); err != nil {
					return trace.Wrap(err)
				}
			}

			if member.IsList() {
				if err := a.updateAccessListMemberOf(ctx, parentListName, memberName, true); err != nil {
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
	return a.DeleteAccessListMemberV2(ctx, accesslistv1.DeleteAccessListMemberRequest_builder{
		AccessList: accessList,
		MemberName: memberName,
	}.Build())
}

// DeleteAccessListMemberV2 hard deletes the specified access list member resource.
func (a *AccessListService) DeleteAccessListMemberV2(ctx context.Context, req *accesslistv1.DeleteAccessListMemberRequest) error {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	memberName := accesslists.NormalizedSQN(scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	})
	lockName, err := scopeAwareLockName(accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	memberService, err := a.memberServiceForNamedMember(accessListName, memberName)
	if err != nil {
		return trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			_, err := a.getAccessList(ctx, accessListName)
			if err != nil {
				return trace.Wrap(err)
			}

			member, err := memberService.get(ctx)
			if err != nil {
				return trace.Wrap(err)
			}

			if err := memberService.delete(ctx); err != nil {
				return trace.Wrap(err)
			}

			if member.IsList() {
				if err := a.updateAccessListMemberOf(ctx, accessListName, memberName, false); err != nil {
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
	return a.DeleteAllAccessListMembersForAccessListV2(ctx, accesslistv1.DeleteAllAccessListMembersForAccessListRequest_builder{
		AccessList: accessList,
	}.Build())
}

// DeleteAllAccessListMembersForAccessListV2 hard deletes all access list members
// for an access list. Note that deleting all members is the only member operation
// allowed on a list with implicit membership, as it provides a mechanism for
// cleaning out the user list if a list is converted from explicit to implicit.
func (a *AccessListService) DeleteAllAccessListMembersForAccessListV2(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) error {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	membersService, err := a.membersServiceForAccessList(accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	lockName, err := scopeAwareLockName(accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			_, err := a.getAccessList(ctx, accessListName)
			if err != nil {
				return trace.Wrap(err)
			}

			allMembers, err := membersService.getResources(ctx)
			if err != nil {
				return trace.Wrap(err)
			}

			if err := membersService.deleteAllResources(ctx); err != nil {
				return trace.Wrap(err)
			}

			for _, member := range allMembers {
				if !member.IsList() {
					continue
				}
				memberName, err := accesslists.MemberScopeQualifiedName(member)
				if err != nil {
					return trace.Wrap(err)
				}
				if err := a.updateAccessListMemberOf(ctx, accessListName, memberName, false); err != nil {
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

func (a *AccessListService) writeAccessList(
	ctx context.Context,
	accessList *accesslist.AccessList,
	op opType,
) (*accesslist.AccessList, error) {
	switch op {
	case opTypeUpdate:
		return a.service.ConditionalUpdateResource(ctx, accessList)
	case opTypeUpsert:
		return a.service.UpsertResource(ctx, accessList)
	default:
		return nil, trace.BadParameter("Unknown Access List write operation: %d", op)
	}
}

// writeAccessListWithMembers holds all of the common logic for updating and
// upserting an access list and its collection of members.
func (a *AccessListService) writeAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember, op opType) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessListName := accesslists.ScopeQualifiedName(accessList)

	for _, m := range membersIn {
		if err := m.CheckAndSetDefaults(); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	var existingAccessList *accesslist.AccessList

	validateAccessList := func() error {
		var err error
		existingAccessList, err = a.getAccessList(ctx, accessListName)
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

		if err := a.checkScopesFeatures(existingAccessList, accessList); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	reconcileMembers := func() error {
		// Convert the members slice to a map for easier lookup.
		membersMap := make(map[accesslists.NormalizedSQN]*accesslist.AccessListMember, len(membersIn))
		for _, member := range membersIn {
			memberName, err := accesslists.MemberScopeQualifiedName(member)
			if err != nil {
				return trace.Wrap(err)
			}
			membersMap[memberName] = member
		}

		membersService, err := a.membersServiceForAccessList(accessListName)
		if err != nil {
			return trace.Wrap(err)
		}

		for existingMember, err := range membersService.resources(ctx, "", "") {
			if err != nil {
				return trace.Wrap(err)
			}

			existingMemberName, err := accesslists.MemberScopeQualifiedName(existingMember)
			if err != nil {
				return trace.Wrap(err)
			}

			memberService, err := a.memberServiceForNamedMember(accessListName, existingMemberName)
			if err != nil {
				return trace.Wrap(err)
			}

			// If the member is not in the new members map (request), delete it.
			if newMember, ok := membersMap[existingMemberName]; !ok {
				if err := memberService.delete(ctx); err != nil {
					return trace.Wrap(err)
				}
				// Update memberOf field if nested list.
				if existingMember.IsList() {
					if err := a.updateAccessListMemberOf(ctx, accessListName, existingMemberName, false); err != nil {
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
				if !newMember.IsEqual(existingMember) {
					// Update the member.
					upserted, err := memberService.upsert(ctx, newMember)
					if err != nil {
						return trace.Wrap(err)
					}

					existingMember.SetRevision(upserted.GetRevision())
				}

				// Update the status member_of ref if membership kind has changed.
				if existingMember.IsList() != newMember.IsList() {
					if err := a.updateAccessListMemberOf(ctx, accessListName, existingMemberName, newMember.IsList()); err != nil {
						return trace.Wrap(err)
					}
				}
			}

			// Remove the member from the map.
			delete(membersMap, existingMemberName)
		}

		if err := a.insertMembersAndUpdateNestedRelationships(ctx, accessListName, slices.Collect(maps.Values(membersMap))); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	reconcileOldOwners := func() error {
		if existingAccessList == nil {
			return nil
		}
		for _, existingOwner := range existingAccessList.Spec.Owners {
			if !existingOwner.IsMembershipKindList() {
				continue
			}
			existingOwnerName, err := accesslists.OwnerScopeQualifiedName(existingOwner)
			if err != nil {
				return trace.Wrap(err)
			}
			isStillAnOwnerList := slices.ContainsFunc(accessList.Spec.Owners, func(owner accesslist.Owner) bool {
				if !owner.IsMembershipKindList() {
					return false
				}
				ownerName, err := accesslists.OwnerScopeQualifiedName(owner)
				if err != nil {
					return false
				}
				return ownerName == existingOwnerName
			})
			if !isStillAnOwnerList {
				if err := a.updateAccessListOwnerOf(ctx, accesslists.ScopeQualifiedName(existingAccessList), existingOwnerName, false); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		return nil
	}

	writeAccessList := func() (err error) {
		accessList, err = a.writeAccessList(ctx, accessList, op)
		return trace.Wrap(err)
	}

	reconcileNewOwners := func() error {
		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}
			ownerName, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListOwnerOf(ctx, accessListName, ownerName, true); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}

	var actions []func() error

	// If IGS is not enabled for this cluster we need to wrap the whole update and
	// member reconciliation in *another* lock so that we can accurately count the
	// access lists in the cluster in order to  prevent un-authorized use of the
	// AccessList feature
	if !a.modules.Features().GetEntitlement(entitlements.Identity).Enabled {
		actions = append(actions, func() error { return a.verifyAccessListCreateLimit(ctx, accessListName) })
	}

	// Note we need to reconcile the old owners (clean status.owner_of for the owner lists
	// which are removed with this request) first, then update the access list and then
	// reconcile the new owners (set status.owner_of of the owner lists that are added with
	// this request). This is to make sure the operation doesn't escalate privileges if
	// interrupted as we use status.owner_of to calculate hierarchy.
	actions = append(actions, validateAccessList, reconcileMembers, reconcileOldOwners, writeAccessList, reconcileNewOwners)

	lockName, err := scopeAwareLockName(accessListName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
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

// checkScopesFeatures requires the scopes feature to be enabled if a new
// scoped access list is being created or there are any *new* scoped role
// grants in a list update.
func (a *AccessListService) checkScopesFeatures(existingList, newList *accesslist.AccessList) error {
	if a.scopesFeatures.Enabled {
		return nil
	}

	if existingList == nil && newList.GetScope() != "" {
		return trace.Wrap(a.scopesFeatures.AssertEnabled())
	}

	if len(newList.Spec.Grants.ScopedRoles) == 0 && len(newList.Spec.OwnerGrants.ScopedRoles) == 0 {
		// The new list does not grant any scoped roles, so no checks are needed.
		return nil
	}

	var existingListGrants set.Set[accesslist.ScopedRoleGrant]
	if existingList != nil {
		existingListGrants = set.New(existingList.Spec.Grants.ScopedRoles...).Add(existingList.Spec.OwnerGrants.ScopedRoles...)
	}
	newListGrants := set.New(newList.Spec.Grants.ScopedRoles...).Add(newList.Spec.OwnerGrants.ScopedRoles...)
	addedGrants := newListGrants.Subtract(existingListGrants)

	if addedGrants.Len() == 0 {
		// The new list does not grant any scoped roles at scopes not already
		// granted by the existing list.
		return nil
	}

	// This list has new scoped role grants, the scopes feature must be enabled.
	return trace.Wrap(a.scopesFeatures.AssertEnabled())
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
	return a.ListAccessListReviewsV2(ctx, accesslistv1.ListAccessListReviewsRequest_builder{
		AccessList: accessList,
		PageSize:   int32(pageSize),
		NextToken:  pageToken,
	}.Build())
}

func (a *AccessListService) ListAccessListReviewsV2(ctx context.Context, req *accesslistv1.ListAccessListReviewsRequest) (reviews []*accesslist.Review, nextToken string, err error) {
	listName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	_, err = a.getAccessList(ctx, listName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	reviewService, err := a.reviewService.WithScopedResourcePrefix(listName.ToScopesQualifiedName())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	reviews, nextToken, err = reviewService.ListResources(ctx, int(req.GetPageSize()), req.GetNextToken())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return reviews, nextToken, nil
}

// ListAllAccessListReviews will list access list reviews for all unscoped access lists.
func (a *AccessListService) ListAllAccessListReviews(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
	return a.reviewService.UnscopedService.ListResources(ctx, pageSize, pageToken)
}

// ListAllAccessListReviewsV2 will list access list reviews for all access lists.
func (a *AccessListService) ListAllAccessListReviewsV2(ctx context.Context, req *accesslistv1.ListAllAccessListReviewsRequest) ([]*accesslist.Review, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	return a.reviewService.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetNextToken(), func(review *accesslist.Review) bool {
		return scopes.MatchScope(scopeFilter, review.GetScope())
	})
}

// CreateAccessListReview will create a new review for an access list.
func (a *AccessListService) CreateAccessListReview(ctx context.Context, review *accesslist.Review) (*accesslist.Review, time.Time, error) {
	reviewName := uuid.New().String()
	createdReview, err := accesslist.NewReviewWithScope(header.Metadata{
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
	}, review.Scope)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	reviewedListName, err := accesslists.ReviewedList(review)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	reviewService, err := a.reviewService.WithScopedResourcePrefix(reviewedListName.ToScopesQualifiedName())
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	allRemovedMembers, err := accesslists.AllRemovedMembers(review)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	if review.Scope != "" && !review.Spec.Changes.MembershipRequirementsChanged.IsEmpty() {
		return nil, time.Time{}, trace.BadParameter("scoped access lists cannot contain membership requirements")
	}
	if review.Scope == "" && len(review.Spec.Changes.ScopedRemovedMembers) > 0 {
		return nil, time.Time{}, trace.BadParameter("unscoped access lists cannot have scoped members")
	}

	var nextAuditDate time.Time

	lockName, err := scopeAwareLockName(reviewedListName)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	err = a.service.RunWhileLocked(ctx, lockName, accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		accessList, err := a.getAccessList(ctx, reviewedListName)
		if err != nil {
			return trace.Wrap(err)
		}

		if !accessList.IsReviewable() {
			return trace.BadParameter("access_list %q is not reviewable", reviewedListName.String())
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

		createdReview, err = reviewService.CreateResource(ctx, createdReview)
		if err != nil {
			return trace.Wrap(err)
		}

		nextAuditDate, err = accessList.SelectNextReviewDate()
		if err != nil {
			return trace.Wrap(err, "selecting next review date")
		}
		accessList.Spec.Audit.NextAuditDate = nextAuditDate

		for _, removedMember := range allRemovedMembers {
			memberService, err := a.memberServiceForNamedMember(reviewedListName, removedMember)
			if err != nil {
				return trace.Wrap(err)
			}
			_, err = memberService.get(ctx)
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
			isAccessListMember := err == nil

			if isAccessListMember {
				if err := a.updateAccessListMemberOf(ctx, reviewedListName, removedMember, false); err != nil {
					return trace.Wrap(err)
				}
			}
			if err := memberService.delete(ctx); err != nil {
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
	return a.DeleteAccessListReviewV2(ctx, accesslistv1.DeleteAccessListReviewRequest_builder{
		AccessListName: accessListName,
		ReviewName:     reviewName,
	}.Build())
}

// DeleteAccessListReviewV2 will delete an access list review from the backend.
func (a *AccessListService) DeleteAccessListReviewV2(ctx context.Context, req *accesslistv1.DeleteAccessListReviewRequest) error {
	accessListName := accesslists.NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessListName(),
	})
	_, err := a.getAccessList(ctx, accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	reviewService, err := a.reviewService.WithScopedResourcePrefix(accessListName.ToScopesQualifiedName())
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(reviewService.DeleteResource(ctx, req.GetReviewName()))
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

func scopeAwareLockName(accessListName accesslists.NormalizedSQN) ([]string, error) {
	if accessListName.Scope == "" {
		return lockName(accessListName.Name), nil
	}
	encodedScope, err := scopes.EncodeForKey(accessListName.Scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []string{scopedPrefix, accessListPrefix, encodedScope, accessListName.Name}, nil
}

// verifyAccessListCreateLimit ensures creating access list is limited to no more than 1 (updating is allowed).
// It differentiates request for `creating` and `updating` by checking to see if the request
// access list name matches the ones we retrieved.
// Returns error if limit has been reached.
func (a *AccessListService) verifyAccessListCreateLimit(ctx context.Context, targetAccessListName accesslists.NormalizedSQN) error {
	f := a.modules.Features()
	if f.GetEntitlement(entitlements.Identity).Enabled {
		return nil // unlimited
	}

	// Iterate through all lists, to check if the request was an update, which
	// is allowed.
	numLists := 0
	for list, err := range a.service.Resources(ctx, "", "") {
		if err != nil {
			return trace.Wrap(err)
		}
		if accesslists.ScopeQualifiedName(list) == targetAccessListName {
			return nil
		}
		numLists++
	}

	// We are *always* allowed to create at least one AccessLists in order to
	// demonstrate the functionality.
	// TODO(tcsc): replace with a default OSS entitlement of 1
	if numLists == 0 {
		return nil
	}

	accessListEntitlement := f.GetEntitlement(entitlements.AccessLists)
	if accessListEntitlement.UnderLimit(numLists) {
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
		accessList.Status.ScopedMemberOf = existingAccessList.Status.ScopedMemberOf
		accessList.Status.ScopedOwnerOf = existingAccessList.Status.ScopedOwnerOf
	} else {
		// For newly created AccessList make sure MemberOf/OwnerOf are empty.
		accessList.Status.MemberOf = []string{}
		accessList.Status.OwnerOf = []string{}
		// Must be nil for the tests in teleport.e to pass.
		accessList.Status.ScopedMemberOf = nil
		accessList.Status.ScopedOwnerOf = nil
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

func (a *AccessListService) insertMembersAndUpdateNestedRelationships(ctx context.Context, accessListName accesslists.NormalizedSQN, members []*accesslist.AccessListMember) error {
	if err := a.insertMembers(ctx, accessListName, members); err != nil {
		return trace.Wrap(err)
	}
	// In case of nested access list members.
	if err := a.updatedMembersNestedRelationships(ctx, accessListName, members); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *AccessListService) insertMembers(ctx context.Context, acl accesslists.NormalizedSQN, members []*accesslist.AccessListMember) error {
	items, err := stream.Collect(a.membersToBackendItems(acl, members))
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

func (a *AccessListService) membersToBackendItems(acl accesslists.NormalizedSQN, members []*accesslist.AccessListMember) iter.Seq2[backend.Item, error] {
	return func(yield func(backend.Item, error) bool) {
		for _, member := range members {
			memberName, err := accesslists.MemberScopeQualifiedName(member)
			if err != nil {
				yield(backend.Item{}, trace.Wrap(err))
				return
			}
			memberService, err := a.memberServiceForNamedMember(acl, memberName)
			if err != nil {
				yield(backend.Item{}, trace.Wrap(err))
				return
			}
			item, err := memberService.makeBackendItem(member)
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

func (a *AccessListService) updatedMembersNestedRelationships(ctx context.Context, acl accesslists.NormalizedSQN, members []*accesslist.AccessListMember) error {
	for _, member := range members {
		if !member.IsList() {
			continue
		}
		memberName, err := accesslists.MemberScopeQualifiedName(member)
		if err != nil {
			return trace.Wrap(err)
		}
		// Update memberOf field if nested list.
		if err := a.updateAccessListMemberOf(ctx, acl, memberName, true); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CleanupAccessListStatus removes invalid Status.OwnerOf and Status.MemberOf references.
func (a *AccessListService) CleanupAccessListStatus(ctx context.Context, accessListName string) (*accesslist.AccessList, error) {
	return a.CleanupAccessListStatusV2(ctx, accesslists.NormalizedSQN{Name: accessListName})
}

// CleanupAccessListStatusV2 removes invalid Status.(Scoped)OwnerOf and Status.(Scoped)MemberOf references.
func (a *AccessListService) CleanupAccessListStatusV2(ctx context.Context, accessListName accesslists.NormalizedSQN) (*accesslist.AccessList, error) {
	return a.runWithGlobalLockAccessList(ctx, accessListName, func() (*accesslist.AccessList, error) {
		accessList, err := a.getAccessList(ctx, accessListName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		isActualOwner := func(ownedList *accesslist.AccessList) bool {
			return slices.ContainsFunc(ownedList.Spec.Owners, func(ownedListOwner accesslist.Owner) bool {
				if !ownedListOwner.IsMembershipKindList() {
					return false
				}
				ownedListOwnerName, err := accesslists.OwnerScopeQualifiedName(ownedListOwner)
				if err != nil {
					return false
				}
				return ownedListOwnerName == accessListName
			})
		}

		var ownerRefreshErr error

		// OwnerOf names unscoped access lists that this access list supposedly owns.
		accessList.Status.OwnerOf = slices.DeleteFunc(accessList.Status.OwnerOf, func(ownerOf string) bool {
			if accessList.Scope != "" {
				// Scoped access lists can never own unscoped access lists.
				return true
			}
			ownedList, err := a.GetAccessList(ctx, ownerOf)
			if err != nil {
				if trace.IsNotFound(err) {
					return true
				}
				ownerRefreshErr = err
				return false
			}
			return !isActualOwner(ownedList)
		})
		if ownerRefreshErr != nil {
			return nil, trace.Wrap(ownerRefreshErr)
		}

		// ScopedOwnerOf names scoped access lists that this access list supposedly owns.
		accessList.Status.ScopedOwnerOf = slices.DeleteFunc(accessList.Status.ScopedOwnerOf, func(scopedOwnerOf string) bool {
			ownedListName, err := accesslists.ParseScopeQualifiedName(scopedOwnerOf)
			if err != nil {
				// If we can't parse the name, it is invalid and should be removed.
				return true
			}
			ownedList, err := a.getAccessList(ctx, ownedListName)
			if err != nil {
				if trace.IsNotFound(err) {
					return true
				}
				ownerRefreshErr = err
				return false
			}
			return !isActualOwner(ownedList)
		})
		if ownerRefreshErr != nil {
			return nil, trace.Wrap(ownerRefreshErr)
		}

		var memberRefreshErr error

		// MemberOf names unscoped access lists that this access list is supposedly a member of.
		accessList.Status.MemberOf = slices.DeleteFunc(accessList.Status.MemberOf, func(memberOf string) bool {
			if accessList.Scope != "" {
				// Scoped access lists can never be members of unscoped access lists.
				return true
			}
			if _, err := a.memberService.UnscopedService.WithPrefix(memberOf).GetResource(ctx, accessList.GetName()); err != nil {
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

		// ScopedMemberOf names scoped access lists that this access list is supposedly a member of.
		accessList.Status.ScopedMemberOf = slices.DeleteFunc(accessList.Status.ScopedMemberOf, func(scopedMemberOf string) bool {
			parentListName, err := accesslists.ParseScopeQualifiedName(scopedMemberOf)
			if err != nil {
				// If we can't parse the name, it is invalid and should be removed.
				return true
			}
			memberService, err := a.memberServiceForNamedMember(parentListName, accessListName)
			if err != nil {
				memberRefreshErr = err
				return false
			}
			if _, err := memberService.get(ctx); err != nil {
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
	return a.EnsureNestedAccessListStatusesV2(ctx, accesslists.NormalizedSQN{Name: accessListName})
}

// EnsureNestedAccessListStatusesV2 goes over all nested owners and nested members of the named
// access list and ensures nested lists' statuses owner_of/member_of contain the access list name.
func (a *AccessListService) EnsureNestedAccessListStatusesV2(ctx context.Context, accessListName accesslists.NormalizedSQN) error {
	return a.runWithGlobalLock(ctx, accessListName, func() error {
		accessList, err := a.getAccessList(ctx, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}
			ownerName, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListOwnerOf(ctx, accessListName, ownerName, true); err != nil {
				return trace.Wrap(err)
			}
		}

		membersService, err := a.membersServiceForAccessList(accessListName)
		if err != nil {
			return trace.Wrap(err)
		}
		for member, err := range membersService.resources(ctx, "", "") {
			if err != nil {
				return trace.Wrap(err)
			}
			if !member.IsList() {
				continue
			}
			memberName, err := accesslists.MemberScopeQualifiedName(member)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := a.updateAccessListMemberOf(ctx, accessListName, memberName, true); err != nil {
				return trace.Wrap(err)
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
				item, err := a.memberService.UnscopedService.WithPrefix(member.Spec.AccessList).MakeBackendItem(member)
				if err != nil {
					yield(backend.Item{}, trace.Wrap(err))
					return
				}
				if !yield(item, nil) {
					return
				}
			}
			item, err := a.service.UnscopedService.MakeBackendItem(acl)
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

// InsertScopedAccessListCollection inserts a complete collection of access lists and their members from a single
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
func (a *AccessListService) InsertScopedAccessListCollection(ctx context.Context, collection *accesslists.ScopedCollection) error {
	if err := a.scopesFeatures.AssertEnabled(); err != nil {
		return trace.Wrap(err)
	}

	if err := collection.Validate(ctx); err != nil {
		return trace.Wrap(err)
	}
	// Collect backend items in chunks of 800 items each to avoid high memory consumption
	// from constructing a large slice of all backend items at once. PutBatch will then
	// leverage its own internal chunking to write items to the backend in smaller batches.
	// TODO(smallinsky) align the chunk size with the one used in backend.PutBatch
	for chunk, err := range stream.Chunks(a.scopedCollectionToBackendItemsIter(collection), 800) {
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := backend.PutBatch(ctx, a.backend, chunk); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// scopedCollectionToBackendItemsIter converts access list collection to an iterator of backend items.
// The iterator yields access list members first, followed by their parent access list.
func (a *AccessListService) scopedCollectionToBackendItemsIter(collection *accesslists.ScopedCollection) iter.Seq2[backend.Item, error] {
	return func(yield func(backend.Item, error) bool) {
		seenLists := make(map[accesslists.NormalizedSQN]struct{}, len(collection.MembersByAccessList))
		for aclName, members := range collection.MembersByAccessList {
			acl, ok := collection.AccessListsByName[aclName]
			if !ok {
				yield(backend.Item{}, trace.NotFound("access list %q not found", aclName))
				return
			}
			for item, err := range a.membersToBackendItems(aclName, members) {
				if err != nil {
					yield(backend.Item{}, err)
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
			seenLists[aclName] = struct{}{}
		}
		for aclName, acl := range collection.AccessListsByName {
			if _, ok := seenLists[aclName]; ok {
				continue
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

func (a *AccessListService) runWithGlobalLock(ctx context.Context, accessListName accesslists.NormalizedSQN, fn func() error) error {
	lockName, err := scopeAwareLockName(accessListName)
	if err != nil {
		return trace.Wrap(err)
	}
	return a.service.RunWhileLocked(ctx, []string{accessListResourceLockName}, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return a.service.RunWhileLocked(ctx, lockName, 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
			return trace.Wrap(fn())
		})
	})
}

func (a *AccessListService) runWithGlobalLockAccessList(ctx context.Context, accessListName accesslists.NormalizedSQN, fn func() (*accesslist.AccessList, error)) (*accesslist.AccessList, error) {
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
	accessListName := accesslists.ScopeQualifiedName(accessList)
	allParentLists, err := accesslists.AllParentLists(accessList)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(allParentLists) == 0 {
		return nil
	}

	memberOfTitles := make([]string, 0, len(allParentLists))
	for _, parentListName := range allParentLists {
		parentList, err := a.getAccessList(ctx, parentListName)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching parent list "%s"`, parentListName)
		}
		memberService, err := a.memberServiceForNamedMember(parentListName, accessListName)
		if err != nil {
			return trace.Wrap(err)
		}
		member, err := memberService.get(ctx)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching access list member for "%s"`, parentListName.String())
		}
		if member.IsList() {
			memberOfTitles = append(memberOfTitles, parentList.Spec.Title)
		}
	}

	if len(memberOfTitles) > 0 {
		errMsg := fmt.Sprintf(`Cannot delete "%s", as it is a member of Access Lists: %s`,
			accessList.Spec.Title, quoteAndJoin(memberOfTitles))
		return trace.Wrap(accesslists.ErrDeniedAccessListDeletion, errMsg)
	}

	return nil
}

// checkDeletionBlockingOwnerRelationships checks if the access list owns any other access lists that would block deletion
func (a *AccessListService) checkDeletionBlockingOwnerRelationships(ctx context.Context, accessList *accesslist.AccessList) error {
	allOwnedLists, err := accesslists.AllOwnedLists(accessList)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(allOwnedLists) == 0 {
		return nil
	}

	ownerOfTitles := make([]string, 0, len(allOwnedLists))
	for _, ownedListName := range allOwnedLists {
		ownedList, err := a.getAccessList(ctx, ownedListName)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err, `fetching owned list "%s"`, ownedListName.String())
		}
		isActualOwner := slices.ContainsFunc(ownedList.Spec.Owners, func(owner accesslist.Owner) bool {
			if !owner.IsMembershipKindList() {
				return false
			}
			actualOwnerSQN, err := accesslists.OwnerScopeQualifiedName(owner)
			if err != nil {
				return false
			}
			return actualOwnerSQN == accesslists.ScopeQualifiedName(accessList)
		})
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
