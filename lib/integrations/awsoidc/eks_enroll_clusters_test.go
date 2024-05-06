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
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGetChartUrl(t *testing.T) {
	testCases := []struct {
		version  string
		expected string
	}{
		{
			version:  "14.3.3",
			expected: "https://charts.releases.teleport.dev/teleport-kube-agent-14.3.3.tgz",
		},
		{
			version:  "15.0.2",
			expected: "https://charts.releases.teleport.dev/teleport-kube-agent-15.0.2.tgz",
		},
		{
			version:  "15.0.0-alpha.5",
			expected: "https://charts.releases.teleport.dev/teleport-kube-agent-15.0.0-alpha.5.tgz",
		},
	}

	for _, tt := range testCases {
		require.Equal(t, tt.expected, getChartURL(tt.version).String())
	}
}

func TestEnrollEKSClusters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	proxyAddr := "https://example.com"
	credsProvider := &mockCredentialsProvider{}
	awsAccountID := "880713328506"
	awsUserID := "AIDAJQABLZS4A3QDU576Q"
	clustersBaseArn := "arn:aws:eks:us-east-1:880713328506:cluster/EKS"
	roleArn := "arn:aws:sts::880713328506:assumed-role/TeleportRole/1404549515185351000"
	testCAData := "VGVzdENBREFUQQ=="

	testEKSClusters := []eksTypes.Cluster{
		{
			Name:                 aws.String("EKS1"),
			Arn:                  aws.String(clustersBaseArn + "1"),
			Tags:                 map[string]string{"label1": "value1"},
			CertificateAuthority: &eksTypes.Certificate{Data: aws.String(testCAData)},
			Status:               eksTypes.ClusterStatusActive,
		},
		{
			Name:                 aws.String("EKS2"),
			Arn:                  aws.String(clustersBaseArn + "2"),
			Tags:                 map[string]string{"label2": "value2"},
			CertificateAuthority: &eksTypes.Certificate{Data: aws.String(testCAData)},
			Status:               eksTypes.ClusterStatusActive,
		},
	}

	baseClient := func(t *testing.T, clusters []eksTypes.Cluster) EnrollEKSCLusterClient {
		clt := &mockEnrollEKSClusterClient{}
		clt.describeCluster = func(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
			for _, cluster := range clusters {
				if *params.Name == *cluster.Name {
					return &eks.DescribeClusterOutput{
						Cluster:        &cluster,
						ResultMetadata: middleware.Metadata{},
					}, nil
				}
			}

			return nil, trace.NotFound("cluster not found")
		}
		clt.getCallerIdentity = func(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
			return &sts.GetCallerIdentityOutput{
				Account:        aws.String(awsAccountID),
				Arn:            aws.String(roleArn),
				UserId:         aws.String(awsUserID),
				ResultMetadata: middleware.Metadata{},
			}, nil
		}

		return clt
	}
	baseRequest := EnrollEKSClustersRequest{
		Region:             "us-east-1",
		AgentVersion:       "1.2.3",
		EnableAppDiscovery: true,
	}

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))

	testCases := []struct {
		name                string
		enrollClient        func(*testing.T, []eksTypes.Cluster) EnrollEKSCLusterClient
		eksClusters         []eksTypes.Cluster
		request             EnrollEKSClustersRequest
		requestClusterNames []string
		responseCheck       func(*testing.T, *EnrollEKSClusterResponse)
	}{
		{
			name:                "one cluster, success",
			enrollClient:        baseClient,
			eksClusters:         testEKSClusters[:1],
			request:             baseRequest,
			requestClusterNames: []string{"EKS1"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 1)
				require.Equal(t, "EKS1", response.Results[0].ClusterName)
				require.Empty(t, response.Results[0].Error)
				require.NotEmpty(t, response.Results[0].ResourceId)
			},
		},
		{
			name:                "two clusters, success",
			enrollClient:        baseClient,
			eksClusters:         testEKSClusters[:2],
			request:             baseRequest,
			requestClusterNames: []string{"EKS1", "EKS2"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 2)
				slices.SortFunc(response.Results, func(a, b EnrollEKSClusterResult) int {
					return strings.Compare(a.ClusterName, b.ClusterName)
				})
				require.Equal(t, "EKS1", response.Results[0].ClusterName)
				require.Empty(t, response.Results[0].Error)
				require.NotEmpty(t, response.Results[0].ResourceId)
				require.Equal(t, "EKS2", response.Results[1].ClusterName)
				require.Empty(t, response.Results[1].Error)
				require.NotEmpty(t, response.Results[1].ResourceId)
			},
		},
		{
			name:                "one cluster, not found",
			enrollClient:        baseClient,
			eksClusters:         testEKSClusters[:2],
			request:             baseRequest,
			requestClusterNames: []string{"EKS3"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 1)
				require.Equal(t, "EKS3", response.Results[0].ClusterName)
				require.ErrorContains(t, response.Results[0].Error, "cluster not found")
			},
		},
		{
			name:                "two clusters, one success, one error",
			enrollClient:        baseClient,
			eksClusters:         testEKSClusters[:2],
			request:             baseRequest,
			requestClusterNames: []string{"EKS1", "EKS3"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 2)
				for _, result := range response.Results {
					if result.ClusterName == "EKS1" {
						require.NoError(t, result.Error, "cluster not found")
					} else if result.ClusterName == "EKS3" {
						require.ErrorContains(t, result.Error, "cluster not found")
					} else {
						require.Fail(t, "unexpected cluster present in the response")
					}
				}
			},
		},
		{
			name:         "inactive cluster is not enrolled",
			enrollClient: baseClient,
			eksClusters: []eksTypes.Cluster{
				{
					Name:                 aws.String("EKS1"),
					Arn:                  aws.String(clustersBaseArn + "1"),
					Tags:                 map[string]string{"label1": "value1"},
					CertificateAuthority: &eksTypes.Certificate{Data: aws.String(testCAData)},
					Status:               eksTypes.ClusterStatusPending,
				},
			},
			request:             baseRequest,
			requestClusterNames: []string{"EKS1"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 1)
				require.ErrorContains(t, response.Results[0].Error,
					`can't enroll EKS cluster "EKS1" - expected "ACTIVE" state, got "PENDING".`)
			},
		},
		{
			name: "cluster with already present agent is not enrolled",
			enrollClient: func(t *testing.T, clusters []eksTypes.Cluster) EnrollEKSCLusterClient {
				clt := baseClient(t, clusters)
				mockClt, ok := clt.(*mockEnrollEKSClusterClient)
				require.True(t, ok)
				mockClt.checkAgentAlreadyInstalled = func(ctx context.Context, getter genericclioptions.RESTClientGetter, logger *slog.Logger) (bool, error) {
					return true, nil
				}
				return mockClt
			},
			eksClusters:         testEKSClusters,
			request:             baseRequest,
			requestClusterNames: []string{"EKS1"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 1)
				require.ErrorContains(t, response.Results[0].Error,
					`teleport-kube-agent is already installed on the cluster "EKS1"`)
			},
		},
		{
			name: "if access entry is already present we don't create another one and don't delete it",
			enrollClient: func(t *testing.T, clusters []eksTypes.Cluster) EnrollEKSCLusterClient {
				clt := baseClient(t, clusters)
				mockClt, ok := clt.(*mockEnrollEKSClusterClient)
				require.True(t, ok)
				mockClt.listAccessEntries = func(ctx context.Context, input *eks.ListAccessEntriesInput, f ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error) {
					return &eks.ListAccessEntriesOutput{
						AccessEntries: []string{"arn:aws:iam::880713328506:role/TeleportRole"},
					}, nil
				}
				mockClt.createAccessEntry = func(ctx context.Context, input *eks.CreateAccessEntryInput, f ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error) {
					require.Fail(t, "CreateAccessEntry shouldn't not have been called.")
					return nil, nil
				}
				mockClt.deleteAccessEntry = func(ctx context.Context, input *eks.DeleteAccessEntryInput, f ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error) {
					require.Fail(t, "DeleteAccessEntry shouldn't not have been called.")
					return nil, nil
				}
				return mockClt
			},
			eksClusters:         testEKSClusters,
			request:             baseRequest,
			requestClusterNames: []string{"EKS1"},
			responseCheck: func(t *testing.T, response *EnrollEKSClusterResponse) {
				require.Len(t, response.Results, 1)
				require.NoError(t, response.Results[0].Error)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.request
			if len(req.ClusterNames) == 0 {
				req.ClusterNames = tc.requestClusterNames
			}

			response, err := EnrollEKSClusters(
				ctx, utils.NewSlogLoggerForTests().With("test", t.Name()), clock, proxyAddr, credsProvider, tc.enrollClient(t, tc.eksClusters), req)
			require.NoError(t, err)

			tc.responseCheck(t, response)
		})
	}

	t.Run("CreateAccessEntry and DeleteAccessEntry are called if there wasn't existing entry for Teleport", func(t *testing.T) {
		req := baseRequest
		req.ClusterNames = []string{"EKS1"}
		client := baseClient(t, testEKSClusters)
		mockClt, ok := client.(*mockEnrollEKSClusterClient)
		require.True(t, ok)
		createCalled, deleteCalled := false, false
		mockClt.createAccessEntry = func(ctx context.Context, input *eks.CreateAccessEntryInput, f ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error) {
			createCalled = true
			return nil, nil
		}
		mockClt.deleteAccessEntry = func(ctx context.Context, input *eks.DeleteAccessEntryInput, f ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error) {
			deleteCalled = true
			return nil, nil
		}

		response, err := EnrollEKSClusters(
			ctx, utils.NewSlogLoggerForTests().With("test", t.Name()), clock, proxyAddr, credsProvider, mockClt, req)
		require.NoError(t, err)
		require.Len(t, response.Results, 1)
		require.Equal(t, "EKS1", response.Results[0].ClusterName)
		require.True(t, createCalled)
		require.True(t, deleteCalled)
	})
}

