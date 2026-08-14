package auditlogs

import (
	"context"
	"maps"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

var adminRoles = map[string]bool{
	"SUPER_ADMIN":                 true,
	"ORG_ADMIN":                   true,
	"GROUP_ADMIN":                 true,
	"USER_ADMIN":                  true,
	"APP_ADMIN":                   true,
	"API_ACCESS_MANAGEMENT_ADMIN": true,
	"READ_ONLY_ADMIN":             true,
	"HELP_DESK_ADMIN":             true,
	"REPORT_ADMIN":                true,
	"MOBILE_ADMIN":                true,
}

type pollResults struct {
	tokens          []*accessgraphv1alpha.OktaTokenV1
	roles           []*accessgraphv1alpha.OktaRoleV1
	roleAssignments []*accessgraphv1alpha.OktaRoleAssignmentV1
}

func (s *Service) fetchOktaState(ctx context.Context) (*pollResults, error) {
	// List API tokens
	tokens, rsp, err := s.client.ListApiTokens(ctx, nil)
	out := &pollResults{}
	for {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, apiToken := range tokens {
			out.tokens = append(out.tokens, normalizeApiToken(apiToken, s.orgURL))
		}
		if !rsp.HasNextPage() {
			break
		}
		rsp, err = rsp.Next(ctx, &tokens)
	}

	// Get role assignments
	rolesMap := make(map[string]*accessgraphv1alpha.OktaRoleV1)
	var roleAssignments []*accessgraphv1alpha.OktaRoleAssignmentV1
	users, rsp, err := s.client.ListUsersWithRoleAssignments(ctx)
	for {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, user := range users.Value {
			if user.Id == nil {
				continue
			}
			userID := *user.Id
			roles, rolesResp, err := s.client.ListAssignedRolesForUser(ctx, userID)
			for {
				if err != nil {
					return nil, trace.Wrap(err)
				}

				for _, role := range roles {
					if !adminRoles[role.Type] {
						continue
					}
					roleAssignments = append(roleAssignments, accessgraphv1alpha.OktaRoleAssignmentV1_builder{
						RoleId:       role.Type,
						UserId:       userID,
						Organization: s.orgURL,
					}.Build())
					_, ok := rolesMap[role.Type]
					if !ok {
						oktaRole := accessgraphv1alpha.OktaRoleV1_builder{
							RoleId:       role.Type,
							Type:         role.Type,
							Organization: s.orgURL,
						}.Build()
						rolesMap[role.Type] = oktaRole
					}
				}
				if !rolesResp.HasNextPage() {
					break
				}
				rolesResp, err = rolesResp.Next(ctx, &roles)
			}

		}
		if !rsp.HasNextPage() {
			break
		}
		rsp, err = rsp.Next(ctx, &users)
	}
	out.roles = slices.Collect(maps.Values(rolesMap))
	out.roleAssignments = roleAssignments
	return out, nil
}

func normalizeApiToken(apiToken *oktaapi.ApiToken, organization string) *accessgraphv1alpha.OktaTokenV1 {
	token := accessgraphv1alpha.OktaTokenV1_builder{
		Name:         apiToken.Name,
		Organization: organization,
	}.Build()
	if apiToken.Id != nil {
		token.SetId(*apiToken.Id)
	}
	if apiToken.UserId != nil {
		token.SetOwner(*apiToken.UserId)
	}
	if apiToken.Created != nil {
		token.SetCreated(timestamppb.New(*apiToken.Created))
	}
	if apiToken.LastUpdated != nil {
		token.SetUpdated(timestamppb.New(*apiToken.LastUpdated))
	}
	if apiToken.ExpiresAt != nil {
		token.SetExpires(timestamppb.New(*apiToken.ExpiresAt))
	}
	return token
}
