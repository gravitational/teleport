/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"fmt"
	"testing"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCheckImpersonate(t *testing.T) {
	noLabelsRole := &types.RoleV5{
		Metadata: types.Metadata{
			Name:      "no-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	wildcardRole := &types.RoleV5{
		Metadata: types.Metadata{
			Name:      "wildcard",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Impersonate: &types.ImpersonateConditions{
					Users: []string{types.Wildcard},
					Roles: []string{types.Wildcard},
				},
			},
		},
	}
	wildcardDenyRole := &types.RoleV5{
		Metadata: types.Metadata{
			Name:      "wildcard-deny-user",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV5{
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
				&types.RoleV5{
					Metadata: types.Metadata{
						Name:      "limited",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV5{
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
			user: newUser("alice", props{traits: map[string][]string{"team": []string{"dev"}}}),
			roles: []types.Role{
				&types.RoleV5{
					Metadata: types.Metadata{
						Name:      "team-impersonator",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV5{
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
						traits: map[string][]string{"team": []string{"dev"}},
					}),
					roles: []types.Role{
						&types.RoleV5{
							Metadata: types.Metadata{
								Name:      "dev",
								Namespace: apidefaults.Namespace,
								Labels: map[string]string{
									"team": "dev",
								},
							},
							Spec: types.RoleSpecV5{},
						},
					},
				},
				{
					name:    "all roles in the set have to match where condition",
					allowed: false,
					user: newUser("bob", props{
						traits: map[string][]string{"team": []string{"dev"}},
					}),
					roles: []types.Role{
						wildcardRole,
						&types.RoleV5{
							Metadata: types.Metadata{
								Name:      "dev",
								Namespace: apidefaults.Namespace,
								Labels: map[string]string{
									"team": "dev",
								},
							},
							Spec: types.RoleSpecV5{},
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
