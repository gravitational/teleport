/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResourceIDs tests that ResourceIDs are correctly marshaled to and from
// their string representation.
func TestResourceIDs(t *testing.T) {
	testCases := []struct {
		desc             string
		in               []ResourceID
		expected         string
		expectParseError bool
	}{
		{
			desc: "single id",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected: `["/one/node/uuid"]`,
		},
		{
			desc: "multiple ids",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        "uuid-1",
			}, {
				ClusterName: "two",
				Kind:        KindDatabase,
				Name:        "uuid-2",
			}},
			expected: `["/one/node/uuid-1","/two/db/uuid-2"]`,
		},
		{
			desc: "no cluster name",
			in: []ResourceID{{
				ClusterName: "",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected:         `["//node/uuid"]`,
			expectParseError: true,
		},
		{
			desc: "bad cluster name",
			in: []ResourceID{{
				ClusterName: "/,",
				Kind:        KindNode,
				Name:        "uuid",
			}},
			expected:         `["//,/node/uuid"]`,
			expectParseError: true,
		},
		{
			desc: "bad resource kind",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "not,/a,/kind",
				Name:        "uuid",
			}},
			expected:         `["/one/not,/a,/kind/uuid"]`,
			expectParseError: true,
		},
		{
			// Any resource name is actually fine, test that the list parsing
			// doesn't break.
			desc: "bad resource name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        `really"--,bad resource\"\\"name`,
			}},
			expected: `["/one/node/really\"--,bad resource\\\"\\\\\"name"]`,
		},
		{
			desc: "resource name with slash",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindNode,
				Name:        "node/id",
			}},
			expected: `["/one/node/node/id"]`,
		},
		{
			desc: "pod resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster/1",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/kube:ns:pods/cluster/1/namespace/pod*"]`,
		},
		{
			desc: "pod resource name",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/kube:ns:pods/cluster/namespace/pod*"]`,
		},
		{
			desc: "pod resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "/pod*",
			}},
			expected:         `["/one/kube:ns:pods/cluster//pod*"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:pods",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:pods/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/kube:ns:pods/cluster/namespace/pod*"]`,
		},
		{
			desc: "secret resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:secrets",
				Name:            "cluster",
				SubResourceName: "/secret*",
			}},
			expected:         `["/one/kube:ns:secrets/cluster//secret*"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:secrets",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:secrets/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:secrets",
				Name:            "cluster",
				SubResourceName: "namespace/secret*",
			}},
			expected: `["/one/kube:ns:secrets/cluster/namespace/secret*"]`,
		},
		{
			desc: "configmap resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:configmaps",
				Name:            "cluster",
				SubResourceName: "/configmap*",
			}},
			expected:         `["/one/kube:ns:configmaps/cluster//configmap*"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:configmaps",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:configmaps/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:configmaps",
				Name:            "cluster",
				SubResourceName: "namespace/configmap*",
			}},
			expected: `["/one/kube:ns:configmaps/cluster/namespace/configmap*"]`,
		},
		{
			desc: "service resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:services",
				Name:            "cluster",
				SubResourceName: "/service*",
			}},
			expected:         `["/one/kube:ns:services/cluster//service*"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:services",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:services/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:services",
				Name:            "cluster",
				SubResourceName: "namespace/service*",
			}},
			expected: `["/one/kube:ns:services/cluster/namespace/service*"]`,
		},
		{
			desc: "service_account resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:serviceaccounts",
				Name:            "cluster",
				SubResourceName: "/service_account*",
			}},
			expected:         `["/one/kube:ns:serviceaccounts/cluster//service_account*"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:serviceaccounts",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:serviceaccounts/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:serviceaccounts",
				Name:            "cluster",
				SubResourceName: "namespace/service_account*",
			}},
			expected: `["/one/kube:ns:serviceaccounts/cluster/namespace/service_account*"]`,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:persistentvolumeclaims",
				Name:            "cluster",
				SubResourceName: "/persistent_volume_claim*",
			}},
			expected:         `["/one/kube:ns:persistentvolumeclaims/cluster//persistent_volume_claim*"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:persistentvolumeclaims",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:ns:persistentvolumeclaims/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:namespaces",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:namespaces/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:namespaces",
				Name:            "cluster",
				SubResourceName: "namespace*",
			}},
			expected: `["/one/kube:cw:namespaces/cluster/namespace*"]`,
		},
		{
			desc: "kube_node resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:nodes",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:nodes/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "kube_node resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:nodes",
				Name:            "cluster",
				SubResourceName: "kube_node*",
			}},
			expected: `["/one/kube:cw:nodes/cluster/kube_node*"]`,
		},
		{
			desc: "persistent_volume resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:persistentvolumes",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:persistentvolumes/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:persistentvolumes",
				Name:            "cluster",
				SubResourceName: "persistent_volume*",
			}},
			expected: `["/one/kube:cw:persistentvolumes/cluster/persistent_volume*"]`,
		},
		{
			desc: "cluster_role resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:clusterroles.rbac.authorization.k8s.io",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:clusterroles.rbac.authorization.k8s.io/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:clusterroles.rbac.authorization.k8s.io",
				Name:            "cluster",
				SubResourceName: "cluster_role*",
			}},
			expected: `["/one/kube:cw:clusterroles.rbac.authorization.k8s.io/cluster/cluster_role*"]`,
		},
		{
			desc: "cluster_role_binding resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:clusterrolebindings.rbac.authorization.k8s.io",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:clusterrolebindings.rbac.authorization.k8s.io/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role_binding resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:clusterrolebindings.rbac.authorization.k8s.io",
				Name:            "cluster",
				SubResourceName: "cluster_role_binding*",
			}},
			expected: `["/one/kube:cw:clusterrolebindings.rbac.authorization.k8s.io/cluster/cluster_role_binding*"]`,
		},
		{
			desc: "certificate_signing_request resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:certificatesigningrequests.certificates.k8s.io",
				Name:        "cluster",
			}},
			expected:         `["/one/kube:cw:certificatesigningrequests.certificates.k8s.io/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "certificate_signing_request resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:certificatesigningrequests.certificates.k8s.io",
				Name:            "cluster",
				SubResourceName: "certificate_signing_request*",
			}},
			expected: `["/one/kube:cw:certificatesigningrequests.certificates.k8s.io/cluster/certificate_signing_request*"]`,
		},
		{
			desc: "full kube namespace access",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:*.*", // kind: *, api group: *.
				Name:            "cluster",
				SubResourceName: "default/*", // namespace: default, resource name: *.
			}},
			expected: `["/one/kube:ns:*.*/cluster/default/*"]`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			resourceIdsStr, err := ResourceIDsToString(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.expected, resourceIdsStr, "marshaled resource IDs do not match expected")

			t.Run("ResourceIDsFromString", func(t *testing.T) {
				parsed, err := ResourceIDsFromString(resourceIdsStr)
				if tc.expectParseError {
					require.Error(t, err, "expected to get an error parsing resource IDs")
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.in, parsed, "parsed resource IDs do not match the originals")
			})

			t.Run("ResourceIDsFromStrings", func(t *testing.T) {
				resourceIDStrs := make([]string, len(tc.in))
				for i, resourceID := range tc.in {
					resourceIDStrs[i] = ResourceIDToString(resourceID)
				}
				parsed, err := ResourceIDsFromStrings(resourceIDStrs)
				if tc.expectParseError {
					require.Error(t, err, "expected to get an error parsing resource IDs")
					return
				}
				require.NoError(t, err)
				require.Equal(t, tc.in, parsed, "parsed resource IDs do not match the originals")
			})
		})
	}
}

