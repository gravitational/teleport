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
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tlscatest"
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
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
		},
		{
			name: "public addr does not conflict",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "app.example.com"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
		},
		{
			name: "public addr matches proxy host",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
			wantErr:    "conflicts with the Teleport Proxy public address",
		},
		{
			name: "public addr with trailing dot is rejected, with colliding proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com."})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr with trailing dot is rejected, no proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com."})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr with multiple trailing dots is rejected, with colliding proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com..."})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr with multiple trailing dots is rejected, no proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com..."})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr with mixed casing is rejected, with colliding proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "WeB.ExAmPle.CoM"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"web.example.com:443"},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr with mixed casing is rejected, no proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "WeB.ExAmPle.CoM"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "multiple proxy addrs, one matches",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "web.example.com"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"other.com:443", "web.example.com:443"},
			wantErr:    "conflicts with the Teleport Proxy public address",
		},
		{
			name: "public addr IDN Unicode is rejected, with colliding proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "例.cn"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"xn--fsq.cn:443"},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "public addr IDN Unicode is rejected, no proxy",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "例.cn"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{},
			wantErr:    "must be a valid DNS name",
		},
		{
			name: "punycode IDN matches proxy host",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "xn--fsq.cn"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"xn--fsq.cn:443"},
			wantErr:    "conflicts with the Teleport Proxy public address",
		},
		{
			name: "empty proxy addrs",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "example.com"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{},
		},
		{
			name: "multiple conflicting proxies",
			app: func() types.Application {
				app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: "example.com"})
				require.NoError(t, err)
				return app
			}(),
			proxyAddrs: []string{"example.com:443", "example.com:80"},
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

func TestValidateAppServer(t *testing.T) {
	proxyGetter := &mockProxyGetter{addrs: []string{"proxy.example.com:443"}}

	makeServer := func(t *testing.T, outerName, innerName string) types.AppServer {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: innerName}, types.AppSpecV3{URI: "http://localhost:8080"})
		require.NoError(t, err)
		srv, err := types.NewAppServerV3FromApp(app, "localhost", "host-id")
		require.NoError(t, err)
		srv.Metadata.Name = outerName
		return srv
	}

	t.Run("nil server rejected", func(t *testing.T) {
		require.ErrorContains(t, ValidateAppServer(nil, proxyGetter), "nil app server")
	})
	t.Run("valid outer and inner", func(t *testing.T) {
		require.NoError(t, ValidateAppServer(makeServer(t, "myapp", "myapp"), proxyGetter))
	})
	t.Run("mixed-case outer rejected", func(t *testing.T) {
		err := ValidateAppServer(makeServer(t, "MyApp", "myapp"), proxyGetter)
		require.ErrorContains(t, err, "app server name")
		require.ErrorContains(t, err, "MyApp")
	})
	t.Run("underscore in outer accepted", func(t *testing.T) {
		require.NoError(t, ValidateAppServer(makeServer(t, "ok_name", "good-name"), proxyGetter))
	})
	t.Run("mixed-case inner rejected", func(t *testing.T) {
		err := ValidateAppServer(makeServer(t, "myapp", "MyApp"), proxyGetter)
		require.ErrorContains(t, err, "application name")
	})
}

