/*
Copyright 2023 Gravitational, Inc.

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

package fetchers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
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
		protoChecker      services.ProtocolChecker
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
			objects := []runtime.Object{}
			for _, s := range tt.services {
				objects = append(objects, s)
			}
			fakeClient := fake.NewSimpleClientset(objects...)

			fetcher, err := NewKubeAppsFetcher(KubeAppsFetcherConfig{
				ClusterName:      "test-cluster",
				KubernetesClient: fakeClient,
				FilterLabels:     tt.matcherLabels,
				Namespaces:       tt.matcherNamespaces,
				protocolChecker:  tt.protoChecker,
				Log:              utils.NewLogger(),
			})
			require.NoError(t, err)

			result, err := fetcher.Get(context.Background())
			require.NoError(t, err)
			require.Equal(t, len(tt.expected), len(result))
			slices.SortFunc(result, func(a, b types.ResourceWithLabels) bool {
				return a.GetName() < b.GetName()
			})
			require.Empty(t, cmp.Diff(tt.expected.AsResources(), result))
		})

	}
}

type mockProtocolChecker struct {
	results map[string]string
}

func (m *mockProtocolChecker) CheckProtocol(uri string) string {
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
