/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package userloginstate

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

const ownerUser = "owner"

var emptyGrants = accesslist.Grants{}

func TestAccessLists(t *testing.T) {
	owner, err := types.NewUser(ownerUser)
	require.NoError(t, err)
	owner.SetRoles([]string{"orole1"})
	owner.SetTraits(map[string][]string{
		"otrait1": {"value1", "value2"},
	})

	user, err := types.NewUser("user")
	require.NoError(t, err)
	user.SetStaticLabels(map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	user.SetRoles([]string{"orole1"})
	user.SetTraits(map[string][]string{
		"otrait1": {"value1", "value2"},
	})

	userNoRolesOrTraits, err := types.NewUser("user")
	require.NoError(t, err)
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name                        string
		user                        types.User
		cloud                       bool
		accessLists                 []*accesslist.AccessList
		members                     []*accesslist.AccessListMember
		locks                       []types.Lock
		roles                       []string
		wantErr                     require.ErrorAssertionFunc
		expected                    *userloginstate.UserLoginState
		expectedRoleCount           int
		expectedTraitCount          int
		expectedInheritedRoleCount  int
		expectedInheritedTraitCount int
	}{
		{
			name:    "access lists are empty",
			user:    user,
			cloud:   true,
			roles:   []string{"orole1"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
			),
			expectedRoleCount:           0,
			expectedTraitCount:          0,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add roles and traits",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3"}},
			),
			expectedRoleCount:           2,
			expectedTraitCount:          3,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "lock prevents adding roles and traits",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			locks: []types.Lock{
				newUserLock(t, "test-lock", user.GetName()),
			},
			roles:   []string{"orole1", "role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": []string{"value1", "value2"}},
			),
			expectedRoleCount:           0,
			expectedTraitCount:          0,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add member roles and traits (cloud disabled)",
			user:  user,
			cloud: false,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3"}},
			),
			expectedRoleCount:           0,
			expectedTraitCount:          0,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add owner roles and traits",
			user:  owner,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), grants([]string{"owner-role1", "owner-role2"}, trait.Traits{
					"owner-trait1": []string{"owner-value1"},
				})),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1", "owner-role1", "owner-role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, ownerUser,
				nil,
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "owner-role1", "owner-role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "owner-trait1": {"owner-value1"}},
			),
			expectedRoleCount:           2,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add owner and member roles and traits",
			user:  owner,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), grants([]string{"owner-role1", "owner-role2"}, trait.Traits{
					"trait1": []string{"owner-value1"},
				})),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: newAccessListMembers(t, clock, "1", ownerUser),
			roles:   []string{"orole1", "owner-role1", "owner-role2", "role1"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, ownerUser,
				nil,
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "owner-role1", "owner-role2", "role1"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"owner-value1", "value1"}},
			),
			expectedRoleCount:           3,
			expectedTraitCount:          2,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists only a member of some lists",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "not-user")...),
			roles:   []string{"orole1", "role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1"},
				trait.Traits{
					"otrait1": {"value1", "value2"}, "trait1": {"value1"},
				},
			),
			expectedRoleCount:           1,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add roles with duplicates",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1", "role2"}, trait.Traits{}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role2", "role3"}, trait.Traits{}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2", "role3"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2", "role3"},
				trait.Traits{"otrait1": {"value1", "value2"}},
			),
			expectedRoleCount:           3,
			expectedTraitCount:          0,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add traits with duplicates",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{},
					trait.Traits{
						"trait1": []string{"value1", "value2"},
						"trait2": []string{"value3", "value4"},
					},
				), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{},
					trait.Traits{
						"trait2": []string{"value3", "value1"},
						"trait3": []string{"value5", "value6"},
					},
				), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3", "value4", "value1"}, "trait3": {"value5", "value6"}},
			),
			expectedRoleCount:           0,
			expectedTraitCount:          7,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists add traits with no roles or traits in original",
			user:  userNoRolesOrTraits,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"},
					trait.Traits{
						"trait1": []string{"value1", "value2"},
						"trait2": []string{"value3", "value4"},
					},
				), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{},
					trait.Traits{
						"trait3": []string{"value5", "value6"},
					},
				), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"role1"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				nil,
				nil,
				nil,
				[]string{"role1"},
				trait.Traits{
					"trait1": {"value1", "value2"},
					"trait2": {"value3", "value4"},
					"trait3": {"value5", "value6"},
				},
			),
			expectedRoleCount:           1,
			expectedTraitCount:          6,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
		{
			name:  "access lists member of nested list",
			cloud: true,
			user:  userNoRolesOrTraits,
			// user is member of acl 3, acl 1 includes acl 2, which includes 3
			// so user will be granted role1 and 2, and trait1
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"},
					trait.Traits{
						"trait1": {"value"},
					}),
					emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role1"}, trait.Traits{}),
					emptyGrants),
				newAccessList(t, clock, "3", grants([]string{"role2"}, trait.Traits{}), emptyGrants),
			},
			members: append(
				newAccessListMembers(t, clock, "3", "user"),
				newAccessListMemberWithKind(t, clock, "2", accesslist.MembershipKindList, "3"),
				newAccessListMemberWithKind(t, clock, "1", accesslist.MembershipKindList, "2")),
			roles:   []string{"role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				nil,
				nil,
				nil,
				[]string{"role1", "role2"},
				trait.Traits{"trait1": {"value"}},
			),
			expectedRoleCount:           2,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  1,
			expectedInheritedTraitCount: 1,
		},
		{
			name:  "access lists member of nested list",
			cloud: true,
			user:  userNoRolesOrTraits,
			// user is member of acl 3, acl 1 includes acl 2, which includes 3
			// so user will be granted role1 and 2, and trait1
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"},
					trait.Traits{
						"trait1": {"value"},
					}),
					emptyGrants),
				newAccessList(t, clock, "2", grants([]string{"role1"}, trait.Traits{}),
					emptyGrants),
				newAccessList(t, clock, "3", grants([]string{"role2"}, trait.Traits{}), emptyGrants),
			},
			members: append(
				newAccessListMembers(t, clock, "3", "user"),
				newAccessListMemberWithKind(t, clock, "2", accesslist.MembershipKindList, "3"),
				newAccessListMemberWithKind(t, clock, "1", accesslist.MembershipKindList, "2")),
			roles:   []string{"role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				nil,
				nil,
				nil,
				[]string{"role1", "role2"},
				trait.Traits{"trait1": {"value"}},
			),
			expectedRoleCount:           2,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  1,
			expectedInheritedTraitCount: 1,
		},
		{
			name:  "access lists member of nested list, in diamond formation",
			cloud: true,
			user:  userNoRolesOrTraits,
			// user is member of acl 1, acl 2 and 3 include acl 1, acl 4 includes acls 2 and 3
			// so user will be granted {trait: [1,2,3,4]}
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{}, trait.Traits{"trait": {"1"}}), emptyGrants),
				newAccessList(t, clock, "2", grants([]string{}, trait.Traits{"trait": {"2"}}), emptyGrants),
				newAccessList(t, clock, "3", grants([]string{}, trait.Traits{"trait": {"3"}}), emptyGrants),
				newAccessList(t, clock, "4", grants([]string{}, trait.Traits{"trait": {"4"}}), emptyGrants),
			},
			members: append(
				newAccessListMembers(t, clock, "1", "user"),
				newAccessListMemberWithKind(t, clock, "2", accesslist.MembershipKindList, "1"),
				newAccessListMemberWithKind(t, clock, "3", accesslist.MembershipKindList, "1"),
				newAccessListMemberWithKind(t, clock, "4", accesslist.MembershipKindList, "3"),
				newAccessListMemberWithKind(t, clock, "4", accesslist.MembershipKindList, "2"),
			),
			roles:   nil,
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				nil,
				nil,
				nil,
				nil,
				trait.Traits{"trait": {"1", "2", "3", "4"}},
			),
			expectedRoleCount:          0,
			expectedTraitCount:         4,
			expectedInheritedRoleCount: 0,
			// trait 1 is directly granted to user via acl 1; it is not inherited
			expectedInheritedTraitCount: 3,
		},
		{
			name:  "members in nested access lists inherit parent's owner grants",
			cloud: true,
			user:  userNoRolesOrTraits,
			// user is member of acl 1, acl 3, includes members of acl 2, which includes members of acl 1 as owners
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", emptyGrants, grants([]string{"oroleA"}, trait.Traits{"okey": {"oval1"}})),
				newAccessList(t, clock, "2", emptyGrants, grants([]string{"oroleB"}, trait.Traits{"okey": {"oval2"}})),
				newAccessListWithOwners(t, clock, "3", emptyGrants, grants([]string{"oroleC"}, trait.Traits{"okey": {"oval3"}}), []accesslist.Owner{{
					Name:           "2",
					Description:    "hello",
					MembershipKind: accesslist.MembershipKindList},
				}),
			},
			members: append(
				newAccessListMembers(t, clock, "1", "user"),
				newAccessListMemberWithKind(t, clock, "2", accesslist.MembershipKindList, "1"),
				newAccessListMemberWithKind(t, clock, "3", accesslist.MembershipKindList, "2"),
			),
			roles:   []string{"oroleA", "oroleB", "oroleC"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				nil,
				nil,
				nil,
				[]string{"oroleC"},
				trait.Traits{"okey": {"oval3"}},
			),
			expectedRoleCount:           1,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  1,
			expectedInheritedTraitCount: 1,
		},
		{
			name:  "an access list that references a non-existent role should be skipped entirely",
			user:  user,
			cloud: true,
			accessLists: []*accesslist.AccessList{
				newAccessList(t, clock, "1", grants([]string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}), emptyGrants),
				// role3 doesn't exist, so this access list is invalid and should be skipped.
				newAccessList(t, clock, "2", grants([]string{"role2", "role3"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}), emptyGrants),
			},
			members: append(newAccessListMembers(t, clock, "1", "user"), newAccessListMembers(t, clock, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				// only role1 will be granted by the access lists, since role2 comes from an invalid access list.
				[]string{"orole1", "role1"},
				// traits from the invalid access list won't be granted.
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1"}},
			),
			expectedRoleCount:           1,
			expectedTraitCount:          1,
			expectedInheritedRoleCount:  0,
			expectedInheritedTraitCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Cloud: test.cloud,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.Identity: {Enabled: true},
					},
				},
			})

			ctx := context.Background()
			svc, backendSvc := initGeneratorSvc(t)

			for _, accessList := range test.accessLists {
				_, err = backendSvc.UpsertAccessList(ctx, accessList)
				require.NoError(t, err)
			}

			for _, member := range test.members {
				_, err = backendSvc.UpsertAccessListMember(ctx, member)
				require.NoError(t, err)
			}

			for _, role := range test.roles {
				role, err := types.NewRole(role, types.RoleSpecV6{})
				require.NoError(t, err)
				_, err = backendSvc.UpsertRole(ctx, role)
				require.NoError(t, err)
			}

			for _, lock := range test.locks {
				require.NoError(t, backendSvc.UpsertLock(ctx, lock))
			}

			state, err := svc.Generate(ctx, test.user, backendSvc)
			test.wantErr(t, err)

			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(test.expected, state,
				cmpopts.SortSlices(func(str1, str2 string) bool {
					return str1 < str2
				})))

			if test.expectedRoleCount == 0 && test.expectedTraitCount == 0 {
				require.Nil(t, backendSvc.event)
			} else {
				require.NotNil(t, backendSvc.event)
				require.IsType(t, &usageeventsv1.UsageEventOneOf_AccessListGrantsToUser{}, backendSvc.event.Event)
				event := (backendSvc.event.Event).(*usageeventsv1.UsageEventOneOf_AccessListGrantsToUser)

				require.Equal(t, test.expectedRoleCount, int(event.AccessListGrantsToUser.CountRolesGranted))
				require.Equal(t, test.expectedTraitCount, int(event.AccessListGrantsToUser.CountTraitsGranted))
				require.Equal(t, test.expectedInheritedRoleCount, int(event.AccessListGrantsToUser.CountInheritedRolesGranted))
				require.Equal(t, test.expectedInheritedTraitCount, int(event.AccessListGrantsToUser.CountInheritedTraitsGranted))
			}
		})
	}
}

