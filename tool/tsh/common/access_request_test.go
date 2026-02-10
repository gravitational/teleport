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

package common

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAccessRequestSearch(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.K8s: {Enabled: true},
			},
		},
	},
	)
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })
	ctx := context.Background()
	const (
		rootClusterName = "root-cluster"
		rootKubeCluster = "first-cluster"
		leafClusterName = "leaf-cluster"
		leafKubeCluster = "second-cluster"
	)
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.ClusterName.SetClusterName(rootClusterName)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Kube.Enabled = true
			cfg.SSH.Enabled = false
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, rootKubeCluster)
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
				cfg.Auth.ClusterName.SetClusterName(leafClusterName)
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				cfg.Kube.Enabled = true
				cfg.SSH.Enabled = false
				cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
				cfg.Kube.KubeconfigPath = newKubeConfigFile(t, leafKubeCluster)
			},
		),
		withValidationFunc(func(s *suite) bool {
			rootClusters, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			leafClusters, err := s.leaf.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			return len(rootClusters) >= 1 && len(leafClusters) >= 1
		}),
	)

	// We create another kube_server in the leaf cluster to test that we can
	// deduplicate the results when multiple replicas of the same resource
	// exist.
	kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: leafKubeCluster,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.leaf.GetAuthServer().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	type args struct {
		teleportCluster string
		kind            string
		extraArgs       []string
	}
	tests := []struct {
		name      string
		args      args
		wantTable func() string
	}{
		{
			name: "list pods in root cluster for default namespace",
			args: args{
				teleportCluster: rootClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", rootKubeCluster)},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", rootClusterName, rootKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-2", rootClusterName, rootKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/test", rootClusterName, rootKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in root cluster for dev namespace with search",
			args: args{
				teleportCluster: rootClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", rootKubeCluster), "--kube-namespace=dev", "--search=nginx-1"},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/kube:ns:pods/%s/dev/nginx-1", rootClusterName, rootKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in leaf cluster for default namespace",
			args: args{
				teleportCluster: leafClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", leafKubeCluster)},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-2", leafClusterName, leafKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/test", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in leaf cluster for all namespaces",
			args: args{
				teleportCluster: leafClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", leafKubeCluster), "--all-kube-namespaces", "--search=nginx-1"},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/kube:ns:pods/%s/dev/nginx-1", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list kube clusters in leaf cluster",
			args: args{
				teleportCluster: leafClusterName,
				kind:            types.KindKubernetesCluster,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Hostname", "Labels", "Resource ID"},
					[][]string{
						{leafKubeCluster, "", "", fmt.Sprintf("/%s/kube_cluster/%s", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			homePath, _ := mustLoginLegacy(t, s, tc.args.teleportCluster)
			captureStdout := new(bytes.Buffer)
			err := Run(
				ctx,
				append([]string{
					"--insecure",
					"request",
					"search",
					fmt.Sprintf("--kind=%s", tc.args.kind),
				},
					tc.args.extraArgs...,
				),
				setCopyStdout(captureStdout),
				setHomePath(homePath),
			)
			require.NoError(t, err)
			// We append a newline to the expected output to esnure that the table
			// does not contain any more rows than expected.
			require.Contains(t, captureStdout.String(), tc.wantTable()+"\n")
		})
	}
}

func TestShowRequestTable(t *testing.T) {
	createdAtTime := time.Now()
	expiresTime := time.Now().Add(time.Hour)
	assumeStartTime := createdAtTime.Add(30 * time.Minute)
	tests := []struct {
		name        string
		reqs        []types.AccessRequest
		wantPresent []string
	}{
		{
			name: "Access Requests without assume time",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "someName",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:  "someUser",
						Roles: []string{"role1", "role2"},

						Expires:    expiresTime,
						SessionTTL: expiresTime,
						Created:    createdAtTime,
					},
				},
			},
			wantPresent: []string{"someName", "someUser", "role1,role2"},
		},
		{
			name: "Access Requests with assume time",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "someName",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:  "someUser",
						Roles: []string{"role1", "role2"},

						Expires:         expiresTime,
						SessionTTL:      expiresTime,
						AssumeStartTime: &assumeStartTime,
						Created:         createdAtTime,
					},
				},
			},
			wantPresent: []string{"someName", "someUser", "role1,role2", assumeStartTime.UTC().Format(time.RFC822)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			captureStdout := new(bytes.Buffer)
			cf := &CLIConf{
				OverrideStdout: captureStdout,
			}
			err := showRequestTable(cf, tc.reqs)
			require.NoError(t, err)
			for _, wanted := range tc.wantPresent {
				require.Contains(t, captureStdout.String(), wanted)
			}
		})
	}
}

