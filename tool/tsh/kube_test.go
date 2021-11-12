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

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
)

func TestGetKubeTLSServerName(t *testing.T) {
	tests := []struct {
		name          string
		kubeProxyAddr string
		want          string
	}{
		{
			name:          "ipv4 format, API domain should be used",
			kubeProxyAddr: "127.0.0.1",
			want:          "kube.teleport.cluster.local",
		},
		{
			name:          "ipv4 with port, API domain should be used",
			kubeProxyAddr: "127.0.0.1:3080",
			want:          "kube.teleport.cluster.local",
		},
		{
			name:          "ipv4 missing host, API domain should be used",
			kubeProxyAddr: ":3080",
			want:          "kube.teleport.cluster.local",
		},
		{
			name:          "ipv4 unspecified, API domain should be used ",
			kubeProxyAddr: "0.0.0.0:3080",
			want:          "kube.teleport.cluster.local",
		},
		{
			name:          "valid hostname with port",
			kubeProxyAddr: "example.com:3080",
			want:          "kube.example.com",
		},
		{
			name:          "valid hostname without port",
			kubeProxyAddr: "example.com",
			want:          "kube.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &client.TeleportClient{
				Config: client.Config{
					WebProxyAddr: tt.kubeProxyAddr,
				},
			}
			got := getKubeTLSServerName(tc)
			require.Equal(t, tt.want, got)

		})
	}
}