func TestGetKubeClientGetter(t *testing.T) {
	credsProvider := &mockCredentialsProvider{}
	testCAData := "VGVzdENBREFUQQ=="

	testCases := []struct {
		endpoint      string
		region        string
		caData        string
		timestamp     time.Time
		expectedToken string
		errorCheck    require.ErrorAssertionFunc
	}{
		{
			endpoint:      "https://test.example.com",
			region:        "us-east-1",
			caData:        testCAData,
			timestamp:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedToken: "k8s-aws-v1.aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8_QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNSZYLUFtei1BbGdvcml0aG09QVdTNC1ITUFDLVNIQTI1NiZYLUFtei1DcmVkZW50aWFsPSUyRjIwMjMwMTAxJTJGdXMtZWFzdC0xJTJGc3RzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyMzAxMDFUMDAwMDAwWiZYLUFtei1FeHBpcmVzPTYwJlgtQW16LVNpZ25lZEhlYWRlcnM9aG9zdCUzQngtazhzLWF3cy1pZCZYLUFtei1TaWduYXR1cmU9N2RiNWZlMWNjNDBhNzc4MTQ2MWJkMzkzMmE2NzBhNmVhNDFhNDNlZjEwZWQxZjMzNDE3NjI1ZDhkMTQ1Njg4NQ",
		},
		{
			endpoint:      "https://test2.example.com",
			region:        "us-east-1",
			caData:        testCAData,
			timestamp:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedToken: "k8s-aws-v1.aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8_QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNSZYLUFtei1BbGdvcml0aG09QVdTNC1ITUFDLVNIQTI1NiZYLUFtei1DcmVkZW50aWFsPSUyRjIwMjMwMTAxJTJGdXMtZWFzdC0xJTJGc3RzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyMzAxMDFUMDAwMDAwWiZYLUFtei1FeHBpcmVzPTYwJlgtQW16LVNpZ25lZEhlYWRlcnM9aG9zdCUzQngtazhzLWF3cy1pZCZYLUFtei1TaWduYXR1cmU9N2RiNWZlMWNjNDBhNzc4MTQ2MWJkMzkzMmE2NzBhNmVhNDFhNDNlZjEwZWQxZjMzNDE3NjI1ZDhkMTQ1Njg4NQ",
		},
		{
			endpoint:      "https://test.example.com",
			region:        "us-east-2",
			caData:        testCAData,
			timestamp:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedToken: "k8s-aws-v1.aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8_QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNSZYLUFtei1BbGdvcml0aG09QVdTNC1ITUFDLVNIQTI1NiZYLUFtei1DcmVkZW50aWFsPSUyRjIwMjMwMTAxJTJGdXMtZWFzdC0yJTJGc3RzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyMzAxMDFUMDAwMDAwWiZYLUFtei1FeHBpcmVzPTYwJlgtQW16LVNpZ25lZEhlYWRlcnM9aG9zdCUzQngtazhzLWF3cy1pZCZYLUFtei1TaWduYXR1cmU9ZmE2MGNiYjRlMGNkYTNhMDU5ZmRlNWMwNjgzMGZlYTc4NTVkNTcwMTg4MWE4MzMzNmIwYjg0MjliYWIyMGYwNQ",
		},
		{
			endpoint:      "https://test.example.com",
			region:        "us-east-1",
			caData:        testCAData,
			timestamp:     time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedToken: "k8s-aws-v1.aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8_QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNSZYLUFtei1BbGdvcml0aG09QVdTNC1ITUFDLVNIQTI1NiZYLUFtei1DcmVkZW50aWFsPSUyRjIwMjIwMTAxJTJGdXMtZWFzdC0xJTJGc3RzJTJGYXdzNF9yZXF1ZXN0JlgtQW16LURhdGU9MjAyMjAxMDFUMDAwMDAwWiZYLUFtei1FeHBpcmVzPTYwJlgtQW16LVNpZ25lZEhlYWRlcnM9aG9zdCUzQngtazhzLWF3cy1pZCZYLUFtei1TaWduYXR1cmU9YmE3ZTA5ZTcyZTUzNDIzNjRiNDZlZmQzOTViNmU1MjU5YWU1YmFjMTY1YmViYWVmNDMyZDA5MmVmZWI0ZDBkOA",
		},
		{
			endpoint:      "https://test.example.com",
			region:        "us-east-1",
			caData:        "badCA",
			timestamp:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedToken: "",
			errorCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "illegal base64 data")
			},
		},
	}

	for c, tc := range testCases {
		t.Run(fmt.Sprintf("test#%d", c), func(t *testing.T) {
			config, err := getKubeClientGetter(context.Background(), tc.timestamp, credsProvider, "EKS1", tc.region, tc.caData, tc.endpoint)

			if tc.errorCheck == nil {
				cfg, err := config.ToRESTConfig()
				require.NoError(t, err)
				require.NoError(t, err)
				require.Equal(t, tc.expectedToken, cfg.BearerToken)
				require.Equal(t, tc.endpoint, cfg.Host)
			} else {
				tc.errorCheck(t, err)
			}
		})
	}
}

