/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

// fakeKubeServerClient provides a minimal implementation of
// ServerWithRoles.ListResources() that applies real filters to a known set of
// resources. Useful for testing against more complex queries.
type fakeKubeServerClient struct {
	apiclient.GetResourcesClient

	availableKubeServers []types.KubeServer
}

func (f fakeKubeServerClient) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	var matched []types.KubeServer

	filter := services.MatchResourceFilter{
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	for _, server := range f.availableKubeServers {
		switch match, err := services.MatchResourceByFilters(server, filter, nil /* ignore dup matches  */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			matched = append(matched, server)
		}
	}

	out := make([]*proto.PaginatedResource, 0, len(matched))
	for _, match := range matched {
		server, ok := match.(*types.KubernetesServerV3)
		if !ok {
			return nil, trace.BadParameter("invalid type %T, expected *types.KubernetesServerV3", server)
		}
		out = append(out, &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: server}})
	}

	return &proto.ListResourcesResponse{Resources: out}, nil
}

// newTestKubeServer creates a minimal KubeServer appropriate for use with
// `fakeKubeServerClient`.
func newTestKubeServer(t *testing.T, name string, labels map[string]string) types.KubeServer {
	t.Helper()

	server, err := types.NewKubernetesServerV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.KubernetesServerSpecV3{
		HostID: name,
		Cluster: &types.KubernetesClusterV3{
			Metadata: types.Metadata{
				Name:   name,
				Labels: labels,
			},
		},
	})
	require.NoError(t, err)

	return server
}

func TestKubernetesV2OutputService_fetch(t *testing.T) {
	servers := []types.KubeServer{
		newTestKubeServer(t, "a", map[string]string{}),
		newTestKubeServer(t, "b", map[string]string{"foo": "1"}),
		newTestKubeServer(t, "c", map[string]string{"foo": "1", "bar": "2"}),
		newTestKubeServer(t, "d", map[string]string{"bar": "2"}),
	}

	client := &fakeKubeServerClient{
		availableKubeServers: servers,
	}

	tests := []struct {
		name                 string
		selectors            []*KubernetesSelector
		expectError          require.ErrorAssertionFunc
		expectedClusterNames []string
		wantNamespaces       map[string]string
	}{
		{
			name: "matches by name",
			selectors: []*KubernetesSelector{
				{
					Name:             "a",
					DefaultNamespace: "a selector",
				},
				{
					Name: "c",
				},
			},
			expectedClusterNames: []string{"a", "c"},
			wantNamespaces: map[string]string{
				"a": "a selector",
			},
		},
		{
			name: "errors when direct lookup fails",
			selectors: []*KubernetesSelector{
				{
					Name: "nonexistent",
				},
			},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "unable to fetch cluster \"nonexistent\" by name")
			},
		},
		{
			name: "matches with simple label selector",
			selectors: []*KubernetesSelector{
				{
					Labels: map[string]string{
						"foo": "1",
					},
					DefaultNamespace: "my-namespace",
				},
			},
			expectedClusterNames: []string{"b", "c"},
			wantNamespaces: map[string]string{
				"b": "my-namespace",
				"c": "my-namespace",
			},
		},
		{
			name: "matches with complex label selector",
			selectors: []*KubernetesSelector{
				{
					Labels: map[string]string{
						"foo": "1",
						"bar": "2",
					},
				},
			},
			expectedClusterNames: []string{"c"},
			wantNamespaces:       map[string]string{},
		},
		{
			name: "matches with multiple selectors",
			selectors: []*KubernetesSelector{
				{
					Labels: map[string]string{
						"foo": "1",
					},
					DefaultNamespace: "bar",
				},
				{
					Labels: map[string]string{
						"bar": "2",
					},
				},
				{
					Name: "a",
				},
			},
			expectedClusterNames: []string{"a", "b", "c", "d"},
			wantNamespaces: map[string]string{
				"b": "bar",
				"c": "bar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := fetchAllMatchingKubeClusters(context.Background(), client, tt.selectors)
			if tt.expectError != nil {
				tt.expectError(t, err)
			} else {
				require.NoError(t, err)

				var names []string
				for _, match := range matches {
					names = append(names, match.cluster.GetName())
				}

				// `generate()` dedupes downstream, so we'll replicate that
				// here, otherwise we might see duplicates if some label
				// selectors overlap.
				names = apiutils.Deduplicate(names)
				assert.ElementsMatch(t, tt.expectedClusterNames, names)
				namespaces := make(map[string]string)
				for _, match := range matches {
					if match.selector.DefaultNamespace != "" {
						namespaces[match.cluster.GetName()] = match.selector.DefaultNamespace
					}
				}
				assert.Equal(t, tt.wantNamespaces, namespaces)
			}
		})
	}
}

// TestKubernetesV2OutputService_render renders the Kubernetes template and
// compares it to the saved golden result.
func TestKubernetesV2OutputService_render(t *testing.T) {
	// We need a fixed cert/key pair here for the golden files testing
	// to behave properly.
	id := &identity.Identity{
		PrivateKeyBytes: keyPEM,
		TLSCertBytes:    tlsCert,
		ClusterName:     mockClusterName,
	}

	tests := []struct {
		name              string
		useRelativePath   bool
		disableExecPlugin bool
	}{
		{
			name: "absolute path",
		},
		{
			name:            "relative path",
			useRelativePath: true,
		},
		{
			name:              "exec plugin disabled",
			disableExecPlugin: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			dest := &destination.Directory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			}
			if tt.useRelativePath {
				wd, err := os.Getwd()
				require.NoError(t, err)
				relativePath, err := filepath.Rel(wd, dir)
				require.NoError(t, err)
				dest.Path = relativePath
			}

			svc := OutputV2Service{
				cfg: &OutputV2Config{
					DisableExecPlugin: tt.disableExecPlugin,
					Destination:       dest,
				},
				executablePath: fakeGetExecutablePath,
				log:            logtest.NewLogger(),
			}

			keyRing, err := internal.NewClientKeyRing(
				id,
				[]types.CertAuthority{fakeCA(t, types.HostCA, mockClusterName)},
			)
			require.NoError(t, err)
			status := &kubernetesStatusV2{
				kubernetesClusterNames: []string{"a", "b", "c"},
				defaultNamespaces: map[string]string{
					"a": "namespace-a",
				},
				teleportClusterName: mockClusterName,
				tlsServerName:       client.GetKubeTLSServerName(mockClusterName),
				credentials:         keyRing,
				clusterAddr:         fmt.Sprintf("https://%s:443", mockClusterName),
			}

			err = svc.render(
				context.Background(),
				status,
				id,
				[]types.CertAuthority{fakeCA(t, types.HostCA, mockClusterName)},
			)
			require.NoError(t, err)

			kubeconfigBytes, err := os.ReadFile(filepath.Join(dir, defaultKubeconfigPath))
			require.NoError(t, err)
			kubeconfigBytes = bytes.ReplaceAll(kubeconfigBytes, []byte(dir), []byte("/test/dir"))

			if golden.ShouldSet() {
				golden.SetNamed(t, "kubeconfig.yaml", kubeconfigBytes)
			}
			require.Equal(
				t, string(golden.GetNamed(t, "kubeconfig.yaml")), string(kubeconfigBytes),
			)
		})
	}
}
