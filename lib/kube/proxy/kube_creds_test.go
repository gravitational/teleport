/*
Copyright 2023 Gravitational, Inc.

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

package proxy

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
)

// Test_DynamicKubeCreds tests the dynamic kube credrentials generator for
// AWS, GCP, and Azure clusters accessed using their respective IAM credentials.
// This test mocks the cloud provider clients and the STS client to generate
// rest.Config objects for each cluster. It also tests the renewal of the
// credentials when they expire.
func Test_DynamicKubeCreds(t *testing.T) {
	t.Parallel()
	var (
		fakeClock = clockwork.NewFakeClock()
		log       = logrus.New()
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

	// mock sts client
	u := &url.URL{
		Scheme: "https",
		Host:   "sts.amazonaws.com",
		Path:   "/?Action=GetCallerIdentity&Version=2011-06-15",
	}
	sts := &mocks.STSMock{
		// u is used to presign the request
		// here we just verify the pre-signed request includes this url.
		URL: u,
	}
	// mock clients
	cloudclients := &cloud.TestCloudClients{
		STS: sts,
		EKS: &mocks.EKSMock{
			Notify: notify,
			Clusters: []*eks.Cluster{
				{
					Endpoint: aws.String("https://api.eks.us-west-2.amazonaws.com"),
					Name:     aws.String(awsKube.GetAWSConfig().Name),
					CertificateAuthority: &eks.Certificate{
						Data: aws.String(base64.RawStdEncoding.EncodeToString([]byte(fixtures.TLSCACertPEM))),
					},
				},
			},
		},
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
				cluster: awsKube,
				client:  getAWSClientRestConfig(cloudclients, fakeClock, nil),
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
			wantAddr: "api.eks.us-west-2.amazonaws.com:443",
		},
		{
			name: "aws eks cluster with unmatched assume role",
			args: args{
				cluster: awsKube,
				client: getAWSClientRestConfig(cloudclients, fakeClock, []services.ResourceMatcher{
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
			wantAddr: "api.eks.us-west-2.amazonaws.com:443",
		},
		{
			name: "aws eks cluster with assume role",
			args: args{
				cluster: awsKube,
				client: getAWSClientRestConfig(
					cloudclients,
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
					log:                  log,
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
				require.Equal(t, got.getTargetAddr(), tt.wantAddr)
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

			require.Equal(t, tt.wantAssumedRole, utils.Deduplicate(sts.GetAssumedRoleARNs()))
			require.Equal(t, tt.wantExternalIds, utils.Deduplicate(sts.GetAssumedRoleExternalIDs()))
			sts.ResetAssumeRoleHistory()
		})
	}
}
