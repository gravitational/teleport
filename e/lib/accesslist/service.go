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
	"github.com/gravitational/teleport/lib/services"
)

// ignoreFieldsDuringUpsert will be used to ignore fields that are allowed to be modified
// during upsert.
var ignoreFieldsDuringUpsert = []cmp.Option{
	// ID is handled by the backend, so it'll be ignored here.
	cmpopts.IgnoreFields(header.Metadata{}, "ID"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "MembershipRequires"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "Audit"),
	cmpopts.IgnoreFields(accesslist.Spec{}, "Members"),
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
	results, getErr := s.accessLists.GetAccessLists(ctx)
	_, authErr := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)

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
			if err := services.IsOwner(identity, result); err == nil {
				filteredResults = append(filteredResults, result)
			} else if err := services.IsMember(identity, s.clock, result); err == nil {
				// Clear out membership information of the user is only a member of the list.
				result.Spec.Members = nil
				filteredResults = append(filteredResults, result)
			}
		}

		// The user owns no access lists and received an auth err earlier.
		if len(filteredResults) == 0 {
			return nil, trace.Wrap(authErr)
		}

		results = filteredResults
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.GetAccessListsResponse{
		AccessLists: accessLists,
	}, nil
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	result, getErr := s.accessLists.GetAccessList(ctx, req.GetName())

	isOwner := true
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
		if ownerErr := services.IsOwner(identity, result); ownerErr != nil {
			if memberErr := services.IsMember(identity, s.clock, result); memberErr != nil {
				return nil, trace.Wrap(authErr)
			}
			isOwner = false
		}
	}

	// We've confirmed that the user should have access to this, so now it's okay to return the
	// get error.
	if getErr != nil {
		return nil, trace.Wrap(getErr)
	}

	// If the user is a member, we'll strip off the membership so that the user can't see
	// who belongs to this list.
	if !isOwner {
		result.Spec.Members = nil
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
		if ownerErr := services.IsOwner(identity, oldAccessList); ownerErr != nil {
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

	oldMembersLookup := map[string]bool{}
	if oldAccessList != nil {
		for _, member := range oldAccessList.Spec.Members {
			oldMembersLookup[member.Name] = true
		}
	}

	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberExists := make(map[string]bool, len(newAccessList.Spec.Members))
	currentTime := s.clock.Now()
	// Adjust any new members to make sure that they're added by the user and the joined time is set to now.
	for i, member := range newAccessList.Spec.Members {
		if memberExists[member.Name] {
			return nil, trace.BadParameter("duplicate user in member list: %s", member.Name)
		}
		memberExists[member.Name] = true

		if !oldMembersLookup[member.Name] {
			newAccessList.Spec.Members[i].AddedBy = user.GetIdentity().Username
			newAccessList.Spec.Members[i].Joined = currentTime
		}
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
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.accessLists.DeleteAllAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