func TestGetAccessEntryPrincipalArn(t *testing.T) {
	awsAccountID := "880713328506"
	awsUserID := "AIDAJQABLZS4A3QDU576Q"

	testCases := []struct {
		identity sts.GetCallerIdentityOutput
		expected string
	}{
		{
			identity: sts.GetCallerIdentityOutput{
				Account:        aws.String(awsAccountID),
				Arn:            aws.String("arn:aws:sts::880713328506:assumed-role/TeleportRole/1404549515185351000"),
				UserId:         aws.String(awsUserID),
				ResultMetadata: middleware.Metadata{},
			},
			expected: "arn:aws:iam::880713328506:role/TeleportRole",
		},
		{
			identity: sts.GetCallerIdentityOutput{
				Account:        aws.String(awsAccountID),
				Arn:            aws.String("arn:aws:sts::880713328506:role/TeleportRole"),
				UserId:         aws.String(awsUserID),
				ResultMetadata: middleware.Metadata{},
			},
			expected: "arn:aws:iam::880713328506:role/TeleportRole",
		},
		{
			identity: sts.GetCallerIdentityOutput{
				Account:        aws.String(awsAccountID),
				Arn:            aws.String("arn:aws:iam::880713328506:role/TeleportRole"),
				UserId:         aws.String(awsUserID),
				ResultMetadata: middleware.Metadata{},
			},
			expected: "arn:aws:iam::880713328506:role/TeleportRole",
		},
	}

	for _, tc := range testCases {
		i := func(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
			return &tc.identity, nil
		}

		result, err := getAccessEntryPrincipalArn(context.Background(), i)
		require.NoError(t, err)
		require.Equal(t, tc.expected, result)
	}
}

