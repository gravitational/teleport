/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package awsoidc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockListEKSClustersClient struct {
	pageSize int

	eksClusters []eksTypes.Cluster
}

func (m mockListEKSClustersClient) ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	startPos := 0
	var err error
	if params.NextToken != nil {
		startPos, err = strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, err
		}
	}

	endPos := startPos + m.pageSize
	var nextToken = aws.String(strconv.Itoa(endPos))
	if endPos > len(m.eksClusters) {
		endPos = len(m.eksClusters)
		nextToken = nil
	}

	var clusters []string
	for _, c := range m.eksClusters[startPos:endPos] {
		clusters = append(clusters, *c.Name)
	}

	return &eks.ListClustersOutput{
		Clusters:       clusters,
		NextToken:      nextToken,
		ResultMetadata: middleware.Metadata{},
	}, nil
}

func (m mockListEKSClustersClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	if strings.Contains(*params.Name, "error") {
		return nil, errors.New(*params.Name)
	}

	for _, c := range m.eksClusters {
		if *c.Name == *params.Name {
			return &eks.DescribeClusterOutput{
				Cluster:        &c,
				ResultMetadata: middleware.Metadata{},
			}, nil
		}
	}

	return nil, trace.NotFound("cluster not found")
}

func TestListEKSClusters(t *testing.T) {
	ctx := context.Background()
	region := "us-east-1"
	baseArn := "arn:aws:eks:us-east-1:880713328506:cluster/EKS"

	t.Run("pagination", func(t *testing.T) {
		eksClustersAmount := 203
		allClusters := make([]eksTypes.Cluster, 0, eksClustersAmount)

		for c := 0; c < eksClustersAmount; c++ {
			allClusters = append(allClusters, eksTypes.Cluster{
				Name:   aws.String(fmt.Sprintf("EKS_%d", c)),
				Arn:    aws.String(fmt.Sprintf("%s_%d", baseArn, c)),
				Tags:   map[string]string{"label": "value"},
				Status: "active",
				AccessConfig: &eksTypes.AccessConfigResponse{
					AuthenticationMode: eksTypes.AuthenticationModeApi,
				},
				ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
					EndpointPublicAccess: true,
				},
			})
		}

		pageSize := 100
		mockListClient := mockListEKSClustersClient{
			pageSize:    pageSize,
			eksClusters: allClusters,
		}

		// First call must return pageSize number of clusters from the first page.
		resp, err := ListEKSClusters(ctx, mockListClient, ListEKSClustersRequest{
			Region: region,
		})
		require.NoError(t, err)
		require.Len(t, resp.Clusters, pageSize)
		require.Regexp(t, `EKS_\d\d?`, resp.Clusters[0].Name, "EKS cluster is not from the first page")
		require.NotEmpty(t, resp.NextToken)

		// Second call must also return pageSize number of clusters, from the next page.
		resp, err = ListEKSClusters(ctx, mockListClient, ListEKSClustersRequest{
			Region:    region,
			NextToken: resp.NextToken,
		})
		require.NoError(t, err)
		require.Len(t, resp.Clusters, pageSize)
		require.Regexp(t, `EKS_\d\d\d`, resp.Clusters[0].Name, "EKS cluster is not from the second page")
		require.NotEmpty(t, resp.NextToken)

		// Third call musts return remaining amount of clusters and empty NextToken.
		resp, err = ListEKSClusters(ctx, mockListClient, ListEKSClustersRequest{
			Region:    region,
			NextToken: resp.NextToken,
		})
		require.NoError(t, err)
		require.Len(t, resp.Clusters, eksClustersAmount-2*pageSize)
		require.Empty(t, resp.NextToken)
	})

	testCases := []struct {
		name                   string
		inputEKSClusters       []eksTypes.Cluster
		expectedClusters       []EKSCluster
		expectedFetchingErrors map[string]error
	}{
		{
			name: "success, one cluster",
			inputEKSClusters: []eksTypes.Cluster{
				{
					Arn:    aws.String(baseArn),
					Name:   aws.String("EKS"),
					Status: "active",
					Tags:   map[string]string{"label": "value"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeApi,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
			},
			expectedClusters: []EKSCluster{
				{
					Name:   "EKS",
					Region: region,
					Arn:    baseArn,
					Labels: map[string]string{"label": "value"},
					JoinLabels: map[string]string{
						"account-id":         "880713328506",
						"region":             "us-east-1",
						"teleport.dev/cloud": "AWS",
					},
					Status:               "active",
					AuthenticationMode:   "API",
					EndpointPublicAccess: true,
				},
			},
			expectedFetchingErrors: map[string]error{},
		},
		{
			name: "success, two clusters",
			inputEKSClusters: []eksTypes.Cluster{
				{
					Arn:    aws.String(baseArn),
					Name:   aws.String("EKS"),
					Status: "active",
					Tags:   map[string]string{"label": "value"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeApi,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
				{
					Arn:    aws.String(baseArn + "2"),
					Name:   aws.String("EKS2"),
					Status: "active",
					Tags:   map[string]string{"label2": "value2"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeApi,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
			},
			expectedClusters: []EKSCluster{
				{
					Name:   "EKS",
					Region: region,
					Arn:    baseArn,
					Labels: map[string]string{"label": "value"},
					JoinLabels: map[string]string{
						"account-id":         "880713328506",
						"region":             "us-east-1",
						"teleport.dev/cloud": "AWS",
					},
					Status:               "active",
					AuthenticationMode:   "API",
					EndpointPublicAccess: true,
				},
				{
					Name:   "EKS2",
					Region: region,
					Arn:    baseArn + "2",
					Labels: map[string]string{"label2": "value2"},
					JoinLabels: map[string]string{
						"account-id":         "880713328506",
						"region":             "us-east-1",
						"teleport.dev/cloud": "AWS",
					},
					Status:               "active",
					AuthenticationMode:   "API",
					EndpointPublicAccess: true,
				},
			},
			expectedFetchingErrors: map[string]error{},
		},
		{
			name: "three clusters, one success, two error",
			inputEKSClusters: []eksTypes.Cluster{
				{
					Arn:    aws.String(baseArn),
					Name:   aws.String("EKS"),
					Status: "active",
					Tags:   map[string]string{"label": "value"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeApi,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
				{
					Arn:    aws.String(baseArn),
					Name:   aws.String("erroredCluster"),
					Status: "active",
					Tags:   map[string]string{"label2": "value2"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeApi,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
				{
					Arn:    aws.String(baseArn),
					Name:   aws.String("erroredCluster"),
					Status: "active",
					Tags:   map[string]string{"label2": "value2"},
					AccessConfig: &eksTypes.AccessConfigResponse{
						AuthenticationMode: eksTypes.AuthenticationModeConfigMap,
					},
					ResourcesVpcConfig: &eksTypes.VpcConfigResponse{
						EndpointPublicAccess: true,
					},
				},
			},
			expectedClusters: []EKSCluster{
				{
					Name:   "EKS",
					Region: region,
					Arn:    baseArn,
					Labels: map[string]string{"label": "value"},
					JoinLabels: map[string]string{
						"account-id":         "880713328506",
						"region":             "us-east-1",
						"teleport.dev/cloud": "AWS",
					},
					Status:               "active",
					AuthenticationMode:   "API",
					EndpointPublicAccess: true,
				},
			},
			expectedFetchingErrors: map[string]error{"erroredCluster": errors.New("erroredCluster")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockListClient := mockListEKSClustersClient{
				pageSize:    100,
				eksClusters: tc.inputEKSClusters,
			}

			resp, err := ListEKSClusters(ctx, mockListClient, ListEKSClustersRequest{Region: region})
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.ElementsMatch(t, tc.expectedClusters, resp.Clusters)
			require.Len(t, resp.ClusterFetchingErrors, len(tc.expectedFetchingErrors))
			for clusterName := range tc.expectedFetchingErrors {
				require.Equal(t, tc.expectedFetchingErrors[clusterName], resp.ClusterFetchingErrors[clusterName])
			}
		})
	}
}