func TestGitHubIdentity(t *testing.T) {
	ctx := context.Background()
	svc, backendSvc := initGeneratorSvc(t)

	noGitHubIdentity, err := types.NewUser("alice")
	require.NoError(t, err)

	withGitHubIdentity, err := types.NewUser("alice")
	require.NoError(t, err)
	withGitHubIdentity.SetGithubIdentities([]types.ExternalIdentity{{
		UserID:   "1234567",
		Username: "username1234567",
	}})

	withGitHubIdentityUpdated, err := types.NewUser("alice")
	require.NoError(t, err)
	withGitHubIdentityUpdated.SetGithubIdentities([]types.ExternalIdentity{{
		UserID:   "7654321",
		Username: "username7654321",
	}})

	tests := []struct {
		name                 string
		user                 types.User
		expectGitHubIdentity *userloginstate.ExternalIdentity
	}{
		{
			name:                 "no github identity",
			user:                 noGitHubIdentity,
			expectGitHubIdentity: nil,
		},
		{
			name: "with github identity",
			user: withGitHubIdentity,
			expectGitHubIdentity: &userloginstate.ExternalIdentity{
				UserID:   "1234567",
				Username: "username1234567",
			},
		},
		{
			// at this point alice's GitHub identity should be saved in old
			// states.
			name: "github identity preserved",
			user: noGitHubIdentity,
			expectGitHubIdentity: &userloginstate.ExternalIdentity{
				UserID:   "1234567",
				Username: "username1234567",
			},
		},
		{
			name: "github identity updated",
			user: withGitHubIdentityUpdated,
			expectGitHubIdentity: &userloginstate.ExternalIdentity{
				UserID:   "7654321",
				Username: "username7654321",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uls, err := svc.Generate(ctx, test.user, backendSvc)
			require.NoError(t, err)
			require.Equal(t, test.expectGitHubIdentity, uls.Spec.GitHubIdentity)

			// Upsert the state for the next test case.
			_, err = backendSvc.UpsertUserLoginState(ctx, uls)
			require.NoError(t, err)
		})
	}
}

