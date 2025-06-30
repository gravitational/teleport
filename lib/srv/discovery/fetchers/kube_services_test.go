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

package fetchers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestKubeAppFetcher_Get(t *testing.T) {
	t.Parallel()

	appProtocolHTTP := "http"
	mockServices := []*corev1.Service{
		newMockService("service1", "ns1", "", map[string]string{"test-label": "testval"}, nil,
			[]corev1.ServicePort{{Port: 42, Name: "http", Protocol: corev1.ProtocolTCP}}),
		newMockService("service2", "ns2", "", map[string]string{"test-label": "testval",
			"test-label2": "testval2"}, nil, []corev1.ServicePort{{Port: 42, Name: "custom", AppProtocol: &appProtocolHTTP, Protocol: corev1.ProtocolTCP}}),
		newMockService("service3", "ns2", "", map[string]string{"test-label2": "testval2"}, nil,
			[]corev1.ServicePort{
				{Port: 42, Name: "custom", Protocol: corev1.ProtocolTCP},
				{Port: 80, Name: "http", Protocol: corev1.ProtocolTCP},
				{Port: 443, Name: "https", Protocol: corev1.ProtocolTCP},
				{Port: 43, Name: "custom-udp", Protocol: corev1.ProtocolUDP},
			}),
		newMockService("service4", "ns3", "", map[string]string{"test-label2": "testval2"}, map[string]string{
			types.DiscoveryProtocolLabel: "https",
			types.DiscoveryPortLabel:     "42",
		},
			[]corev1.ServicePort{
				{Port: 42, Name: "custom", Protocol: corev1.ProtocolTCP},
				{Port: 80, Name: "http", Protocol: corev1.ProtocolTCP},
				{Port: 443, Name: "https", Protocol: corev1.ProtocolTCP},
				{Port: 43, Name: "custom-udp", Protocol: corev1.ProtocolUDP},
			}),
		newMockService("service5", "ns3", "", map[string]string{"test-label3": "testval3"}, map[string]string{
			types.DiscoveryProtocolLabel: "tcp",
		},
			[]corev1.ServicePort{
				{Port: 42, Name: "custom", Protocol: corev1.ProtocolTCP},
				{Port: 43, Name: "custom-udp", Protocol: corev1.ProtocolUDP},
			}),
	}

	apps := map[string]*types.AppV3{
		"service1.app1": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service1-ns1-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label": "testval", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "http://service1.ns1.svc.cluster.local:42",
			},
		},
		"service2.app1": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service2-ns2-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label": "testval", "test-label2": "testval2", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "http://service2.ns2.svc.cluster.local:42",
			},
		},
		"service3.app1": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service3-http-ns2-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label2": "testval2", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "http://service3.ns2.svc.cluster.local:80",
			},
		},
		"service3.app2": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service3-https-ns2-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label2": "testval2", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "https://service3.ns2.svc.cluster.local:443",
			},
		},
		"service3.app4": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service3-custom-ns2-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label2": "testval2", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "http://service3.ns2.svc.cluster.local:42",
			},
		},
		"service4.app1": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service4-ns3-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label2": "testval2", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "https://service4.ns3.svc.cluster.local:42",
			},
		},
		"service5.app1": {
			Kind:    "app",
			Version: "v3",
			Metadata: types.Metadata{
				Name:        "service5-ns3-test-cluster",
				Namespace:   "default",
				Description: "Discovered application in Kubernetes cluster \"test-cluster\"",
				Labels:      map[string]string{"test-label3": "testval3", types.KubernetesClusterLabel: "test-cluster"},
			},
			Spec: types.AppSpecV3{
				URI: "tcp://service5.ns3.svc.cluster.local:42",
			},
		},
	}

	tests := []struct {
		desc              string
		services          []*corev1.Service
		matcherNamespaces []string
		matcherLabels     types.Labels
		expected          types.Apps
		protoChecker      ProtocolChecker
	}{
		{
			desc:              "No services",
			matcherNamespaces: []string{"ns1"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{},
		},
		{
			desc:              "One service - one http app",
			services:          []*corev1.Service{mockServices[0]},
			matcherNamespaces: []string{"ns1"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{apps["service1.app1"]},
		},
		{
			desc:              "No matching services in the namespace",
			services:          mockServices,
			matcherNamespaces: []string{"ns5"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{},
		},
		{
			desc:              "No matching services by the labels",
			services:          mockServices,
			matcherNamespaces: []string{"ns1"},
			matcherLabels:     types.Labels{"test-label": []string{"wrongval"}},
			expected:          types.Apps{},
		},
		{
			desc:              "Matching all namespaces",
			services:          mockServices,
			matcherNamespaces: []string{"*"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{apps["service1.app1"], apps["service2.app1"]},
		},
		{
			desc:              "Matching all labels in a namespace",
			services:          mockServices,
			matcherNamespaces: []string{"ns1"},
			matcherLabels:     types.Labels{"*": []string{"*"}},
			expected:          types.Apps{apps["service1.app1"]},
		},
		{
			desc:              "Matching 2 services by a label in 2 namespaces",
			services:          mockServices,
			matcherNamespaces: []string{"ns1", "ns2"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{apps["service1.app1"], apps["service2.app1"]},
		},
		{
			desc:              "Matching 2 services by a label in all namespaces",
			services:          mockServices,
			matcherNamespaces: []string{"*"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}},
			expected:          types.Apps{apps["service1.app1"], apps["service2.app1"]},
		},
		{
			desc:              "Matching services by 2 labels in all namespaces",
			services:          mockServices,
			matcherNamespaces: []string{"*"},
			matcherLabels:     types.Labels{"test-label": []string{"testval"}, "test-label2": []string{"testval2"}},
			expected:          types.Apps{apps["service2.app1"]},
		},
		{
			desc:              "Matching 2 services into 3 apps by a label in a namespace",
			services:          mockServices,
			matcherNamespaces: []string{"ns2"},
			matcherLabels:     types.Labels{"test-label2": []string{"testval2"}},
			expected:          types.Apps{apps["service2.app1"], apps["service3.app1"], apps["service3.app2"]},
		},
		{
			desc:              "Service with annotations",
			services:          mockServices,
			matcherNamespaces: []string{"ns3"},
			matcherLabels:     types.Labels{"test-label2": []string{"testval2"}},
			expected:          types.Apps{apps["service4.app1"]},
		},
		{
			desc:              "Service with protocol annotation 'tcp'",
			services:          mockServices,
			matcherNamespaces: []string{"ns3"},
			matcherLabels:     types.Labels{"test-label3": []string{"testval3"}},
			expected:          types.Apps{apps["service5.app1"]},
		},
		{
			desc:              "Matching service with protocol checker",
			services:          mockServices,
			matcherNamespaces: []string{"ns2"},
			matcherLabels:     types.Labels{"test-label2": []string{"testval2"}},
			protoChecker:      &mockProtocolChecker{results: map[string]string{"service3.ns2.svc.cluster.local:42": "http"}},
			expected:          types.Apps{apps["service2.app1"], apps["service3.app4"], apps["service3.app1"], apps["service3.app2"]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var objects []runtime.Object
			for _, s := range tt.services {
				objects = append(objects, s)
			}
			fakeClient := fake.NewSimpleClientset(objects...)

			fetcher, err := NewKubeAppsFetcher(KubeAppsFetcherConfig{
				ClusterName:      "test-cluster",
				KubernetesClient: fakeClient,
				FilterLabels:     tt.matcherLabels,
				Namespaces:       tt.matcherNamespaces,
				ProtocolChecker:  tt.protoChecker,
				Logger:           utils.NewSlogLoggerForTests(),
			})
			require.NoError(t, err)

			result, err := fetcher.Get(context.Background())
			require.NoError(t, err)
			require.Len(t, tt.expected, len(result))
			slices.SortFunc(result, func(a, b types.ResourceWithLabels) int {
				return strings.Compare(a.GetName(), b.GetName())
			})
			require.Empty(t, cmp.Diff(tt.expected.AsResources(), result))
		})
	}
}

type mockProtocolChecker struct {
	results map[string]string
}

func (m *mockProtocolChecker) CheckProtocol(service corev1.Service, port corev1.ServicePort) string {
	uri := fmt.Sprintf("%s:%d", services.GetServiceFQDN(service), port.Port)
	if result, ok := m.results[uri]; ok {
		return result
	}
	return "tcp"
}

func newMockService(name, namespace, externalName string, labels, annotations map[string]string, ports []corev1.ServicePort) *corev1.Service {
	serviceType := corev1.ServiceTypeClusterIP
	if externalName != "" {
		serviceType = corev1.ServiceTypeExternalName
	}
	return &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports:        ports,
			ClusterIP:    "192.168.100.100",
			ClusterIPs:   []string{"192.168.100.100"},
			Type:         serviceType,
			ExternalName: externalName,
		},
	}
}

func TestGetServicePorts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc        string
		annotations map[string]string
		ports       []corev1.ServicePort
		expected    []corev1.ServicePort
		wantErr     string
	}{
		{
			desc:     "Empty input",
			expected: nil,
		},
		{
			desc: "One unsupported port (UDP)",
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolUDP},
			},
			expected: nil,
		},
		{
			desc: "One supported port",
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			expected: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			desc: "One supported port, one unsupported port",
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 81, Protocol: corev1.ProtocolUDP},
			},
			expected: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			desc: "One supported port, one unsupported port",
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 81, Protocol: corev1.ProtocolUDP},
			},
			expected: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			desc: "Two supported ports",
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 81, Protocol: corev1.ProtocolTCP},
			},
			expected: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 81, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			desc:        "Two supported ports with numeric annotation",
			annotations: map[string]string{types.DiscoveryPortLabel: "42"},
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 42, Protocol: corev1.ProtocolTCP},
			},
			expected: []corev1.ServicePort{
				{Port: 42, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			desc:        "Two supported ports with name annotation",
			annotations: map[string]string{types.DiscoveryPortLabel: "right-port"},
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 42, Protocol: corev1.ProtocolTCP},
				{Port: 43, Protocol: corev1.ProtocolTCP, Name: "right-port"},
			},
			expected: []corev1.ServicePort{
				{Port: 43, Protocol: corev1.ProtocolTCP, Name: "right-port"},
			},
		},
		{
			desc:        "Annotation doesn't match available port, want error",
			annotations: map[string]string{types.DiscoveryPortLabel: "43"},
			ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 42, Protocol: corev1.ProtocolTCP},
			},
			expected: nil,
			wantErr:  "specified preferred port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service := corev1.Service{
				ObjectMeta: v1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: corev1.ServiceSpec{
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
	checker := NewProtoChecker()
	// Increasing client Timeout because CI/CD fails with a lower value.
	checker.client.Timeout = 5 * time.Second

	// Allow connections to HTTPS server created below.
	checker.client.Transport = &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
	}}

	totalNetworkHits := &atomic.Int32{}

	httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalNetworkHits.Add(1)
		_, _ = fmt.Fprintln(w, "httpsServer")
	}))
	httpsServerBaseURL, err := url.Parse(httpsServer.URL)
	require.NoError(t, err)

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this never gets called because the HTTP server will not accept the HTTPS request.
	}))
	httpServerBaseURL, err := url.Parse(httpServer.URL)
	require.NoError(t, err)

	tcpServer := newTCPServer(t, func(conn net.Conn) {
		totalNetworkHits.Add(1)
		_, _ = conn.Write([]byte("tcpServer"))
		_ = conn.Close()
	})
	tcpServerBaseURL := &url.URL{
		Host: tcpServer.Addr().String(),
	}

	t.Cleanup(func() {
		httpsServer.Close()
		httpServer.Close()
		_ = tcpServer.Close()
	})

	tests := []struct {
		host     string
		expected string
	}{
		{
			host:     httpServerBaseURL.Host,
			expected: "http",
		},
		{
			host:     httpsServerBaseURL.Host,
			expected: "https",
		},
		{
			host:     tcpServerBaseURL.Host,
			expected: "tcp",
		},
	}

	for _, tt := range tests {
		service, servicePort := createServiceAndServicePort(t, tt.expected, tt.host)
		res := checker.CheckProtocol(service, servicePort)
		require.Equal(t, tt.expected, res)
	}

	t.Run("caching prevents more than 1 network request to the same service", func(t *testing.T) {
		service, servicePort := createServiceAndServicePort(t, "https", httpsServerBaseURL.Host)
		checker.CheckProtocol(service, servicePort)
		// There can only be two hits recorded: one for the HTTPS Server and another one for the TCP Server.
		// The HTTP Server does not generate a network hit. See above.
		require.Equal(t, int32(2), totalNetworkHits.Load())
	})
}

func createServiceAndServicePort(t *testing.T, serviceName, host string) (corev1.Service, corev1.ServicePort) {
	host, portString, err := net.SplitHostPort(host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portString)
	require.NoError(t, err)
	service := corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceName,
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: host,
		},
	}
	servicePort := corev1.ServicePort{Port: int32(port)}
	return service, servicePort
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
			port := corev1.ServicePort{
				Name: tt.portName,
				Port: tt.portNumber,
			}
			if tt.appProtocol != "" {
				port.AppProtocol = &tt.appProtocol
			}

			result := autoProtocolDetection(corev1.Service{}, port, nil)

			require.Equal(t, tt.expected, result)
		})
	}
}
