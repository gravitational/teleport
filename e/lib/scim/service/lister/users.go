package lister

import (
	"context"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	scimfilter "github.com/gravitational/teleport/e/lib/scim/service/filter"
)

// UserLister handles SCIM-compatible listing of Teleport users.
//
// It applies filtering and pagination according to the SCIM protocol.
type UserLister struct {
	common.Config
	// Predicate determines if a given user is eligible for SCIM inclusion.
	Predicate func(context.Context, types.User) bool
	// UserToResource converts a Teleport user into a SCIM resource.
	UserToResource func(user types.User) (*scimpb.Resource, error)
}

// ListResources returns an SCIM-compliant paginated list of users,
// applying any filtering and transformation required.
func (l *UserLister) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	filter, err := scimfilter.ParseFilter(req.GetFilter())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		startIndex      = int(req.GetPage().GetStartIndex())
		count           = int(req.GetPage().GetCount())
		currentIndex    = 0
		scimUserResults []*scimpb.Resource
	)

	err = l.forEachUser(ctx, func(user types.User) error {
		if !l.Predicate(ctx, user) {
			return nil
		}
		// Filter attributes by SCIM criteria
		matchAttrs := map[string]string{
			common.UsernameAttribute: user.GetName(),
		}
		if err := scimfilter.EvaluateFilter(filter, matchAttrs); err != nil {
			return nil
		}
		currentIndex++
		if currentIndex < startIndex {
			return nil
		}
		if len(scimUserResults) < count {
			resource, err := l.UserToResource(user)
			if err != nil {
				return trace.Wrap(err)
			}
			scimUserResults = append(scimUserResults, resource)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return scimpb.ResourceList_builder{
		TotalResults: int32(currentIndex),
		StartIndex:   int32(startIndex),
		ItemsPerPage: int32(len(scimUserResults)),
		Resources:    scimUserResults,
	}.Build(), nil
}

// forEachUser iterates through all Teleport users in a paginated manner
// and applies the provided function `fn` to each user.
func (l *UserLister) forEachUser(ctx context.Context, fn func(user types.User) error) error {
	for user, err := range clientutils.Resources(ctx, func(ctx context.Context, pageSize int, startKey string) ([]*types.UserV2, string, error) {
		resp, err := l.ListUsers(ctx, userspb.ListUsersRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: startKey,
		}.Build())

		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		return resp.GetUsers(), resp.GetNextPageToken(), nil
	}) {
		if err != nil {
			return trace.Wrap(err)
		}

		if err := fn(user); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
