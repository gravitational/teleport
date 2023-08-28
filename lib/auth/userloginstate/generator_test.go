/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package userloginstate

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestAccessLists(t *testing.T) {
	user, err := types.NewUser("user")
	user.SetRoles([]string{"orole1"})
	user.SetTraits(map[string][]string{
		"otrait1": {"value1", "value2"},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		accessLists []*accesslist.AccessList
		members     []*accesslist.AccessListMember
		roles       []string
		expected    *userloginstate.UserLoginState
	}{
		{
			name:  "access lists are empty",
			roles: []string{"orole1"},
			expected: newUserLoginState(t, "user", []string{
				"orole1",
			}, map[string][]string{
				"otrait1": {"value1", "value2"},
			}),
		},
		{
			name: "access lists add roles and traits",
			accessLists: []*accesslist.AccessList{
				newAccessList(t, "1", []string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}),
				newAccessList(t, "2", []string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}),
			},
			members: append(newAccessListMembers(t, "1", "user"), newAccessListMembers(t, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2"},
			expected: newUserLoginState(t, "user",
				[]string{
					"orole1",
					"role1",
					"role2",
				}, trait.Traits{
					"otrait1": []string{"value1", "value2"},
					"trait1":  []string{"value1", "value2"},
					"trait2":  []string{"value3"},
				}),
		},
		{
			name: "access lists add roles and traits, roles missing from backend",
			accessLists: []*accesslist.AccessList{
				newAccessList(t, "1", []string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}),
				newAccessList(t, "2", []string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}),
			},
			members: append(newAccessListMembers(t, "1", "user"), newAccessListMembers(t, "2", "user")...),
			roles:   []string{"orole1"},
			expected: newUserLoginState(t, "user",
				[]string{"orole1"}, trait.Traits{
					"otrait1": []string{"value1", "value2"},
					"trait1":  []string{"value1", "value2"},
					"trait2":  []string{"value3"},
				}),
		},
		{
			name: "access lists only a member of some lists",
			accessLists: []*accesslist.AccessList{
				newAccessList(t, "1", []string{"role1"}, trait.Traits{
					"trait1": []string{"value1"},
				}),
				newAccessList(t, "2", []string{"role2"}, trait.Traits{
					"trait1": []string{"value2"},
					"trait2": []string{"value3"},
				}),
			},
			members: append(newAccessListMembers(t, "1", "user"), newAccessListMembers(t, "2", "not-user")...),
			roles:   []string{"orole1", "role1", "role2"},
			expected: newUserLoginState(t, "user",
				[]string{
					"orole1",
					"role1",
				}, trait.Traits{
					"otrait1": []string{"value1", "value2"},
					"trait1":  []string{"value1"},
				}),
		},
		{
			name: "access lists add roles with duplicates",
			accessLists: []*accesslist.AccessList{
				newAccessList(t, "1", []string{"role1", "role2"}, trait.Traits{}),
				newAccessList(t, "2", []string{"role2", "role3"}, trait.Traits{}),
			},
			members: append(newAccessListMembers(t, "1", "user"), newAccessListMembers(t, "2", "user")...),
			roles:   []string{"orole1", "role1", "role2", "role3"},
			expected: newUserLoginState(t, "user",
				[]string{
					"orole1",
					"role1",
					"role2",
					"role3",
				}, trait.Traits{
					"otrait1": []string{"value1", "value2"},
				}),
		},
		{
			name: "access lists add traits with duplicates",
			accessLists: []*accesslist.AccessList{
				newAccessList(t, "1", []string{},
					trait.Traits{
						"trait1": []string{"value1", "value2"},
						"trait2": []string{"value3", "value4"},
					},
				),
				newAccessList(t, "2", []string{},
					trait.Traits{
						"trait2": []string{"value3", "value1"},
						"trait3": []string{"value5", "value6"},
					},
				),
			},
			members: append(newAccessListMembers(t, "1", "user"), newAccessListMembers(t, "2", "user")...),
			roles:   []string{"orole1"},
			expected: newUserLoginState(t, "user",
				[]string{
					"orole1",
				},
				trait.Traits{
					"otrait1": []string{"value1", "value2"},
					"trait1":  []string{"value1", "value2"},
					"trait2":  []string{"value3", "value4", "value1"},
					"trait3":  []string{"value5", "value6"},
				}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
				require.NoError(t, backendSvc.UpsertRole(ctx, role))
			}

			state, err := svc.Generate(ctx, user)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, state,
				cmpopts.SortSlices(func(str1, str2 string) bool {
					return str1 < str2
				})))
		})
	}
}

type svc struct {
	services.AccessLists
	services.Access
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
	return &Generator{log: log, accessLists: accessListsSvc, access: accessSvc, clock: clock},
		&svc{AccessLists: accessListsSvc, Access: accessSvc}
}

func newAccessList(t *testing.T, name string, roles []string, traits trait.Traits) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: "title",
		Audit: accesslist.Audit{
			Frequency:     time.Hour,
			NextAuditDate: time.Now().Add(time.Hour * 48),
		},
		Owners: []accesslist.Owner{
			{
				Name:        "owner",
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
		Grants: accesslist.Grants{
			Roles:  roles,
			Traits: traits,
		},
	})
	require.NoError(t, err)

	return accessList
}

func newAccessListMembers(t *testing.T, accessList string, members ...string) []*accesslist.AccessListMember {
	alMembers := make([]*accesslist.AccessListMember, len(members))
	for i, member := range members {
		var err error
		alMembers[i], err = accesslist.NewAccessListMember(header.Metadata{
			Name: member,
		}, accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       member,
			Joined:     time.Now(),
			Expires:    time.Now().Add(24 * time.Hour),
			Reason:     "added",
			AddedBy:    "owner",
		})
		require.NoError(t, err)
	}

	return alMembers
}

func newUserLoginState(t *testing.T, name string, roles []string, traits map[string][]string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(header.Metadata{
		Name: name,
	}, userloginstate.Spec{
		Roles:  roles,
		Traits: traits,
	})
	require.NoError(t, err)

	return uls
}