type svc struct {
	services.AccessLists
	services.Access
	services.UserLoginStates

	event *usageeventsv1.UsageEventOneOf
}

func (s *svc) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	s.event = req.Event
	return nil
}

func initGeneratorSvc(t *testing.T) (*Generator, *svc) {
	t.Helper()

	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	accessListsSvc, err := local.NewAccessListService(mem, clock)
	require.NoError(t, err)
	accessSvc := local.NewAccessService(mem)
	ulsService, err := local.NewUserLoginStateService(mem)
	require.NoError(t, err)

	svc := &svc{
		AccessLists:     accessListsSvc,
		Access:          accessSvc,
		UserLoginStates: ulsService,
	}

	emitter := &eventstest.MockRecorderEmitter{}

	generator, err := NewGenerator(GeneratorConfig{
		Log:         utils.NewSlogLoggerForTests(),
		AccessLists: svc,
		Access:      svc,
		UsageEvents: svc,
		Clock:       clock,
		Emitter:     emitter,
	})
	require.NoError(t, err)
	return generator, svc
}

func grants(roles []string, traits trait.Traits) accesslist.Grants {
	return accesslist.Grants{
		Roles:  roles,
		Traits: traits,
	}
}

func newAccessList(t *testing.T, clock clockwork.Clock, name string, grants accesslist.Grants, ownerGrants accesslist.Grants) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: "title",
		Audit: accesslist.Audit{
			NextAuditDate: clock.Now().Add(time.Hour * 48),
		},
		Owners: []accesslist.Owner{
			{
				Name:        ownerUser,
				Description: "description",
			},
		},
		OwnershipRequires: accesslist.Requires{
			Roles:  []string{},
			Traits: map[string][]string{},
		},
		MembershipRequires: accesslist.Requires{
			Roles:  []string{},
			Traits: map[string][]string{},
		},
		Grants:      grants,
		OwnerGrants: ownerGrants,
	})
	require.NoError(t, err)

	return accessList
}

