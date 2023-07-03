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
				Kind:            KindKubePod,
				Name:            "cluster/1",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/pod/cluster/1/namespace/pod*"]`,
		},
		{
			desc: "pod resource name",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubePod,
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/pod/cluster/namespace/pod*"]`,
		},
		{
			desc: "pod resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubePod,
				Name:            "cluster",
				SubResourceName: "/pod*",
			}},
			expected:         `["/one/pod/cluster//pod*"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubePod,
				Name:        "cluster",
			}},
			expected:         `["/one/pod/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "pod resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubePod,
				Name:            "cluster",
				SubResourceName: "namespace/pod*",
			}},
			expected: `["/one/pod/cluster/namespace/pod*"]`,
		},
		{
			desc: "secret resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeSecret,
				Name:            "cluster",
				SubResourceName: "/secret*",
			}},
			expected:         `["/one/secret/cluster//secret*"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeSecret,
				Name:        "cluster",
			}},
			expected:         `["/one/secret/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "secret resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeSecret,
				Name:            "cluster",
				SubResourceName: "namespace/secret*",
			}},
			expected: `["/one/secret/cluster/namespace/secret*"]`,
		},
		{
			desc: "configmap resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeConfigmap,
				Name:            "cluster",
				SubResourceName: "/configmap*",
			}},
			expected:         `["/one/configmap/cluster//configmap*"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeConfigmap,
				Name:        "cluster",
			}},
			expected:         `["/one/configmap/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "configmap resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeConfigmap,
				Name:            "cluster",
				SubResourceName: "namespace/configmap*",
			}},
			expected: `["/one/configmap/cluster/namespace/configmap*"]`,
		},
		{
			desc: "service resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeService,
				Name:            "cluster",
				SubResourceName: "/service*",
			}},
			expected:         `["/one/service/cluster//service*"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeService,
				Name:        "cluster",
			}},
			expected:         `["/one/service/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeService,
				Name:            "cluster",
				SubResourceName: "namespace/service*",
			}},
			expected: `["/one/service/cluster/namespace/service*"]`,
		},
		{
			desc: "service_account resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeServiceAccount,
				Name:            "cluster",
				SubResourceName: "/service_account*",
			}},
			expected:         `["/one/serviceaccount/cluster//service_account*"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeServiceAccount,
				Name:        "cluster",
			}},
			expected:         `["/one/serviceaccount/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "service_account resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeServiceAccount,
				Name:            "cluster",
				SubResourceName: "namespace/service_account*",
			}},
			expected: `["/one/serviceaccount/cluster/namespace/service_account*"]`,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubePersistentVolumeClaim,
				Name:            "cluster",
				SubResourceName: "/persistent_volume_claim*",
			}},
			expected:         `["/one/persistentvolumeclaim/cluster//persistent_volume_claim*"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume_claim resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubePersistentVolumeClaim,
				Name:        "cluster",
			}},
			expected:         `["/one/persistentvolumeclaim/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeNamespace,
				Name:        "cluster",
			}},
			expected:         `["/one/namespace/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "namespace resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeNamespace,
				Name:            "cluster",
				SubResourceName: "namespace*",
			}},
			expected: `["/one/namespace/cluster/namespace*"]`,
		},
		{
			desc: "kube_node resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeNode,
				Name:        "cluster",
			}},
			expected:         `["/one/kube_node/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "kube_node resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeNode,
				Name:            "cluster",
				SubResourceName: "kube_node*",
			}},
			expected: `["/one/kube_node/cluster/kube_node*"]`,
		},
		{
			desc: "persistent_volume resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubePersistentVolume,
				Name:        "cluster",
			}},
			expected:         `["/one/persistentvolume/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "persistent_volume resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubePersistentVolume,
				Name:            "cluster",
				SubResourceName: "persistent_volume*",
			}},
			expected: `["/one/persistentvolume/cluster/persistent_volume*"]`,
		},
		{
			desc: "cluster_role resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeClusterRole,
				Name:        "cluster",
			}},
			expected:         `["/one/clusterrole/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeClusterRole,
				Name:            "cluster",
				SubResourceName: "cluster_role*",
			}},
			expected: `["/one/clusterrole/cluster/cluster_role*"]`,
		},
		{
			desc: "cluster_role_binding resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeClusterRoleBinding,
				Name:        "cluster",
			}},
			expected:         `["/one/clusterrolebinding/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "cluster_role_binding resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeClusterRoleBinding,
				Name:            "cluster",
				SubResourceName: "cluster_role_binding*",
			}},
			expected: `["/one/clusterrolebinding/cluster/cluster_role_binding*"]`,
		},
		{
			desc: "certificate_signing_request resource name with missing namespace and pod name",
			in: []ResourceID{{
				ClusterName: "one",
				Kind:        KindKubeCertificateSigningRequest,
				Name:        "cluster",
			}},
			expected:         `["/one/certificatesigningrequest/cluster"]`,
			expectParseError: true,
		},
		{
			desc: "certificate_signing_request resource name in cluster with slash",
			in: []ResourceID{{
				ClusterName:     "one",
				Kind:            KindKubeCertificateSigningRequest,
				Name:            "cluster",
				SubResourceName: "certificate_signing_request*",
			}},
			expected: `["/one/certificatesigningrequest/cluster/certificate_signing_request*"]`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			out, err := ResourceIDsToString(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.expected, out)

			// Parse the ids from the string and make sure they match the
			// original.
			parsed, err := ResourceIDsFromString(out)
			if tc.expectParseError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.in, parsed)
		})
	}
}
