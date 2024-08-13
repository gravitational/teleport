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
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
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
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
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
	user.SetStaticLabels(map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	user.SetRoles([]string{"orole1"})
	user.SetTraits(map[string][]string{
		"otrait1": {"value1", "value2"},
	})
	require.NoError(t, err)

	userNoRolesOrTraits, err := types.NewUser("user")
	require.NoError(t, err)
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name               string
		user               types.User
		cloud              bool
		accessLists        []*accesslist.AccessList
		members            []*accesslist.AccessListMember
		locks              []types.Lock
		roles              []string
		wantErr            require.ErrorAssertionFunc
		expected           *userloginstate.UserLoginState
		expectedRoleCount  int
		expectedTraitCount int
	}{
		{
			name:    "access lists are empty",
			user:    user,
			cloud:   true,
			roles:   []string{"orole1"},
			wantErr: require.NoError,
			expected: newUserLoginState(t, "user",
				map[string]string{
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}}),
			expectedRoleCount:  0,
			expectedTraitCount: 0,
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3"}}),
			expectedRoleCount:  2,
			expectedTraitCount: 3,
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": []string{"value1", "value2"}}),
			expectedRoleCount:  0,
			expectedTraitCount: 0,
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3"}}),
			expectedRoleCount:  0,
			expectedTraitCount: 0,
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
				map[string]string{
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "owner-role1", "owner-role2"},
				trait.Traits{"otrait1": {"value1", "value2"}, "owner-trait1": {"owner-value1"}}),
			expectedRoleCount:  2,
			expectedTraitCount: 1,
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
				map[string]string{
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "owner-role1", "owner-role2", "role1"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"owner-value1", "value1"}}),
			expectedRoleCount:  3,
			expectedTraitCount: 2,
		},
		{
			name:  "access lists add member roles and traits, roles missing from backend",
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
			roles:   []string{"orole1"},
			wantErr: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.NotFound("role role1 is not found"))
			},
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1"},
				trait.Traits{
					"otrait1": {"value1", "value2"}, "trait1": {"value1"}}),
			expectedRoleCount:  1,
			expectedTraitCount: 1,
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1", "role1", "role2", "role3"},
				trait.Traits{"otrait1": {"value1", "value2"}}),
			expectedRoleCount:  3,
			expectedTraitCount: 0,
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
					"label1":                                 "value1",
					"label2":                                 "value2",
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}},
				[]string{"orole1"},
				trait.Traits{"otrait1": {"value1", "value2"}, "trait1": {"value1", "value2"}, "trait2": {"value3", "value4", "value1"}, "trait3": {"value5", "value6"}}),
			expectedRoleCount:  0,
			expectedTraitCount: 7,
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
				map[string]string{
					userloginstate.OriginalRolesAndTraitsSet: "true",
				},
				nil,
				nil,
				[]string{"role1"},
				trait.Traits{
					"trait1": {"value1", "value2"},
					"trait2": {"value3", "value4"},
					"trait3": {"value5", "value6"}}),
			expectedRoleCount:  1,
			expectedTraitCount: 6,
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

			state, err := svc.Generate(ctx, test.user)
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
			}
		})
	}
}

type svc struct {
	services.AccessLists
	services.Access

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

	log := logrus.WithField("test", "logger")
	svc := &svc{AccessLists: accessListsSvc, Access: accessSvc}

	generator, err := NewGenerator(GeneratorConfig{
		Log:         log,
		AccessLists: svc,
		Access:      svc,
		UsageEvents: svc,
		Clock:       clock,
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
