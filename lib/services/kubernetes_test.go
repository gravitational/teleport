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

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
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

func TestNewKubeClusterFromAWSEKS(t *testing.T) {
	for _, overrideLabel := range types.AWSKubeClusterNameOverrideLabels {
		t.Run("with name override via "+overrideLabel, func(t *testing.T) {
			expected, err := types.NewKubernetesClusterV3(types.Metadata{
				Name:        "override-1",
				Description: `AWS EKS cluster "cluster1" in eu-west-1`,
				Labels: map[string]string{
					types.DiscoveryLabelAccountID: "123456789012",
					types.DiscoveryLabelRegion:    "eu-west-1",
					types.CloudLabel:              types.CloudAWS,
					types.DiscoveryLabelAWSArn:    "arn:aws:eks:eu-west-1:123456789012:cluster/cluster1",
					overrideLabel:                 "override-1",
					"env":                         "prod",
				},
			}, types.KubernetesClusterSpecV3{
				AWS: types.KubeAWS{
					Name:      "cluster1",
					Region:    "eu-west-1",
					AccountID: "123456789012",
				},
			})
			require.NoError(t, err)

			cluster := &eks.Cluster{
				Name:   aws.String("cluster1"),
				Arn:    aws.String("arn:aws:eks:eu-west-1:123456789012:cluster/cluster1"),
				Status: aws.String(eks.ClusterStatusActive),
				Tags: map[string]*string{
					overrideLabel: aws.String("override-1"),
					"env":         aws.String("prod"),
				},
			}
			actual, err := NewKubeClusterFromAWSEKS(aws.StringValue(cluster.Name), aws.StringValue(cluster.Arn), cluster.Tags)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
			require.NoError(t, err)
			require.True(t, actual.IsAWS())
			require.False(t, actual.IsAzure())
			require.False(t, actual.IsGCP())
		})
	}
}

func TestNewKubeClusterFromAzureAKS(t *testing.T) {
	overrideLabel := types.AzureKubeClusterNameOverrideLabel
	expected, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:        "override-1",
		Description: `Azure AKS cluster "aks-cluster1" in uswest1`,
		Labels: map[string]string{
			types.DiscoveryLabelRegion:              "uswest1",
			types.DiscoveryLabelAzureResourceGroup:  "group1",
			types.DiscoveryLabelAzureSubscriptionID: "subID",
			types.CloudLabel:                        types.CloudAzure,
			overrideLabel:                           "override-1",
			"env":                                   "prod",
		},
	}, types.KubernetesClusterSpecV3{
		Azure: types.KubeAzure{
			ResourceName:   "aks-cluster1",
			ResourceGroup:  "group1",
			TenantID:       "tenantID",
			SubscriptionID: "subID",
		},
	})
	require.NoError(t, err)

	cluster := &azure.AKSCluster{
		Name:           "aks-cluster1",
		GroupName:      "group1",
		TenantID:       "tenantID",
		Location:       "uswest1",
		SubscriptionID: "subID",
		Tags: map[string]string{
			"env":         "prod",
			overrideLabel: "override-1",
		},
		Properties: azure.AKSClusterProperties{},
	}
	actual, err := NewKubeClusterFromAzureAKS(cluster)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
	require.NoError(t, err)
	require.True(t, actual.IsAzure())
	require.False(t, actual.IsGCP())
	require.False(t, actual.IsAWS())
}

func TestNewKubeClusterFromGCPGKE(t *testing.T) {
	overrideLabel := types.GCPKubeClusterNameOverrideLabel
	expected, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:        "override-1",
		Description: "desc1",
		Labels: map[string]string{
			types.DiscoveryLabelGCPLocation:  "central-1",
			types.DiscoveryLabelGCPProjectID: "p1",
			types.CloudLabel:                 types.CloudGCP,
			overrideLabel:                    "override-1",
			"env":                            "prod",
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
		Name:   "cluster1",
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			overrideLabel: "override-1",
			"env":         "prod",
		},
		ProjectID:   "p1",
		Location:    "central-1",
		Description: "desc1",
	}
	actual, err := NewKubeClusterFromGCPGKE(cluster)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
	require.NoError(t, err)
	require.True(t, actual.IsGCP())
	require.False(t, actual.IsAzure())
	require.False(t, actual.IsAWS())
}

func TestNewKubeClusterFromGCPGKEWithoutLabels(t *testing.T) {
	expected, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:        "cluster1",
		Description: "desc1",
		Labels: map[string]string{
			types.DiscoveryLabelGCPLocation:  "central-1",
			types.DiscoveryLabelGCPProjectID: "p1",
			types.CloudLabel:                 types.CloudGCP,
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
