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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestKubernetesServerUnmarshal verifies an kubernetes server resource can be unmarshaled.
func TestKubernetesServerUnmarshal(t *testing.T) {
	expected, err := types.NewKubernetesServerV3(types.Metadata{
		Name:        "test-kube",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.KubernetesServerSpecV3{
		Version:  "v3",
		HostID:   "host_id",
		Hostname: "host",
		Cluster: &types.KubernetesClusterV3{
			Metadata: types.Metadata{
				Name:        "test-cluster",
				Description: "Test description",
				Labels:      map[string]string{"env": "dev"},
			},
		},
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(kubeServerYAML))
	require.NoError(t, err)
	actual, err := UnmarshalKubeServer(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestKubernetesServernMarshal verifies a marshaled kubernetes resource resource can be unmarshaled back.
func TestKubernetesServerMarshal(t *testing.T) {
	expected, err := types.NewKubernetesServerV3(types.Metadata{
		Name:        "test-kube",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.KubernetesServerSpecV3{
		Version:  "v3",
		HostID:   "host_id",
		Hostname: "host",
		Cluster: &types.KubernetesClusterV3{
			Metadata: types.Metadata{
				Name:        "test-cluster",
				Description: "Test description",
				Labels:      map[string]string{"env": "dev"},
			},
		},
	})
	require.NoError(t, err)
	data, err := MarshalKubeServer(expected)
	require.NoError(t, err)
	actual, err := UnmarshalKubeServer(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var kubeServerYAML = `---
kind: kube_server
version: v3
metadata:
  name: test-kube
  description: Test description
  labels:
    env: dev
spec:
  version: v3
  hostname: 'host'
  host_id: host_id
  rotation:
    current_id: ''
    started: '0001-01-01T00:00:00Z'
    last_rotated: '0001-01-01T00:00:00Z'
    schedule:
      update_clients: '0001-01-01T00:00:00Z'
      update_servers: '0001-01-01T00:00:00Z'
      standby: '0001-01-01T00:00:00Z'
  cluster:
    kind: kube_cluster
    version: v3
    metadata:
      name: test-cluster
      description: Test description
      labels:
        env: dev
    spec: {}
`
