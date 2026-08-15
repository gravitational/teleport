/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
)

// TestGetAccessListsForReview tests the "teleport-get-access-lists-for-review" MCP tool.
func TestGetAccessListsForReview(t *testing.T) {
	env := newMCPTestEnv(t)

	ownerRole1 := createRole(t, env.ctx, env.authClient, "owner-role-1")
	memberRole1 := createRole(t, env.ctx, env.authClient, "member-role-1")

	nextAudit := time.Now().UTC().Add(24 * time.Hour)
	list, member, review := createAccessList(t, env.ctx, env.authClient, accessListConfig{
		name:        "access-list-1",
		owner:       "alice",
		ownerRoles:  []string{ownerRole1.GetName()},
		member:      "bob",
		memberRoles: []string{memberRole1.GetName()},
		nextAudit:   nextAudit,
	})

	res, err := env.mcpClient.CallTool(env.ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: ToolAccessListsForReview,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Content))
	textContent, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var details []accessListDetails
	err = json.Unmarshal([]byte(textContent.Text), &details)
	require.NoError(t, err)
	require.Len(t, details, 1)

	require.Equal(t, []accessListDetails{{
		Name:        list.GetName(),
		Description: list.GetMetadata().Description,
		Labels:      list.GetAllLabels(),
		Title:       list.Spec.Title,
		Owners:      []accesslist.Owner{{Name: "alice", MembershipKind: accesslist.MembershipKindUser}},
		Audit:       list.Spec.Audit,
		MemberRoles: []*types.RoleV6{memberRole1},
		Members:     []*accesslist.AccessListMember{member},
		Reviews:     []*accesslist.Review{review},
		URL:         "https://mcp.test/web/accesslists/" + list.GetName(),
	}}, details)
}

// TestReviewAccessList tests the "teleport-review-access-list" MCP tool.
func TestReviewAccessList(t *testing.T) {
	env := newMCPTestEnv(t)

	ownerRole := createRole(t, env.ctx, env.authClient, "owner-role-review")
	memberRole := createRole(t, env.ctx, env.authClient, "member-role-review")

	list, _, _ := createAccessList(t, env.ctx, env.authClient, accessListConfig{
		name:        "access-list-1",
		owner:       "alice",
		ownerRoles:  []string{ownerRole.GetName()},
		member:      "bob",
		memberRoles: []string{memberRole.GetName()},
		skipReview:  true,
	})

	_, err := env.mcpClient.CallTool(env.ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: ToolReviewAccessList,
			Arguments: map[string]any{
				"name": list.GetName(),
			},
		},
	})
	require.NoError(t, err)

	reviews, _, err := env.authClient.AccessListClient().ListAccessListReviews(env.ctx, list.GetName(), 10, "")
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	require.Equal(t, list.GetName(), reviews[0].Spec.AccessList)
	require.Equal(t, "Reviewed via MCP", reviews[0].Spec.Notes)
}

type accessListConfig struct {
	name        string
	owner       string
	ownerRoles  []string
	member      string
	memberRoles []string
	nextAudit   time.Time
	skipReview  bool
}

func createRole(t *testing.T, ctx context.Context, clt authclient.ClientI, name string) *types.RoleV6 {
	role, err := authtest.CreateRole(ctx, clt, name, types.RoleSpecV6{})
	require.NoError(t, err)
	roleV6, ok := role.(*types.RoleV6)
	require.True(t, ok, "expected role to be RoleV6, got %T", role)
	// Reset namespaces since they are ignored during unmarshal and otherwise
	// comparison in the tests would fail.
	roleV6.Metadata.Namespace = ""
	roleV6.Spec.Allow.Namespaces = []string(nil)
	roleV6.Spec.Deny.Namespaces = []string(nil)
	return roleV6
}

func createAccessList(t *testing.T, ctx context.Context, authClient authclient.ClientI, cfg accessListConfig) (*accesslist.AccessList, *accesslist.AccessListMember, *accesslist.Review) {
	list, err := accesslist.NewAccessList(header.Metadata{
		Name:        cfg.name,
		Description: cfg.name + " description",
		Labels: map[string]string{
			"name": cfg.name,
		},
	}, accesslist.Spec{
		Title: cfg.name + " title",
		Audit: accesslist.Audit{
			NextAuditDate: cfg.nextAudit,
			Recurrence: accesslist.Recurrence{
				Frequency:  accesslist.SixMonths,
				DayOfMonth: accesslist.FirstDayOfMonth,
			},
			Notifications: accesslist.Notifications{
				Start: 24 * time.Hour,
			},
		},
		Owners: []accesslist.Owner{{Name: cfg.owner}},
		OwnerGrants: accesslist.Grants{
			Roles: cfg.ownerRoles,
		},
		Grants: accesslist.Grants{
			Roles: cfg.memberRoles,
		},
	})
	require.NoError(t, err)

	list, err = authClient.AccessListClient().UpsertAccessList(ctx, list)
	require.NoError(t, err)

	member, err := accesslist.NewAccessListMember(header.Metadata{
		Name: cfg.member,
	}, accesslist.AccessListMemberSpec{
		AccessList: list.GetName(),
		Name:       cfg.member,
	})
	require.NoError(t, err)

	member, err = authClient.AccessListClient().UpsertAccessListMember(ctx, member)
	require.NoError(t, err)

	var review *accesslist.Review
	if !cfg.skipReview {
		review, err = accesslist.NewReview(header.Metadata{
			Name: cfg.name + "-review",
		}, accesslist.ReviewSpec{
			AccessList: list.GetName(),
			ReviewDate: time.Now().UTC(),
			Reviewers:  []string{"test"},
			Notes:      "looks good",
		})
		require.NoError(t, err)

		_, _, err = authClient.AccessListClient().CreateAccessListReview(ctx, review)
		require.NoError(t, err)
	}

	return list, member, review
}
