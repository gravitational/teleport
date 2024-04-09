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

package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	eksV1 "github.com/aws/aws-sdk-go/service/eks"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/gravitational/teleport/api/client/proto"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers"
)

func TestGetAgentVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		desc            string
		ping            func(ctx context.Context) (proto.PingResponse, error)
		clusterFeatures proto.Features
		channelVersion  string
		expectedVersion string
		errorAssert     require.ErrorAssertionFunc
	}{
		{
			desc: "ping error",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{}, trace.BadParameter("ping error")
			},
			expectedVersion: "",
			errorAssert:     require.Error,
		},
		{
			desc: "no automatic upgrades",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{ServerVersion: "1.2.3"}, nil
			},
			expectedVersion: "1.2.3",
			errorAssert:     require.NoError,
		},
		{
			desc: "automatic upgrades",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{ServerVersion: "10"}, nil
			},
			clusterFeatures: proto.Features{AutomaticUpgrades: true},
			channelVersion:  "v1.2.3",
			expectedVersion: "1.2.3",
			errorAssert:     require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			server := Server{
				ctx: ctx,
				Config: &Config{
					AccessPoint: &fakeAccessPoint{ping: tt.ping},
					ClusterFeatures: func() proto.Features {
						return tt.clusterFeatures
					},
				},
			}

			var channel *automaticupgrades.Channel
			if tt.channelVersion != "" {
				channel = &automaticupgrades.Channel{StaticVersion: tt.channelVersion}
				err := channel.CheckAndSetDefaults()
				require.NoError(t, err)
			}
			releaseChannels := automaticupgrades.Channels{automaticupgrades.DefaultChannelName: channel}

			version, err := server.getKubeAgentVersion(releaseChannels)

			tt.errorAssert(t, err)
			require.Equal(t, tt.expectedVersion, version)
		})
	}
}

func TestServer_getKubeFetchers(t *testing.T) {
	eks1, err := fetchers.NewEKSFetcher(fetchers.EKSFetcherConfig{
		EKSClientGetter: &cloud.TestCloudClients{},
		FilterLabels:    types.Labels{"l1": []string{"v1"}},
		Region:          "region1",
	})
	require.NoError(t, err)
	eks2, err := fetchers.NewEKSFetcher(fetchers.EKSFetcherConfig{
		EKSClientGetter: &cloud.TestCloudClients{},
		FilterLabels:    types.Labels{"l1": []string{"v1"}},
		Region:          "region1",
		Integration:     "aws1"})
	require.NoError(t, err)
	eks3, err := fetchers.NewEKSFetcher(fetchers.EKSFetcherConfig{
		EKSClientGetter: &cloud.TestCloudClients{},
		FilterLabels:    types.Labels{"l1": []string{"v1"}},
		Region:          "region1",
		Integration:     "aws1"})
	require.NoError(t, err)

	aks1, err := fetchers.NewAKSFetcher(fetchers.AKSFetcherConfig{
		Client:       &mockAKSAPI{},
		FilterLabels: types.Labels{"l1": []string{"v1"}},
		Regions:      []string{"region1"},
	})
	require.NoError(t, err)
	aks2, err := fetchers.NewAKSFetcher(fetchers.AKSFetcherConfig{
		Client:       &mockAKSAPI{},
		FilterLabels: types.Labels{"l1": []string{"v1"}},
		Regions:      []string{"region1"},
	})
	require.NoError(t, err)
	aks3, err := fetchers.NewAKSFetcher(fetchers.AKSFetcherConfig{
		Client:       &mockAKSAPI{},
		FilterLabels: types.Labels{"l1": []string{"v1"}},
		Regions:      []string{"region1"},
	})
	require.NoError(t, err)

	testCases := []struct {
		kubeFetchers                   []common.Fetcher
		kubeDynamicFetchers            map[string][]common.Fetcher
		expectedIntegrationFetchers    []common.Fetcher
		expectedNonIntegrationFetchers []common.Fetcher
	}{
		{
			kubeFetchers:                   []common.Fetcher{eks1},
			expectedNonIntegrationFetchers: []common.Fetcher{eks1},
		},
		{
			kubeFetchers:                   []common.Fetcher{eks1, eks2, eks3, aks1, aks2, aks3},
			expectedIntegrationFetchers:    []common.Fetcher{eks2, eks3},
			expectedNonIntegrationFetchers: []common.Fetcher{eks1, aks1, aks2, aks3},
		},
		{
			kubeFetchers:                   []common.Fetcher{eks1},
			kubeDynamicFetchers:            map[string][]common.Fetcher{"group1": {eks2}},
			expectedIntegrationFetchers:    []common.Fetcher{eks2},
			expectedNonIntegrationFetchers: []common.Fetcher{eks1},
		},
		{
			kubeFetchers:                   []common.Fetcher{aks1, aks2},
			kubeDynamicFetchers:            map[string][]common.Fetcher{"group1": {eks1}},
			expectedIntegrationFetchers:    []common.Fetcher{},
			expectedNonIntegrationFetchers: []common.Fetcher{eks1, aks1, aks2},
		},
	}

	for _, tc := range testCases {
		s := Server{kubeFetchers: tc.kubeFetchers, dynamicKubeFetchers: tc.kubeDynamicFetchers}

		require.ElementsMatch(t, tc.expectedIntegrationFetchers, s.getKubeFetchers(true))
		require.ElementsMatch(t, tc.expectedNonIntegrationFetchers, s.getKubeFetchers(false))
	}
}

