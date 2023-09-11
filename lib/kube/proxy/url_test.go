/*
Copyright 2020 Gravitational, Inc.

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

package proxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
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
		{path: "/api/v1", want: apiResource{skipEvent: true, apiGroup: "core/v1"}},
		{path: "/api/v1/", want: apiResource{skipEvent: true, apiGroup: "core/v1"}},
		{path: "/apis", want: apiResource{skipEvent: true}},
		{path: "/apis/", want: apiResource{skipEvent: true}},
		{path: "/apis/apps", want: apiResource{skipEvent: true}},
		{path: "/apis/apps/", want: apiResource{skipEvent: true}},
		{path: "/apis/apps/v1", want: apiResource{skipEvent: true, apiGroup: "apps/v1"}},
		{path: "/apis/apps/v1/", want: apiResource{skipEvent: true, apiGroup: "apps/v1"}},
		{path: "/api/v1/pods", want: apiResource{apiGroup: "core/v1", resourceKind: "pods"}},
		{path: "/api/v1/watch/pods", want: apiResource{apiGroup: "core/v1", resourceKind: "pods"}},
		{path: "/api/v1/namespaces/kube-system", want: apiResource{apiGroup: "core/v1", resourceKind: "namespaces", resourceName: "kube-system"}},
		{path: "/api/v1/watch/namespaces/kube-system", want: apiResource{apiGroup: "core/v1", resourceKind: "namespaces", resourceName: "kube-system"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/clusterroles", want: apiResource{apiGroup: "rbac.authorization.k8s.io/v1", resourceKind: "clusterroles"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/watch/clusterroles", want: apiResource{apiGroup: "rbac.authorization.k8s.io/v1", resourceKind: "clusterroles"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/clusterroles/foo", want: apiResource{apiGroup: "rbac.authorization.k8s.io/v1", resourceKind: "clusterroles", resourceName: "foo"}},
		{path: "/apis/rbac.authorization.k8s.io/v1/watch/clusterroles/foo", want: apiResource{apiGroup: "rbac.authorization.k8s.io/v1", resourceKind: "clusterroles", resourceName: "foo"}},
		{path: "/api/v1/namespaces/kube-system/pods", want: apiResource{apiGroup: "core/v1", namespace: "kube-system", resourceKind: "pods"}},
		{path: "/api/v1/watch/namespaces/kube-system/pods", want: apiResource{apiGroup: "core/v1", namespace: "kube-system", resourceKind: "pods"}},
		{path: "/api/v1/namespaces/kube-system/pods/foo", want: apiResource{apiGroup: "core/v1", namespace: "kube-system", resourceKind: "pods", resourceName: "foo"}},
		{path: "/api/v1/watch/namespaces/kube-system/pods/foo", want: apiResource{apiGroup: "core/v1", namespace: "kube-system", resourceKind: "pods", resourceName: "foo"}},
		{path: "/api/v1/namespaces/kube-system/pods/foo/exec", want: apiResource{apiGroup: "core/v1", namespace: "kube-system", resourceKind: "pods/exec", resourceName: "foo"}},
		{path: "/apis/apiregistration.k8s.io/v1/apiservices/foo/status", want: apiResource{apiGroup: "apiregistration.k8s.io/v1", resourceKind: "apiservices/status", resourceName: "foo"}},
		{path: "/api/v1/nodes/foo/proxy/bar", want: apiResource{apiGroup: "core/v1", resourceKind: "nodes/proxy/bar", resourceName: "foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := parseResourcePath(tt.path)
			diff := cmp.Diff(got, tt.want, cmp.AllowUnexported(apiResource{}))
			require.Empty(t, diff, "parsing path %q", tt.path)
		})
	}
}
