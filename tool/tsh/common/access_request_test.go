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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAccessRequestSearch(t *testing.T) {
	ctx := context.Background()
	const (
		rootClusterName = "root-cluster"
		rootKubeCluster = "first-cluster"
		leafClusterName = "leaf-cluster"
		leafKubeCluster = "second-cluster"
	)
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Modules = modulestest.EnterpriseModules()
			cfg.InsecureMode = true
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
				cfg.Modules = modulestest.EnterpriseModules()
				cfg.InsecureMode = true
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
				// kube_cluster now lists via ListUnifiedResources: the Resource ID
				// column is replaced by the granted/requestable Access summary,
				// which is empty for kinds without selectable principals.
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Hostname", "Labels", "Access"},
					[][]string{
						{leafKubeCluster, "", "", ""},
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
		{
			name: "Access Requests with constrained resources",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "constrainedRequest",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:       "someUser",
						Roles:      []string{"aws-access"},
						Expires:    expiresTime,
						SessionTTL: expiresTime,
						Created:    createdAtTime,
						RequestedResourceAccessIDs: []types.ResourceAccessID{
							{
								Id: types.ResourceID{
									ClusterName: "test-cluster",
									Kind:        types.KindApp,
									Name:        "awsconsole",
								},
								Constraints: &types.ResourceConstraints{
									Version: types.V1,
									Details: &types.ResourceConstraints_AwsConsole{
										AwsConsole: &types.AWSConsoleResourceConstraints{
											RoleArns: []string{"arn:aws:iam::123456789012:role/Admin"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantPresent: []string{"constrainedRequest", "someUser", "aws-access"},
		},
		{
			name: "Access Requests with namespace resources",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "namespaceRequest",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:       "someUser",
						Roles:      []string{"kube-access"},
						Expires:    expiresTime,
						SessionTTL: expiresTime,
						Created:    createdAtTime,
						RequestedResourceIDs: []types.ResourceID{
							{
								ClusterName:     "test-cluster",
								Kind:            types.KindKubeNamespace,
								Name:            "my-kube-cluster",
								SubResourceName: "production",
							},
						},
					},
				},
			},
			wantPresent: []string{"namespaceRequest", "someUser", "kube-access"},
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

func TestPrintRequest(t *testing.T) {
	t.Parallel()

	createdAtTime := time.Now()
	expiresTime := time.Now().Add(time.Hour)

	tests := []struct {
		name        string
		req         types.AccessRequest
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name: "basic request without resources",
			req: &types.AccessRequestV3{
				Metadata: types.Metadata{
					Name:    "basic-request",
					Expires: &expiresTime,
				},
				Spec: types.AccessRequestSpecV3{
					User:       "testuser",
					Roles:      []string{"admin", "developer"},
					Expires:    expiresTime,
					SessionTTL: expiresTime,
					Created:    createdAtTime,
				},
			},
			wantPresent: []string{
				"Request ID:",
				"basic-request",
				"Username:",
				"testuser",
				"Roles:",
				"admin, developer",
			},
			wantAbsent: []string{"Resources:"},
		},
		{
			name: "request with constrained AWS console resources",
			req: &types.AccessRequestV3{
				Metadata: types.Metadata{
					Name:    "aws-request",
					Expires: &expiresTime,
				},
				Spec: types.AccessRequestSpecV3{
					User:       "testuser",
					Roles:      []string{"aws-access"},
					Expires:    expiresTime,
					SessionTTL: expiresTime,
					Created:    createdAtTime,
					RequestedResourceAccessIDs: []types.ResourceAccessID{
						{
							Id: types.ResourceID{
								ClusterName: "test-cluster",
								Kind:        types.KindApp,
								Name:        "awsconsole",
							},
							Constraints: &types.ResourceConstraints{
								Version: types.V1,
								Details: &types.ResourceConstraints_AwsConsole{
									AwsConsole: &types.AWSConsoleResourceConstraints{
										RoleArns: []string{
											"arn:aws:iam::123456789012:role/Admin",
											"arn:aws:iam::123456789012:role/ReadOnly",
										},
									},
								},
							},
						},
					},
				},
			},
			wantPresent: []string{
				"Request ID:",
				"aws-request",
				"Username:",
				"testuser",
				"Roles:",
				"aws-access",
				"Resources:",
				"/test-cluster/app/awsconsole",
				"role_arns=",
				"arn:aws:iam::123456789012:role/Admin",
				"arn:aws:iam::123456789012:role/ReadOnly",
			},
		},
		{
			name: "request with constrained SSH node resources",
			req: &types.AccessRequestV3{
				Metadata: types.Metadata{
					Name:    "ssh-request",
					Expires: &expiresTime,
				},
				Spec: types.AccessRequestSpecV3{
					User:       "testuser",
					Roles:      []string{"ssh-access"},
					Expires:    expiresTime,
					SessionTTL: expiresTime,
					Created:    createdAtTime,
					RequestedResourceAccessIDs: []types.ResourceAccessID{
						{
							Id: types.ResourceID{
								ClusterName: "test-cluster",
								Kind:        types.KindNode,
								Name:        "web-1",
							},
							Constraints: &types.ResourceConstraints{
								Version: types.V1,
								Details: &types.ResourceConstraints_Ssh{
									Ssh: &types.SSHResourceConstraints{
										Logins: []string{"root", "admin"},
									},
								},
							},
						},
					},
				},
			},
			wantPresent: []string{
				"Request ID:",
				"ssh-request",
				"Username:",
				"testuser",
				"Roles:",
				"ssh-access",
				"Resources:",
				"/test-cluster/node/web-1",
				"logins=root,admin",
			},
		},
		{
			name: "request with kubernetes namespace resources",
			req: &types.AccessRequestV3{
				Metadata: types.Metadata{
					Name:    "kube-request",
					Expires: &expiresTime,
				},
				Spec: types.AccessRequestSpecV3{
					User:       "testuser",
					Roles:      []string{"kube-access"},
					Expires:    expiresTime,
					SessionTTL: expiresTime,
					Created:    createdAtTime,
					RequestedResourceIDs: []types.ResourceID{
						{
							ClusterName:     "test-cluster",
							Kind:            types.KindKubeNamespace,
							Name:            "my-kube-cluster",
							SubResourceName: "production",
						},
					},
				},
			},
			wantPresent: []string{
				"Request ID:",
				"kube-request",
				"Username:",
				"testuser",
				"Roles:",
				"kube-access",
				"Resources:",
				"/test-cluster/namespace/my-kube-cluster/production",
			},
		},
		{
			name: "request with mixed resources and constraints",
			req: &types.AccessRequestV3{
				Metadata: types.Metadata{
					Name:    "mixed-request",
					Expires: &expiresTime,
				},
				Spec: types.AccessRequestSpecV3{
					User:       "testuser",
					Roles:      []string{"multi-access"},
					Expires:    expiresTime,
					SessionTTL: expiresTime,
					Created:    createdAtTime,
					RequestedResourceIDs: []types.ResourceID{
						{
							ClusterName: "test-cluster",
							Kind:        types.KindNode,
							Name:        "my-server",
						},
					},
					RequestedResourceAccessIDs: []types.ResourceAccessID{
						{
							Id: types.ResourceID{
								ClusterName: "test-cluster",
								Kind:        types.KindApp,
								Name:        "awsconsole",
							},
							Constraints: &types.ResourceConstraints{
								Version: types.V1,
								Details: &types.ResourceConstraints_AwsConsole{
									AwsConsole: &types.AWSConsoleResourceConstraints{
										RoleArns: []string{"arn:aws:iam::123456789012:role/Admin"},
									},
								},
							},
						},
					},
				},
			},
			wantPresent: []string{
				"Request ID:",
				"mixed-request",
				"Resources:",
				"/test-cluster/node/my-server",
				"/test-cluster/app/awsconsole",
				"role_arns=",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			captureStdout := new(bytes.Buffer)
			cf := &CLIConf{
				OverrideStdout: captureStdout,
			}
			err := printRequest(cf, tc.req)
			require.NoError(t, err)
			output := captureStdout.String()
			for _, wanted := range tc.wantPresent {
				require.Contains(t, output, wanted, "expected output to contain %q", wanted)
			}
			for _, unwanted := range tc.wantAbsent {
				require.NotContains(t, output, unwanted, "expected output to not contain %q", unwanted)
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

func TestPrincipalSplits(t *testing.T) {
	tests := []struct {
		name         string
		resourceKind string
		enriched     *types.EnrichedResource
		want         map[string]principalSplit
	}{
		{
			name:         "single dimension",
			resourceKind: types.KindNode,
			enriched: &types.EnrichedResource{
				Logins: []string{"admin", "deploy", "root"},
				Principals: []types.ResourcePrincipalSet{{
					Kind:    types.PrincipalKindLogins,
					All:     []string{"admin", "deploy", "root"},
					Granted: []string{"deploy"},
				}},
			},
			want: map[string]principalSplit{
				types.PrincipalKindLogins: {granted: []string{"deploy"}, requestable: []string{"admin", "root"}},
			},
		},
		{
			name:         "multiple dimensions",
			resourceKind: types.KindDatabase,
			enriched: &types.EnrichedResource{
				Principals: []types.ResourcePrincipalSet{
					{Kind: "db_users", All: []string{"admin", "reader"}, Granted: []string{"reader"}},
					{Kind: "db_names", All: []string{"prod", "reports"}, Granted: []string{"reports"}},
				},
			},
			want: map[string]principalSplit{
				"db_users": {granted: []string{"reader"}, requestable: []string{"admin"}},
				"db_names": {granted: []string{"reports"}, requestable: []string{"prod"}},
			},
		},
		{
			// Older Auth returns only the union; everything is shown as
			// requestable rather than mislabeled as granted.
			name:         "union only (mixed-version fallback)",
			resourceKind: types.KindNode,
			enriched:     &types.EnrichedResource{Logins: []string{"root", "admin"}},
			want: map[string]principalSplit{
				types.PrincipalKindLogins: {requestable: []string{"admin", "root"}},
			},
		},
		{
			// The fallback dimension follows the resource kind: the flat union
			// carries ARNs for apps.
			name:         "union only app fallback",
			resourceKind: types.KindApp,
			enriched:     &types.EnrichedResource{Logins: []string{"arn:aws:iam::123:role/Admin"}},
			want: map[string]principalSplit{
				types.PrincipalKindRoleARNs: {requestable: []string{"arn:aws:iam::123:role/Admin"}},
			},
		},
		{
			name:         "no principals",
			resourceKind: types.KindNode,
			enriched:     &types.EnrichedResource{},
			want:         nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, principalSplits(tt.enriched, tt.resourceKind))
		})
	}
}

func TestFormatAccessSummary(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]principalSplit
		want string
	}{
		{
			"both",
			map[string]principalSplit{"logins": {granted: []string{"a"}, requestable: []string{"b", "c"}}},
			"1 granted, 2 requestable",
		},
		{
			"counts across dimensions",
			map[string]principalSplit{
				"db_users": {granted: []string{"a"}, requestable: []string{"b"}},
				"db_names": {granted: []string{"c"}, requestable: []string{"d", "e"}},
			},
			"2 granted, 3 requestable",
		},
		{"granted only", map[string]principalSplit{"logins": {granted: []string{"a", "b", "c"}}}, "3 granted"},
		{"requestable only", map[string]principalSplit{"logins": {requestable: []string{"a"}}}, "1 requestable"},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatAccessSummary(tt.in))
		})
	}
}

func TestPrintResourcePreview(t *testing.T) {
	server, err := types.NewServer("web-1", types.KindNode, types.ServerSpecV2{Hostname: "web-1.dc1"})
	require.NoError(t, err)
	server.SetStaticLabels(map[string]string{"env": "prod"})
	id := types.ResourceID{ClusterName: "main", Kind: types.KindNode, Name: "web-1"}
	splits := map[string]principalSplit{
		types.PrincipalKindLogins: {granted: []string{"deploy"}, requestable: []string{"admin", "root"}},
	}

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf}
		require.NoError(t, printResourcePreview(cf, id, server, splits))
		out := buf.String()
		require.Contains(t, out, "Resource:  /main/node/web-1")
		require.Contains(t, out, "Hostname:  web-1.dc1")
		require.Contains(t, out, "Logins:")
		require.Contains(t, out, "deploy")
		require.Contains(t, out, "granted")
		require.Contains(t, out, "requestable")
		// The create hint scopes to the requestable principals.
		require.Contains(t, out, "logins=admin,root")
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf, Format: "json"}
		require.NoError(t, printResourcePreview(cf, id, server, splits))
		require.JSONEq(t, `{
			"resource_id": "/main/node/web-1",
			"kind": "node",
			"name": "web-1",
			"hostname": "web-1.dc1",
			"labels": {"env": "prod"},
			"principals": {
				"logins": {"granted": ["deploy"], "requestable": ["admin", "root"]}
			}
		}`, buf.String())
	})

	t.Run("multiple dimensions", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf}
		multi := map[string]principalSplit{
			types.PrincipalKindLogins: {requestable: []string{"root"}},
			"future_kind":             {granted: []string{"x"}, requestable: []string{"y"}},
		}
		require.NoError(t, printResourcePreview(cf, id, server, multi))
		out := buf.String()
		// Dimensions render sorted, unknown kinds fall back to their raw key.
		require.Contains(t, out, "future_kind:")
		require.Contains(t, out, "Logins:")
		require.Less(t, strings.Index(out, "future_kind:"), strings.Index(out, "Logins:"))
		// The create hint joins every requestable dimension inline-style.
		require.Contains(t, out, "|future_kind=y|logins=root")
	})

	t.Run("no principals", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf}
		require.NoError(t, printResourcePreview(cf, id, server, nil))
		require.Contains(t, buf.String(), "No selectable principals")
	})
}

