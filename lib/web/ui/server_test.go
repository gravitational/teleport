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

func TestStripProtocolAndPort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		uri      string
		expected string
	}{
		{uri: "rediss://redis.example.com:6379?mode=cluster", expected: "redis.example.com"},
		{uri: "rediss://redis.example.com:6379", expected: "redis.example.com"},
		{uri: "https://abc12345.snowflakecomputing.com", expected: "abc12345.snowflakecomputing.com"},
		{uri: "mongodb://mongo1.example.com:27017,mongo2.example.com:27017/?replicaSet=rs0&readPreference=secondary", expected: "mongo1.example.com"},
		{uri: "mongodb+srv://cluster0.abcd.mongodb.net", expected: "cluster0.abcd.mongodb.net"},
		{uri: "mongo.example.com:27017", expected: "mongo.example.com"},
		{uri: "example.com", expected: "example.com"},
		{uri: "", expected: ""},
	}

	for _, tc := range cases {
		hostname := stripProtocolAndPort(tc.uri)
		require.Equal(t, tc.expected, hostname)
	}
}

func TestGetAllowedKubeUsersAndGroupsForCluster(t *testing.T) {
	devEnvRole := &types.RoleV6{
		Spec: types.RoleSpecV6{
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

	prodEnvRole := &types.RoleV6{
		Spec: types.RoleSpecV6{
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

	anyEnvRole := &types.RoleV6{
		Spec: types.RoleSpecV6{
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

	rootUser := &types.RoleV6{
		Spec: types.RoleSpecV6{
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

	roleWithMultipleLabels := &types.RoleV6{
		Spec: types.RoleSpecV6{
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
			roleSet: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
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
			roleSet: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
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
			roleSet: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
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
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV6{
				Spec: types.RoleSpecV6{
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
			roleSet: services.NewRoleSet(devEnvRole, &types.RoleV6{
				Spec: types.RoleSpecV6{
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
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", tc.roleSet)
			users, groups := getAllowedKubeUsersAndGroupsForCluster(accessChecker, tc.cluster)
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

func TestMakeClusterHiddenLabels(t *testing.T) {
	type testCase struct {
		name           string
		clusters       []types.KubeCluster
		expectedLabels [][]Label
		roleSet        services.RoleSet
	}

	testCases := []testCase{
		{
			name: "Single server with internal label",
			clusters: []types.KubeCluster{
				makeTestKubeCluster(t, map[string]string{
					"teleport.internal/test": "value1",
					"label2":                 "value2",
				}),
			},
			expectedLabels: [][]Label{
				{
					{
						Name:  "label2",
						Value: "value2",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clusterName", tc.roleSet)
			clusters := MakeKubeClusters(tc.clusters, accessChecker)
			for i, cluster := range clusters {
				require.Equal(t, tc.expectedLabels[i], cluster.Labels)
			}
		})
	}
}

func TestMakeServersHiddenLabels(t *testing.T) {
	type testCase struct {
		name           string
		clusterName    string
		servers        []types.Server
		expectedLabels [][]Label
		roleSet        services.RoleSet
	}

	testCases := []testCase{
		{
			name:        "Single server with internal label",
			clusterName: "cluster1",
			servers: []types.Server{
				makeTestServer(t, "server1", map[string]string{
					"simple":                "value1",
					"teleport.internal/app": "app1",
				}),
			},
			expectedLabels: [][]Label{
				{
					{
						Name:  "simple",
						Value: "value1",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", tc.roleSet)
			servers, err := MakeServers(tc.clusterName, tc.servers, accessChecker)
			require.NoError(t, err)
			for i, server := range servers {
				require.Equal(t, tc.expectedLabels[i], server.Labels)
			}
		})
	}
}

func makeTestServer(t *testing.T, name string, labels map[string]string) types.Server {
	server, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{}, labels)
	require.NoError(t, err)
	return server
}

func TestMakeDatabaseHiddenLabels(t *testing.T) {
	inputDb := &types.DatabaseV3{
		Metadata: types.Metadata{
			Name: "db name",
			Labels: map[string]string{
				"label":                    "value1",
				"teleport.internal/label2": "value2",
			},
		},
	}

	outputDb := MakeDatabase(inputDb, nil, nil)

	require.Equal(t, []Label{
		{
			Name:  "label",
			Value: "value1",
		},
	}, outputDb.Labels)
}

func TestMakeDesktopHiddenLabel(t *testing.T) {
	windowsDesktop, err := types.NewWindowsDesktopV3(
		"test",
		map[string]string{
			"teleport.internal/t2": "tt",
			"label3":               "value2",
		},
		types.WindowsDesktopSpecV3{Addr: "addr"},
	)
	require.NoError(t, err)

	accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.RoleSet{})
	desktop, err := MakeDesktop(windowsDesktop, accessChecker)
	require.NoError(t, err)
	labels := []Label{
		{
			Name:  "label3",
			Value: "value2",
		},
	}

	require.Equal(t, labels, desktop.Labels)
}

func TestMakeDesktopServiceHiddenLabel(t *testing.T) {
	windowsDesktopService := &types.WindowsDesktopServiceV3{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Labels: map[string]string{
					"teleport.internal/t2": "tt",
					"label3":               "value2",
				},
			},
		},
	}

	desktopService := MakeDesktopService(windowsDesktopService)
	labels := []Label{
		{
			Name:  "label3",
			Value: "value2",
		},
	}

	require.Equal(t, labels, desktopService.Labels)
}
