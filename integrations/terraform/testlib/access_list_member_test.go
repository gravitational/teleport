// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testlib

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/accesslist"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/lib/scopes"
)

func checkAccessListMemberExists(ctx context.Context, clt *accesslist.Client, accessList, memberName string, expires time.Time) resource.TestCheckFunc {
	return checkAccessListMemberExistsWithScopes(ctx, clt, "", accessList, "", memberName, "", accessList, expires)
}

func checkAccessListMemberExistsWithScopes(ctx context.Context, clt *accesslist.Client, accessListScope, accessListName, memberScope, memberName, expectedScope, expectedAccessList string, expires time.Time) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		member, err := clt.GetStaticAccessListMemberV2(ctx, accesslistv1.GetStaticAccessListMemberRequest_builder{
			AccessListScope: accessListScope,
			AccessList:      accessListName,
			MemberScope:     memberScope,
			MemberName:      memberName,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}
		if member.GetName() != memberName {
			return trace.CompareFailed("member name mismatch: got %q, want %q", member.GetName(), memberName)
		}
		if member.Spec.Name != memberName {
			return trace.CompareFailed("member spec name mismatch: got %q, want %q", member.Spec.Name, memberName)
		}
		if member.Scope != expectedScope {
			return trace.CompareFailed("member scope mismatch: got %q, want %q", member.Scope, expectedScope)
		}
		if member.Spec.AccessList != expectedAccessList {
			return trace.CompareFailed("member access list mismatch: got %q, want %q", member.Spec.AccessList, expectedAccessList)
		}
		if !member.Spec.Expires.Equal(expires) {
			return trace.CompareFailed("member expiry mismatch: got %q, want %q", member.Spec.Expires, expires)
		}
		return nil
	}
}

func checkScopedListAccessListMemberExists(ctx context.Context, clt *accesslist.Client, scope, accessListName, memberName string, expires time.Time) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		member, err := clt.GetStaticAccessListMemberV2(ctx, accesslistv1.GetStaticAccessListMemberRequest_builder{
			AccessListScope: scope,
			AccessList:      accessListName,
			MemberScope:     scope,
			MemberName:      memberName,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}

		accessListSQN := scopes.QualifiedName{Scope: scope, Name: accessListName}.String()
		memberSQN := scopes.QualifiedName{Scope: scope, Name: memberName}.String()
		if member.GetName() != memberSQN {
			return trace.CompareFailed("member name mismatch: got %q, want %q", member.GetName(), memberSQN)
		}
		if member.Spec.Name != memberSQN {
			return trace.CompareFailed("member spec name mismatch: got %q, want %q", member.Spec.Name, memberSQN)
		}
		if member.Scope != scope {
			return trace.CompareFailed("member scope mismatch: got %q, want %q", member.Scope, scope)
		}
		if member.Spec.AccessList != accessListSQN {
			return trace.CompareFailed("member access list mismatch: got %q, want %q", member.Spec.AccessList, accessListSQN)
		}
		if member.Spec.MembershipKind != "MEMBERSHIP_KIND_SCOPED_LIST" {
			return trace.CompareFailed("member kind mismatch: got %q, want %q", member.Spec.MembershipKind, "MEMBERSHIP_KIND_SCOPED_LIST")
		}
		if !member.Spec.Expires.Equal(expires) {
			return trace.CompareFailed("member expiry mismatch: got %q, want %q", member.Spec.Expires, expires)
		}
		return nil
	}
}

