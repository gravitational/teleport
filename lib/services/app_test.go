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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestValidateApp(t *testing.T) {
	tests := []struct {
		name       string
		app        types.Application
		proxyAddrs []string
		wantErr    string
	}{
		{
			name: "no public addr, no error",
			app: func() types.Application {
				app, _ := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080"})
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
		},
		{
			name: "public addr does not conflict",
			app: func() types.Application {
				app, _ := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "app.example.com"})
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
		},
		{
			name: "public addr matches proxy host",
			app: func() types.Application {
				app, _ := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com"})
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
			wantErr:    "conflicts with the Teleport Proxy public address",
		},
		{
			name: "multiple proxy addrs, one matches",
			app: func() types.Application {
				app, _ := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com"})
				return app
			}(),
			proxyAddrs: []string{"other.com:443", "web.example.com:443"},
			wantErr:    "conflicts with the Teleport Proxy public address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateApp(tt.app, &mockProxyGetter{addrs: tt.proxyAddrs})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// mockProxyGetter is a test implementation of ProxyGetter.
type mockProxyGetter struct {
	addrs []string
}

func (m *mockProxyGetter) GetProxies() ([]types.Server, error) {
	servers := make([]types.Server, 0, len(m.addrs))

	for _, addr := range m.addrs {
		servers = append(
			servers,
			&types.ServerV2{
				Spec: types.ServerSpecV2{
					PublicAddrs: []string{addr},
				},
			},
		)
	}

	return servers, nil
}

// TestApplicationUnmarshal verifies an app resource can be unmarshaled.
func TestApplicationUnmarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(appYAML))
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestApplicationMarshal verifies a marshaled app resource can be unmarshaled back.
func TestApplicationMarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := MarshalApp(expected)
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var appYAML = `kind: app
version: v3
metadata:
  name: test-app
  description: "Test description"
  labels:
    env: dev
spec:
  uri: "http://localhost:8080"`

func TestGetAppName(t *testing.T) {
	tests := []struct {
		serviceName string
		namespace   string
		clusterName string
		portName    string
		annotation  string
		wantErr     string
		expected    string
	}{
		{
			serviceName: "service1",
			namespace:   "ns1",
			clusterName: "cluster1",
			portName:    "port1",
			expected:    "service1-port1-ns1-cluster1",
		},
		{
			serviceName: "service2",
			namespace:   "ns2",
			clusterName: "cluster2",
			expected:    "service2-ns2-cluster2",
		},
		{
			serviceName: "service3",
			namespace:   "ns3",
			clusterName: "cluster.3",
			expected:    "service3-ns3-cluster-3",
		},
		{
			serviceName: "service3",
			namespace:   "ns3",
			clusterName: "cluster.3",
			annotation:  "overridden-name",
			expected:    "overridden-name",
		},
		{
			serviceName: "service3",
			namespace:   "ns3",
			clusterName: "cluster.3",
			portName:    "http",
			annotation:  "overridden-name",
			expected:    "overridden-name-http",
		},
		{
			serviceName: "service3",
			namespace:   "ns3",
			clusterName: "cluster.3",
			portName:    "http",
			annotation:  "overridden*name",
			wantErr:     "s",
		},
	}

	for _, tt := range tests {
		result, err := getAppName(tt.serviceName, tt.namespace, tt.clusterName, tt.portName, tt.annotation)
		if tt.wantErr != "" {
			require.ErrorContains(t, err, tt.wantErr)
		} else {
			require.Equal(t, tt.expected, result)
		}
	}
}

func TestGetKubeClusterDomain(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "k8s")
	tests := []struct {
		name     string
		envVar   string
		expected string
	}{
		{
			name:     "service1 fallback to cluster.local",
			expected: "cluster.local",
		},
		{
			name:     "service1 dns resolution",
			envVar:   "k8s.com",
			expected: "k8s.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("TELEPORT_KUBE_CLUSTER_DOMAIN", tt.envVar)
			}
			require.Equal(t, tt.expected, getClusterDomain())
		})
	}
}

func TestGetServiceFQDN(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		namespace    string
		externalName string
		expected     string
	}{
		{
			name:        "service1 fallback to cluster.local",
			serviceName: "service1",
			namespace:   "ns1",
			expected:    "service1.ns1.svc.cluster.local",
		},
		{
			name:         "service2",
			serviceName:  "service2",
			externalName: "external-service2",
			namespace:    "ns2",
			expected:     "external-service2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.serviceName,
					Namespace: tt.namespace,
				},
				Spec: v1.ServiceSpec{
					ExternalName: tt.externalName,
				},
			}
			if tt.externalName != "" {
				service.Spec.Type = v1.ServiceTypeExternalName
			}
			require.Equal(t, tt.expected, GetServiceFQDN(service))
		})
	}
}