func newAccessListWithOwners(t *testing.T, clock clockwork.Clock, name string, grants accesslist.Grants, ownerGrants accesslist.Grants, owners []accesslist.Owner) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: "title",
		Audit: accesslist.Audit{
			NextAuditDate: clock.Now().Add(time.Hour * 48),
		},
		Owners: owners,
		OwnershipRequires: accesslist.Requires{
			Roles:  []string{},
			Traits: map[string][]string{},
		},
		MembershipRequires: accesslist.Requires{
			Roles:  []string{},
			Traits: map[string][]string{},
		},
		Grants:      grants,
		OwnerGrants: ownerGrants,
	})
	require.NoError(t, err)

	return accessList
}

func newAccessListMembers(t *testing.T, clock clockwork.Clock, accessList string, members ...string) []*accesslist.AccessListMember {
	alMembers := make([]*accesslist.AccessListMember, len(members))
	for i, member := range members {
		var err error
		alMembers[i], err = accesslist.NewAccessListMember(header.Metadata{
			Name: member,
		}, accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       member,
			Joined:     clock.Now(),
			Expires:    clock.Now().Add(24 * time.Hour),
			Reason:     "added",
			AddedBy:    ownerUser,
		})
		require.NoError(t, err)
	}

	return alMembers
}

func newAccessListMemberWithKind(t *testing.T, clock clockwork.Clock, accessList string, kind string, member string) *accesslist.AccessListMember {

	var err error
	res, err := accesslist.NewAccessListMember(header.Metadata{
		Name: member,
	}, accesslist.AccessListMemberSpec{
		AccessList:     accessList,
		Name:           member,
		Joined:         clock.Now(),
		Expires:        clock.Now().Add(24 * time.Hour),
		Reason:         "added",
		AddedBy:        ownerUser,
		MembershipKind: kind,
	})
	require.NoError(t, err)

	return res
}

func newUserLoginState(t *testing.T, name string, labels map[string]string, originalRoles []string, originalTraits map[string][]string,
	roles []string, traits map[string][]string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(header.Metadata{
		Name:   name,
		Labels: labels,
	}, userloginstate.Spec{
		OriginalRoles:  originalRoles,
		OriginalTraits: originalTraits,
		Roles:          roles,
		Traits:         traits,
	})
	require.NoError(t, err)

	return uls
}

func newUserLock(t *testing.T, name string, username string) types.Lock {
	t.Helper()

	lock, err := types.NewLock(name, types.LockSpecV2{
		Target: types.LockTarget{
			User: username,
		},
	})
	require.NoError(t, err)

	return lock
}
