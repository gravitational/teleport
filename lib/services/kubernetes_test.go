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

package services

import (
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
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

func TestNewKubeClusterFromGCPGKEWithoutLabels(t *testing.T) {
	expected, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:        "cluster1",
		Description: "desc1",
		Labels: map[string]string{
			labelLocation:     "central-1",
			labelProjectID:    "p1",
			types.CloudLabel:  types.CloudGCP,
			types.OriginLabel: types.OriginCloud,
		},
	}, types.KubernetesClusterSpecV3{
		GCP: types.KubeGCP{
			Name:      "cluster1",
			ProjectID: "p1",
			Location:  "central-1",
		},
	})
	require.NoError(t, err)

	cluster := gcp.GKECluster{
		Name:        "cluster1",
		Status:      containerpb.Cluster_RUNNING,
		Labels:      nil,
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	}
	actual, err := NewKubeClusterFromGCPGKE(cluster)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
	require.True(t, actual.IsGCP())
	require.False(t, actual.IsAzure())
	require.False(t, actual.IsAWS())
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
