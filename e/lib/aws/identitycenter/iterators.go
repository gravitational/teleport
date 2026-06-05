package identitycenter

import (
	"context"
	"fmt"
	"iter"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type permissionSetLister interface {
	ListPermissionSets(context.Context, int, string) ([]*identitycenterv1.PermissionSet, string, error)
}

func allPermissionSets(ctx context.Context, src permissionSetLister) iter.Seq2[*identitycenterv1.PermissionSet, error] {
	return clientutils.Resources(ctx, src.ListPermissionSets)
}

func allAccountAssignments(ctx context.Context, src services.IdentityCenterAccountAssignments) iter.Seq2[*identitycenterv1.AccountAssignment, error] {
	return clientutils.Resources(ctx, src.ListIdentityCenterAccountAssignments)
}

func allAccounts(ctx context.Context, src services.IdentityCenterAccounts) iter.Seq2[*identitycenterv1.Account, error] {
	return clientutils.Resources(ctx, src.ListIdentityCenterAccounts)
}

// listTeleportUsers returns a map with a key containing username for each users
// that exist in Teleport user database.
func listTeleportUsers(ctx context.Context, service UsersService) (map[string]struct{}, error) {
	var users []types.User

	req := usersv1.ListUsersRequest_builder{
		PageSize:    apidefaults.DefaultChunkSize,
		WithSecrets: false,
	}.Build()
	for {
		resp, err := service.ListUsers(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, user := range resp.GetUsers() {
			users = append(users, user)
		}

		req.SetPageToken(resp.GetNextPageToken())
		if req.GetPageToken() == "" {
			break
		}
	}

	out := make(map[string]struct{})
	for _, u := range users {
		out[u.GetName()] = struct{}{}
	}

	return out, nil
}

// ListICOriginatedAccessLists lists all Identity Center originated access lists.
func ListICOriginatedAccessLists(ctx context.Context, service AccessListsService) (map[string]*accesslist.AccessList, error) {
	outList := map[string]*accesslist.AccessList{}

	var accessLists []*accesslist.AccessList
	var pageToken string
	var err error
	for {
		accessLists, pageToken, err = service.ListAccessLists(ctx, apidefaults.DefaultChunkSize, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, al := range accessLists {
			if matchByOriginAWSIdentityCenterLabel(al) {
				outList[al.GetName()] = al
			}
		}

		if pageToken == "" {
			break
		}
	}

	return outList, nil
}

// ListICOriginatedRoles lists all Identity Center originated roles.
func ListICOriginatedRoles(ctx context.Context, service RolesService) ([]*types.RoleV6, error) {
	return stream.Collect(iciter.AllAccountAssignmentRoles(ctx, service))
}

// accessListMembersFromTeleport returns all existing members for each accessListNames.
func accessListMembersFromTeleport(ctx context.Context, accessListNames []string, service AccessListsService) (map[string]*accesslist.AccessListMember, error) {
	out := map[string]*accesslist.AccessListMember{}

	var members []*accesslist.AccessListMember
	var pageToken string
	var err error
	for _, acl := range accessListNames {
		for {
			members, pageToken, err = service.ListAccessListMembers(ctx, acl, apidefaults.DefaultChunkSize, pageToken)
			if err != nil {
				return nil, trace.Wrap(err, "listing existing Access List members from Teleport")
			}

			for _, m := range members {
				out[memberMapKey(m)] = m
			}
			if pageToken == "" {
				break
			}
		}
	}

	return out, nil
}

func memberMapKey(member *accesslist.AccessListMember) string {
	return fmt.Sprintf("%s/%s", member.Spec.AccessList, member.GetName())
}
