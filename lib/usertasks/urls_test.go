/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package usertasks

import (
	"testing"

	"github.com/stretchr/testify/require"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
)

func TestEKSURLs(t *testing.T) {
	clusterName := "my-cluster"
	dummyCluster := &usertasksv1.DiscoverEKSCluster{Name: clusterName}
	baseClusterData := &usertasksv1.DiscoverEKS{
		Region: "us-east-1",
		Clusters: map[string]*usertasksv1.DiscoverEKSCluster{
			clusterName: dummyCluster,
		},
	}

	for _, tt := range []struct {
		name                      string
		issueType                 string
		expectedEKSClusterWithURL *DiscoverEKSClusterWithURLs
		expected                  *UserTaskDiscoverEKSWithURLs
	}{
		{
			name:      "url for eks agent not connecting",
			issueType: usertasksapi.AutoDiscoverEKSIssueAgentNotConnecting,
			expectedEKSClusterWithURL: &DiscoverEKSClusterWithURLs{
				ResourceURL:          "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
				OpenTeleportAgentURL: "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster/statefulsets/teleport-kube-agent?namespace=teleport-agent",
			},
		},
		{
			name:      "url for eks authentication mode unsupported",
			issueType: usertasksapi.AutoDiscoverEKSIssueAuthenticationModeUnsupported,
			expectedEKSClusterWithURL: &DiscoverEKSClusterWithURLs{
				ResourceURL:     "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
				ManageAccessURL: "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster/manage-access",
			},
		},
		{
			name:      "url for eks cluster unreachable",
			issueType: usertasksapi.AutoDiscoverEKSIssueClusterUnreachable,
			expectedEKSClusterWithURL: &DiscoverEKSClusterWithURLs{
				ResourceURL:             "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
				ManageEndpointAccessURL: "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster/manage-endpoint-access",
			},
		},
		{
			name:      "url for eks missing endpoint public access",
			issueType: usertasksapi.AutoDiscoverEKSIssueMissingEndpoingPublicAccess,
			expectedEKSClusterWithURL: &DiscoverEKSClusterWithURLs{
				ResourceURL:             "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
				ManageEndpointAccessURL: "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster/manage-endpoint-access",
			},
		},
		{
			name:      "url for eks cluster status not active",
			issueType: usertasksapi.AutoDiscoverEKSIssueStatusNotActive,
			expectedEKSClusterWithURL: &DiscoverEKSClusterWithURLs{
				ResourceURL:      "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
				ManageClusterURL: "https://console.aws.amazon.com/eks/home?region=us-east-1#/clusters/my-cluster",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clusterWithURL := tt.expectedEKSClusterWithURL
			clusterWithURL.DiscoverEKSCluster = dummyCluster
			expected := &UserTaskDiscoverEKSWithURLs{
				DiscoverEKS: baseClusterData,
				Clusters: map[string]*DiscoverEKSClusterWithURLs{
					clusterName: clusterWithURL,
				},
			}

			got := EKSClustersWithURLs(&usertasksv1.UserTask{
				Spec: &usertasksv1.UserTaskSpec{
					IssueType:   tt.issueType,
					DiscoverEks: baseClusterData,
				},
			})
			require.Equal(t, expected, got)
		})
	}
}

func TestEC2URLs(t *testing.T) {
	instanceID := "i-12345678"
	dummyInstance := &usertasksv1.DiscoverEC2Instance{InstanceId: instanceID}
	baseInstancesData := &usertasksv1.DiscoverEC2{
		Region: "us-east-1",
		Instances: map[string]*usertasksv1.DiscoverEC2Instance{
			instanceID: dummyInstance,
		},
	}

	for _, tt := range []struct {
		name                       string
		issueType                  string
		expectedEC2InstanceWithURL *DiscoverEC2InstanceWithURLs
		expected                   *UserTaskDiscoverEC2WithURLs
	}{
		{
			name:      "url for ec2 resource",
			issueType: usertasksapi.AutoDiscoverEC2IssueSSMScriptFailure,
			expectedEC2InstanceWithURL: &DiscoverEC2InstanceWithURLs{
				ResourceURL: "https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-12345678",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			instanceWithURL := tt.expectedEC2InstanceWithURL
			instanceWithURL.DiscoverEC2Instance = dummyInstance
			expected := &UserTaskDiscoverEC2WithURLs{
				DiscoverEC2: baseInstancesData,
				Instances: map[string]*DiscoverEC2InstanceWithURLs{
					instanceID: instanceWithURL,
				},
			}

			got := EC2InstancesWithURLs(&usertasksv1.UserTask{
				Spec: &usertasksv1.UserTaskSpec{
					IssueType:   tt.issueType,
					DiscoverEc2: baseInstancesData,
				},
			})
			require.Equal(t, expected, got)
		})
	}
}

func TestRDSURLs(t *testing.T) {
	databaseName := "my-database"
	dummyDatabase := &usertasksv1.DiscoverRDSDatabase{Name: databaseName}
	baseDatabaseData := &usertasksv1.DiscoverRDS{
		Region: "us-east-1",
		Databases: map[string]*usertasksv1.DiscoverRDSDatabase{
			databaseName: dummyDatabase,
		},
	}

	for _, tt := range []struct {
		name                       string
		issueType                  string
		expectedRDSDatabaseWithURL *DiscoverRDSDatabaseWithURLs
		expected                   *UserTaskDiscoverRDSWithURLs
	}{
		{
			name:      "url for rds database without IAM Authentication",
			issueType: usertasksapi.AutoDiscoverRDSIssueIAMAuthenticationDisabled,
			expectedRDSDatabaseWithURL: &DiscoverRDSDatabaseWithURLs{
				ResourceURL:      "https://console.aws.amazon.com/rds/home?region=us-east-1#database:id=my-database;is-cluster=false",
				ConfigurationURL: "https://console.aws.amazon.com/rds/home?region=us-east-1#database:id=my-database;is-cluster=false;tab=configuration",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			databaseWithURL := tt.expectedRDSDatabaseWithURL
			databaseWithURL.DiscoverRDSDatabase = dummyDatabase
			expected := &UserTaskDiscoverRDSWithURLs{
				DiscoverRDS: baseDatabaseData,
				Databases: map[string]*DiscoverRDSDatabaseWithURLs{
					databaseName: databaseWithURL,
				},
			}

			got := RDSDatabasesWithURLs(&usertasksv1.UserTask{
				Spec: &usertasksv1.UserTaskSpec{
					IssueType:   tt.issueType,
					DiscoverRds: baseDatabaseData,
				},
			})
			require.Equal(t, expected, got)
		})
	}
}