func TestValidateAppName(t *testing.T) {
	proxyGetter := &mockProxyGetter{addrs: []string{"proxy.example.com:443"}}

	t.Run("nil app rejected", func(t *testing.T) {
		err := ValidateApp(nil, proxyGetter)
		require.ErrorContains(t, err, "nil application")
	})

	t.Run("required_apps mixed case rejected", func(t *testing.T) {
		app, err := types.NewAppV3(types.Metadata{Name: "main"}, types.AppSpecV3{
			URI:              "http://localhost:8080",
			RequiredAppNames: []string{"MixedCase"},
		})
		require.NoError(t, err)
		err = ValidateApp(app, proxyGetter)
		require.ErrorContains(t, err, `references required_apps entry "MixedCase"`)
	})

	t.Run("required_apps valid entries accepted", func(t *testing.T) {
		app, err := types.NewAppV3(types.Metadata{Name: "main"}, types.AppSpecV3{
			URI:              "http://localhost:8080",
			RequiredAppNames: []string{"other-app", "another.app"},
		})
		require.NoError(t, err)
		require.NoError(t, ValidateApp(app, proxyGetter))
	})

	makeApp := func(t *testing.T, name string) types.Application {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "http://localhost:8080"})
		require.NoError(t, err)
		return app
	}

	tests := []struct {
		name    string
		appName string
		wantErr string
	}{
		{name: "valid lowercase", appName: "myapp"},
		{name: "valid with hyphen", appName: "my-app"},
		{name: "valid leading digit", appName: "1stapp"},
		{name: "valid all digits", appName: "123"},
		{name: "valid dotted name", appName: "env.prod"},
		{name: "reject uppercase", appName: "MyApp", wantErr: "must be a valid DNS name"},
		{name: "accept underscore", appName: "my_app"},
		{name: "reject trailing hyphen", appName: "foo-", wantErr: "must be a valid DNS name"},
		{name: "accept 63-char label", appName: strings.Repeat("a", 63)},
		{name: "reject too long", appName: strings.Repeat("a", 254), wantErr: "must be a valid DNS name"},
		{name: "accept single char", appName: "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := makeApp(t, tt.appName)
			err := ValidateApp(app, proxyGetter)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			// ValidateApp must not rewrite GetName; UpdateApp would
			// retarget the wrong record.
			require.Equal(t, tt.appName, app.GetName())
		})
	}
}

