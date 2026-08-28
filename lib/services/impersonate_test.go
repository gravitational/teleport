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

package services

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestCheckImpersonate(t *testing.T) {
	noLabelsRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "no-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	wildcardRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "wildcard",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Impersonate: &types.ImpersonateConditions{
					Users: []string{types.Wildcard},
					Roles: []string{types.Wildcard},
				},
			},
		},
	}
	wildcardDenyRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "wildcard-deny-user",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Deny: types.RoleConditions{
				Impersonate: &types.ImpersonateConditions{
					Users: []string{types.Wildcard},
					Roles: []string{types.Wildcard},
				},
			},
		},
	}
	type props struct {
		traits map[string][]string
		labels map[string]string
	}
	var empty props
	newUser := func(name string, props props) types.User {
		u := &types.UserV2{
			Kind:    types.KindUser,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
				Labels:    props.labels,
			},
			Spec: types.UserSpecV2{
				Traits: props.traits,
			},
		}
		if err := u.CheckAndSetDefaults(); err != nil {
			t.Fatal(err)
		}
		return u
	}

	type impersonate struct {
		name    string
		allowed bool
		user    types.User
		roles   []types.Role
	}
	testCases := []struct {
		name   string
		user   types.User
		roles  []types.Role
		checks []impersonate
	}{
		{
			name: "empty role set can impersonate no other user",
			user: newUser("alice", empty),
			checks: []impersonate{
				{
					allowed: false,
					user:    newUser("bob", empty),
					roles: []types.Role{
						noLabelsRole,
					},
				},
			},
		},
		{
			name: "wildcard role can impersonate any user or role",
			user: newUser("alice", empty),
			roles: []types.Role{
				wildcardRole,
			},
			checks: []impersonate{
				{
					allowed: true,
					user:    newUser("bob", empty),
					roles: []types.Role{
						noLabelsRole,
					},
				},
			},
		},
		{
			name: "wildcard deny user overrides wildcard allow",
			user: newUser("alice", empty),
			roles: []types.Role{
				wildcardRole,
				wildcardDenyRole,
			},
			checks: []impersonate{
				{
					allowed: false,
					user:    newUser("bob", empty),
					roles: []types.Role{
						noLabelsRole,
					},
				},
			},
		},
		{
			name: "impersonate condition is limited to a certain set users and roles",
			user: newUser("alice", empty),
			roles: []types.Role{
				&types.RoleV6{
					Metadata: types.Metadata{
						Name:      "limited",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Impersonate: &types.ImpersonateConditions{
								Users: []string{"bob"},
								Roles: []string{noLabelsRole.GetName()},
							},
						},
					},
				},
			},
			checks: []impersonate{
				{
					allowed: true,
					user:    newUser("bob", empty),
					roles: []types.Role{
						noLabelsRole,
					},
				},
				{
					allowed: false,
					user:    newUser("alice", empty),
					roles: []types.Role{
						noLabelsRole,
					},
				},
				{
					allowed: false,
					user:    newUser("bob", empty),
					roles: []types.Role{
						wildcardRole,
					},
				},
			},
		},
		{
			name: "Alice can impersonate any user and role from dev team",
			user: newUser("alice", props{traits: map[string][]string{"team": {"dev"}}}),
			roles: []types.Role{
				&types.RoleV6{
					Metadata: types.Metadata{
						Name:      "team-impersonator",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Impersonate: &types.ImpersonateConditions{
								Users: []string{types.Wildcard},
								Roles: []string{types.Wildcard},
								Where: `equals(user.spec.traits["team"], impersonate_user.spec.traits["team"]) && contains(user.spec.traits["team"], impersonate_role.metadata.labels["team"])`,
							},
						},
					},
				},
			},
			checks: []impersonate{
				{
					allowed: true,
					user: newUser("bob", props{
						traits: map[string][]string{"team": {"dev"}},
					}),
					roles: []types.Role{
						&types.RoleV6{
							Metadata: types.Metadata{
								Name:      "dev",
								Namespace: apidefaults.Namespace,
								Labels: map[string]string{
									"team": "dev",
								},
							},
							Spec: types.RoleSpecV6{},
						},
					},
				},
				{
					name:    "all roles in the set have to match where condition",
					allowed: false,
					user: newUser("bob", props{
						traits: map[string][]string{"team": {"dev"}},
					}),
					roles: []types.Role{
						wildcardRole,
						&types.RoleV6{
							Metadata: types.Metadata{
								Name:      "dev",
								Namespace: apidefaults.Namespace,
								Labels: map[string]string{
									"team": "dev",
								},
							},
							Spec: types.RoleSpecV6{},
						},
					},
				},
			},
		},
	}
	for i, tc := range testCases {
		var set RoleSet
		for i := range tc.roles {
			set = append(set, tc.roles[i])
		}
		for j, impersonate := range tc.checks {
			comment := fmt.Sprintf("test case %v '%v', check %v %v", i, tc.name, j, impersonate.name)
			result := set.CheckImpersonate(tc.user, impersonate.user, impersonate.roles)
			if impersonate.allowed {
				require.NoError(t, result, comment)
			} else {
				require.True(t, trace.IsAccessDenied(result), fmt.Sprintf("%v: %v", comment, result))
			}
		}
	}
}
