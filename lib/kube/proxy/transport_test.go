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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_allServersSupportImpersonation(t *testing.T) {
	type args struct {
		servers []teleportVersionInterface
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "all servers support impersonation",
			args: args{
				servers: []teleportVersionInterface{
					newKubeServerForVersion(t, "13.0.0"),
					newKubeServerForVersion(t, "13.0.0-alpha.1"),
					newKubeServerForVersion(t, "14.0.0"),
					newKubeServerForVersion(t, "15.0.0"),
				},
			},
			want: true,
		},
		{
			name: "a server does support impersonation",
			args: args{
				servers: []teleportVersionInterface{
					newKubeServerForVersion(t, "13.0.0"),
					newKubeServerForVersion(t, "13.0.0-alpha.1"),
					newKubeServerForVersion(t, "14.0.0"),
					newKubeServerForVersion(t, "12.1.0"),
				},
			},
			want: false,
		},
		{
			name: "a server without valid version",
			args: args{
				servers: []teleportVersionInterface{
					newKubeServerForVersion(t, "13.0.0"),
					newKubeServerForVersion(t, ".0.0-alpha.1"),
				},
			},
			want: false,
		},
		{
			name: "all proxy support impersonation",
			args: args{
				servers: []teleportVersionInterface{
					newProxyServerForVersion(t, "13.0.0"),
					newProxyServerForVersion(t, "13.0.0-alpha.1"),
					newProxyServerForVersion(t, "14.0.0"),
					newProxyServerForVersion(t, "15.0.0"),
				},
			},
			want: true,
		},
		{
			name: "a proxy does support impersonation",
			args: args{
				servers: []teleportVersionInterface{
					newProxyServerForVersion(t, "13.0.0"),
					newProxyServerForVersion(t, "13.0.0-alpha.1"),
					newProxyServerForVersion(t, "14.0.0"),
					newProxyServerForVersion(t, "12.1.0"),
				},
			},
			want: false,
		},
		{
			name: "a proxy without valid version",
			args: args{
				servers: []teleportVersionInterface{
					newProxyServerForVersion(t, "13.0.0"),
					newProxyServerForVersion(t, ".0.0-alpha.1"),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, allServersSupportImpersonation(tt.args.servers))
		})
	}
}

func newKubeServerForVersion(t *testing.T, version string) types.KubeServer {
	k, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "cluster",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	ks, err := types.NewKubernetesServerV3FromCluster(k, "host", "uiid")
	require.NoError(t, err)
	ks.Spec.Version = version
	return ks
}

func newProxyServerForVersion(t *testing.T, version string) types.Server {
	server, err := types.NewServer("host", types.KindProxy, types.ServerSpecV2{
		Version: version,
	})
	require.NoError(t, err)
	return server
}
