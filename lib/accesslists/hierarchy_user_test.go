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
package accesslists_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestGetHierarchyForUser(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name   string
		start  string
		state  state
		traits trait.Traits
		user   types.User
		kind   accesslists.RelationshipKind
		want   []string
	}{
		{
			name: "access list hierarchy for user",
			state: state{
				mustMakeAccessList("root"):   {mustCreateMember("level1", withACLMemKind())},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  []string{"root", "level1", "level2"},
		},
		{
			name: "member/expired direct user membership => empty",
			state: state{
				mustMakeAccessList("root"):   {mustCreateMember("level1", withACLMemKind())},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice", withExpiration(clock.Now().Add(-time.Hour)))},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  nil,
		},
		{
			name: "member/stops at expired parent edge",
			state: state{
				mustMakeAccessList("root"):   {mustCreateMember("level1", withACLMemKind(), withExpiration(clock.Now().Add(-time.Hour)))},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  []string{"level1", "level2"},
		},
		{
			name: "owner/includes start when user is direct owner (and member)",
			state: state{
				mustMakeAccessList("start", withOwners("alice")): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "start",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"start"},
		},
		{
			name: "owner/expired direct user membership stops traversal but keeps start if direct owner",
			state: state{
				mustMakeAccessList("start", withOwners("alice")): {mustCreateMember("alice", withExpiration(clock.Now().Add(-time.Hour)))},
			},
			user:  makeUser("alice"),
			start: "start",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"start"},
		},
		{
			name: "member/no direct user membership => empty",
			state: state{
				mustMakeAccessList("root"):   {mustCreateMember("level1", withACLMemKind())},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("bob")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  nil,
		},
		{
			name: "member/parent filtered by membership requirements",
			state: state{
				mustMakeAccessList("root"): {mustCreateMember("level1", withACLMemKind())},
				mustMakeAccessList("level1", withMemberTraitReq("need")): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  []string{"level2"},
		},
		{
			name: "owner/owner target filtered by ownership requirements",
			state: state{
				mustMakeAccessList("root", withOwners("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  nil,
		},
		{
			name: "owner/user is indirect owner of two root access list where meets only one ownership requirement",
			state: state{
				mustMakeAccessList("side", withOwnerList("level1")): {},
				mustMakeAccessList("root", withOwners("level1"), withOwnerTraitReq("need")): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"side"},
		},
		{
			name: "owner/user is indirect owner of two root access list and don't meet ownership requirements for both",
			state: state{
				mustMakeAccessList("side", withOwnerList("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("root", withOwners("level1"), withOwnerTraitReq("need")): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{},
		},
		{
			name: "owner/user is indirect owner of two root access list and meet ownership requirements for both",
			state: state{
				mustMakeAccessList("side", withOwnerList("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("root", withOwnerList("level1"), withOwnerTraitReq("need")): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice", mkTrait("need")),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"root", "side"},
		},
		{
			name: "owner/owner target filtered by ownership requirements",
			state: state{
				mustMakeAccessList("root", withOwners("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  nil,
		},
		{
			name: "owner/collect owner targets from ancestors' OwnerOf",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1")): {},
				mustMakeAccessList("level1"):                        {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"):                        {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"root"},
		},
		{
			name: "owner/not a direct owner and no owner targets => empty",
			state: state{
				mustMakeAccessList("lonely"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "lonely",
			kind:  accesslists.RelationshipKindOwner,
			want:  nil,
		},
		{
			name: "owner/owner via direct nested ownerships requirements are met",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("level1"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice", mkTrait("need")),
			start: "level1",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"root"},
		},
		{
			name: "owner/owner via direct nested ownerships requirements not met",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("level1"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level1",
			kind:  accesslists.RelationshipKindOwner,
			want:  nil,
		},
		{
			name: "owner/owner many levels up the ownership chain",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1")): {},
				mustMakeAccessList("level1"):                        {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"):                        {mustCreateMember("level3", withACLMemKind())},
				mustMakeAccessList("level3"):                        {mustCreateMember("level4", withACLMemKind())},
				mustMakeAccessList("level4"):                        {mustCreateMember("levelTail", withACLMemKind())},
				mustMakeAccessList("levelTail"):                     {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "levelTail",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"root"},
		},
		{
			name: "member/multiple parents included",
			state: state{
				mustMakeAccessList("rootA"):  {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("rootB"):  {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  []string{"rootA", "rootB", "level2"},
		},
		{
			name: "member/parent included when membership requirements are met",
			state: state{
				mustMakeAccessList("root"): {mustCreateMember("level1", withACLMemKind())},
				mustMakeAccessList("level1", withMemberTraitReq("ok")): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice", mkTrait("ok")),
			start: "level2",
			kind:  accesslists.RelationshipKindMember,
			want:  []string{"root", "level1", "level2"},
		},
		{
			name: "owner/owner target included when ownership requirements are met",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1"), withOwnerTraitReq("need")): {},
				mustMakeAccessList("level1"): {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"): {mustCreateMember("alice")},
			},
			user:  makeUser("alice", mkTrait("need")),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"root"},
		},
		{
			name: "owner/multiple owner targets discovered via ancestors",
			state: state{
				mustMakeAccessList("rootA", withOwnerList("level1")): {},
				mustMakeAccessList("rootB", withOwnerList("level1")): {},
				mustMakeAccessList("level1"):                         {mustCreateMember("level2", withACLMemKind())},
				mustMakeAccessList("level2"):                         {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"rootA", "rootB"},
		},
		{
			name: "member/start filtered out when start has unmet membership requirements",
			state: state{
				mustMakeAccessList("start", withMemberTraitReq("must")): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "start",
			kind:  accesslists.RelationshipKindMember,
			want:  nil,
		},
		{
			name: "owner/start NOT included when ownership requirements are NOT met",
			state: state{
				mustMakeAccessList("start", withOwners("alice"), withOwnerTraitReq("need")): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "start",
			kind:  accesslists.RelationshipKindOwner,
			want:  nil,
		},
		{
			name: "owner/start included when user is among multiple direct owners",
			state: state{
				mustMakeAccessList("start", withOwners("bob", "alice")): {mustCreateMember("alice")},
			},
			user:  makeUser("alice"),
			start: "start",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"start"},
		},
		{
			name: "owner/direct and indirect owner => include both",
			state: state{
				mustMakeAccessList("root", withOwnerList("level1")): {},
				mustMakeAccessList("level1"): {
					mustCreateMember("level2", withACLMemKind()),
				},
				mustMakeAccessList("level2", withOwners("alice")): {
					mustCreateMember("alice"),
				},
			},
			user:  makeUser("alice"),
			start: "level2",
			kind:  accesslists.RelationshipKindOwner,
			want:  []string{"level2", "root"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			bk, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)
			svc, err := local.NewAccessListService(bk, clock)
			require.NoError(t, err)

			require.NoError(t, upsertState(svc, tt.state))

			acl, err := svc.GetAccessList(ctx, tt.start)
			require.NoError(t, err)

			h, err := accesslists.NewHierarchy(accesslists.HierarchyConfig{
				AccessListsService: svc,
				Clock:              clock,
			})
			require.NoError(t, err)

			memberOf, ownerOf, err := h.GetHierarchyForUser(t.Context(), acl, tt.user)
			require.NoError(t, err)

			accessList := memberOf
			if tt.kind == accesslists.RelationshipKindOwner {
				accessList = ownerOf
			}

			got := make([]string, 0, len(accessList))
			for _, a := range accessList {
				got = append(got, a.GetName())
			}
			require.ElementsMatch(t, got, tt.want)
		})
	}
}

func withOwnerTraitReq(name string) option {
	return func(opts *options) {
		opts.ownershipRequire = accesslist.Requires{
			Traits: mkTrait(name),
		}
	}
}

func withMemberTraitReq(name string) option {
	return func(opts *options) {
		opts.membershipRequire = accesslist.Requires{
			Traits: mkTrait(name),
		}
	}
}

func mkTrait(name string) trait.Traits {
	return trait.Traits{name: nil}
}

type state map[*accesslist.AccessList][]*accesslist.AccessListMember

func upsertState(svc *local.AccessListService, state state) error {
	ctx := context.Background()
	for acl := range state {
		for _, owner := range acl.GetOwners() {
			if owner.MembershipKind == accesslist.MembershipKindList {
				// Create placeholder access list for owner lists.
				// to prevent backend check for the nested access list.
				_, err := svc.UpsertAccessList(ctx, mustMakeAccessList(owner.Name))
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}
		if _, err := svc.UpsertAccessList(ctx, acl); err != nil {
			return trace.Wrap(err)
		}
	}
	for acl, members := range state {
		for _, member := range members {
			member.Spec.AccessList = acl.GetName()
			if _, err := svc.UpsertAccessListMember(ctx, member); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}
func makeUser(name string, traits ...trait.Traits) types.User {
	user, err := types.NewUser(name)
	if err != nil {
		panic(err)
	}
	for _, t := range traits {
		user.SetTraits(t)
	}
	return user
}

func withOwners(owners ...string) option {
	return func(opts *options) {
		for _, owner := range owners {
			opts.owners = append(opts.owners, accesslist.Owner{Name: owner})
		}
	}
}

func withOwnerList(owners ...string) option {
	return func(opts *options) {
		for _, owner := range owners {
			opts.owners = append(opts.owners, accesslist.Owner{Name: owner, MembershipKind: accesslist.MembershipKindList})
		}
	}
}

func mustMakeAccessList(name string, opts ...option) *accesslist.AccessList {
	cfg := options{
		owners: []accesslist.Owner{{Name: "system"}},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	acl, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title:              name,
		Owners:             cfg.owners,
		Audit:              accesslist.Audit{NextAuditDate: time.Now()},
		MembershipRequires: cfg.membershipRequire,
		OwnershipRequires:  cfg.ownershipRequire,
	})
	if err != nil {
		panic(err)
	}
	return acl
}

type options struct {
	kind              string
	Expiration        time.Time
	membershipRequire accesslist.Requires
	ownershipRequire  accesslist.Requires
	owners            []accesslist.Owner
}

type option func(*options)

func withKind(kind string) option {
	return func(opts *options) {
		opts.kind = kind
	}
}

func withACLMemKind() option {
	return withKind(accesslist.MembershipKindList)
}

func withExpiration(expiration time.Time) option {
	return func(opts *options) {
		opts.Expiration = expiration
	}
}

func mustCreateMember(memberName string, opts ...option) *accesslist.AccessListMember {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	clock := clockwork.NewRealClock()
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     "random", // will be updated during insert
			Name:           memberName,
			Joined:         clock.Now(),
			AddedBy:        "added by",
			Expires:        cfg.Expiration,
			MembershipKind: cfg.kind,
		},
	)
	if err != nil {
		panic(err)
	}
	return member
}
