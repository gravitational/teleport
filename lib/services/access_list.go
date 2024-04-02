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

package services

import (
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var _ AccessLists = (*accesslistclient.Client)(nil)

// AccessListsGetter defines an interface for reading access lists.
type AccessListsGetter interface {
	AccessListMembersGetter

	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
	// ListAccessLists returns a paginated list of access lists.
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// GetAccessListsToReview returns access lists that the user needs to review.
	GetAccessListsToReview(context.Context) ([]*accesslist.AccessList, error)
}

// AccessListsSuggestionsGetter defines an interface for reading access lists suggestions.
type AccessListsSuggestionsGetter interface {
	// GetSuggestedAccessLists returns a list of access lists that are suggested for a given request.
	GetSuggestedAccessLists(ctx context.Context, accessRequestID string) ([]*accesslist.AccessList, error)
}

// AccessLists defines an interface for managing AccessLists.
type AccessLists interface {
	AccessListsGetter
	AccessListsSuggestionsGetter
	AccessListMembers
	AccessListReviews

	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	// UpdateAccessList updates an access list resource.
	UpdateAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	// DeleteAccessList removes the specified access list resource.
	DeleteAccessList(context.Context, string) error
	// DeleteAllAccessLists removes all access lists.
	DeleteAllAccessLists(context.Context) error

	// UpsertAccessListWithMembers creates or updates an access list resource and its members.
	UpsertAccessListWithMembers(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)

	// AccessRequestPromote promotes an access request to an access list.
	AccessRequestPromote(ctx context.Context, req *accesslistv1.AccessRequestPromoteRequest) (*accesslistv1.AccessRequestPromoteResponse, error)
}

// MarshalAccessList marshals the access list resource to JSON.
func MarshalAccessList(accessList *accesslist.AccessList, opts ...MarshalOption) ([]byte, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *accessList
		copy.SetResourceID(0)
		copy.SetRevision("")
		accessList = &copy
	}
	return utils.FastMarshal(accessList)
}

// UnmarshalAccessList unmarshals the access list resource from JSON.
func UnmarshalAccessList(data []byte, opts ...MarshalOption) (*accesslist.AccessList, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accessList accesslist.AccessList
	if err := utils.FastUnmarshal(data, &accessList); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		accessList.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		accessList.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		accessList.SetExpiry(cfg.Expires)
	}
	return &accessList, nil
}

// ImplicitAccessListError indicates that an operation that only makes sense for
// AccessLists with an explicit Member list has been attempted on an implicit-
// membership AccessList
type ImplicitAccessListError struct{}

// Error implements the `error` interface for ImplicitAccessListError
func (ImplicitAccessListError) Error() string {
	return "requested AccessList does not have explicit member list"
}

// AccessListMemberGetter defines an interface that can retrieve access list members.
type AccessListMemberGetter interface {
	// GetAccessListMember returns the specified access list member resource.
	// May return a DynamicAccessListError if the requested access list has an
	// implicit member list and the underlying implementation does not have
	// enough information to compute the dynamic member record.
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
}

// AccessListMembersGetter defines an interface for reading access list members.
type AccessListMembersGetter interface {
	AccessListMemberGetter

	// CountAccessListMembers will count all access list members.
	CountAccessListMembers(ctx context.Context, accessListName string) (uint32, error)
	// ListAccessListMembers returns a paginated list of all access list members.
	// May return a DynamicAccessListError if the requested access list has an
	// implicit member list and the underlying implementation does not have
	// enough information to compute the dynamic member list.
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
}

// AccessListMembers defines an interface for managing AccessListMembers.
type AccessListMembers interface {
	AccessListMembersGetter

	// UpsertAccessListMember creates or updates an access list member resource.
	UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	// UpdateAccessListMember conditionally updates an access list member resource.
	UpdateAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	// DeleteAccessListMember hard deletes the specified access list member resource.
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
	// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list.
	DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error
	// DeleteAllAccessListMembers hard deletes all access list members.
	DeleteAllAccessListMembers(ctx context.Context) error
}

// MarshalAccessListMember marshals the access list member resource to JSON.
func MarshalAccessListMember(member *accesslist.AccessListMember, opts ...MarshalOption) ([]byte, error) {
	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *member
		copy.SetResourceID(0)
		copy.SetRevision("")
		member = &copy
	}
	return utils.FastMarshal(member)
}

// UnmarshalAccessListMember unmarshals the access list member resource from JSON.
func UnmarshalAccessListMember(data []byte, opts ...MarshalOption) (*accesslist.AccessListMember, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list member data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var member accesslist.AccessListMember
	if err := utils.FastUnmarshal(data, &member); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		member.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		member.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		member.SetExpiry(cfg.Expires)
	}
	return &member, nil
}

// IsAccessListOwner will return true if the user is an owner for the current list.
func IsAccessListOwner(identity tlsca.Identity, accessList *accesslist.AccessList) error {
	// An opaque access denied error.
	accessDenied := trace.AccessDenied("access denied")

	// Is the supplied identity in the owners list?
	ownerIdx := slices.IndexFunc(accessList.GetOwners(), func(owner accesslist.Owner) bool {
		return owner.Name == identity.Username
	})
	if ownerIdx == -1 {
		return accessDenied
	}

	// Does the supplied Identity meet the ownership requirements?
	if !UserMeetsRequirements(identity, accessList.Spec.OwnershipRequires) {
		return accessDenied
	}

	// We've gotten through all the checks, so the user is an owner.
	return nil
}