func TestValidateAppPublicAddr(t *testing.T) {
	proxyGetter := &mockProxyGetter{addrs: []string{"proxy.example.com:443"}}

	makeApp := func(t *testing.T, publicAddr string) types.Application {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "http://localhost:8080", PublicAddr: publicAddr})
		require.NoError(t, err)
		return app
	}

	tests := []struct {
		name    string
		addr    string
		wantErr string
	}{
		{name: "bare hostname", addr: "app.example.com"},
		{name: "reject mixed case", addr: "MyApp.example.com", wantErr: "must be a valid DNS name"},
		{name: "reject all-upper", addr: "APP.EXAMPLE.COM", wantErr: "must be a valid DNS name"},
		{name: "reject scheme http", addr: "http://foo.bar", wantErr: "must be a valid DNS name"},
		{name: "reject scheme https with port", addr: "https://foo.bar:443", wantErr: "must be a valid DNS name"},
		{name: "reject port", addr: "foo.bar:443", wantErr: "must be a valid DNS name"},
		{name: "reject IPv4", addr: "192.168.1.1", wantErr: "must not be an IP address"},
		{name: "reject bare IPv6", addr: "::1", wantErr: "must not be an IP address"},
		{name: "reject bracketed IPv6", addr: "[::1]", wantErr: "must be a valid DNS name"},
		{name: "reject bracketed IPv6 with port", addr: "[::1]:443", wantErr: "must be a valid DNS name"},
		{name: "reject mailto opaque", addr: "mailto:victim@example.com", wantErr: "must be a valid DNS name"},
		{name: "reject path", addr: "app.example.com/path", wantErr: "must be a valid DNS name"},
		{name: "reject query", addr: "app.example.com?x=y", wantErr: "must be a valid DNS name"},
		{name: "reject fragment", addr: "app.example.com#frag", wantErr: "must be a valid DNS name"},
		{name: "reject userinfo", addr: "user@app.example.com", wantErr: "must be a valid DNS name"},
		{name: "reject single trailing dot", addr: "app.example.com.", wantErr: "must be a valid DNS name"},
		{name: "reject double trailing dot", addr: "app.example.com..", wantErr: "must be a valid DNS name"},
		{name: "reject only dots", addr: "...", wantErr: "must be a valid DNS name"},
		{name: "reject IDN unicode", addr: "münchen.de", wantErr: "must be a valid DNS name"},
		{name: "accept punycode", addr: "xn--mnchen-3ya.de"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := makeApp(t, tt.addr)
			err := ValidateApp(app, proxyGetter)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNormalizeAppServerForHeartbeat(t *testing.T) {
	makeServer := func(t *testing.T, outerName, innerName string) *types.AppServerV3 {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: innerName}, types.AppSpecV3{
			URI: "http://localhost:8080",
		})
		require.NoError(t, err)
		srv, err := types.NewAppServerV3FromApp(app, "localhost", "host-id")
		require.NoError(t, err)
		srv.Metadata.Name = outerName
		return srv
	}

	tests := []struct {
		name          string
		outerName     string
		innerName     string
		wantOuterName string
		wantInnerName string
	}{
		{
			name:          "both lowercase unchanged",
			outerName:     "myapp",
			innerName:     "myapp",
			wantOuterName: "myapp",
			wantInnerName: "myapp",
		},
		{
			name:          "both mixed-case lowercased together",
			outerName:     "MyApp",
			innerName:     "MyApp",
			wantOuterName: "myapp",
			wantInnerName: "myapp",
		},
		{
			name:          "outer mixed-case inner lowercase",
			outerName:     "MyApp",
			innerName:     "myapp",
			wantOuterName: "myapp",
			wantInnerName: "myapp",
		},
		{
			name:          "true mismatch left unchanged",
			outerName:     "different",
			innerName:     "myapp",
			wantOuterName: "different",
			wantInnerName: "myapp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := makeServer(t, tt.outerName, tt.innerName)
			NormalizeAppServerForHeartbeat(srv)
			require.Equal(t, tt.wantOuterName, srv.GetName())
			require.Equal(t, tt.wantInnerName, srv.GetApp().GetName())
		})
	}

	t.Run("required_apps lowercased", func(t *testing.T) {
		app, err := types.NewAppV3(types.Metadata{Name: "main"}, types.AppSpecV3{
			URI:              "http://localhost:8080",
			RequiredAppNames: []string{"MixedCase", "Other.App"},
		})
		require.NoError(t, err)
		srv, err := types.NewAppServerV3FromApp(app, "localhost", "host-id")
		require.NoError(t, err)
		NormalizeAppServerForHeartbeat(srv)
		require.Equal(t, []string{"mixedcase", "other.app"}, srv.GetApp().GetRequiredAppNames())
	})
}