func TestDiscoveryKubeIntegrationEKS(t *testing.T) {
	const (
		mainDiscoveryGroup = "main"
		awsAccountID       = "880713328506"
		awsUserID          = "AIDAJQABLZS4A3QDU576Q"
		roleArn            = "arn:aws:sts::880713328506:assumed-role/TeleportRole/1404549515185351000"
		testCAData         = "VGVzdENBREFUQQ=="
	)

	testEKSClusters := []eksTypes.Cluster{
		{
			Name:                 aws.String("eks-cluster1"),
			Arn:                  aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
			Tags:                 map[string]string{"env": "prod", "location": "eu-west-1"},
			CertificateAuthority: &eksTypes.Certificate{Data: aws.String(testCAData)},
			Status:               eksTypes.ClusterStatusActive,
		},
		{
			Name:                 aws.String("eks-cluster2"),
			Arn:                  aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster2"),
			Tags:                 map[string]string{"env": "prod", "location": "eu-west-1"},
			CertificateAuthority: &eksTypes.Certificate{Data: aws.String(testCAData)},
			Status:               eksTypes.ClusterStatusActive,
		},
	}

	getDc := func() *discoveryconfig.DiscoveryConfig {
		dc, _ := discoveryconfig.NewDiscoveryConfig(
			header.Metadata{Name: uuid.NewString()},
			discoveryconfig.Spec{
				DiscoveryGroup: mainDiscoveryGroup,
				AWS: []types.AWSMatcher{
					{
						Types:       []string{types.AWSMatcherEKS},
						Regions:     []string{"eu-west-1"},
						Integration: "integration1",
					},
				},
			},
		)
		return dc
	}

	clusterFinder := func(clusterName string) *eksTypes.Cluster {
		for _, c := range testEKSClusters {
			if aws.ToString(c.Name) == clusterName {
				return &c
			}
		}
		return nil
	}
	clusterUpserter := func(ctx context.Context, authServer *auth.Server, request *integrationpb.EnrollEKSClustersRequest) (*integrationpb.EnrollEKSClustersResponse, error) {
		response := &integrationpb.EnrollEKSClustersResponse{}
		for _, c := range request.EksClusterNames {
			eksCluster := clusterFinder(c)
			if eksCluster == nil {
				response.Results = append(response.Results, &integrationpb.EnrollEKSClusterResult{
					EksClusterName: c,
					Error:          "not found",
				})
				continue
			}

			kubeServer := mustConvertEKSToKubeServerV2(t, eksCluster, "resourceID", mainDiscoveryGroup)

			_, err := authServer.UpsertKubernetesServer(ctx, kubeServer)
			if err != nil {
				return nil, err
			}
			assert.NoError(t, err)

			response.Results = append(response.Results, &integrationpb.EnrollEKSClusterResult{
				EksClusterName: c,
				ResourceId:     "resourceID",
			})
		}
		return response, nil
	}

	testCases := []struct {
		name                         string
		existingKubeClusters         []types.KubeCluster
		existingKubeServers          []types.KubeServer
		awsMatchers                  []types.AWSMatcher
		expectedServersToExistInAuth []types.KubeServer
		accessPoint                  func(*testing.T, *auth.Server, auth.ClientI) auth.DiscoveryAccessPoint
		discoveryConfig              func(*testing.T) *discoveryconfig.DiscoveryConfig
	}{
		{
			name: "no clusters in auth server, discover two clusters from EKS",
			discoveryConfig: func(t *testing.T) *discoveryconfig.DiscoveryConfig {
				return getDc()
			},
			accessPoint: func(t *testing.T, authServer *auth.Server, authClient auth.ClientI) auth.DiscoveryAccessPoint {
				return &accessPointWrapper{
					DiscoveryAccessPoint: getDiscoveryAccessPoint(authServer, authClient),
					enrollEKSClusters: func(ctx context.Context, request *integrationpb.EnrollEKSClustersRequest, _ ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
						response, err := clusterUpserter(ctx, authServer, request)
						assert.NoError(t, err)
						return response, err
					},
				}
			},
			awsMatchers: []types.AWSMatcher{
				{
					Types:       []string{"eks"},
					Regions:     []string{"eu-west-1"},
					Integration: "integration1",
				},
			},
			expectedServersToExistInAuth: []types.KubeServer{
				mustConvertEKSToKubeServerV1(t, eksMockClusters[0], "resourceID", mainDiscoveryGroup),
				mustConvertEKSToKubeServerV1(t, eksMockClusters[1], "resourceID", mainDiscoveryGroup),
			},
		},
		{
			name:                "one cluster in auth server, discover one cluster from EKS and ignore another one",
			existingKubeServers: []types.KubeServer{mustConvertEKSToKubeServerV1(t, eksMockClusters[0], "resourceID", mainDiscoveryGroup)},
			discoveryConfig: func(t *testing.T) *discoveryconfig.DiscoveryConfig {
				return getDc()
			},
			accessPoint: func(t *testing.T, authServer *auth.Server, authClient auth.ClientI) auth.DiscoveryAccessPoint {
				return &accessPointWrapper{
					DiscoveryAccessPoint: getDiscoveryAccessPoint(authServer, authClient),
					enrollEKSClusters: func(ctx context.Context, request *integrationpb.EnrollEKSClustersRequest, _ ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
						assert.Len(t, request.EksClusterNames, 1)

						response, err := clusterUpserter(ctx, authServer, request)
						assert.NoError(t, err)
						return response, err
					},
				}
			},
			awsMatchers: []types.AWSMatcher{
				{
					Types:       []string{"eks"},
					Regions:     []string{"eu-west-1"},
					Integration: "integration1",
				},
			},
			expectedServersToExistInAuth: []types.KubeServer{
				mustConvertEKSToKubeServerV1(t, eksMockClusters[0], "resourceID", mainDiscoveryGroup),
				mustConvertEKSToKubeServerV1(t, eksMockClusters[1], "resourceID", mainDiscoveryGroup),
			},
		},
		{
			name:                "one non-matching cluster in auth server, discover two cluster from EKS",
			existingKubeServers: []types.KubeServer{mustConvertEKSToKubeServerV1(t, eksMockClusters[2], "resourceID", mainDiscoveryGroup)},
			discoveryConfig: func(t *testing.T) *discoveryconfig.DiscoveryConfig {
				return getDc()
			},
			accessPoint: func(t *testing.T, authServer *auth.Server, authClient auth.ClientI) auth.DiscoveryAccessPoint {
				return &accessPointWrapper{
					DiscoveryAccessPoint: getDiscoveryAccessPoint(authServer, authClient),
					enrollEKSClusters: func(ctx context.Context, request *integrationpb.EnrollEKSClustersRequest, _ ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
						assert.Len(t, request.EksClusterNames, 2)

						response, err := clusterUpserter(ctx, authServer, request)
						assert.NoError(t, err)
						return response, err
					},
				}
			},
			awsMatchers: []types.AWSMatcher{
				{
					Types:       []string{"eks"},
					Regions:     []string{"eu-west-1"},
					Integration: "integration1",
				},
			},
			expectedServersToExistInAuth: []types.KubeServer{
				mustConvertEKSToKubeServerV1(t, eksMockClusters[0], "resourceID", mainDiscoveryGroup),
				mustConvertEKSToKubeServerV1(t, eksMockClusters[1], "resourceID", mainDiscoveryGroup),
				mustConvertEKSToKubeServerV1(t, eksMockClusters[2], "resourceID", mainDiscoveryGroup),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testCloudClients := &cloud.TestCloudClients{
				STS: &mocks.STSMock{},
				EKS: &mockEKSAPI{
					clusters: eksMockClusters[:2],
				},
			}

			ctx := context.Background()
			// Create and start test auth server.
			testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
				Dir: t.TempDir(),
			})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

			tlsServer, err := testAuthServer.NewTestTLSServer()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

			// Auth client for discovery service.
			identity := auth.TestServerID(types.RoleDiscovery, "hostID")
			authClient, err := tlsServer.NewClient(identity)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, authClient.Close()) })

			integration, err := types.NewIntegrationAWSOIDC(
				types.Metadata{Name: "integration1"},
				&types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::123456789012:role/IntegrationRole",
				},
			)
			require.NoError(t, err)

			testAuthServer.AuthServer.IntegrationsTokenGenerator = &mockIntegrationsTokenGenerator{
				proxies: nil,
				integrations: map[string]types.Integration{
					integration.GetName(): integration,
				},
			}

			_, err = tlsServer.Auth().CreateIntegration(ctx, integration)
			require.NoError(t, err)

			for _, kubeCluster := range tc.existingKubeClusters {
				err := tlsServer.Auth().CreateKubernetesCluster(ctx, kubeCluster)
				require.NoError(t, err)
			}

			for _, kubeServer := range tc.existingKubeServers {
				_, err := tlsServer.Auth().UpsertKubernetesServer(ctx, kubeServer)
				require.NoError(t, err)
			}

			reporter := &mockUsageReporter{}

			tlsServer.Auth().SetUsageReporter(reporter)
			discServer, err := New(
				authz.ContextWithUser(ctx, identity.I),
				&Config{
					CloudClients:     testCloudClients,
					ClusterFeatures:  func() proto.Features { return proto.Features{} },
					KubernetesClient: fake.NewSimpleClientset(),
					AccessPoint:      tc.accessPoint(t, tlsServer.Auth(), authClient),
					Matchers: Matchers{
						AWS: tc.awsMatchers,
					},
					Emitter:        authClient,
					Log:            logrus.New(),
					DiscoveryGroup: mainDiscoveryGroup,
				})

			require.NoError(t, err)

			if tc.discoveryConfig != nil {
				dc := tc.discoveryConfig(t)
				_, err := tlsServer.Auth().DiscoveryConfigClient().CreateDiscoveryConfig(ctx, dc)
				require.NoError(t, err)

				// Wait for the DiscoveryConfig to be added to the dynamic fetchers
				require.Eventually(t, func() bool {
					discServer.muDynamicKubeFetchers.RLock()
					defer discServer.muDynamicKubeFetchers.RUnlock()
					return len(discServer.dynamicKubeFetchers) > 0
				}, 1*time.Second, 100*time.Millisecond)
			}

			t.Cleanup(func() {
				discServer.Stop()
			})
			go discServer.Start()

			require.Eventually(t, func() bool {
				kubeServers, err := tlsServer.Auth().GetKubernetesServers(ctx)
				require.NoError(t, err)

				if len(kubeServers) == len(tc.expectedServersToExistInAuth) {
					k1 := types.KubeServers(kubeServers).ToMap()
					k2 := types.KubeServers(tc.expectedServersToExistInAuth).ToMap()
					for k := range k1 {
						if services.CompareResources(k1[k], k2[k]) != services.Equal {
							return false
						}
					}
					return true
				}

				return false
			}, 315*time.Second, 200*time.Millisecond)
		})
	}
}