// AccessListMembershipChecker will check if users are members of an access list and
// makes sure the user is not locked and meets membership requirements.
type AccessListMembershipChecker struct {
	members AccessListMemberGetter
	locks   LockGetter
	clock   clockwork.Clock
}

// NewAccessListMembershipChecker will create a new access list membership checker.
func NewAccessListMembershipChecker(clock clockwork.Clock, members AccessListMemberGetter, locks LockGetter) *AccessListMembershipChecker {
	return &AccessListMembershipChecker{
		members: members,
		locks:   locks,
		clock:   clock,
	}
}

// IsAccessListMember will return true if the user is a member for the current list.
func (a AccessListMembershipChecker) IsAccessListMember(ctx context.Context, identity tlsca.Identity, accessList *accesslist.AccessList) error {
	username := identity.Username

	// Allow for nil locks while we transition away from using `IsAccessListMember` outside of this struct.
	if a.locks != nil {
		locks, err := a.locks.GetLocks(ctx, true, types.LockTarget{
			User: username,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if len(locks) > 0 {
			return trace.AccessDenied("user %s is currently locked", username)
		}
	}

	member, err := a.members.GetAccessListMember(ctx, accessList.GetName(), username)
	if trace.IsNotFound(err) {
		// The member has not been found, so we know they're not a member of this list.
		return trace.NotFound("user %s is not a member of the access list", username)
	} else if err != nil {
		// Some other error has occurred
		return trace.Wrap(err)
	}

	expires := member.Spec.Expires
	if !expires.IsZero() && !a.clock.Now().Before(expires) {
		return trace.AccessDenied("user %s's membership has expired in the access list", username)
	}

	if !UserMeetsRequirements(identity, accessList.Spec.MembershipRequires) {
		return trace.AccessDenied("user %s is a member, but does not have the roles or traits required to be a member of this list", username)
	}

	return nil
}

// TODO(mdwn): Remove this in favor of using the access list membership checker.
func IsAccessListMember(ctx context.Context, identity tlsca.Identity, clock clockwork.Clock, accessList *accesslist.AccessList, members AccessListMemberGetter) error {
	// See if the member getter also implements lock getter. If so, use it. Otherwise, nil is fine.
	lockGetter, _ := members.(LockGetter)
	return AccessListMembershipChecker{
		members: members,
		locks:   lockGetter,
		clock:   clock,
	}.IsAccessListMember(ctx, identity, accessList)
}

// UserMeetsRequirements will return true if the user meets the requirements for the access list.
func UserMeetsRequirements(identity tlsca.Identity, requires accesslist.Requires) bool {
	// Assemble the user's roles for easy look up.
	userRolesMap := map[string]struct{}{}
	for _, role := range identity.Groups {
		userRolesMap[role] = struct{}{}
	}

	// Check that the user meets the role requirements.
	for _, role := range requires.Roles {
		if _, ok := userRolesMap[role]; !ok {
			return false
		}
	}

	// Assemble traits for easy lookup.
	userTraitsMap := map[string]map[string]struct{}{}
	for k, values := range identity.Traits {
		if _, ok := userTraitsMap[k]; !ok {
			userTraitsMap[k] = map[string]struct{}{}
		}

		for _, v := range values {
			userTraitsMap[k][v] = struct{}{}
		}
	}

	// Check that user meets trait requirements.
	for k, values := range requires.Traits {
		if _, ok := userTraitsMap[k]; !ok {
			return false
		}

		for _, v := range values {
			if _, ok := userTraitsMap[k][v]; !ok {
				return false
			}
		}
	}

	// The user meets all requirements.
	return true
}

// AccessListReviews defines an interface for managing Access List reviews.
type AccessListReviews interface {
	// ListAccessListReviews will list access list reviews for a particular access list.
	ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error)

	// ListAllAccessListReviews will list access list reviews for all access lists. Only to be used by the cache.
	ListAllAccessListReviews(ctx context.Context, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error)

	// CreateAccessListReview will create a new review for an access list.
	CreateAccessListReview(ctx context.Context, review *accesslist.Review) (updatedReview *accesslist.Review, nextReviewDate time.Time, err error)

	// DeleteAccessListReview will delete an access list review from the backend.
	DeleteAccessListReview(ctx context.Context, accessListName, reviewName string) error

	// DeleteAllAccessListReviews will delete all access list reviews from all access lists.
	DeleteAllAccessListReviews(ctx context.Context) error
}

// MarshalAccessListReview marshals the access list review resource to JSON.
func MarshalAccessListReview(review *accesslist.Review, opts ...MarshalOption) ([]byte, error) {
	if err := review.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *review
		copy.SetResourceID(0)
		copy.SetRevision("")
		review = &copy
	}
	return utils.FastMarshal(review)
}

// UnmarshalAccessListReview unmarshals the access list review resource from JSON.
func UnmarshalAccessListReview(data []byte, opts ...MarshalOption) (*accesslist.Review, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list review data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var review accesslist.Review
	if err := utils.FastUnmarshal(data, &review); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := review.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		review.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		review.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		review.SetExpiry(cfg.Expires)
	}
	return &review, nil
}