func (s *TerraformSuiteEnterprise) TestAccessListMember() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	ctx := s.T().Context()
	accessListName := "test-member-list"
	memberName := "fighter"
	resourceName := "teleport_access_list_member.fighter"

	checkDestroyed := func(state *terraform.State) error {
		_, err := s.client.AccessListClient().GetStaticAccessListMember(ctx, accessListName, memberName)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	createdExpires := time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedExpires := time.Date(2038, 2, 1, 0, 0, 0, 0, time.UTC)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_list_member_0_create.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "header.metadata.name", memberName),
					resource.TestCheckResourceAttr(resourceName, "spec.access_list", accessListName),
					resource.TestCheckResourceAttr(resourceName, "spec.membership_kind", "1"),
					resource.TestCheckResourceAttr(resourceName, "spec.expires", createdExpires.Format(time.RFC3339)),
					checkAccessListMemberExists(ctx, s.client.AccessListClient(), accessListName, memberName, createdExpires),
				),
			},
			{
				Config:   s.getFixture("access_list_member_0_create.tf"),
				PlanOnly: true,
			},
			{
				Config: s.getFixture("access_list_member_1_update.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "header.metadata.labels.class", "veteran"),
					resource.TestCheckResourceAttr(resourceName, "spec.expires", updatedExpires.Format(time.RFC3339)),
					checkAccessListMemberExists(ctx, s.client.AccessListClient(), accessListName, memberName, updatedExpires),
				),
			},
			{
				Config:   s.getFixture("access_list_member_1_update.tf"),
				PlanOnly: true,
			},
		},
	})
}

func (s *TerraformSuiteEnterpriseScopedResources) TestAccessListMemberScopedAndUnscoped() {
	require.True(s.T(),
		s.teleportFeatures.GetAdvancedAccessWorkflows(),
		"Test requires Advanced Access Workflows",
	)

	ctx := s.T().Context()
	accessListClient := s.client.AccessListClient()

	checkDestroyed := func(state *terraform.State) error {
		for _, tc := range []struct {
			accessListScope string
			accessListName  string
			memberScope     string
			memberName      string
		}{
			{accessListName: "test-unscoped-member-list", memberName: "unscoped-fighter"},
			{accessListScope: "/foo/bar", accessListName: "test-scoped-member-list", memberName: "scoped-fighter"},
			{accessListScope: "/foo/bar", accessListName: "test-scoped-member-list", memberScope: "/foo/bar", memberName: "test-scoped-child-member-list"},
		} {
			_, err := accessListClient.GetStaticAccessListMemberV2(ctx, accesslistv1.GetStaticAccessListMemberRequest_builder{
				AccessListScope: tc.accessListScope,
				AccessList:      tc.accessListName,
				MemberScope:     tc.memberScope,
				MemberName:      tc.memberName,
			}.Build())
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		return nil
	}

	expires := time.Date(2038, 3, 1, 0, 0, 0, 0, time.UTC)

	resource.Test(s.T(), resource.TestCase{
		ProtoV6ProviderFactories: s.terraformProviders,
		CheckDestroy:             checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: s.getFixture("access_list_member_scoped_and_unscoped.tf"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("teleport_access_list_member.unscoped", "header.metadata.name", "unscoped-fighter"),
					resource.TestCheckResourceAttr("teleport_access_list_member.unscoped", "spec.access_list", "test-unscoped-member-list"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped", "header.metadata.name", "scoped-fighter"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped", "spec.access_list", "/foo/bar::test-scoped-member-list"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped", "scope", "/foo/bar"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped_list", "header.metadata.name", "/foo/bar::test-scoped-child-member-list"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped_list", "spec.access_list", "/foo/bar::test-scoped-member-list"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped_list", "spec.membership_kind", "3"),
					resource.TestCheckResourceAttr("teleport_access_list_member.scoped_list", "scope", "/foo/bar"),
					checkAccessListMemberExistsWithScopes(ctx, accessListClient, "", "test-unscoped-member-list", "", "unscoped-fighter", "", "test-unscoped-member-list", expires),
					checkAccessListMemberExistsWithScopes(ctx, accessListClient, "/foo/bar", "test-scoped-member-list", "", "scoped-fighter", "/foo/bar", "/foo/bar::test-scoped-member-list", expires),
					checkScopedListAccessListMemberExists(ctx, accessListClient, "/foo/bar", "test-scoped-member-list", "test-scoped-child-member-list", expires),
				),
			},
			{
				Config:   s.getFixture("access_list_member_scoped_and_unscoped.tf"),
				PlanOnly: true,
			},
		},
	})
}
