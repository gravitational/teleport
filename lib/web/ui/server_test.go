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

package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/ui"
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
		expectedLabels [][]ui.Label
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
			expectedLabels: [][]ui.Label{
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
		expectedLabels [][]ui.Label
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
			expectedLabels: [][]ui.Label{
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
			for i, srv := range tc.servers {
				server := MakeServer(tc.clusterName, srv, nil, false)
				assert.Equal(t, tc.expectedLabels[i], server.Labels)
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

	accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clusterName", nil)
	outputDb := MakeDatabase(inputDb, accessChecker, &mockDatabaseInteractiveChecker{}, false)

	require.Equal(t, []ui.Label{
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

	desktop := MakeDesktop(windowsDesktop, nil, false)
	labels := []ui.Label{
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
	labels := []ui.Label{
		{
			Name:  "label3",
			Value: "value2",
		},
	}

	require.Equal(t, labels, desktopService.Labels)
}

func TestSortedLabels(t *testing.T) {
	type testCase struct {
		name           string
		clusterName    string
		servers        []types.Server
		expectedLabels [][]ui.Label
	}

	testCases := []testCase{
		{
			name:        "Server with aws labels pushed to back",
			clusterName: "cluster1",
			servers: []types.Server{
				makeTestServer(t, "server1", map[string]string{
					"teleport.dev/origin":   "config-file",
					"aws/asdfasdf":          "hello",
					"simple":                "value1",
					"ultra-cool-label":      "value1",
					"teleport.internal/app": "app1",
				}),
			},
			expectedLabels: [][]ui.Label{
				{
					{
						Name:  "simple",
						Value: "value1",
					},
					{
						Name:  "ultra-cool-label",
						Value: "value1",
					},
					{
						Name:  "teleport.dev/origin",
						Value: "config-file",
					},
					{
						Name:  "aws/asdfasdf",
						Value: "hello",
					},
				},
			},
		},
		{
			name:        "database with azure labels pushed to back",
			clusterName: "cluster1",
			servers: []types.Server{
				makeTestServer(t, "server1", map[string]string{
					"azure/asdfasdf":        "hello",
					"simple":                "value1",
					"anotherone":            "value2",
					"teleport.internal/app": "app1",
				}),
			},
			expectedLabels: [][]ui.Label{
				{
					{
						Name:  "anotherone",
						Value: "value2",
					},
					{
						Name:  "simple",
						Value: "value1",
					},
					{
						Name:  "azure/asdfasdf",
						Value: "hello",
					},
				},
			},
		},
		{
			name:        "Server with gcp labels pushed to back",
			clusterName: "cluster1",
			servers: []types.Server{
				makeTestServer(t, "server1", map[string]string{
					"gcp/asdfasdf":          "hello",
					"simple":                "value1",
					"teleport.internal/app": "app1",
				}),
			},
			expectedLabels: [][]ui.Label{
				{
					{
						Name:  "simple",
						Value: "value1",
					},
					{
						Name:  "gcp/asdfasdf",
						Value: "hello",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, srv := range tc.servers {
				server := MakeServer(tc.clusterName, srv, nil, false)
				assert.Equal(t, tc.expectedLabels[i], server.Labels)
			}
		})
	}
}

func TestMakeDatabaseSupportsInteractive(t *testing.T) {
	db := &types.DatabaseV3{}
	accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clusterName", nil)

	for name, tc := range map[string]struct {
		supports bool
	}{
		"supported":   {supports: true},
		"unsupported": {supports: false},
	} {
		t.Run(name, func(t *testing.T) {
			interactiveChecker := &mockDatabaseInteractiveChecker{supports: tc.supports}
			single := MakeDatabase(db, accessChecker, interactiveChecker, false)
			require.Equal(t, tc.supports, single.SupportsInteractive)

			multi := MakeDatabases([]*types.DatabaseV3{db}, accessChecker, interactiveChecker)
			require.Len(t, multi, 1)
			require.Equal(t, tc.supports, multi[0].SupportsInteractive)
		})
	}
}

func TestMakeDatabaseConnectOptions(t *testing.T) {
	interactiveChecker := &mockDatabaseInteractiveChecker{}

	for name, tc := range map[string]struct {
		roles        services.RoleSet
		db           *types.DatabaseV3
		assertResult require.ValueAssertionFunc
		username     string
	}{
		"names wildcard": {
			db: makeTestDatabase(t, map[string]string{"env": "dev"}, false),
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseNames:  []string{"*", "mydatabase"},
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.ElementsMatch(t, []string{"*", "mydatabase"}, db.DatabaseNames)
			},
		},
		"users wildcard": {
			db: makeTestDatabase(t, map[string]string{"env": "dev"}, false),
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseUsers:  []string{"*", "myuser"},
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.ElementsMatch(t, []string{"*", "myuser"}, db.DatabaseUsers)
			},
		},
		"roles wildcard": {
			db: makeTestDatabase(t, map[string]string{"env": "dev"}, false),
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseRoles:  []string{"*", "myrole"},
					},
					Options: types.RoleOptions{
						CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.ElementsMatch(t, []string{"*", "myrole"}, db.DatabaseRoles)
			},
		},
		"only wildcards": {
			db: makeTestDatabase(t, map[string]string{"env": "dev"}, false),
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseNames:  []string{"*"},
						DatabaseUsers:  []string{"*"},
						DatabaseRoles:  []string{"*"},
					},
					Options: types.RoleOptions{
						CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.ElementsMatch(t, []string{"*"}, db.DatabaseNames)
				require.ElementsMatch(t, []string{"*"}, db.DatabaseUsers)
				require.ElementsMatch(t, []string{"*"}, db.DatabaseRoles)
			},
		},
		"auto-user provisioning enabled": {
			db:       makeTestDatabase(t, map[string]string{"env": "dev"}, true),
			username: "alice",
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseUsers:  []string{"otheruser"},
						DatabaseRoles:  []string{"myrole"},
					},
					Options: types.RoleOptions{
						CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.True(t, db.AutoUsersEnabled)
				require.ElementsMatch(t, []string{"alice"}, db.DatabaseUsers)
				require.ElementsMatch(t, []string{"myrole"}, db.DatabaseRoles)
			},
		},
		"auto-user provisioning at database but disabled on role": {
			db:       makeTestDatabase(t, map[string]string{"env": "dev"}, true),
			username: "alice",
			roles: services.NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseUsers:  []string{"*", "myuser"},
					},
					Options: types.RoleOptions{
						CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
					},
				},
			}),
			assertResult: func(t require.TestingT, v interface{}, _ ...interface{}) {
				db, _ := v.(Database)
				require.False(t, db.AutoUsersEnabled)
				require.ElementsMatch(t, []string{"*", "myuser"}, db.DatabaseUsers)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{Username: tc.username}, "clusterName", tc.roles)
			single := MakeDatabase(tc.db, accessChecker, interactiveChecker, false)
			tc.assertResult(t, single)

			multi := MakeDatabases([]*types.DatabaseV3{tc.db}, accessChecker, interactiveChecker)
			require.Len(t, multi, 1)
			tc.assertResult(t, multi[0])
		})
	}
}

// makeTestDatabase creates a database with labels and admin options.
func makeTestDatabase(t *testing.T, labels map[string]string, autoProvisioningEnabled bool) *types.DatabaseV3 {
	t.Helper()

	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	}
	if autoProvisioningEnabled {
		spec.AdminUser = &types.DatabaseAdminUser{
			Name:            "teleport-admin",
			DefaultDatabase: "teleport",
		}
	}

	db, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   "db",
			Labels: labels,
		}, spec,
	)
	require.NoError(t, err)
	if autoProvisioningEnabled {
		require.True(t, db.IsAutoUsersEnabled(), "The database was expected to have auto-users enabled but it isn't. Check if the auto-users enabled definition changed and update this helper to match it.")
	}

	return db
}

type mockDatabaseInteractiveChecker struct {
	supports bool
}

func (m *mockDatabaseInteractiveChecker) IsSupported(_ string) bool {
	return m.supports
}