func TestBuildAppURI(t *testing.T) {
	tests := []struct {
		serviceFQDN string
		port        int32
		path        string
		protocol    string
		expected    string
	}{
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    "http",
			expected:    "http://service.example:8080",
		},
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    "http",
			path:        "foo",
			expected:    "http://service.example:8080/foo",
		},
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    "http",
			path:        "/foo",
			expected:    "http://service.example:8080/foo",
		},
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    "http",
			path:        "foo/bar",
			expected:    "http://service.example:8080/foo/bar",
		},
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    "http",
			path:        "foo bar",
			expected:    "http://service.example:8080/foo%20bar",
		},
		{
			serviceFQDN: "service.example",
			port:        443,
			protocol:    "https",
			expected:    "https://service.example:443",
		},
		{
			serviceFQDN: "special.service.example",
			port:        42,
			protocol:    "tcp",
			expected:    "tcp://special.service.example:42",
		},
	}

	for _, tt := range tests {
		require.Equal(t, tt.expected, buildAppURI(tt.protocol, tt.serviceFQDN, tt.path, tt.port))
	}
}

func TestGetAppRewriteConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected types.Rewrite
		wantErr  string
	}{
		{
			input: `redirect:
  - "redirect1"
  - "redirect2"`,
			expected: types.Rewrite{
				Redirect: []string{"redirect1", "redirect2"},
			},
		},
		{
			input:    `wrong:"`,
			expected: types.Rewrite{},
			wantErr:  "failed decoding rewrite config",
		},
		{
			input: `redirect:
  - "localhost"
  - "jenkins.internal.dev"
headers:
  - name: "X-Custom-Header"
    value: "example"
  - name: "Authorization"
    value: "Bearer {{internal.jwt}}"`,
			expected: types.Rewrite{
				Redirect: []string{"localhost", "jenkins.internal.dev"},
				Headers: []*types.Header{
					{
						Name:  "X-Custom-Header",
						Value: "example",
					},
					{
						Name:  "Authorization",
						Value: "Bearer {{internal.jwt}}",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		result, err := getAppRewriteConfig(map[string]string{types.DiscoveryAppRewriteLabel: tt.input})
		if tt.wantErr != "" {
			require.ErrorContains(t, err, tt.wantErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expected, *result)
		}
	}
}

func TestGetAppLabels(t *testing.T) {
	tests := []struct {
		labels      map[string]string
		clusterName string
		expected    map[string]string
	}{
		{
			labels:      map[string]string{"label1": "value1"},
			clusterName: "cluster1",
			expected:    map[string]string{"label1": "value1", types.KubernetesClusterLabel: "cluster1"},
		},
		{
			labels:      map[string]string{"label1": "value1", "label2": "value2"},
			clusterName: "cluster2",
			expected:    map[string]string{"label1": "value1", "label2": "value2", types.KubernetesClusterLabel: "cluster2"},
		},
	}

	for _, tt := range tests {
		result, err := getAppLabels(tt.labels, tt.clusterName)
		require.NoError(t, err)

		require.Equal(t, tt.expected, result)
	}
}

func TestInsecureSkipVerify(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		expected    bool
	}{
		{
			annotations: map[string]string{types.DiscoveryAppInsecureSkipVerify: "true"},
			expected:    true,
		},
		{
			annotations: map[string]string{types.DiscoveryAppInsecureSkipVerify: "false"},
			expected:    false,
		},
		{
			annotations: map[string]string{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		result := getTLSInsecureSkipVerify(tt.annotations)
		require.Equal(t, tt.expected, result)
	}
}

func TestRewriteHeadersAndApplyValueTraits(t *testing.T) {
	r := httptest.NewRequest("GET", "/foo", nil)
	r.Header.Set("x-no-rewrite", "no-rewrite")
	rewrites := []*types.Header{
		{Name: "host", Value: "1.2.3.4"},
		{Name: "x-rewrite", Value: "{{external.rewrite}}"},
		// Missing traits should log a debug message that this rewrite is skipped.
		{Name: "x-bad-rewrite", Value: "{{external.bad_rewrite}}"},
	}
	traits := map[string][]string{
		"rewrite": {"value1", "value2"},
	}
	RewriteHeadersAndApplyValueTraits(r, slices.Values(rewrites), traits, slog.Default())

	assert.Equal(t, "1.2.3.4", r.Host)
	wantHeaders := make(http.Header)
	wantHeaders.Add("x-rewrite", "value1")
	wantHeaders.Add("x-rewrite", "value2")
	wantHeaders.Add("x-no-rewrite", "no-rewrite")
	assert.Equal(t, wantHeaders, r.Header)
}
