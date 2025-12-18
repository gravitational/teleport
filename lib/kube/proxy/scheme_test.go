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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestNewClusterSchemaBuilder tests that newClusterSchemaBuilder doesn't panic
// when it's given types already registered in the global scheme.
func Test_newClusterSchemaBuilder(t *testing.T) {
	_, _, _, err := newClusterSchemaBuilder(logtest.NewLogger(), &clientSet{})
	require.NoError(t, err)
}

type clientSet struct {
	kubernetes.Interface
	discovery.DiscoveryInterface
}

func (c *clientSet) Discovery() discovery.DiscoveryInterface {
	return c
}

var fakeAPIResource = metav1.APIResourceList{
	GroupVersion: "extensions/v1beta1",
	APIResources: []metav1.APIResource{
		{
			Name:       "ingresses",
			Kind:       "Ingress",
			Namespaced: true,
		},
	},
}

func (c *clientSet) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, []*metav1.APIResourceList{
		&fakeAPIResource,
	}, nil
}

func TestRegisterDefaultKubeTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	err := registerDefaultKubeTypes(scheme)
	require.NoError(t, err)

	// Check that some known types are registered
	tests := []struct {
		gvk          schema.GroupVersionKind
		expectedType runtime.Object
	}{
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			expectedType: &corev1.Pod{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PodList"},
			expectedType: &corev1.PodList{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "metrics.k8s.io", Version: "v1beta1", Kind: "PodMetrics"},
			expectedType: &metricsv1beta1.PodMetrics{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "metrics.k8s.io", Version: "v1beta1", Kind: "NodeMetrics"},
			expectedType: &metricsv1beta1.NodeMetrics{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "metrics.k8s.io", Version: runtime.APIVersionInternal, Kind: "PodMetrics"},
			expectedType: &metrics.PodMetrics{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "metrics.k8s.io", Version: runtime.APIVersionInternal, Kind: "NodeMetrics"},
			expectedType: &metrics.NodeMetrics{},
		},

		// --- metav1 types ---
		{
			gvk:          schema.GroupVersionKind{Group: "meta.k8s.io", Version: "v1", Kind: "PartialObjectMetadata"},
			expectedType: &metav1.PartialObjectMetadata{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "meta.k8s.io", Version: "v1", Kind: "PartialObjectMetadataList"},
			expectedType: &metav1.PartialObjectMetadataList{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Status"},
			expectedType: &metav1.Status{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "APIVersions"},
			expectedType: &metav1.APIVersions{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "APIGroupList"},
			expectedType: &metav1.APIGroupList{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "APIGroup"},
			expectedType: &metav1.APIGroup{},
		},
		{
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "APIResourceList"},
			expectedType: &metav1.APIResourceList{},
		},
	}

	for _, testCase := range tests {
		newType, err := scheme.New(testCase.gvk)
		require.NoError(t, err, "expected type %v to be registered", testCase.gvk)
		require.IsType(t, testCase.expectedType, newType, "expected type %v to be of type %T", testCase.gvk, testCase.expectedType)
	}
}
