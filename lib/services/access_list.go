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
	"encoding/base32"
	"slices"
	"strings"
	"time"

	"github.com/charlievieth/strcase"
	"github.com/gravitational/trace"
	"golang.org/x/text/cases"

	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	// ListAccessListsV2 returns a filtered and sorted paginated list of access lists.
	ListAccessListsV2(context.Context, *accesslistv1.ListAccessListsV2Request) ([]*accesslist.AccessList, string, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// GetAccessListsToReview returns access lists that the user needs to review.
	GetAccessListsToReview(context.Context) ([]*accesslist.AccessList, error)
	// GetInheritedGrants returns grants inherited by access list accessListID from parent access lists.
	GetInheritedGrants(context.Context, string) (*accesslist.Grants, error)
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

	if !cfg.PreserveRevision {
		copy := *accessList
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
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
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
}

// AccessListMembersGetter defines an interface for reading access list members.
type AccessListMembersGetter interface {
	AccessListMemberGetter

	// CountAccessListMembers will count all access list members.
	CountAccessListMembers(ctx context.Context, accessListName string) (membersCount uint32, listCount uint32, err error)
	// ListAccessListMembers returns a paginated list of all access list members.
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	// GetAccessListOwners returns a list of all owners in an Access List, including those inherited from nested Access Lists.
	GetAccessListOwners(ctx context.Context, accessList string) ([]*accesslist.Owner, error)
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

	if !cfg.PreserveRevision {
		copy := *member
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := member.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		member.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		member.SetExpiry(cfg.Expires)
	}
	return &member, nil
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

	if !cfg.PreserveRevision {
		copy := *review
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
		return nil, trace.BadParameter("%s", err)
	}
	if err := review.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		review.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		review.SetExpiry(cfg.Expires)
	}
	return &review, nil
}

// CreateAccessListNextKey creates a pagination token based on the requested sort index name
func CreateAccessListNextKey(al *accesslist.AccessList, indexName string) (string, error) {
	switch indexName {
	case "name":
		return AccessListNameIndexKey(al), nil
	case "auditNextDate":
		return AccessListAuditDateIndexKey(al), nil
	case "title":
		return AccessListTitleIndexKey(al), nil
	default:
		return "", trace.BadParameter("unsupported sort %s but expected name, title or auditNextDate", indexName)
	}
}

// AccessListNameIndexKey returns the resource name returned from GetName().
func AccessListNameIndexKey(al *accesslist.AccessList) string {
	return al.GetName()
}

// AccessListAuditDateIndexKey returns the DateOnly formatted next audit date
// followed by the resource name for disambiguation.
func AccessListAuditDateIndexKey(al *accesslist.AccessList) string {
	if !al.IsReviewable() || al.Spec.Audit.NextAuditDate.IsZero() {
		// Use last lexical character to ensure that ACLs without an audit date
		// appear at the end when sorted. Otherwise we would compare against
		// `0001-01-01 00:00:00` which would sort first, but actually means
		// the access list is not eligible for review.
		return "z/" + al.GetName()
	}
	return al.Spec.Audit.NextAuditDate.Format(time.DateOnly) + "/" + al.GetName()
}

// AccessListTitleIndexKey returns the access list title base32hex encoded
// followed by the resource name for disambiguation.
func AccessListTitleIndexKey(al *accesslist.AccessList) string {
	title := cases.Fold().String(al.Spec.Title)
	title = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(title))
	return title + "/" + al.GetName()
}

// MatchAccessList returns true if the access list matches the given filter criteria.
// The function applies filters in sequence: owners, then roles, then search.
// All provided filters must match for the access list to be included.
//
//   - If owners filter is provided, the access list must have at least one matching owner
//   - If roles filter is provided, the access list must grant at least one matching role
//   - If search filter is provided, all search terms must be found across the access list's
//     title, name, owner names, description, granted roles, and origin fields
//
// All matching is case-insensitive and supports partial matches.
func MatchAccessList(al *accesslist.AccessList, req *accesslistv1.AccessListsFilter) bool {
	if req == nil {
		return true
	}
	search := req.GetSearch()
	owners := req.GetOwners()
	roles := req.GetRoles()
	origin := req.GetOrigin()

	if search == "" && len(owners) == 0 && len(roles) == 0 && origin == "" {
		return true
	}

	// Step 1: Check owner filter if provided
	if len(owners) > 0 {
		ownerMatched := slices.ContainsFunc(owners, func(filterOwner string) bool {
			return slices.ContainsFunc(al.Spec.Owners, func(alOwner accesslist.Owner) bool {
				return strcase.Contains(alOwner.Name, filterOwner)
			})
		})
		if !ownerMatched {
			return false
		}
	}

	// Step 2: Check role filter if provided
	if len(roles) > 0 {
		roleMatched := slices.ContainsFunc(roles, func(filterRole string) bool {
			return slices.ContainsFunc(al.Spec.Grants.Roles, func(alRole string) bool {
				return strcase.Contains(alRole, filterRole)
			})
		})
		if !roleMatched {
			return false
		}
	}

	// Step 3: check the origin
	if origin != "" && !strcase.Contains(al.Origin(), origin) {
		return false
	}

	// Step 4: Check search filter if provided
	if searchTerms := strings.Fields(search); len(searchTerms) > 0 {
		// Check if all search terms are found across the access list fields
		// without creating an intermediate slice
		for _, term := range searchTerms {
			termFound := false

			// Check title
			if strcase.Contains(al.Spec.Title, term) {
				termFound = true
			}

			// Check name
			if !termFound && strcase.Contains(al.GetName(), term) {
				termFound = true
			}

			// Check description
			if !termFound && strcase.Contains(al.Spec.Description, term) {
				termFound = true
			}

			// Check origin
			if !termFound && strcase.Contains(al.Origin(), term) {
				termFound = true
			}

			// Check owner names
			if !termFound {
				for _, owner := range al.Spec.Owners {
					if strcase.Contains(owner.Name, term) {
						termFound = true
						break
					}
				}
			}

			// Check roles
			if !termFound {
				for _, role := range al.Spec.Grants.Roles {
					if strcase.Contains(role, term) {
						termFound = true
						break
					}
				}
			}

			// If this term wasn't found in any field, the search fails
			if !termFound {
				return false
			}
		}
	}

	return true
}