// TODO(@creack): Remove in v20 when we don't support legacy kube kinds.
func TestLegacyKubeResourceIDs(t *testing.T) {
	testCases := []struct {
		desc             string
		expect           []ResourceID
		in               string
		expectParseError bool
	}{
		{
			desc: "pod resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster/1",
				SubResourceName: "namespace/pod*",
			}},
			in: `["/one/pod/cluster/1/namespace/pod*"]`,
		},
		{
			desc: "pod resource name",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			in: `["/one/pod/cluster/namespace/pod*"]`,
		},
		{
			desc: "pod resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "/pod*",
			}},
			in:               `["/one/pod/cluster//pod*"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:pods",
				Name:        "cluster",
			}},
			in:               `["/one/pod/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:pods",
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			in: `["/one/pod/cluster/namespace/pod*"]`,
		},
		{
			desc: "secret resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:secrets",
				Name:            "cluster",
				SubResourceName: "/secret*",
			}},
			in:               `["/one/secret/cluster//secret*"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:secrets",
				Name:        "cluster",
			}},
			in:               `["/one/secret/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:secrets",
				Name:            "cluster",
				SubResourceName: "namespace/secret*",
			}},
			in: `["/one/secret/cluster/namespace/secret*"]`,
		},
		{
			desc: "configmap resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:configmaps",
				Name:            "cluster",
				SubResourceName: "/configmap*",
			}},
			in:               `["/one/configmap/cluster//configmap*"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:configmaps",
				Name:        "cluster",
			}},
			in:               `["/one/configmap/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:configmaps",
				Name:            "cluster",
				SubResourceName: "namespace/configmap*",
			}},
			in: `["/one/configmap/cluster/namespace/configmap*"]`,
		},
		{
			desc: "service resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:services",
				Name:            "cluster",
				SubResourceName: "/service*",
			}},
			in:               `["/one/service/cluster//service*"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:services",
				Name:        "cluster",
			}},
			in:               `["/one/service/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:services",
				Name:            "cluster",
				SubResourceName: "namespace/service*",
			}},
			in: `["/one/service/cluster/namespace/service*"]`,
		},
		{
			desc: "service_account resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:serviceaccounts",
				Name:            "cluster",
				SubResourceName: "/service_account*",
			}},
			in:               `["/one/serviceaccount/cluster//service_account*"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:serviceaccounts",
				Name:        "cluster",
			}},
			in:               `["/one/serviceaccount/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:serviceaccounts",
				Name:            "cluster",
				SubResourceName: "namespace/service_account*",
			}},
			in: `["/one/serviceaccount/cluster/namespace/service_account*"]`,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:ns:persistentvolumeclaims",
				Name:            "cluster",
				SubResourceName: "/persistent_volume_claim*",
			}},
			in:               `["/one/persistentvolumeclaim/cluster//persistent_volume_claim*"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:ns:persistentvolumeclaims",
				Name:        "cluster",
			}},
			in:               `["/one/persistentvolumeclaim/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "namespaces",
				Name:        "cluster",
			}},
			in:               `["/one/namespace/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "namespace",
				Name:            "cluster",
				SubResourceName: "namespace*",
			}},
			in: `["/one/namespace/cluster/namespace*"]`,
		},
		{
			desc: "kube_node resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:nodes",
				Name:        "cluster",
			}},
			in:               `["/one/kube_node/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "kube_node resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:nodes",
				Name:            "cluster",
				SubResourceName: "kube_node*",
			}},
			in: `["/one/kube_node/cluster/kube_node*"]`,
		},
		{
			desc: "persistent_volume resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:persistentvolumes",
				Name:        "cluster",
			}},
			in:               `["/one/persistentvolume/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:persistentvolumes",
				Name:            "cluster",
				SubResourceName: "persistent_volume*",
			}},
			in: `["/one/persistentvolume/cluster/persistent_volume*"]`,
		},
		{
			desc: "cluster_role resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:clusterroles.rbac.authorization.k8s.io",
				Name:        "cluster",
			}},
			in:               `["/one/clusterrole/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:clusterroles.rbac.authorization.k8s.io",
				Name:            "cluster",
				SubResourceName: "cluster_role*",
			}},
			in: `["/one/clusterrole/cluster/cluster_role*"]`,
		},
		{
			desc: "cluster_role_binding resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:clusterrolebindings.rbac.authorization.k8s.io",
				Name:        "cluster",
			}},
			in:               `["/one/clusterrolebinding/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role_binding resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:clusterrolebindings.rbac.authorization.k8s.io",
				Name:            "cluster",
				SubResourceName: "cluster_role_binding*",
			}},
			in: `["/one/clusterrolebinding/cluster/cluster_role_binding*"]`,
		},
		{
			desc: "certificate_signing_request resource name with missing namespace and pod name",
			expect: []ResourceID{{
				ClusterName: "one",
				Kind:        "kube:cw:certificatesigningrequests.certificates.k8s.io",
				Name:        "cluster",
			}},
			in:               `["/one/certificatesigningrequest/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "certificate_signing_request resource name in cluster with slash",
			expect: []ResourceID{{
				ClusterName:     "one",
				Kind:            "kube:cw:certificatesigningrequests.certificates.k8s.io",
				Name:            "cluster",
				SubResourceName: "certificate_signing_request*",
			}},
			in: `["/one/certificatesigningrequest/cluster/certificate_signing_request*"]`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			parsed, err := ResourceIDsFromString(tc.in)
			if tc.expectParseError {
				require.Error(t, err, "expected to get an error parsing resource IDs")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, parsed, "parsed resource IDs do not match the originals")
		})
	}
}
