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

package resource

import (
	"testing"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalServerResourceKubernetes(t *testing.T) {
	// Regression test for
	// https://github.com/gravitational/teleport/issues/4862
	//
	// Verifies unmarshaling succeeds, when provided a 4.4 server JSON
	// definition.
	tests := []struct {
		desc string
		in   string
		want *ServerV2
	}{
		{
			desc: "4.4 kubernetes_clusters field",
			in: `{
	"version": "v2",
	"kind": "kube_service",
	"metadata": {
		"name": "foo"
	},
	"spec": {
		"kubernetes_clusters": ["a", "b", "c"]
	}
}`,
			want: &ServerV2{
				Version: V2,
				Kind:    KindKubeService,
				Metadata: Metadata{
					Name:      "foo",
					Namespace: defaults.Namespace,
				},
			},
		},
		{
			desc: "5.0 kubernetes_clusters field",
			in: `{
	"version": "v2",
	"kind": "kube_service",
	"metadata": {
		"name": "foo"
	},
	"spec": {
		"kube_clusters": [{"name": "a"}, {"name": "b"}, {"name": "c"}]
	}
}`,
			want: &ServerV2{
				Version: V2,
				Kind:    KindKubeService,
				Metadata: Metadata{
					Name:      "foo",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					KubernetesClusters: []*KubernetesCluster{
						{Name: "a"},
						{Name: "b"},
						{Name: "c"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := UnmarshalServer([]byte(tt.in), KindKubeService)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(got, tt.want))
		})
	}
}
