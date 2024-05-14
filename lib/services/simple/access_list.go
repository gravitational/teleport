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

package simple

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	accessListPrefix      = "access_list"
	accessListMaxPageSize = 100

	accessListMemberPrefix      = "access_list_member"
	accessListMemberMaxPageSize = 200

	accessListReviewPrefix      = "access_list_review"
	accessListReviewMaxPageSize = 200
)

// AccessListService is a simple access list backend service for use specifically by the cache.
type AccessListService struct {
	log           logrus.FieldLogger
	service       *generic.Service[*accesslist.AccessList]
	memberService *generic.Service[*accesslist.AccessListMember]
	reviewService *generic.Service[*accesslist.Review]
}

// NewAccessListService creates a new AccessListService. This is a simple, cache focused
// backend service that doesn't perform any of the validation that the main backend service
// does.
func NewAccessListService(backend backend.Backend) (*AccessListService, error) {
	service, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessList]{
		Backend:       backend,
		PageLimit:     accessListMaxPageSize,
		ResourceKind:  types.KindAccessList,
		BackendPrefix: accessListPrefix,
		MarshalFunc:   services.MarshalAccessList,
		UnmarshalFunc: services.UnmarshalAccessList,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberService, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessListMember]{
		Backend:       backend,
		PageLimit:     accessListMemberMaxPageSize,
		ResourceKind:  types.KindAccessListMember,
		BackendPrefix: accessListMemberPrefix,
		MarshalFunc:   services.MarshalAccessListMember,
		UnmarshalFunc: services.UnmarshalAccessListMember,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviewService, err := generic.NewService(&generic.ServiceConfig[*accesslist.Review]{
		Backend:       backend,
		PageLimit:     accessListReviewMaxPageSize,
		ResourceKind:  types.KindAccessListReview,
		BackendPrefix: accessListReviewPrefix,
		MarshalFunc:   services.MarshalAccessListReview,
		UnmarshalFunc: services.UnmarshalAccessListReview,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		log:           logrus.WithFields(logrus.Fields{teleport.ComponentKey: "access-list:simple-service"}),
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

// ListAccessLists returns a paginated list of access lists.
func (a *AccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	return a.service.ListResources(ctx, pageSize, nextToken)
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	return a.service.GetResource(ctx, name)
}

// UpsertAccessList creates or updates an access list resource.
func (a *AccessListService) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	return a.service.UpsertResource(ctx, accessList)
}

// DeleteAccessList removes the specified access list resource.
func (a *AccessListService) DeleteAccessList(ctx context.Context, name string) error {
	return trace.Wrap(a.service.DeleteResource(ctx, name))
}

// DeleteAllAccessLists removes all access lists.
func (a *AccessListService) DeleteAllAccessLists(ctx context.Context) error {
	return trace.Wrap(a.service.DeleteAllResources(ctx))
}

// CountAccessListMembers will count all access list members.
func (a *AccessListService) CountAccessListMembers(ctx context.Context, accessListName string) (uint32, error) {
	count, err := a.memberService.WithPrefix(accessListName).CountResources(ctx)
	return uint32(count), trace.Wrap(err)
}

// ListAccessListMembers returns a paginated list of all access list members.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	return a.memberService.WithPrefix(accessListName).ListResources(ctx, pageSize, nextToken)
}

// GetAccessListMember returns the specified access list member resource.
func (a *AccessListService) GetAccessListMember(ctx context.Context, accessListName string, memberName string) (*accesslist.AccessListMember, error) {
	return a.memberService.WithPrefix(accessListName).GetResource(ctx, memberName)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (a *AccessListService) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	return a.memberService.WithPrefix(member.Spec.AccessList).UpsertResource(ctx, member)
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (a *AccessListService) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	return trace.Wrap(a.memberService.WithPrefix(accessList).DeleteResource(ctx, memberName))
}

// DeleteAllAccessListMembers hard deletes all access list members.
func (a *AccessListService) DeleteAllAccessListMembers(ctx context.Context) error {
	return trace.Wrap(a.memberService.DeleteAllResources(ctx))
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (a *AccessListService) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	return a.reviewService.WithPrefix(accessList).ListResources(ctx, pageSize, pageToken)
}

// CreateAccessListReview will create a new review for an access list.
func (a *AccessListService) CreateAccessListReview(ctx context.Context, review *accesslist.Review) (*accesslist.Review, time.Time, error) {
	review, err := a.reviewService.WithPrefix(review.Spec.AccessList).CreateResource(ctx, review)
	// Return a zero time here, as it will be ignored by the cache.
	return review, time.Time{}, trace.Wrap(err)
}

// DeleteAccessListReview will delete an access list review from the backend.
func (a *AccessListService) DeleteAccessListReview(ctx context.Context, accessListName, reviewName string) error {
	return trace.Wrap(a.reviewService.WithPrefix(accessListName).DeleteResource(ctx, reviewName))
}

// DeleteAllAccessListReviews will delete all access list reviews from the backend.
func (a *AccessListService) DeleteAllAccessListReviews(ctx context.Context) error {
	return trace.Wrap(a.reviewService.DeleteAllResources(ctx))
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (a *AccessListService) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	members, nextToken, err := a.memberService.ListResources(ctx, pageSize, pageToken)
	return members, nextToken, trace.Wrap(err)
}
