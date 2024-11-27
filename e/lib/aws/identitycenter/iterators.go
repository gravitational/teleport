package identitycenter

import (
	"context"
	"fmt"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type permissionSetLister interface {
	ListPermissionSets(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PermissionSet, pagination.NextPageToken, error)
}

func allPermissionSets(ctx context.Context, src permissionSetLister) iter.Seq2[*identitycenterv1.PermissionSet, error] {
	const pageSize = 50

	return func(yield func(*identitycenterv1.PermissionSet, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pss, nextPage, err := src.ListPermissionSets(ctx, pageSize, &pageToken)
			if err != nil {
				yield(nil, trace.Wrap(err, "iterating over all permission sets"))
				return
			}

			for _, ps := range pss {
				if !yield(ps, nil) {
					return
				}
			}

			if nextPage == pagination.EndOfList {
				break
			}
			pageToken.Update(nextPage)
		}
	}
}

func allAccountAssignments(ctx context.Context, src services.IdentityCenterAccountAssignments) iter.Seq2[services.IdentityCenterAccountAssignment, error] {
	const pageSize = 100

	return func(yield func(services.IdentityCenterAccountAssignment, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pageItems, nextPage, err := src.ListAccountAssignments(ctx, pageSize, &pageToken)
			if err != nil {
				yield(services.IdentityCenterAccountAssignment{},
					trace.Wrap(err, "enumerating identity center account assignment resources"))
				return
			}

			for _, asmt := range pageItems {
				if !yield(asmt, nil) {
					return
				}
			}

			if nextPage == pagination.EndOfList {
				break
			}
			pageToken.Update(nextPage)
		}
	}
}

func allAccounts(ctx context.Context, src services.IdentityCenterAccounts) iter.Seq2[services.IdentityCenterAccount, error] {
	const pageSize = 100

	return func(yield func(services.IdentityCenterAccount, error) bool) {
		var pageToken pagination.PageRequestToken
		for {
			pageItems, nextPage, err := src.ListIdentityCenterAccounts(ctx, pageSize, &pageToken)
			if err != nil {
				yield(services.IdentityCenterAccount{},
					trace.Wrap(err, "enumerating identity center account resources"))
				return
			}

			for _, asmt := range pageItems {
				if !yield(asmt, nil) {
					return
				}
			}

			if nextPage == pagination.EndOfList {
				break
			}
			pageToken.Update(nextPage)
		}
	}
}

// listTeleportUsers returns a map with a key containing username for each users
// that exist in Teleport user database.
func listTeleportUsers(ctx context.Context, service UsersService) (map[string]struct{}, error) {
	var users []types.User

	req := &usersv1.ListUsersRequest{
		PageSize:    apidefaults.DefaultChunkSize,
		WithSecrets: false,
	}
	for {
		resp, err := service.ListUsers(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, user := range resp.Users {
			users = append(users, user)
		}

		req.PageToken = resp.NextPageToken
		if req.PageToken == "" {
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
	var pageKey string
	var out []*types.RoleV6
	for {
		response, err := service.ListRoles(ctx, &proto.ListRolesRequest{
			StartKey: pageKey,
			Limit:    apidefaults.DefaultChunkSize,
			Filter:   &types.RoleFilter{SkipSystemRoles: true},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, role := range response.Roles {
			if matchByOriginAWSIdentityCenterLabel(role) {
				out = append(out, role)
			}
		}

		if response.NextKey == "" {
			break
		}
		pageKey = response.NextKey
	}

	return out, nil
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
