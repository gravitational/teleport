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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

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
			portName:    "",
			expected:    "service2-ns2-cluster2",
		},
		{
			serviceName: "service3",
			namespace:   "ns3",
			clusterName: "cluster.3",
			portName:    "",
			expected:    "service3-ns3-cluster-3",
		},
	}

	for _, tt := range tests {
		result := getAppName(tt.serviceName, tt.namespace, tt.clusterName, tt.portName)

		require.Equal(t, tt.expected, result)
	}
}

func TestGetServiceFQDN(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		externalName string
		expected     string
	}{
		{
			name:      "service1",
			namespace: "ns1",
			expected:  "service1.ns1.svc.cluster.local",
		},
		{
			externalName: "external-service2",
			namespace:    "ns2",
			expected:     "external-service2.ns2.svc.cluster.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			service := v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.name,
					Namespace: tt.namespace,
				},
				Spec: v1.ServiceSpec{
					ExternalName: tt.externalName,
				},
			}
			if tt.externalName != "" {
				service.Spec.Type = v1.ServiceTypeExternalName
			}
			require.Equal(t, tt.expected, getServiceFQDN(service))
		})
	}
}

func TestBuildAppURI(t *testing.T) {
	tests := []struct {
		serviceFQDN string
		port        int32
		protocol    string
		expected    string
	}{
		{
			serviceFQDN: "service.example",
			port:        8080,
			protocol:    protoHTTP,
			expected:    "http://service.example:8080",
		},
		{
			serviceFQDN: "service.example",
			port:        443,
			protocol:    protoHTTPS,
			expected:    "https://service.example:443",
		},
		{
			serviceFQDN: "special.service.example",
			port:        42,
			protocol:    protoTCP,
			expected:    "tcp://special.service.example:42",
		},
	}

	for _, tt := range tests {
		require.Equal(t, buildAppURI(tt.protocol, tt.serviceFQDN, tt.port), tt.expected)
	}
}

func TestAutoProtocolDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		portName    string
		portNumber  int32
		appProtocol string
		expected    string
	}{
		{
			portName:   "http",
			portNumber: 80,
			expected:   protoHTTP,
		},
		{
			portName:   "https",
			portNumber: 443,
			expected:   protoHTTPS,
		},
		{
			portNumber: 4242,
			expected:   protoTCP,
		},
		{
			portNumber: 8080,
			expected:   protoHTTP,
		},
		{
			portNumber: 4242,
			portName:   "http",
			expected:   protoHTTP,
		},
		{
			portNumber: 4242,
			portName:   "https",
			expected:   protoHTTPS,
		},
		{
			portNumber:  4242,
			appProtocol: "http",
			expected:    protoHTTP,
		},
		{
			portNumber:  80,
			appProtocol: "https",
			expected:    protoHTTPS,
		},
		{
			portNumber: 4242,
			portName:   "dns",
			expected:   protoTCP,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("portNumber: %d, portName: %s", tt.portNumber, tt.portName), func(t *testing.T) {
			port := v1.ServicePort{
				Name: tt.portName,
				Port: tt.portNumber,
			}
			if tt.appProtocol != "" {
				port.AppProtocol = &tt.appProtocol
			}

			result := autoProtocolDetection("192.1.1.1", port, nil)

			require.Equal(t, tt.expected, result)
		})
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
		result := getAppLabels(tt.labels, tt.clusterName)

		require.Equal(t, tt.expected, result)
	}
}

func TestGetServicePorts(t *testing.T) {
	tests := []struct {
		desc        string
		annotations map[string]string
		ports       []v1.ServicePort
		expected    []v1.ServicePort
		wantErr     string
	}{
		{
			desc:     "Empty input",
			ports:    []v1.ServicePort{},
			expected: []v1.ServicePort{},
		},
		{
			desc: "One unsupported port (UDP)",
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolUDP},
			},
			expected: []v1.ServicePort{},
		},
		{
			desc: "One supported port",
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
			},
			expected: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
			},
		},
		{
			desc: "One supported port, one unsupported port",
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 81, Protocol: v1.ProtocolUDP},
			},
			expected: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
			},
		},
		{
			desc: "One supported port, one unsupported port",
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 81, Protocol: v1.ProtocolUDP},
			},
			expected: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
			},
		},
		{
			desc: "Two supported ports",
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 81, Protocol: v1.ProtocolTCP},
			},
			expected: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 81, Protocol: v1.ProtocolTCP},
			},
		},
		{
			desc:        "Two supported ports with numeric annotation",
			annotations: map[string]string{types.DiscoveryPortLabel: "42"},
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 42, Protocol: v1.ProtocolTCP},
			},
			expected: []v1.ServicePort{
				{Port: 42, Protocol: v1.ProtocolTCP},
			},
		},
		{
			desc:        "Two supported ports with name annotation",
			annotations: map[string]string{types.DiscoveryPortLabel: "right-port"},
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 42, Protocol: v1.ProtocolTCP},
				{Port: 43, Protocol: v1.ProtocolTCP, Name: "right-port"},
			},
			expected: []v1.ServicePort{
				{Port: 43, Protocol: v1.ProtocolTCP, Name: "right-port"},
			},
		},
		{
			desc:        "Annotation doesn't match available port, want error",
			annotations: map[string]string{types.DiscoveryPortLabel: "43"},
			ports: []v1.ServicePort{
				{Port: 80, Protocol: v1.ProtocolTCP},
				{Port: 42, Protocol: v1.ProtocolTCP},
			},
			expected: []v1.ServicePort{},
			wantErr:  "Specified preferred port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service := v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: v1.ServiceSpec{
					Ports: tt.ports,
				},
			}

			result, err := getServicePorts(service)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestProtoChecker_CheckProtocol(t *testing.T) {
	t.Parallel()

	checker := protoChecker{
		InsecureSkipVerify: true,
	}

	httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "httpsServer")
	}))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "httpServer")
	}))
	tcpServer := newTCPServer(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("tcpServer"))
		_ = conn.Close()
	})
	t.Cleanup(func() {
		httpsServer.Close()
		httpServer.Close()
		_ = tcpServer.Close()
	})

	tests := []struct {
		uri      string
		expected string
	}{
		{
			uri:      strings.TrimPrefix(httpServer.URL, "http://"),
			expected: "http",
		},
		{
			uri:      strings.TrimPrefix(httpsServer.URL, "https://"),
			expected: "https",
		},
		{
			uri:      tcpServer.Addr().String(),
			expected: "tcp",
		},
	}

	for _, tt := range tests {
		res := checker.CheckProtocol(tt.uri)
		require.Equal(t, tt.expected, res)
	}
}

func newTCPServer(t *testing.T, handleConn func(net.Conn)) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err == nil {
				go handleConn(conn)
			}
			if err != nil && !utils.IsOKNetworkError(err) {
				t.Error(err)
				return
			}
		}
	}()

	return listener
}