func TestRequestSearchRequestableRoles(t *testing.T) {
	ctx := t.Context()
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)

	accessRole, err := types.NewRole("access", types.RoleSpecV6{})
	require.NoError(t, err)
	accessRole.SetMetadata(types.Metadata{
		Name:        "access",
		Description: "base access role",
	})

	dbAdminRole, err := types.NewRole("db-admin", types.RoleSpecV6{})
	require.NoError(t, err)
	dbAdminRole.SetMetadata(types.Metadata{
		Name:        "db-admin",
		Description: "database administrator role",
	})

	unrequestableRole, err := types.NewRole("unrequestable", types.RoleSpecV6{})
	require.NoError(t, err)
	unrequestableRole.SetMetadata(types.Metadata{
		Name:        "unrequestable",
		Description: "role that exists but is not requestable for this user",
	})

	requesterRole, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"access", "db-admin"},
			},
		},
	})
	require.NoError(t, err)

	user, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	user.SetRoles([]string{"requester"})

	auth, proxy := makeTestServers(t,
		withBootstrap(
			connector,
			accessRole,
			dbAdminRole,
			unrequestableRole,
			requesterRole,
			user,
		),
	)
	authServer := auth.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	wantRolesTable := func() string {
		table := asciitable.MakeTable([]string{"Role", "Description"})
		table.AddRow([]string{"access", "base access role"})
		table.AddRow([]string{"db-admin", "database administrator role"})
		return table.AsBuffer().String()
	}

	tests := []struct {
		name string
		args []string
		// If empty, we expect the command to succeed.
		errMessage string
		want       func() string
	}{
		{
			name:       "list requestable roles",
			args:       []string{"request", "search", "--roles"},
			errMessage: "",
			want:       wantRolesTable,
		},
		{
			name:       "both kind and roles set",
			args:       []string{"request", "search", "--kind=node", "--roles"},
			errMessage: "only one of --kind and --roles may be specified",
			want:       nil,
		},
		{
			name:       "neither kind nor roles set",
			args:       []string{"request", "search"},
			errMessage: "one of --kind and --roles is required",
			want:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer

			err := Run(ctx,
				append([]string{"--insecure"}, tc.args...),
				setHomePath(tmpHomePath),
				setCopyStdout(&stdout),
			)

			if tc.errMessage != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMessage)
				return
			}

			require.NoError(t, err)
			if tc.want != nil {
				require.Equal(t, tc.want(), stdout.String())
			}
		})
	}
}

func TestPrintRequestableResources(t *testing.T) {
	rows := []kubeResourceRow{
		{
			Name:       "pod-1",
			Namespace:  "default",
			Labels:     "env=prod",
			ResourceID: "id1",
		},
		{
			Name:       "pod-2",
			Namespace:  "dev",
			Labels:     "env=dev",
			ResourceID: "id2",
		},
	}
	resourceIDs := []string{"id1", "id2"}

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
		}

		err := printRequestableResources(cf, rows, resourceIDs)
		require.NoError(t, err)

		// Build expected table using asciitable.
		table := asciitable.MakeTable(
			[]string{"Name", "Namespace", "Labels", "Resource ID"},
			[][]string{
				{"pod-1", "default", "env=prod", "id1"},
				{"pod-2", "dev", "env=dev", "id2"},
			}...,
		)
		expectedTable := table.AsBuffer().String()
		out := buf.String()

		require.Contains(t, out, expectedTable)
		require.Contains(t, out, "To request access to these resources, run")
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "json",
		}

		err := printRequestableResources(cf, rows, resourceIDs)
		require.NoError(t, err)

		got := buf.String()
		wantJSON := `
[
{
	"Name": "pod-1",
	"Namespace": "default",
	"Labels": "env=prod",
	"ResourceID": "id1"
},
{
	"Name": "pod-2",
	"Namespace": "dev",
	"Labels": "env=dev",
	"ResourceID": "id2"
}
]
`
		require.JSONEq(t, wantJSON, got)
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "yaml",
		}

		err := printRequestableResources(cf, rows, resourceIDs)
		require.NoError(t, err)

		got := buf.String()
		wantYAML := `
- Name: pod-1
  Namespace: default
  Labels: env=prod
  ResourceID: id1
- Name: pod-2
  Namespace: dev
  Labels: env=dev
  ResourceID: id2
`
		require.YAMLEq(t, wantYAML, got)
	})

	t.Run("empty rows text", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "",
			Verbose:        true,
		}

		err := printRequestableResources(cf, []kubeResourceRow{}, nil)
		require.NoError(t, err)

		out := buf.String()
		table := asciitable.MakeTable(
			[]string{"Name", "Namespace", "Labels", "Resource ID"},
		)
		expectedTable := table.AsBuffer().String()
		require.Equal(t, expectedTable, out)
	})

	t.Run("unsupported format", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "random",
		}

		err := printRequestableResources(cf, rows, resourceIDs)
		require.Error(t, err)
	})
}

func TestPrintRequestableRoles(t *testing.T) {
	rows := []requestableRoleRow{
		{
			Role:        "access",
			Description: "base access role",
		},
		{
			Role:        "db-admin",
			Description: "database administrator role",
		},
	}

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
		}

		err := printRequestableRoles(cf, rows)
		require.NoError(t, err)

		table := asciitable.MakeTable(
			[]string{"Role", "Description"},
			[][]string{
				{"access", "base access role"},
				{"db-admin", "database administrator role"},
			}...,
		)
		expectedTable := table.AsBuffer().String()
		out := buf.String()

		require.Equal(t, expectedTable, out)
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "json",
		}

		err := printRequestableRoles(cf, rows)
		require.NoError(t, err)

		got := buf.String()
		const wantJSON = `
[
  {
    "Role": "access",
    "Description": "base access role"
  },
  {
    "Role": "db-admin",
    "Description": "database administrator role"
  }
]
`
		require.JSONEq(t, wantJSON, got)
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "yaml",
		}

		err := printRequestableRoles(cf, rows)
		require.NoError(t, err)

		got := buf.String()
		const wantYAML = `
- Role: access
  Description: base access role
- Role: db-admin
  Description: database administrator role
`
		require.YAMLEq(t, wantYAML, got)
	})

	t.Run("empty roles text", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
		}

		err := printRequestableRoles(cf, nil)
		require.NoError(t, err)

		require.Equal(t, "No requestable roles found.\n", buf.String())
	})

	t.Run("unsupported_format", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{
			OverrideStdout: &buf,
			Format:         "random",
		}

		err := printRequestableRoles(cf, rows)
		require.Error(t, err)
	})
}
