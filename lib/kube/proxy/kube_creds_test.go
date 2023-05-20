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

package proxy

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/fixtures"
)

func Test_DynamicKubeCreds(t *testing.T) {
	t.Parallel()
	log := logrus.New()
	log.SetOutput(io.Discard)
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
				Name:      "eks",
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
				ResourceName:   "eks",
				SubscriptionID: "12345",
			},
		},
	)
	require.NoError(t, err)

	// mock sts client
	stsMock := &stsMockClient{
		// u is used to presign the request
		// here we just verify the pre-signed request includes this url.
		u: &url.URL{
			Scheme: "https",
			Host:   "sts.amazonaws.com",
			Path:   "/?Action=GetCallerIdentity&Version=2011-06-15",
		},
	}

	// mock clients
	cloudclients := &cloud.TestCloudClients{
		STS: stsMock,
		EKS: &eksMockClient{
			cluster: awsKube,
			t:       t,
		},
		GCPGKE: &gkeMockCLient{kube: gkeKube, t: t},
		AzureAKSClientPerSub: map[string]azure.AKSClient{
			"12345": &azureMockCLient{
				kube: aksKube,
				t:    t,
			},
		},
	}

	type args struct {
		cluster             types.KubeCluster
		client              dynamicCredsClient
		validateBearerToken func(string) error
	}
	tests := []struct {
		name     string
		args     args
		wantAddr string
	}{
		{
			name: "aws eks cluster",
			args: args{
				cluster: awsKube,
				client:  getAWSClientRestConfig(cloudclients),
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
					if string(dec) != stsMock.u.String() {
						return trace.BadParameter("invalid token payload")
					}
					return nil
				},
			},
			wantAddr: "api.eks.us-west-2.amazonaws.com:443",
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
			fakeClock := clockwork.NewFakeClock()
			got, err := newDynamicKubeCreds(
				context.Background(),
				dynamicCredsConfig{
					clock: fakeClock,
					checker: func(_ context.Context, _ string,
						_ authztypes.SelfSubjectAccessReviewInterface,
					) error {
						return nil
					},
					log:         log,
					kubeCluster: tt.args.cluster,
					client:      tt.args.client,
				},
			)
			require.NoError(t, err)
			require.Equal(t, got.getKubeRestConfig().CAData, []byte(fixtures.TLSCACertPEM))
			require.NoError(t, tt.args.validateBearerToken(got.getKubeRestConfig().BearerToken))
			require.Equal(t, got.getTargetAddr(), tt.wantAddr)
			require.NoError(t, got.close())
		})
	}
}

type eksMockClient struct {
	eksiface.EKSAPI
	cluster types.KubeCluster
	t       *testing.T
}

func (e *eksMockClient) DescribeClusterWithContext(_ aws.Context, req *eks.DescribeClusterInput, _ ...request.Option) (*eks.DescribeClusterOutput, error) {
	require.Equal(e.t, e.cluster.GetAWSConfig().Name, *req.Name)
	return &eks.DescribeClusterOutput{
		Cluster: &eks.Cluster{
			Endpoint: aws.String("https://api.eks.us-west-2.amazonaws.com"),
			Name:     req.Name,
			CertificateAuthority: &eks.Certificate{
				Data: aws.String(base64.RawStdEncoding.EncodeToString([]byte(fixtures.TLSCACertPEM))),
			},
		},
	}, nil
}

type stsMockClient struct {
	stsiface.STSAPI
	u *url.URL
}

func (s *stsMockClient) GetCallerIdentityRequest(req *sts.GetCallerIdentityInput) (*request.Request, *sts.GetCallerIdentityOutput) {
	return &request.Request{
		HTTPRequest: &http.Request{
			Header: http.Header{},
			URL:    s.u,
		},
		Operation: &request.Operation{},
		Handlers:  request.Handlers{},
	}, nil
}

type gkeMockCLient struct {
	gcp.GKEClient
	kube types.KubeCluster
	t    *testing.T
}

func (g *gkeMockCLient) GetClusterRestConfig(ctx context.Context, cfg gcp.ClusterDetails) (*rest.Config, time.Time, error) {
	require.Equal(g.t, g.kube.GetGCPConfig().Name, cfg.Name)
	require.Equal(g.t, g.kube.GetGCPConfig().ProjectID, cfg.ProjectID)
	require.Equal(g.t, g.kube.GetGCPConfig().Location, cfg.Location)
	return &rest.Config{
		Host: "https://api.gke.google.com",
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(fixtures.TLSCACertPEM),
		},
	}, time.Now(), nil
}

type azureMockCLient struct {
	azure.AKSClient
	kube types.KubeCluster
	t    *testing.T
}

func (a *azureMockCLient) ClusterCredentials(ctx context.Context, cfg azure.ClusterCredentialsConfig) (*rest.Config, time.Time, error) {
	require.Equal(a.t, a.kube.GetAzureConfig().ResourceName, cfg.ResourceName)
	require.Equal(a.t, a.kube.GetAzureConfig().ResourceGroup, cfg.ResourceGroup)
	require.Equal(a.t, a.kube.GetAzureConfig().TenantID, cfg.TenantID)

	return &rest.Config{
		Host: "https://api.aks.microsoft.com",
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(fixtures.TLSCACertPEM),
		},
	}, time.Now(), nil
}