func TestNormalizeHeartbeatPublicAddr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty unchanged", input: "", want: ""},
		{name: "bare hostname unchanged", input: "app.example.com", want: "app.example.com"},
		{name: "scheme and path stripped", input: "https://app.example.com/start", want: "app.example.com"},
		{name: "scheme path and port stripped", input: "https://app.example.com:443/path", want: "app.example.com"},
		{name: "trailing port stripped", input: "app.example.com:443", want: "app.example.com"},
		{name: "mixed case lowercased", input: "MyApp.Example.Com", want: "myapp.example.com"},
		{name: "mixed case with scheme lowercased", input: "https://MyApp.Example.Com/start", want: "myapp.example.com"},
		{name: "mixed case with port lowercased", input: "MyApp.Example.Com:443", want: "myapp.example.com"},
		{name: "bare IPv6 returned as-is", input: "::1", want: "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeHeartbeatPublicAddr(tt.input))
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

func (m *mockProxyGetter) ListProxyServers(_ context.Context, _ int, _ string) ([]types.Server, string, error) {
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

	return servers, "", nil
}

// TestApplicationUnmarshal verifies an app resource can be unmarshaled.
func TestApplicationUnmarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
		TLS: &types.AppTLS{
			Mode:           types.AppTLSModeVerifyFull,
			ServerName:     "localhost",
			ServerSpiffeId: "spiffe://mycluster/svc/localhost",
			ClientCertMode: types.AppClientCertModeManaged,
		},
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
		URI: "https://localhost:8080",
		TLS: &types.AppTLS{
			Mode:           types.AppTLSModeVerifyFull,
			ServerName:     "localhost",
			ServerSpiffeId: "spiffe://mycluster/svc/localhost",
			ClientCertMode: types.AppClientCertModeManaged,
		},
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
  uri: "http://localhost:8080"
  tls:
    mode: verify-full
    server_name: localhost
    server_spiffe_id: spiffe://mycluster/svc/localhost
    client_cert_mode: managed`

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
		{
			serviceName: "service4",
			namespace:   "ns4",
			clusterName: "cluster4",
			annotation:  "1stapp",
			expected:    "1stapp",
		},
		{
			serviceName: "service5",
			namespace:   "ns5",
			clusterName: "MyGroup",
			expected:    "service5-ns5-mygroup",
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
	rewriteTraits := map[string][]string{
		"rewrite": {"value1", "value2"},
	}
	RewriteHeadersAndApplyValueTraits(r, slices.Values(rewrites), rewriteTraits, slog.Default())

	assert.Equal(t, "1.2.3.4", r.Host)
	wantHeaders := make(http.Header)
	wantHeaders.Add("x-rewrite", "value1")
	wantHeaders.Add("x-rewrite", "value2")
	wantHeaders.Add("x-no-rewrite", "no-rewrite")
	assert.Equal(t, wantHeaders, r.Header)
}

func TestValidateAppTLS(t *testing.T) {
	for name, tc := range map[string]struct {
		uri          string
		tls          *types.AppTLS
		insecureSkip bool
		expectErr    require.ErrorAssertionFunc
	}{
		"full valid configuration": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifyFull,
				ServerName:     "example.com",
				ServerSpiffeId: "spiffe://mycluster/svc/example",
				AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
				ClientCertMode: types.AppClientCertModeManaged,
			},
			expectErr: require.NoError,
		},
		"mcp https valid configuration": {
			uri: "mcp+https://localhost",
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifyFull,
				ServerSpiffeId: "spiffe://mycluster/svc/mcp",
				ClientCertMode: types.AppClientCertModeManaged,
			},
			expectErr: require.NoError,
		},
		"minimal": {
			tls: &types.AppTLS{
				Mode: types.AppTLSModeVerifyServerName,
			},
			expectErr: require.NoError,
		},
		"insecure with client cert mode": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeInsecure,
				ClientCertMode: types.AppClientCertModeManaged,
			},
			expectErr: require.Error,
		},
		"conflicting insecure skip verify and tls mode": {
			tls: &types.AppTLS{
				Mode: types.AppTLSModeVerifyServerName,
			},
			insecureSkip: true,
			expectErr:    require.Error,
		},
		"incomplete server spiffe id": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifySpiffeID,
				ServerSpiffeId: "/svc/example",
			},
			expectErr: require.Error,
		},
		"verify spiffe id mode and with server name": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifySpiffeID,
				ServerName:     "example.com",
				ServerSpiffeId: "spiffe://mycluster/svc/example",
			},
			expectErr: require.Error,
		},
		"invalid mode": {
			tls: &types.AppTLS{
				Mode: "invalid",
			},
			expectErr: require.Error,
		},
		"invalid allowed CA": {
			tls: &types.AppTLS{
				Mode:       types.AppTLSModeVerifyFull,
				AllowedCas: []string{"workload_identity", "malformed cert"},
			},
			expectErr: require.Error,
		},
		"server name with insecure mode": {
			tls: &types.AppTLS{
				Mode:       types.AppTLSModeInsecure,
				ServerName: "example.com",
			},
			expectErr: require.Error,
		},
		"server spiffe id with insecure mode": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeInsecure,
				ServerSpiffeId: "spiffe://mycluster/svc/example",
			},
			expectErr: require.Error,
		},
		"verify spiffe missing server spiffe": {
			tls: &types.AppTLS{
				Mode: types.AppTLSModeVerifySpiffeID,
			},
			expectErr: require.Error,
		},
		"verify spiffe missing server spiffe trust domain": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifySpiffeID,
				ServerSpiffeId: "spiffe:///svc/example",
			},
			expectErr: require.Error,
		},
		"invalid client cert mode value": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeVerifyServerName,
				ClientCertMode: "foo",
			},
			expectErr: require.Error,
		},
		"client cert mode with insecure": {
			tls: &types.AppTLS{
				Mode:           types.AppTLSModeInsecure,
				ClientCertMode: types.AppClientCertModeManaged,
			},
			expectErr: require.Error,
		},
		"insecure skip verify compatible with mode insecure": {
			tls:          &types.AppTLS{Mode: types.AppTLSModeInsecure},
			insecureSkip: true,
			expectErr:    require.NoError,
		},
		"insecure skip verify with empty mode defaults to insecure": {
			tls:          &types.AppTLS{},
			insecureSkip: true,
			expectErr:    require.NoError,
		},
		"allowed cas with only internal alias": {
			tls: &types.AppTLS{
				Mode:       types.AppTLSModeVerifyServerName,
				AllowedCas: []string{types.AppTLSInternalCAWorkloadIdentity},
			},
			expectErr: require.NoError,
		},
		"allowed cas with empty string entry": {
			tls: &types.AppTLS{
				Mode:       types.AppTLSModeVerifyServerName,
				AllowedCas: []string{""},
			},
			expectErr: require.Error,
		},
		"insecure mode with allowed cas": {
			tls: &types.AppTLS{
				Mode:       types.AppTLSModeInsecure,
				AllowedCas: []string{types.AppTLSInternalCAWorkloadIdentity},
			},
			expectErr: require.Error,
		},
		"cloud app with tls block": {
			uri: "cloud://aws",
			tls: &types.AppTLS{
				Mode: types.AppTLSModeInsecure,
			},
			expectErr: require.Error,
		},
		"unsupported protocol with tls block": {
			uri: "http://localhost",
			tls: &types.AppTLS{
				Mode: types.AppTLSModeInsecure,
			},
			expectErr: require.Error,
		},
		"unsupported protocol with empty tls config": {
			uri:       "http://localhost",
			tls:       &types.AppTLS{},
			expectErr: require.Error,
		},
		"empty": {
			tls:       &types.AppTLS{},
			expectErr: require.NoError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			uri := tc.uri
			if uri == "" {
				uri = "https://localhost"
			}

			app, err := types.NewAppV3(types.Metadata{Name: "myapp"}, types.AppSpecV3{
				URI:                uri,
				InsecureSkipVerify: tc.insecureSkip,
				TLS:                tc.tls,
			})
			require.NoError(t, err)
			tc.expectErr(t, validateAppTLS(app))
		})
	}
}

func TestValidateAppAllowedCAsContents(t *testing.T) {
	caKeyPEM, caCertPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{ClusterName: "example.com"})
	require.NoError(t, err)

	identity := &tlsca.Identity{Username: "test-user"}
	subj, err := identity.Subject()
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	ca, err := tlsca.FromKeys(caCertPEM, caKeyPEM)
	require.NoError(t, err)
	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  time.Now().Add(time.Hour),
		DNSNames:  []string{"localhost", "*.localhost"},
	})
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		contents  []byte
		expectErr require.ErrorAssertionFunc
	}{
		"single valid ca": {
			contents:  caCertPEM,
			expectErr: require.NoError,
		},
		"only single ca allowed": {
			contents:  bytes.Join([][]byte{caCertPEM, caCertPEM}, []byte("\n")),
			expectErr: require.Error,
		},
		"no CA certificate": {
			contents:  bytes.Join([][]byte{caCertPEM, tlsCert}, []byte("\n")),
			expectErr: require.Error,
		},
		"no certificate contents": {
			contents:  []byte("hello"),
			expectErr: require.Error,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.expectErr(t, isValidCACertificatePEM(string(tc.contents)))
		})
	}
}
