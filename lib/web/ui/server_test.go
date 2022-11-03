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

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestGetAllowedKubeUsersAndGroupsForCluster(t *testing.T) {
	devEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"devuser"},
				KubeGroups: []string{"devgroup"},
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"env": []string{"dev"},
				},
			},
		},
	}

	prodEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"produser"},
				KubeGroups: []string{"prodgroup"},
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"env": []string{"prod"},
				},
			},
		},
	}

	anyEnvRole := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"anyenvrole"},
				KubeGroups: []string{"anyenvgroup"},
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"env": []string{"*"},
				},
			},
		},
	}

	rootUser := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"root"},
				KubeGroups: []string{"rootgroup"},
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		},
	}

	roleWithMultipleLabels := &types.RoleV5{
		Spec: types.RoleSpecV5{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"multiplelabelsuser"},
				KubeGroups: []string{"multiplelabelsgroup"},
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"region": []string{"*"},
					"env":    []string{"dev"},
				},
			},
		},
	}

	tt := []struct {
		name           string
		cluster        types.KubeCluster
		roleSet        services.RoleSet
		expectedUsers  []string
		expectedGroups []string
	}{
		{
			name: "env dev user and group is added",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "dev",
			}),
			roleSet:        services.NewRoleSet(devEnvRole),
			expectedUsers:  []string{"devuser"},
			expectedGroups: []string{"devgroup"},
		},
		{
			name: "env prod user and group is added",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole),
			expectedUsers:  []string{"produser"},
			expectedGroups: []string{"prodgroup"},
		},
		{
			name: "only the correct prod is added",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole, devEnvRole),
			expectedUsers:  []string{"produser"},
			expectedGroups: []string{"prodgroup"},
		},
		{
			name: "users and groups from role not authorized are denied",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "staging",
			}),
			roleSet:        services.NewRoleSet(devEnvRole, prodEnvRole),
			expectedUsers:  nil,
			expectedGroups: nil,
		},
		{
			name: "role with wildcard gets group and user",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(anyEnvRole),
			expectedUsers:  []string{"anyenvrole"},
			expectedGroups: []string{"anyenvgroup"},
		},
		{
			name: "can return multiple users and groups",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(anyEnvRole, prodEnvRole),
			expectedUsers:  []string{"anyenvrole", "produser"},
			expectedGroups: []string{"anyenvgroup", "prodgroup"},
		},
		{
			name: "can return multiple users and groups from same role",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						KubeUsers:  []string{"role1", "role2", "role3"},
						Namespaces: []string{apidefaults.Namespace},
						KubernetesLabels: types.Labels{
							"env": []string{"*"},
						},
					},
				},
			}),
			expectedUsers: []string{"role1", "role2", "role3"},
		},
		{
			name: "works with full access",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "prod",
			}),
			roleSet:        services.NewRoleSet(rootUser),
			expectedUsers:  []string{"root"},
			expectedGroups: []string{"rootgroup"},
		},
		{
			name: "works with server with multiple labels",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			}),
			roleSet:        services.NewRoleSet(prodEnvRole),
			expectedUsers:  []string{"produser"},
			expectedGroups: []string{"prodgroup"},
		},
		{
			name: "don't add login from unrelated labels",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						KubeGroups: []string{"anyregiongroup"},
						Namespaces: []string{apidefaults.Namespace},
						KubernetesLabels: types.Labels{
							"region": []string{"*"},
						},
					},
				},
			}),
			expectedUsers:  nil,
			expectedGroups: nil,
		},
		{
			name: "works with roles with multiple labels that role shouldn't access",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "dev",
			}),
			roleSet:        services.NewRoleSet(roleWithMultipleLabels),
			expectedUsers:  nil,
			expectedGroups: nil,
		},
		{
			name: "works with roles with multiple labels that role shouldn't access",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env":    "dev",
				"region": "us-west-1",
			}),
			roleSet:        services.NewRoleSet(roleWithMultipleLabels),
			expectedUsers:  []string{"multiplelabelsuser"},
			expectedGroups: []string{"multiplelabelsgroup"},
		},
		{
			name: "works with roles with regular expressions",
			cluster: makeTestKubeCluster(t, map[string]string{
				"region": "us-west-1",
			}),
			roleSet: services.NewRoleSet(&types.RoleV5{
				Spec: types.RoleSpecV5{
					Allow: types.RoleConditions{
						KubeUsers:  []string{"rolewithregexpuser"},
						Namespaces: []string{apidefaults.Namespace},
						KubernetesLabels: types.Labels{
							"region": []string{"^us-west-1|eu-central-1$"},
						},
					},
				},
			}),
			expectedUsers: []string{"rolewithregexpuser"},
		},
		{
			name: "works with denied roles",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV5{
				Spec: types.RoleSpecV5{
					Deny: types.RoleConditions{
						KubeUsers:  []string{"devuser"},
						KubeGroups: []string{"devgroup"},
						Namespaces: []string{apidefaults.Namespace},
						KubernetesLabels: types.Labels{
							"env": []string{"*"},
						},
					},
				},
			}),
			expectedUsers: nil,
		},
		{
			name: "works with denied roles of unrelated labels",
			cluster: makeTestKubeCluster(t, map[string]string{
				"env": "dev",
			}),
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV5{
				Spec: types.RoleSpecV5{
					Deny: types.RoleConditions{
						KubeUsers:  []string{"devuser"},
						KubeGroups: []string{"devgroup"},
						Namespaces: []string{apidefaults.Namespace},
						KubernetesLabels: types.Labels{
							"region": []string{"*"},
						},
					},
				},
			}),
			expectedUsers: nil,
		},
	}
	for _, tc := range tt[:1] {
		t.Run(tc.name, func(t *testing.T) {
			users, groups := getAllowedKubeUsersAndGroupsForCluster(tc.roleSet, tc.cluster)
			require.Equal(t, tc.expectedUsers, users)
			require.Equal(t, tc.expectedGroups, groups)
		})
	}
}

// makeTestKubeCluster creates a kube cluster with labels and an empty spec.
func makeTestKubeCluster(t *testing.T, labels map[string]string) types.KubeCluster {
	s, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   "kube_cluster",
			Labels: labels,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	return s
}
