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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKubeLocalProxyPathPrefix(t *testing.T) {
	tests := []struct {
		name            string
		teleportCluster string
		kubeCluster     string
		want            string
	}{
		{
			name:            "short names",
			teleportCluster: "root-cluster",
			kubeCluster:     "kube1",
			want:            "/v1/teleport/cm9vdC1jbHVzdGVy/a3ViZTE",
		},
		{
			name:            "long kube cluster name (regression for #61439)",
			teleportCluster: "teleport.example.com",
			kubeCluster:     "loooooooooooooooooooooooooooooooooooooooooong-kube-cluster-exceeding-sixty-three-chars",
			want:            "/v1/teleport/dGVsZXBvcnQuZXhhbXBsZS5jb20/bG9vb29vb29vb29vb29vb29vb29vb29vb29vb29vb29vb29vb29vb29vb25nLWt1YmUtY2x1c3Rlci1leGNlZWRpbmctc2l4dHktdGhyZWUtY2hhcnM",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, KubeLocalProxyPathPrefix(tc.teleportCluster, tc.kubeCluster))
		})
	}
}

func TestClustersFromKubeLocalProxyPath(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		const teleportCluster = "teleport.example.com"
		const kubeCluster = "loooooooooooooooooooooooooooooooooooooooooong-kube-cluster-exceeding-sixty-three-chars"

		path := KubeLocalProxyPathPrefix(teleportCluster, kubeCluster) + "/api/v1/namespaces"
		tc, kc, err := ClustersFromKubeLocalProxyPath(path)
		require.NoError(t, err)
		require.Equal(t, teleportCluster, tc)
		require.Equal(t, kubeCluster, kc)
	})

	t.Run("just the prefix", func(t *testing.T) {
		path := KubeLocalProxyPathPrefix("root-cluster", "kube1")
		tc, kc, err := ClustersFromKubeLocalProxyPath(path)
		require.NoError(t, err)
		require.Equal(t, "root-cluster", tc)
		require.Equal(t, "kube1", kc)
	})

	t.Run("prefix with trailing slash", func(t *testing.T) {
		path := KubeLocalProxyPathPrefix("root-cluster", "kube1") + "/"
		tc, kc, err := ClustersFromKubeLocalProxyPath(path)
		require.NoError(t, err)
		require.Equal(t, "root-cluster", tc)
		require.Equal(t, "kube1", kc)
	})

	t.Run("rejects bare API path", func(t *testing.T) {
		_, _, err := ClustersFromKubeLocalProxyPath("/api/v1/namespaces")
		require.ErrorContains(t, err, "invalid kube local proxy path")
	})

	t.Run("rejects wrong leading component", func(t *testing.T) {
		_, _, err := ClustersFromKubeLocalProxyPath("/v2/teleport/abc/def/api/v1/namespaces")
		require.ErrorContains(t, err, "invalid kube local proxy path")
	})

	t.Run("rejects missing kube cluster segment", func(t *testing.T) {
		_, _, err := ClustersFromKubeLocalProxyPath("/v1/teleport/abc")
		require.ErrorContains(t, err, "invalid kube local proxy path")
	})

	t.Run("rejects non-base64url teleport cluster", func(t *testing.T) {
		_, _, err := ClustersFromKubeLocalProxyPath("/v1/teleport/not*base64/a3ViZTE")
		require.ErrorContains(t, err, "decoding teleport cluster")
	})

	t.Run("rejects non-base64url kube cluster", func(t *testing.T) {
		_, _, err := ClustersFromKubeLocalProxyPath("/v1/teleport/cm9vdC1jbHVzdGVy/not*base64")
		require.ErrorContains(t, err, "decoding kube cluster")
	})
}