func mustConvertEKSToKubeServerV1(t *testing.T, eksCluster *eksV1.Cluster, resourceID, discoveryGroup string) types.KubeServer {
	eksCluster.Tags[types.OriginLabel] = aws.String(types.OriginCloud)
	eksCluster.Tags[types.InternalResourceIDLabel] = aws.String(resourceID)

	kubeCluster, err := services.NewKubeClusterFromAWSEKS(aws.ToString(eksCluster.Name), aws.ToString(eksCluster.Arn), eksCluster.Tags)
	assert.NoError(t, err)

	kubeClusterV3 := kubeCluster.(*types.KubernetesClusterV3)
	common.ApplyEKSNameSuffix(kubeClusterV3)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeClusterV3, "host", "uuid")
	assert.NoError(t, err)

	return kubeServer
}

func mustConvertEKSToKubeServerV2(t *testing.T, eksCluster *eksTypes.Cluster, resourceID, discoveryGroup string) types.KubeServer {
	eksTags := make(map[string]*string, len(eksCluster.Tags))
	for k, v := range eksCluster.Tags {
		eksTags[k] = aws.String(v)
	}
	eksTags[types.OriginLabel] = aws.String(types.OriginCloud)
	eksTags[types.InternalResourceIDLabel] = aws.String(resourceID)

	kubeCluster, err := services.NewKubeClusterFromAWSEKS(aws.ToString(eksCluster.Name), aws.ToString(eksCluster.Arn), eksTags)
	assert.NoError(t, err)

	kubeClusterV3 := kubeCluster.(*types.KubernetesClusterV3)
	common.ApplyEKSNameSuffix(kubeClusterV3)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeClusterV3, "host", "uuid")
	assert.NoError(t, err)

	return kubeServer
}

