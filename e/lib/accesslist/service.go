// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// defaultAccessListPageSize is the default page size to be used.
	defaultAccessListPageSize = 100
)

// ignoreFieldsDuringUpsert will be used to ignore fields that are allowed to be modified
// during upsert.
var ignoreFieldsDuringUpsert = []cmp.Option{
	// ID is handled by the backend, so it'll be ignored here.
	cmpopts.IgnoreFields(header.Metadata{}, "ID"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "MembershipRequires"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "Audit"),
}

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// AccessLists is the access list service to use.
	AccessLists services.AccessLists

	// Clock is the clock.
	Clock clockwork.Clock
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	if c.AccessLists == nil {
		return trace.BadParameter("accesslists service is missing")
	}

	if c.Logger == nil {
		c.Logger = logrus.New().WithField(trace.Component, "access_list_crud_service")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

type Service struct {
	accesslistv1.UnimplementedAccessListServiceServer

	log         logrus.FieldLogger
	authorizer  authz.Authorizer
	accessLists services.AccessLists
	clock       clockwork.Clock
}

// NewService creates a new Access List gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:         cfg.Logger,
		authorizer:  cfg.Authorizer,
		accessLists: cfg.AccessLists,
		clock:       cfg.Clock,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (s *Service) GetAccessLists(ctx context.Context, _ *accesslistv1.GetAccessListsRequest) (*accesslistv1.GetAccessListsResponse, error) {
	// We don't return these errors right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	results, getErr := s.accessLists.GetAccessLists(ctx)
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)

	var err error
	results, err = s.filterResults(ctx, results, false, getErr, authErr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.GetAccessListsResponse{
		AccessLists: accessLists,
	}, nil
}

// ListAccessLists returns a paginated list of all access lists.
func (s *Service) ListAccessLists(ctx context.Context, req *accesslistv1.ListAccessListsRequest) (*accesslistv1.ListAccessListsResponse, error) {
	pageSize := int(req.PageSize)

	if pageSize == 0 {
		pageSize = defaultAccessListPageSize
	}
	// We don't return the auth error right away because this endpoint can still return results based on the calling user's
	// ownership/membership to particular access lists.
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)

	var results []*accesslist.AccessList
	nextToken := req.NextToken
	for {
		var page []*accesslist.AccessList
		var getErr error
		page, nextToken, getErr = s.accessLists.ListAccessLists(ctx, 0 /* default page size in backend */, nextToken)

		var err error
		page, err = s.filterResults(ctx, page, true, getErr, authErr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		results = append(results, page...)
		if len(results) >= (pageSize+1) || nextToken == "" {
			break
		}
	}

	// Truncate the results.
	if len(results) > pageSize {
		nextToken = backend.GetPaginationKey(results[pageSize])
		results = results[:pageSize]
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.ListAccessListsResponse{
		AccessLists: accessLists,
		NextToken:   nextToken,
	}, nil
}

// filterResults will return the following:
// * If the user has RBAC access to the access lists (authErr == nil), the access lists will be returned as is.
// * If the user owns any access lists, these will be returned with membership information retained.
// * IF the user is a member of any access lists, these will be returned with membership information stripped.
func (s *Service) filterResults(ctx context.Context, results []*accesslist.AccessList, isPaginated bool, getErr, authErr error) ([]*accesslist.AccessList, error) {
	if getErr != nil && authErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, trace.Wrap(authErr)
	} else if authErr != nil {
		// We successfully got the access lists but had an issue authorizing. Check to see if the user is an
		// owner for any of these lists.
		var filteredResults []*accesslist.AccessList

		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := authCtx.Identity.GetIdentity()
		for _, result := range results {
			if err := services.IsAccessListOwner(identity, result); err == nil {
				filteredResults = append(filteredResults, result)
			} else if err := services.IsAccessListMember(ctx, identity, s.clock, result, s.accessLists); err == nil {
				filteredResults = append(filteredResults, result)
			}
		}

		results = filteredResults
	}

	// The user owns no access lists and received an auth err earlier. Also, we're not looking
	// at paginated lists.
	if len(results) == 0 && isPaginated {
		return nil, trace.Wrap(authErr)
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	return results, nil
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	result, getErr := s.accessLists.GetAccessList(ctx, req.GetName())

	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead)
	if getErr != nil && authErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, trace.Wrap(authErr)
	} else if authErr != nil {
		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := authCtx.Identity.GetIdentity()
		// Check if the user's an owner. If not, then we'll check if the user is a member. If neither are
		// true, we'll return the original auth error.
		if ownerErr := services.IsAccessListOwner(identity, result); ownerErr != nil {
			if memberErr := services.IsAccessListMember(ctx, identity, s.clock, result, s.accessLists); memberErr != nil {
				return nil, trace.Wrap(authErr)
			}
		}
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	return conv.ToProto(result), nil
}

// UpsertAccessList creates or updates an access list resource.
func (s *Service) UpsertAccessList(ctx context.Context, req *accesslistv1.UpsertAccessListRequest) (*accesslistv1.AccessList, error) {
	newAccessList, err := conv.FromProto(req.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oldAccessList, getErr := s.accessLists.GetAccessList(ctx, req.GetAccessList().GetHeader().Metadata.Name)

	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbCreate, types.VerbUpdate)
	// Check if the user is the owner of the list.
	if authErr != nil && getErr != nil {
		// There was an error getting the access lists and an auth error, so return the auth error.
		return nil, trace.Wrap(authErr)
	} else if authErr != nil {
		authCtx, err := s.authorizer.Authorize(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity := authCtx.Identity.GetIdentity()
		if ownerErr := services.IsAccessListOwner(identity, oldAccessList); ownerErr != nil {
			// The user does not own this list, so return the original auth error.
			return nil, trace.Wrap(authErr)
		}

		// Owners are only allowed to modify members, membership_requires, and audit interval.
		if !cmp.Equal(newAccessList, oldAccessList, ignoreFieldsDuringUpsert...) {
			return nil, trace.AccessDenied("owners can only modify audit, members, and membership_requires")
		}
	}

	if getErr != nil && !trace.IsNotFound(getErr) {
		return nil, trace.Wrap(getErr)
	}

	responseAccessList, err := s.accessLists.UpsertAccessList(ctx, newAccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(responseAccessList), nil
}

// DeleteAccessList removes the specified access list resource.
func (s *Service) DeleteAccessList(ctx context.Context, req *accesslistv1.DeleteAccessListRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.accessLists.DeleteAccessList(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllAccessLists removes all access lists.
func (s *Service) DeleteAllAccessLists(ctx context.Context, _ *accesslistv1.DeleteAllAccessListsRequest) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("DeleteAllAccessLists not supported in the gRPC server")
}

// ListAccessListMembers returns a paginated list of all access list members.
func (s *Service) ListAccessListMembers(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := s.accessLists.ListAccessListMembers(ctx, req.AccessList, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	members := make([]*accesslistv1.Member, len(results))
	for i, r := range results {
		members[i] = conv.ToMemberProto(r)
	}

	return &accesslistv1.ListAccessListMembersResponse{
		Members:       members,
		NextPageToken: nextToken,
	}, nil
}

// GetAccessListMember returns the specified access list member resource.
func (s *Service) GetAccessListMember(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslistv1.Member, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := s.accessLists.GetAccessListMember(ctx, req.AccessList, req.MemberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), nil
}

// UpsertAccessListMember creates or updates an access list member resource.
func (s *Service) UpsertAccessListMember(ctx context.Context, req *accesslistv1.UpsertAccessListMemberRequest) (*accesslistv1.Member, error) {
	if err := s.authOrIsOwner(ctx, req.Member.Spec.AccessList, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := conv.FromMemberProto(req.Member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the user didn't exist before, make sure the current user is recorded as the user that added it.
	if oldMember, err := s.accessLists.GetAccessListMember(ctx, member.Spec.AccessList, member.GetName()); trace.IsNotFound(err) {
		user, err := authz.UserFromContext(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		member.Spec.AddedBy = user.GetIdentity().Username
		member.Spec.Joined = s.clock.Now()
	} else {
		// If the user already existed, use the old added by, reason, and joined.
		member.Spec.AddedBy = oldMember.Spec.AddedBy
		member.Spec.Joined = oldMember.Spec.Joined
	}

	result, err := s.accessLists.UpsertAccessListMember(ctx, member)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToMemberProto(result), nil
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (s *Service) DeleteAccessListMember(ctx context.Context, req *accesslistv1.DeleteAccessListMemberRequest) (*emptypb.Empty, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err := s.accessLists.DeleteAccessListMember(ctx, req.AccessList, req.MemberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list (without deleting the access list itself).
func (s *Service) DeleteAllAccessListMembersForAccessList(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersForAccessListRequest) (*emptypb.Empty, error) {
	if err := s.authOrIsOwner(ctx, req.AccessList, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err := s.accessLists.DeleteAllAccessListMembersForAccessList(ctx, req.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllAccessListMembers hard deletes all access list members for all access lists (without deleting the access lists themselves).
func (s *Service) DeleteAllAccessListMembers(ctx context.Context, req *accesslistv1.DeleteAllAccessListMembersRequest) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("DeleteAllAccessListMembers not supported in the gRPC service")
}

// Check if the user is either authorized for the access list or owns this access list.
func (s *Service) authOrIsOwner(ctx context.Context, accessListName string, verbs ...string) error {
	// Make sure the user is authorized within Teleport.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.log.WithError(err).Debug("Failed to authorize user")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}

	// Test if the user has RBAC access to access lists. If so, we can exit early.
	_, authErr := authz.AuthorizeContextWithVerbs(ctx, s.log, authCtx, true, ruleCtx, types.KindAccessList, verbs...)
	if authErr == nil {
		return nil
	}

	// Otherwise, we need to check if the user owns the access list.
	identity := authCtx.Identity.GetIdentity()

	accessList, err := s.accessLists.GetAccessList(ctx, accessListName)
	if err != nil {
		s.log.WithError(err).Debug("Failed to get access list")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	if err := services.IsAccessListOwner(identity, accessList); err != nil {
		s.log.WithError(err).Debug("IsOwner returned error")
		// Return an opaque error
		return trace.AccessDenied("access denied")
	}

	return nil
}
