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

package proxy

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/fixtures"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type mockEKSClientGetter struct {
	mocks.AWSConfigProvider
	stsPresignClient *mockSTSPresignAPI
	eksClient        *mockEKSAPI
}

func (e *mockEKSClientGetter) GetAWSEKSClient(aws.Config) EKSClient {
	return e.eksClient
}

func (e *mockEKSClientGetter) GetAWSSTSPresignClient(aws.Config) kubeutils.STSPresignClient {
	return e.stsPresignClient
}

type mockSTSPresignAPI struct {
	url *url.URL
}

func (a *mockSTSPresignAPI) PresignGetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{URL: a.url.String()}, nil
}

type mockEKSAPI struct {
	EKSClient

	notify   chan struct{}
	clusters []*ekstypes.Cluster
}

func (m *mockEKSAPI) ListClusters(ctx context.Context, req *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	defer func() { m.notify <- struct{}{} }()

	var names []string
	for _, cluster := range m.clusters {
		names = append(names, aws.ToString(cluster.Name))
	}
	return &eks.ListClustersOutput{
		Clusters: names,
	}, nil
}

func (m *mockEKSAPI) DescribeCluster(_ context.Context, req *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	defer func() { m.notify <- struct{}{} }()

	for _, cluster := range m.clusters {
		if aws.ToString(cluster.Name) == aws.ToString(req.Name) {
			return &eks.DescribeClusterOutput{
				Cluster: cluster,
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %q not found", aws.ToString(req.Name))
}

// Test_DynamicKubeCreds tests the dynamic kube credrentials generator for
// AWS, GCP, and Azure clusters accessed using their respective IAM credentials.
// This test mocks the cloud provider clients and the STS client to generate
// rest.Config objects for each cluster. It also tests the renewal of the
// credentials when they expire.
func Test_DynamicKubeCreds(t *testing.T) {
	t.Parallel()
	var (
		fakeClock = clockwork.NewFakeClock()
		notify    = make(chan struct{}, 1)
		ttl       = 14 * time.Minute
	)

	awsKube, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "aws",
		},
		types.KubernetesClusterSpecV3{
			AWS: types.KubeAWS{
				Region:    "us-west-2",
				AccountID: "1234567890",
				Name:      "eks",
			},
		},
	)
	require.NoError(t, err)
	gkeKube, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "gke",
		},
		types.KubernetesClusterSpecV3{
			GCP: types.KubeGCP{
				Location:  "us-west-2",
				ProjectID: "1234567890",
				Name:      "gke",
			},
		},
	)
	require.NoError(t, err)
	aksKube, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "aks",
		},
		types.KubernetesClusterSpecV3{
			Azure: types.KubeAzure{
				TenantID:       "id",
				ResourceGroup:  "1234567890",
				ResourceName:   "aks-name",
				SubscriptionID: "12345",
			},
		},
	)
	require.NoError(t, err)

	// Mock sts client.
	u := &url.URL{
		Scheme: "https",
		Host:   "sts.amazonaws.com",
		Path:   "/?Action=GetCallerIdentity&Version=2011-06-15",
	}
	// EKS clients.
	eksClients := &mockEKSClientGetter{
		AWSConfigProvider: mocks.AWSConfigProvider{
			STSClient: &mocks.STSClient{},
		},
		stsPresignClient: &mockSTSPresignAPI{
			// u is used to presign the request
			// here we just verify the pre-signed request includes this url.
			url: u,
		},
		eksClient: &mockEKSAPI{
			notify: notify,
			clusters: []*ekstypes.Cluster{
				{
					Endpoint: aws.String("https://api.eks.us-west-2.amazonaws.com"),
					Name:     aws.String(awsKube.GetAWSConfig().Name),
					CertificateAuthority: &ekstypes.Certificate{
						Data: aws.String(base64.RawStdEncoding.EncodeToString([]byte(fixtures.TLSCACertPEM))),
					},
				},
			},
		},
	}
	// Mock clients.
	cloudclients := &cloud.TestCloudClients{
		GCPGKE: &mocks.GKEMock{
			Notify: notify,
			Clock:  fakeClock,
			Clusters: []mocks.GKEClusterEntry{
				{
					Config: &rest.Config{
						Host: "https://api.gke.google.com",
						TLSClientConfig: rest.TLSClientConfig{
							CAData: []byte(fixtures.TLSCACertPEM),
						},
					},
					ClusterDetails: gcp.ClusterDetails{
						Name:      gkeKube.GetGCPConfig().Name,
						ProjectID: gkeKube.GetGCPConfig().ProjectID,
						Location:  gkeKube.GetGCPConfig().Location,
					},
					TTL: ttl,
				},
			},
		},
		AzureAKSClientPerSub: map[string]azure.AKSClient{
			"12345": &mocks.AKSMock{
				Notify: notify,
				Clock:  fakeClock,
				Clusters: []mocks.AKSClusterEntry{
					{
						Config: &rest.Config{
							Host: "https://api.aks.microsoft.com",
							TLSClientConfig: rest.TLSClientConfig{
								CAData: []byte(fixtures.TLSCACertPEM),
							},
						},
						TTL: ttl,
						ClusterCredentialsConfig: azure.ClusterCredentialsConfig{
							ResourceName:  aksKube.GetAzureConfig().ResourceName,
							ResourceGroup: aksKube.GetAzureConfig().ResourceGroup,
							TenantID:      aksKube.GetAzureConfig().TenantID,
						},
					},
				},
			},
		},
	}
	validateEKSToken := func(token string) error {
		if token == "" {
			return trace.BadParameter("missing bearer token")
		}
		tokens := strings.Split(token, ".")
		if len(tokens) != 2 {
			return trace.BadParameter("invalid bearer token")
		}
		if tokens[0] != "k8s-aws-v1" {
			return trace.BadParameter("token must start with k8s-aws-v1")
		}
		dec, err := base64.RawStdEncoding.DecodeString(tokens[1])
		if err != nil {
			return trace.Wrap(err)
		}
		if string(dec) != u.String() {
			return trace.BadParameter("invalid token payload")
		}
		return nil
	}
	type args struct {
		cluster             types.KubeCluster
		client              dynamicCredsClient
		validateBearerToken func(string) error
	}
	tests := []struct {
		name            string
		args            args
		wantAddr        string
		wantAssumedRole []string
		wantExternalIds []string
	}{
		{
			name: "aws eks cluster without assume role",
			args: args{
				cluster:             awsKube,
				client:              getAWSClientRestConfig(eksClients, fakeClock, nil),
				validateBearerToken: validateEKSToken,
			},
			wantAddr: "api.eks.us-west-2.amazonaws.com:443",
		},
		{
			name: "aws eks cluster with unmatched assume role",
			args: args{
				cluster: awsKube,
				client: getAWSClientRestConfig(eksClients, fakeClock, []services.ResourceMatcher{
					{
						Labels: types.Labels{
							"rand": []string{"value"},
						},
						AWS: services.ResourceMatcherAWS{
							AssumeRoleARN: "arn:aws:iam::123456789012:role/eks-role",
							ExternalID:    "1234567890",
						},
					},
				}),
				validateBearerToken: validateEKSToken,
			},
			wantAddr: "api.eks.us-west-2.amazonaws.com:443",
		},
		{
			name: "aws eks cluster with assume role",
			args: args{
				cluster: awsKube,
				client: getAWSClientRestConfig(
					eksClients,
					fakeClock,
					[]services.ResourceMatcher{
						{
							Labels: types.Labels{
								types.Wildcard: []string{types.Wildcard},
							},
							AWS: services.ResourceMatcherAWS{
								AssumeRoleARN: "arn:aws:iam::123456789012:role/eks-role",
								ExternalID:    "1234567890",
							},
						},
					},
				),
				validateBearerToken: func(token string) error {
					if token == "" {
						return trace.BadParameter("missing bearer token")
					}
					tokens := strings.Split(token, ".")
					if len(tokens) != 2 {
						return trace.BadParameter("invalid bearer token")
					}
					if tokens[0] != "k8s-aws-v1" {
						return trace.BadParameter("token must start with k8s-aws-v1")
					}
					dec, err := base64.RawStdEncoding.DecodeString(tokens[1])
					if err != nil {
						return trace.Wrap(err)
					}
					if string(dec) != u.String() {
						return trace.BadParameter("invalid token payload")
					}
					return nil
				},
			},
			wantAddr:        "api.eks.us-west-2.amazonaws.com:443",
			wantAssumedRole: []string{"arn:aws:iam::123456789012:role/eks-role"},
			wantExternalIds: []string{"1234567890"},
		},
		{
			name: "gcp gke cluster",
			args: args{
				cluster:             gkeKube,
				client:              gcpRestConfigClient(cloudclients),
				validateBearerToken: func(_ string) error { return nil },
			},
			wantAddr: "api.gke.google.com:443",
		},
		{
			name: "azure aks cluster",
			args: args{
				cluster:             aksKube,
				client:              azureRestConfigClient(cloudclients),
				validateBearerToken: func(_ string) error { return nil },
			},
			wantAddr: "api.aks.microsoft.com:443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newDynamicKubeCreds(
				context.Background(),
				dynamicCredsConfig{
					clock: fakeClock,
					checker: func(_ context.Context, _ string,
						_ authztypes.SelfSubjectAccessReviewInterface,
					) error {
						return nil
					},
					log:                  utils.NewSlogLoggerForTests(),
					kubeCluster:          tt.args.cluster,
					client:               tt.args.client,
					initialRenewInterval: ttl / 2,
				},
			)
			require.NoError(t, err)
			select {
			case <-notify:
			case <-time.After(5 * time.Second):
				t.Fatalf("timeout waiting for cluster to be ready")
			}
			for i := 0; i < 10; i++ {
				require.Equal(t, got.getKubeRestConfig().CAData, []byte(fixtures.TLSCACertPEM))
				require.NoError(t, tt.args.validateBearerToken(got.getKubeRestConfig().BearerToken))
				require.Equal(t, tt.wantAddr, got.getTargetAddr())
				fakeClock.BlockUntil(1)
				fakeClock.Advance(ttl / 2)
				// notify receives a signal when the cloud client is invoked.
				// this is used to test that the credentials are refreshed each time
				// they are about to expire.
				select {
				case <-notify:
				case <-time.After(5 * time.Second):
					t.Fatalf("timeout waiting for cluster to be ready, i=%d", i)
				}
			}
			require.NoError(t, got.close())

			sts := eksClients.AWSConfigProvider.STSClient
			require.Equal(t, tt.wantAssumedRole, apiutils.Deduplicate(sts.GetAssumedRoleARNs()))
			require.Equal(t, tt.wantExternalIds, apiutils.Deduplicate(sts.GetAssumedRoleExternalIDs()))
			sts.ResetAssumeRoleHistory()
		})
	}
}
