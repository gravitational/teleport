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
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
)

// ListEKSClustersRequest contains the required fields to list AWS EKS Clusters.
type ListEKSClustersRequest struct {
	// Region is the AWS Region.
	Region string

	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListEKSClustersRequest) CheckAndSetDefaults() error {
	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	return nil
}

// EKSCluster represents a cluster in AWS EKS.
type EKSCluster struct {
	// Name is the name of AWS EKS cluster.
	Name string

	// Region is an AWS region.
	Region string

	// Arn is an AWS ARN identification of the EKS cluster.
	Arn string

	// Labels are labels of a EKS cluster.
	Labels map[string]string

	// JoinLabels are Teleport labels that should be injected into kube agent
	// if the cluster will be enrolled into Teleport (agent installed on it).
	JoinLabels map[string]string

	// Status is a current status of an EKS cluster in AWS.
	Status string

	// AuthenticationMode contains the authentication mode of the cluster.
	// Expected values are: API, API_AND_CONFIG_MAP, CONFIG_MAP.
	// Only API and API_AND_CONFIG_MAP are supported when installing the Teleport Helm chart.
	// https://aws.amazon.com/blogs/containers/a-deep-dive-into-simplified-amazon-eks-access-management-controls/
	AuthenticationMode string

	// EndpointPublicAccess indicates whether the Cluster's VPC Config has its endpoint as a public address.
	// For Teleport Cloud, this is required to access the cluster and proceed with the installation.
	EndpointPublicAccess bool
}

// ListEKSClustersResponse contains a page of AWS EKS Clusters.
type ListEKSClustersResponse struct {
	// Servers contains the page of Servers.
	Clusters []EKSCluster

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string

	// ClusterFetchingErrors contains errors for fetching detailed information about specific cluster, if any happened.
	ClusterFetchingErrors map[string]error
}

// NewListEKSClustersClient creates a new ListEKSClusters client using AWSClientRequest.
func NewListEKSClustersClient(ctx context.Context, req *AWSClientRequest) (ListEKSClustersClient, error) {
	clt, err := newEKSClient(ctx, req)
	return clt, trace.Wrap(err)
}

// ListEKSClustersClient describes the required methods to List EKS clusters using a 3rd Party API.
type ListEKSClustersClient interface {
	// ListClusters lists the EKS clusters.
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)

	// DescribeCluster returns detailed information about an EKS cluster.
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// concurrentEKSFetchingLimit is a limit of how many clusters we are trying to describe concurrently, after receiving a list of clusters.
const concurrentEKSFetchingLimit = 5

// ListEKSClusters calls the following AWS API:
// https://docs.aws.amazon.com/eks/latest/APIReference/API_ListClusters.html - to list available EKS clusters
// https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html - to get more detailed information about
// the each cluster in the list we received.
// It returns a list of EKS clusters with detailed information about them.
func ListEKSClusters(ctx context.Context, clt ListEKSClustersClient, req ListEKSClustersRequest) (*ListEKSClustersResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	listEKSClusters := &eks.ListClustersInput{}
	if req.NextToken != "" {
		listEKSClusters.NextToken = &req.NextToken
	}
	eksClusters, err := clt.ListClusters(ctx, listEKSClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var mu sync.Mutex
	ret := &ListEKSClustersResponse{
		NextToken:             aws.ToString(eksClusters.NextToken),
		ClusterFetchingErrors: map[string]error{},
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrentEKSFetchingLimit)

	ret.Clusters = make([]EKSCluster, 0, len(eksClusters.Clusters))
	for _, clusterName := range eksClusters.Clusters {
		clusterName := clusterName
		if clusterName == "" {
			continue
		}
		group.Go(func() error {
			eksClusterInfo, err := clt.DescribeCluster(groupCtx, &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				ret.ClusterFetchingErrors[clusterName] = err
				return nil
			}

			cluster := eksClusterInfo.Cluster

			extraLabels, err := getExtraEKSLabels(cluster)
			if err != nil {
				ret.ClusterFetchingErrors[clusterName] = err
				return nil
			}

			ret.Clusters = append(ret.Clusters, EKSCluster{
				Name:                 aws.ToString(cluster.Name),
				Region:               req.Region,
				Arn:                  aws.ToString(cluster.Arn),
				Labels:               cluster.Tags,
				JoinLabels:           extraLabels,
				Status:               strings.ToLower(string(cluster.Status)),
				AuthenticationMode:   string(cluster.AccessConfig.AuthenticationMode),
				EndpointPublicAccess: cluster.ResourcesVpcConfig.EndpointPublicAccess,
			})
			return nil
		})
	}

	// We don't return error from individual group goroutines, they are gathered in the returned value.
	_ = group.Wait()

	return ret, nil
}

func getExtraEKSLabels(cluster *eksTypes.Cluster) (map[string]string, error) {
	parsedARN, err := arn.Parse(aws.ToString(cluster.Arn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return map[string]string{
		types.CloudLabel:              types.CloudAWS,
		types.DiscoveryLabelAccountID: parsedARN.AccountID,
		types.DiscoveryLabelRegion:    parsedARN.Region,
	}, nil
}
