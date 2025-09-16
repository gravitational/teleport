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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gravitational/teleport/api/types"
)

func TestParseResourcePath(t *testing.T) {
	tests := []struct {
		path string
		want apiResource
	}{
		{path: "", want: apiResource{}},
		{path: "/", want: apiResource{}},
		{path: "/api", want: apiResource{skipEvent: true}},
		{path: "/api/", want: apiResource{skipEvent: true}},
		{path: "/api/v1", want: apiResource{skipEvent: true, apiGroup: "core", apiGroupVersion: "v1"}},
		{path: "/api/v1/", want: apiResource{skipEvent: true, apiGroup: "core", apiGroupVersion: "v1"}},
		{path: "/apis", want: apiResource{skipEvent: true}},
		{path: "/apis/", want: apiResource{skipEvent: true}},
		{path: "/apis/apps", want: apiResource{skipEvent: true}},
		{path: "/apis/apps/", want: apiResource{skipEvent: true}},
		{path: "/apis/apps/v1", want: apiResource{skipEvent: true, apiGroup: "apps", apiGroupVersion: "v1"}},
		{path: "/apis/apps/v1/", want: apiResource{skipEvent: true, apiGroup: "apps", apiGroupVersion: "v1"}},
		{path: "/api/v1/pods", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", resourceKind: "pods"}},
		{path: "/api/v1/watch/pods", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", resourceKind: "pods", isWatch: true}},
		{path: "/api/v1/namespaces/kube-system", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", resourceKind: "namespaces", resourceName: "kube-system"}},
		{path: "/api/v1/watch/namespaces/kube-system", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", resourceKind: "namespaces", resourceName: "kube-system", isWatch: true}},
		{path: "/apis/rbac.authorization.k8s.io/v1/clusterroles", want: apiResource{apiGroup: "rbac.authorization.k8s.io", apiGroupVersion: "v1", resourceKind: "clusterroles"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/watch/clusterroles", want: apiResource{apiGroup: "rbac.authorization.k8s.io", apiGroupVersion: "v1", resourceKind: "clusterroles", isWatch: true}},
		{path: "/apis/rbac.authorization.k8s.io/v1/clusterroles/foo", want: apiResource{apiGroup: "rbac.authorization.k8s.io", apiGroupVersion: "v1", resourceKind: "clusterroles", resourceName: "foo"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/watch/clusterroles/foo", want: apiResource{apiGroup: "rbac.authorization.k8s.io", apiGroupVersion: "v1", resourceKind: "clusterroles", resourceName: "foo", isWatch: true}},
		{path: "/api/v1/namespaces/kube-system/pods", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", namespace: "kube-system", resourceKind: "pods"}},
		{path: "/api/v1/watch/namespaces/kube-system/pods", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", namespace: "kube-system", resourceKind: "pods", isWatch: true}},
		{path: "/api/v1/namespaces/kube-system/pods/foo", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", namespace: "kube-system", resourceKind: "pods", resourceName: "foo"}},
		{path: "/api/v1/watch/namespaces/kube-system/pods/foo", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", namespace: "kube-system", resourceKind: "pods", resourceName: "foo", isWatch: true}},
		{path: "/api/v1/namespaces/kube-system/pods/foo/exec", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", namespace: "kube-system", resourceKind: "pods/exec", resourceName: "foo"}},
		{path: "/apis/apiregistration.k8s.io/v1/apiservices/foo/status", want: apiResource{apiGroup: "apiregistration.k8s.io", apiGroupVersion: "v1", resourceKind: "apiservices/status", resourceName: "foo"}},
		{path: "/api/v1/nodes/foo/proxy/bar", want: apiResource{apiGroup: "core", apiGroupVersion: "v1", resourceKind: "nodes/proxy/bar", resourceName: "foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := parseResourcePath(tt.path)
			diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(apiResource{}))
			require.Empty(t, diff, "parsing path %q", tt.path)
		})
	}
}

func Test_getResourceFromRequest(t *testing.T) {
	t.Parallel()
	bodyFunc := func(t, api string) io.ReadCloser {
		return io.NopCloser(strings.NewReader(`{"kind":"` + t + `","apiVersion":"` + api + `","metadata":{"name":"foo-create"}}`))
	}
	bodyFuncWithoutGVK := func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(`{"metadata":{"name":"foo-create"}}`))
	}
	tests := []struct {
		path string
		body io.ReadCloser
		want *types.KubernetesResource
	}{
		{path: "", want: nil},
		{path: "/", want: nil},
		{path: "/api", want: nil},
		{path: "/api/", want: nil},
		{path: "/api/v1", want: nil},
		{path: "/api/v1/", want: nil},
		{path: "/apis", want: nil},
		{path: "/apis/", want: nil},
		{path: "/apis/apps", want: nil},
		{path: "/apis/apps/", want: nil},
		{path: "/apis/apps/v1", want: nil},
		{path: "/apis/apps/v1/", want: nil},
		{path: "/api/v1/pods", want: nil},
		{path: "/api/v1/watch/pods", want: nil},
		{path: "/api/v1/namespaces/kube-system", want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "kube-system", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/kube-system", want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "kube-system", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces/kube-system/pods", want: nil},
		{path: "/api/v1/watch/namespaces/kube-system/pods", want: nil},
		{path: "/api/v1/namespaces/kube-system/pods/foo", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "kube-system", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/kube-system/pods/foo", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "kube-system", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/apis/apiregistration.k8s.io/v1/apiservices/foo/status", want: nil},

		// core
		// Pods
		{path: "/api/v1/pods", want: nil},
		{path: "/api/v1/namespaces/default/pods", want: nil},
		{path: "/api/v1/namespaces/default/pods/foo", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/pods/foo", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces/kube-system/pods/foo/exec", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "kube-system", Name: "foo", Verbs: []string{"exec"}}},
		{path: "/api/v1/namespaces/kube-system/pods/foo/attach", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "kube-system", Name: "foo", Verbs: []string{"exec"}}},
		{path: "/api/v1/namespaces/kube-system/pods/foo/portforward", want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "kube-system", Name: "foo", Verbs: []string{"portforward"}}},
		{path: "/api/v1/namespaces/default/pods", body: bodyFunc("Pod", "v1"), want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/api/v1/namespaces/default/pods", body: bodyFuncWithoutGVK(), want: &types.KubernetesResource{Kind: types.KindKubePod, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},

		// Secrets
		{path: "/api/v1/secrets", want: nil},
		{path: "/api/v1/namespaces/default/secrets", want: nil},
		{path: "/api/v1/namespaces/default/secrets/foo", want: &types.KubernetesResource{Kind: types.KindKubeSecret, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/secrets/foo", want: &types.KubernetesResource{Kind: types.KindKubeSecret, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces/default/secrets", body: bodyFunc("Secret", "v1"), want: &types.KubernetesResource{Kind: types.KindKubeSecret, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/api/v1/namespaces/default/secrets", body: bodyFuncWithoutGVK(), want: &types.KubernetesResource{Kind: types.KindKubeSecret, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},

		// Configmaps
		{path: "/api/v1/configmaps", want: nil},
		{path: "/api/v1/namespaces/default/configmaps", want: nil},
		{path: "/api/v1/namespaces/default/configmaps/foo", want: &types.KubernetesResource{Kind: types.KindKubeConfigmap, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/configmaps/foo", want: &types.KubernetesResource{Kind: types.KindKubeConfigmap, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces/default/configmaps", body: bodyFunc("ConfigMap", "v1"), want: &types.KubernetesResource{Kind: types.KindKubeConfigmap, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/api/v1/namespaces/default/configmaps", body: bodyFuncWithoutGVK(), want: &types.KubernetesResource{Kind: types.KindKubeConfigmap, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},

		// Namespaces
		{path: "/api/v1/namespaces", want: nil},
		{path: "/api/v1/namespaces/default", want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "default", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default", want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "default", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces", body: bodyFunc("Namespace", "v1"), want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/api/v1/namespaces", body: bodyFuncWithoutGVK(), want: &types.KubernetesResource{Kind: types.KindKubeNamespace, Name: "foo-create", Verbs: []string{"create"}}},

		// Nodes
		{path: "/api/v1/nodes", want: nil},
		{path: "/api/v1/nodes/foo/proxy/bar", want: &types.KubernetesResource{Kind: types.KindKubeNode, Name: "foo", Verbs: []string{"get"}}},
		// Services
		{path: "/api/v1/services", want: nil},
		{path: "/api/v1/namespaces/default/services", want: nil},
		{path: "/api/v1/namespaces/default/services/foo", want: &types.KubernetesResource{Kind: types.KindKubeService, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/services/foo", want: &types.KubernetesResource{Kind: types.KindKubeService, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/api/v1/namespaces/default/services", body: bodyFunc("Service", "v1"), want: &types.KubernetesResource{Kind: types.KindKubeService, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},

		// ServiceAccounts
		{path: "/api/v1/serviceaccounts", want: nil},
		{path: "/api/v1/namespaces/default/serviceaccounts", want: nil},
		{path: "/api/v1/namespaces/default/serviceaccounts/foo", want: &types.KubernetesResource{Kind: types.KindKubeServiceAccount, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/serviceaccounts/foo", want: &types.KubernetesResource{Kind: types.KindKubeServiceAccount, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		// PersistentVolumes
		{path: "/api/v1/persistentvolumes", want: nil},
		{path: "/api/v1/namespaces/default/persistentvolumes", want: nil},
		{path: "/api/v1/namespaces/default/persistentvolumes/foo", want: &types.KubernetesResource{Kind: types.KindKubePersistentVolume, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/persistentvolumes/foo", want: &types.KubernetesResource{Kind: types.KindKubePersistentVolume, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		// PersistentVolumeClaims
		{path: "/api/v1/persistentvolumeclaims", want: nil},
		{path: "/api/v1/namespaces/default/persistentvolumeclaims", want: nil},
		{path: "/api/v1/namespaces/default/persistentvolumeclaims/foo", want: &types.KubernetesResource{Kind: types.KindKubePersistentVolumeClaim, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/api/v1/watch/namespaces/default/persistentvolumeclaims/foo", want: &types.KubernetesResource{Kind: types.KindKubePersistentVolumeClaim, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},

		// apis/apps
		// Deployments
		{path: "/apis/apps/v1/deployments", want: nil},
		{path: "/apis/apps/v1/namespaces/default/deployments", want: nil},
		{path: "/apis/apps/v1/namespaces/default/deployments/foo", want: &types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/apps/v1/watch/namespaces/default/deployments/foo", want: &types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		{path: "/apis/apps/v1/namespaces/default/deployments", body: bodyFunc("Deployment", "apps/v1"), want: &types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/apis/apps/v1beta2/namespaces/default/deployments", body: bodyFunc("Deployment", "apps/v1beta2"), want: &types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},
		{path: "/apis/apps/v1/namespaces/default/deployments", body: bodyFuncWithoutGVK(), want: &types.KubernetesResource{Kind: types.KindKubeDeployment, Namespace: "default", Name: "foo-create", Verbs: []string{"create"}}},

		// Statefulsets
		{path: "/apis/apps/v1/statefulsets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/statefulsets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/statefulsets/foo", want: &types.KubernetesResource{Kind: types.KindKubeStatefulset, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/apps/v1/watch/namespaces/default/statefulsets/foo", want: &types.KubernetesResource{Kind: types.KindKubeStatefulset, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		// Replicasets
		{path: "/apis/apps/v1/replicasets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/replicasets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/replicasets/foo", want: &types.KubernetesResource{Kind: types.KindKubeReplicaSet, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/apps/v1/watch/namespaces/default/replicasets/foo", want: &types.KubernetesResource{Kind: types.KindKubeReplicaSet, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		// Daemonsets
		{path: "/apis/apps/v1/daemonsets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/daemonsets", want: nil},
		{path: "/apis/apps/v1/namespaces/default/daemonsets/foo", want: &types.KubernetesResource{Kind: types.KindKubeDaemonSet, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/apps/v1/watch/namespaces/default/daemonsets/foo", want: &types.KubernetesResource{Kind: types.KindKubeDaemonSet, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},

		// apis/batch
		// Job
		{path: "/apis/batch/v1/jobs", want: nil},
		{path: "/apis/batch/v1/namespaces/default/jobs", want: nil},
		{path: "/apis/batch/v1/namespaces/default/jobs/foo", want: &types.KubernetesResource{Kind: types.KindKubeJob, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/batch/v1/watch/namespaces/default/jobs/foo", want: &types.KubernetesResource{Kind: types.KindKubeJob, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},
		// CronJob
		{path: "/apis/batch/v1/cronjobs", want: nil},
		{path: "/apis/batch/v1/namespaces/default/cronjobs", want: nil},
		{path: "/apis/batch/v1/namespaces/default/cronjobs/foo", want: &types.KubernetesResource{Kind: types.KindKubeCronjob, Namespace: "default", Name: "foo", Verbs: []string{"get"}}},
		{path: "/apis/batch/v1/watch/namespaces/default/cronjobs/foo", want: &types.KubernetesResource{Kind: types.KindKubeCronjob, Namespace: "default", Name: "foo", Verbs: []string{"watch"}}},

		// apis/certificates.k8s.io
		{path: "/apis/certificates.k8s.io/v1/certificatesigningrequests", want: nil},
		{path: "/apis/certificates.k8s.io/v1/certificatesigningrequests/foo", want: &types.KubernetesResource{Kind: types.KindKubeCertificateSigningRequest, Name: "foo", Verbs: []string{"get"}}},

		// apis/networking.k8s.io
		{path: "/apis/networking.k8s.io/v1/ingresses", want: nil},
		{path: "/apis/networking.k8s.io/v1/ingresses/foo", want: &types.KubernetesResource{Kind: types.KindKubeIngress, Name: "foo", Verbs: []string{"get"}}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			verb := http.MethodGet
			if tt.body != nil {
				verb = http.MethodPost
			}
			got, _, err := getResourceFromRequest(&http.Request{Method: verb, URL: &url.URL{Path: tt.path}, Body: tt.body}, &kubeDetails{
				kubeCodecs:         globalKubeCodecs,
				rbacSupportedTypes: defaultRBACResources,
				gvkSupportedResources: map[gvkSupportedResourcesKey]*schema.GroupVersionKind{
					{
						apiGroup: "",
						version:  "v1",
						name:     "pods",
					}: {
						Group:   "",
						Version: "v1",
						Kind:    "Pod",
					},
					{
						apiGroup: "",
						version:  "v1",
						name:     "secrets",
					}: {
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					{
						apiGroup: "",
						version:  "v1",
						name:     "configmaps",
					}: {
						Group:   "",
						Version: "v1",
						Kind:    "ConfigMap",
					},
					{
						apiGroup: "",
						version:  "v1",
						name:     "namespaces",
					}: {
						Group:   "",
						Version: "v1",
						Kind:    "Namespace",
					},
					{
						apiGroup: "apps",
						version:  "v1",
						name:     "deployments",
					}: {
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "parsing path %q", tt.path)
		})
	}
}