type accessPointWrapper struct {
	auth.DiscoveryAccessPoint

	enrollEKSClusters func(context.Context, *integrationpb.EnrollEKSClustersRequest, ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error)
}

func (a *accessPointWrapper) EnrollEKSClusters(ctx context.Context, req *integrationpb.EnrollEKSClustersRequest, _ ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
	if a.enrollEKSClusters != nil {
		return a.enrollEKSClusters(ctx, req)
	}
	if a.DiscoveryAccessPoint != nil {
		return a.DiscoveryAccessPoint.EnrollEKSClusters(ctx, req)
	}
	return &integrationpb.EnrollEKSClustersResponse{}, trace.NotImplemented("not implemented")
}

type mockIntegrationsTokenGenerator struct {
	proxies         []types.Server
	integrations    map[string]types.Integration
	tokenCallsCount int
}

// GetIntegration returns the specified integration resources.
func (m *mockIntegrationsTokenGenerator) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if ig, found := m.integrations[name]; found {
		return ig, nil
	}

	return nil, trace.NotFound("integration not found")
}

// GetProxies returns a list of registered proxies.
func (m *mockIntegrationsTokenGenerator) GetProxies() ([]types.Server, error) {
	return m.proxies, nil
}

// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
func (m *mockIntegrationsTokenGenerator) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	m.tokenCallsCount++
	return uuid.NewString(), nil
}

