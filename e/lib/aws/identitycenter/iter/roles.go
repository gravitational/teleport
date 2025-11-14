package iter

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// RolesLister is an abstraction over listing roles
type RolesLister interface {
	// ListRoles returns a list of roles registered with the local cluster
	ListRoles(context.Context, *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
}

// AllAccountAssignmentRoles returns a single-use iterator over sequence of
// (*types.RoleV6, error) pairs. If an error is encountered listing the roles
// then the sequence will yield a non-nil error value and the sequence will end
// immediately.
func AllAccountAssignmentRoles(ctx context.Context, svc RolesLister) iter.Seq2[*types.RoleV6, error] {
	const pageSize = 200
	return AllAccountAssignmentRolesWithPageSize(ctx, svc, pageSize)
}

// AllAccountAssignmentRoles returns a single-use iterator over sequence of
// (*types.RoleV6, error) pairs. If an error is encountered listing the roles
// then the sequence will yield a non-nil error value and the sequence will end
// immediately.
func AllAccountAssignmentRolesWithPageSize(ctx context.Context, svc RolesLister, pageSize int) stream.Stream[*types.RoleV6] {
	getPage := func(ctx context.Context, pageSize int, pageToken string) ([]*types.RoleV6, string, error) {
		response, err := svc.ListRoles(ctx, &proto.ListRolesRequest{
			StartKey: pageToken,
			Limit:    int32(pageSize),
			Filter:   &types.RoleFilter{SkipSystemRoles: true},
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return response.Roles, response.NextKey, nil
	}

	isAccountAssignmentRole := func(role *types.RoleV6) (*types.RoleV6, bool) {
		return role, role.GetSubKind() == types.KindIdentityCenter
	}

	return stream.FilterMap(
		clientutils.ResourcesWithPageSize(ctx, getPage, pageSize),
		isAccountAssignmentRole)
}