// mockCredentialsProvider mocks AWS credentials.Provider interface.
type mockCredentialsProvider struct {
	retrieveValue aws.Credentials
	retrieveError error
}

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return m.retrieveValue, m.retrieveError
}

type mockEnrollEKSClusterClient struct {
	createAccessEntry          func(context.Context, *eks.CreateAccessEntryInput, ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error)
	associateAccessPolicy      func(context.Context, *eks.AssociateAccessPolicyInput, ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)
	listAccessEntries          func(context.Context, *eks.ListAccessEntriesInput, ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error)
	deleteAccessEntry          func(context.Context, *eks.DeleteAccessEntryInput, ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error)
	describeCluster            func(context.Context, *eks.DescribeClusterInput, ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	getCallerIdentity          func(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	checkAgentAlreadyInstalled func(context.Context, genericclioptions.RESTClientGetter, *slog.Logger) (bool, error)
	installKubeAgent           func(context.Context, *eksTypes.Cluster, string, string, string, genericclioptions.RESTClientGetter, *slog.Logger, EnrollEKSClustersRequest) error
	createToken                func(ctx context.Context, token types.ProvisionToken) error
}

func (m *mockEnrollEKSClusterClient) CreateAccessEntry(ctx context.Context, params *eks.CreateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error) {
	if m.createAccessEntry != nil {
		return m.createAccessEntry(ctx, params, optFns...)
	}
	return &eks.CreateAccessEntryOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) AssociateAccessPolicy(ctx context.Context, params *eks.AssociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error) {
	if m.associateAccessPolicy != nil {
		return m.associateAccessPolicy(ctx, params, optFns...)
	}
	return &eks.AssociateAccessPolicyOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) ListAccessEntries(ctx context.Context, params *eks.ListAccessEntriesInput, optFns ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error) {
	if m.listAccessEntries != nil {
		return m.listAccessEntries(ctx, params, optFns...)
	}
	return &eks.ListAccessEntriesOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) DeleteAccessEntry(ctx context.Context, params *eks.DeleteAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error) {
	if m.deleteAccessEntry != nil {
		return m.deleteAccessEntry(ctx, params, optFns...)
	}
	return &eks.DeleteAccessEntryOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	if m.describeCluster != nil {
		return m.describeCluster(ctx, params, optFns...)
	}
	return &eks.DescribeClusterOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if m.getCallerIdentity != nil {
		return m.getCallerIdentity(ctx, params, optFns...)
	}
	return &sts.GetCallerIdentityOutput{}, nil
}

func (m *mockEnrollEKSClusterClient) CheckAgentAlreadyInstalled(ctx context.Context, kubeconfig genericclioptions.RESTClientGetter, log *slog.Logger) (bool, error) {
	if m.checkAgentAlreadyInstalled != nil {
		return m.checkAgentAlreadyInstalled(ctx, kubeconfig, log)
	}
	return false, nil
}

func (m *mockEnrollEKSClusterClient) InstallKubeAgent(ctx context.Context, eksCluster *eksTypes.Cluster, proxyAddr, joinToken, resourceId string, kubeconfig genericclioptions.RESTClientGetter, log *slog.Logger, req EnrollEKSClustersRequest) error {
	if m.installKubeAgent != nil {
		return m.installKubeAgent(ctx, eksCluster, proxyAddr, joinToken, resourceId, kubeconfig, log, req)
	}
	return nil
}

func (m *mockEnrollEKSClusterClient) CreateToken(ctx context.Context, token types.ProvisionToken) error {
	if m.createToken != nil {
		return m.createToken(ctx, token)
	}
	return nil
}

var _ EnrollEKSCLusterClient = &mockEnrollEKSClusterClient{}