type mockEnrollEKSClusterClient struct {
	createAccessEntry          func(context.Context, *eks.CreateAccessEntryInput, ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error)
	associateAccessPolicy      func(context.Context, *eks.AssociateAccessPolicyInput, ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)
	listAccessEntries          func(context.Context, *eks.ListAccessEntriesInput, ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error)
	deleteAccessEntry          func(context.Context, *eks.DeleteAccessEntryInput, ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error)
	describeCluster            func(context.Context, *eks.DescribeClusterInput, ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	getCallerIdentity          func(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	checkAgentAlreadyInstalled func(context.Context, genericclioptions.RESTClientGetter, logrus.FieldLogger) (bool, error)
	installKubeAgent           func(context.Context, *eksTypes.Cluster, string, string, string, genericclioptions.RESTClientGetter, logrus.FieldLogger, awsoidc.EnrollEKSClustersRequest) error
	createToken                func(context.Context, types.ProvisionToken) error
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

func (m *mockEnrollEKSClusterClient) CheckAgentAlreadyInstalled(ctx context.Context, kubeconfig genericclioptions.RESTClientGetter, log logrus.FieldLogger) (bool, error) {
	if m.checkAgentAlreadyInstalled != nil {
		return m.checkAgentAlreadyInstalled(ctx, kubeconfig, log)
	}
	return false, nil
}

func (m *mockEnrollEKSClusterClient) InstallKubeAgent(ctx context.Context, eksCluster *eksTypes.Cluster, proxyAddr, joinToken, resourceId string, kubeconfig genericclioptions.RESTClientGetter, log logrus.FieldLogger, req awsoidc.EnrollEKSClustersRequest) error {
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

var _ awsoidc.EnrollEKSCLusterClient = &mockEnrollEKSClusterClient{}