func TestPrintRequestableResourcesAccess(t *testing.T) {
	rows := []genericResourceRow{
		{
			Name:       "web-1",
			Hostname:   "web-1.dc1",
			Labels:     "env=prod",
			Access:     "1 granted, 2 requestable",
			ResourceID: "/main/node/web-1",
			Principals: map[string]principalSplitJSON{
				"logins": {Granted: []string{"deploy"}, Requestable: []string{"admin", "root"}},
			},
		},
	}
	resourceIDs := []string{"/main/node/web-1"}

	t.Run("text shows Access column, not Resource ID", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf}
		require.NoError(t, printRequestableResources(cf, rows, resourceIDs))
		out := buf.String()
		require.Contains(t, out, "Access")
		// The summary column can truncate at narrow (80-col) widths; the full
		// value is asserted via --format json below.
		require.Contains(t, out, "1 granted")
		require.NotContains(t, out, "Resource ID")
		require.Contains(t, out, "tsh request preview")
	})

	t.Run("json carries principal splits and resource id", func(t *testing.T) {
		var buf bytes.Buffer
		cf := &CLIConf{OverrideStdout: &buf, Format: "json"}
		require.NoError(t, printRequestableResources(cf, rows, resourceIDs))
		out := buf.String()
		require.Contains(t, out, `"ResourceID": "/main/node/web-1"`)
		require.Contains(t, out, `"Principals"`)
		require.Contains(t, out, `"logins"`)
		require.Contains(t, out, `"granted"`)
		require.Contains(t, out, `"requestable"`)
		// Access is the text-only summary and must not leak into JSON.
		require.NotContains(t, out, `"Access"`)
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
