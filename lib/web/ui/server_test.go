/*
Copyright 2022 Gravitational, Inc.

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
package ui

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
)

func TestGetServerLogins(t *testing.T) {
	devEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Logins: []string{"devuser"},
				NodeLabels: types.Labels{
					"env": []string{"dev"},
				},
			},
		},
	}

	prodEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Logins: []string{"produser"},
				NodeLabels: types.Labels{
					"env": []string{"prod"},
				},
			},
		},
	}

	anyEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Logins: []string{"anyenvrole"},
				NodeLabels: types.Labels{
					"env": []string{"*"},
				},
			},
		},
	}

	rootUser := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Logins: []string{"root"},
				NodeLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		},
	}

	roleWithMultipleLabels := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				Logins: []string{"multiplelabelsuser"},
				NodeLabels: types.Labels{
					"region": []string{"*"},
					"env":    []string{"dev"},
				},
			},
		},
	}

	tt := []struct {
		name           string
		server         types.Server
		roleSet        services.RoleSet
		expectedLogins []string
	}{
		{
			name: "env dev login is added",
			server: makeTestServer(map[string]string{
				"env": "dev",
			}),
			roleSet:        services.NewRoleSet(devEnvRole),
			expectedLogins: []string{"devuser"},
		},
		{
			name: "env prod login is added",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "only the correct login is added",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole, devEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "logins from role not authorizeds are not added",
			server: makeTestServer(map[string]string{
				"env": "staging",
			}),
			roleSet:        services.NewRoleSet(devEnvRole, prodEnvRole),
			expectedLogins: []string{},
		},
		{
			name: "role with wildcard get its logins",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(anyEnvRole),
			expectedLogins: []string{"anyenvrole"},
		},
		{
			name: "can return multiple logins",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(anyEnvRole, prodEnvRole),
			expectedLogins: []string{"anyenvrole", "produser"},
		},
		{
			name: "can return multiple logins from same role",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						Logins: []string{"role1", "role2", "role3"},
						NodeLabels: types.Labels{
							"env": []string{"*"},
						},
					},
				},
			}),
			expectedLogins: []string{"role1", "role2", "role3"},
		},
		{
			name: "works with user with full access",
			server: makeTestServer(map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(rootUser),
			expectedLogins: []string{"root"},
		},
		{
			name: "works with server with multiple labels",
			server: makeTestServer(map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "don't add login from unrelated labels",
			server: makeTestServer(map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						Logins: []string{"anyregionuser"},
						NodeLabels: types.Labels{
							"region": []string{"*"},
						},
					},
				},
			}),
			expectedLogins: []string{},
		},
		{
			name: "works with roles with multiple labels that role shouldn't access",
			server: makeTestServer(map[string]string{
				"env": "dev",
			}),
			roleSet:        services.NewRoleSet(roleWithMultipleLabels),
			expectedLogins: []string{},
		},
		{
			name: "works with roles with multiple labels that role shouldn access",
			server: makeTestServer(map[string]string{
				"env":    "dev",
				"region": "us-west-1",
			}),
			roleSet:        services.NewRoleSet(roleWithMultipleLabels),
			expectedLogins: []string{"multiplelabelsuser"},
		},
		{
			name: "works with roles with regular expressions",
			server: makeTestServer(map[string]string{
				"region": "us-west-1",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						Logins: []string{"rolewithregexpuser"},
						NodeLabels: types.Labels{
							"region": []string{"^us-west-1|eu-central-1$"},
						},
					},
				},
			}),
			expectedLogins: []string{"rolewithregexpuser"},
		},
		{
			name: "works with denied roles",
			server: makeTestServer(map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV5{
				Spec: types.RoleSpecV5{
					Deny: types.RoleConditions{
						Logins: []string{"devuser"},
						NodeLabels: types.Labels{
							"env": []string{"*"},
						},
					},
				},
			}),
			expectedLogins: []string{},
		},
		{
			name: "works with denied roles of unrelated labels",
			server: makeTestServer(map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV5{
				Spec: types.RoleSpecV5{
					Deny: types.RoleConditions{
						Logins: []string{"devuser"},
						NodeLabels: types.Labels{
							"region": []string{"*"},
						},
					},
				},
			}),
			expectedLogins: []string{},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logins := getServerLogins(tc.server, tc.roleSet)
			require.Equal(t, tc.expectedLogins, logins)
		})
	}
}

// makeTestServer creates a server with labels and an empty spec.
// It panics in case of an error. Used only for testing
func makeTestServer(labels map[string]string) types.Server {
	s, err := types.NewServerWithLabels("server", types.KindNode, types.ServerSpecV2{}, labels)
	if err != nil {
		panic(err)
	}
	return s
}
