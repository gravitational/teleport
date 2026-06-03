package directory

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func listTeleportAccessListsWithMembers(ctx context.Context, svc accessPoint) (map[string]*accessListWithMembers, error) {
	aclsWithMembersMap := make(map[string]*accessListWithMembers)

	for al, err := range clientutils.Resources(ctx, svc.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err, "listing access lists")
		}
		if !matchByLabel(al) {
			continue
		}
		listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			r, token, err := svc.ListAccessListMembers(ctx, al.GetName(), pageSize, pageToken)
			return r, token, trace.Wrap(err)
		}
		var members []*accesslist.AccessListMember
		members, err := stream.Collect(clientutils.Resources(ctx, listMembersFn))
		if err != nil {
			return nil, trace.Wrap(err, "listing access list %q members", al.GetName())
		}
		aclsWithMembersMap[al.GetName()] = &accessListWithMembers{
			AccessList: al,
			Members:    members,
		}
	}

	return aclsWithMembersMap, nil
}

func listTeleportUsers(ctx context.Context, svc accessPoint, connectorID string) (map[string]types.User, error) {
	result := map[string]types.User{}

	var pageToken string
	for {
		resp, err := svc.ListUsers(ctx, &userspb.ListUsersRequest{PageToken: pageToken})
		if err != nil {
			return nil, trace.Wrap(err, "listing teleport entra users")
		}

		for _, user := range resp.Users {
			if matchByLabel(user) {
				result[user.GetName()] = user
				continue
			}

			// Fallback to match by connector since it's possible for a
			// user to log in to Teleport before their account is created by the
			// plugin. In such a case, user account gets created by the SAML
			// connector without the origin label assigned to the user resource.
			// Note: All users created by the integration also have the "CreatedBy"
			// field and will be matched with the [matchByConnector]. This arguably
			// makes the origin checker redundant. But the origin checker gets
			// precedence for now until its decided if we should consolidate to use
			// connector matcher as default and the only supported matcher.
			if matchByConnector(user.GetCreatedBy().Connector, connectorID) {
				slog.InfoContext(ctx, "User account found to be created by the referenced Entra ID SAML connector, overwriting", "user", user.GetName())
				result[user.GetName()] = user
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return result, nil
}
