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

package discovery

import (
	"context"
	"iter"
	"maps"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func discoveryConfigWithAWSMatchers(t *testing.T, discoveryGroup string, m ...types.AWSMatcher) *discoveryconfig.DiscoveryConfig {
	t.Helper()
	discoveryConfigName := uuid.NewString()
	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: discoveryConfigName},
		discoveryconfig.Spec{
			DiscoveryGroup: discoveryGroup,
			AWS:            m,
		},
	)
	require.NoError(t, err)
	return discoveryConfig
}

func awsMatcherForEKS(t *testing.T, integrationName string) types.AWSMatcher {
	t.Helper()
	return types.AWSMatcher{
		Types:       []string{"eks"},
		Regions:     []string{"us-west-2"},
		Tags:        map[string]utils.Strings{"RunDiscover": {"Please"}},
		Integration: integrationName,
	}
}

func awsMatcherForEKSWithAppDiscovery(t *testing.T, integrationName string) types.AWSMatcher {
	t.Helper()
	matcher := awsMatcherForEKS(t, integrationName)
	matcher.KubeAppDiscovery = true
	return matcher
}

func TestDiscoveryServerEKS(t *testing.T) {
	t.Parallel()
	integrationName := "my-integration"
	defaultDiscoveryGroup := "dc001"

	eksMatcher := awsMatcherForEKS(t, integrationName)
	discoveryConfigForUserTaskEKSTest := discoveryConfigWithAWSMatchers(t, defaultDiscoveryGroup, eksMatcher)

	matcherWithAppDiscovery := awsMatcherForEKSWithAppDiscovery(t, integrationName)
	matcherWithoutAppDiscovery := awsMatcherForEKS(t, integrationName)
	discoveryConfigWithAndWithoutAppDiscovery := discoveryConfigWithAWSMatchers(t, defaultDiscoveryGroup, matcherWithAppDiscovery, matcherWithoutAppDiscovery)

	awsOIDCIntegration, err := types.NewIntegrationAWSOIDC(types.Metadata{
		Name: integrationName,
	}, &types.AWSOIDCIntegrationSpecV1{
		RoleARN: "arn:aws:iam::123456789012:role/teleport",
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name                      string
		emitter                   *mockEmitter
		eksClusters               []*ekstypes.Cluster
		eksEnroller               eksClustersEnroller
		discoveryConfig           *discoveryconfig.DiscoveryConfig
		staticMatchers            Matchers
		wantInstalledInstances    []string
		wantDiscoveryConfigStatus *discoveryconfig.Status
		userTasksDiscoverCheck    func(t *testing.T, existingTasks []*usertasksv1.UserTask)
	}{
		{
			name: "multiple EKS clusters failed to autoenroll and user tasks are created",
			eksClusters: []*ekstypes.Cluster{
				{
					Name:   aws.String("cluster01"),
					Arn:    aws.String("arn:aws:eks:us-west-2:123456789012:cluster/cluster01"),
					Status: ekstypes.ClusterStatusActive,
					Tags: map[string]string{
						"RunDiscover": "Please",
					},
				},
				{
					Name:   aws.String("cluster02"),
					Arn:    aws.String("arn:aws:eks:us-west-2:123456789012:cluster/cluster02"),
					Status: ekstypes.ClusterStatusActive,
					Tags: map[string]string{
						"RunDiscover": "Please",
					},
				},
			},
			eksEnroller: &mockEKSClusterEnroller{
				resp: &integrationpb.EnrollEKSClustersResponse{
					Results: []*integrationpb.EnrollEKSClusterResult{
						{
							EksClusterName: "cluster01",
							Error:          "access endpoint is not reachable",
							IssueType:      "eks-cluster-unreachable",
						},
						{
							EksClusterName: "cluster02",
							Error:          "access endpoint is not reachable",
							IssueType:      "eks-cluster-unreachable",
						},
					},
				},
				err: nil,
			},
			emitter:                &mockEmitter{},
			staticMatchers:         Matchers{},
			discoveryConfig:        discoveryConfigForUserTaskEKSTest,
			wantInstalledInstances: []string{},
			userTasksDiscoverCheck: func(t *testing.T, existingTasks []*usertasksv1.UserTask) {
				require.NotEmpty(t, existingTasks)
				existingTask := existingTasks[0]

				require.Equal(t, "OPEN", existingTask.GetSpec().State)
				require.Equal(t, integrationName, existingTask.GetSpec().Integration)
				require.Equal(t, "eks-cluster-unreachable", existingTask.GetSpec().IssueType)
				require.Equal(t, "123456789012", existingTask.GetSpec().GetDiscoverEks().GetAccountId())
				require.Equal(t, "us-west-2", existingTask.GetSpec().GetDiscoverEks().GetRegion())

				taskClusters := existingTask.GetSpec().GetDiscoverEks().Clusters
				require.Contains(t, taskClusters, "cluster01")
				taskCluster := taskClusters["cluster01"]

				require.Equal(t, "cluster01", taskCluster.Name)
				require.Equal(t, discoveryConfigForUserTaskEKSTest.GetName(), taskCluster.DiscoveryConfig)
				require.Equal(t, defaultDiscoveryGroup, taskCluster.DiscoveryGroup)

				require.Contains(t, taskClusters, "cluster02")
				taskCluster2 := taskClusters["cluster02"]

				require.Equal(t, "cluster02", taskCluster2.Name)
				require.Equal(t, discoveryConfigForUserTaskEKSTest.GetName(), taskCluster2.DiscoveryConfig)
				require.Equal(t, defaultDiscoveryGroup, taskCluster2.DiscoveryGroup)
			},
		},
		{
			name: "multiple EKS clusters with different KubeAppDiscovery setting failed to autoenroll and user tasks are created",
			eksClusters: []*ekstypes.Cluster{
				{
					Name:   aws.String("cluster01"),
					Arn:    aws.String("arn:aws:eks:us-west-2:123456789012:cluster/cluster01"),
					Status: ekstypes.ClusterStatusActive,
					Tags: map[string]string{
						"RunDiscover": "Please",
					},
				},
				{
					Name:   aws.String("cluster02"),
					Arn:    aws.String("arn:aws:eks:us-west-2:123456789012:cluster/cluster02"),
					Status: ekstypes.ClusterStatusActive,
					Tags: map[string]string{
						"RunDiscover": "Please",
					},
				},
			},
			eksEnroller: &mockEKSClusterEnroller{
				resp: &integrationpb.EnrollEKSClustersResponse{
					Results: []*integrationpb.EnrollEKSClusterResult{
						{
							EksClusterName: "cluster01",
							Error:          "access endpoint is not reachable",
							IssueType:      "eks-cluster-unreachable",
						},
						{
							EksClusterName: "cluster02",
							Error:          "access endpoint is not reachable",
							IssueType:      "eks-cluster-unreachable",
						},
					},
				},
				err: nil,
			},
			emitter:                &mockEmitter{},
			staticMatchers:         Matchers{},
			discoveryConfig:        discoveryConfigWithAndWithoutAppDiscovery,
			wantInstalledInstances: []string{},
			userTasksDiscoverCheck: func(t *testing.T, existingTasks []*usertasksv1.UserTask) {
				require.Len(t, existingTasks, 2)
				existingTask := existingTasks[0]
				if existingTask.Spec.DiscoverEks.AppAutoDiscover == false {
					existingTask = existingTasks[1]
				}

				require.Equal(t, "OPEN", existingTask.GetSpec().State)
				require.Equal(t, integrationName, existingTask.GetSpec().Integration)
				require.Equal(t, "eks-cluster-unreachable", existingTask.GetSpec().IssueType)
				require.Equal(t, "123456789012", existingTask.GetSpec().GetDiscoverEks().GetAccountId())
				require.Equal(t, "us-west-2", existingTask.GetSpec().GetDiscoverEks().GetRegion())

				taskClusters := existingTask.GetSpec().GetDiscoverEks().Clusters
				require.Contains(t, taskClusters, "cluster01")
				taskCluster := taskClusters["cluster01"]

				require.Equal(t, "cluster01", taskCluster.Name)
				require.Equal(t, discoveryConfigWithAndWithoutAppDiscovery.GetName(), taskCluster.DiscoveryConfig)
				require.Equal(t, defaultDiscoveryGroup, taskCluster.DiscoveryGroup)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				fakeConfigProvider := mocks.AWSConfigProvider{
					AWSConfig: &aws.Config{},
					OIDCIntegrationClient: &mocks.FakeOIDCIntegrationClient{
						Integration: awsOIDCIntegration,
					},
				}

				bk, err := memory.New(memory.Config{})
				require.NoError(t, err)

				mockAccessPoint := &mockAuthServer{
					events:            local.NewEventsService(bk),
					enrollEKSClusters: tt.eksEnroller.EnrollEKSClusters,
					storeDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
						tt.discoveryConfig.GetName(): tt.discoveryConfig,
					},
					storeUserTasks: map[string]*usertasksv1.UserTask{},
				}

				server, err := New(ctx, &Config{
					AWSConfigProvider: &fakeConfigProvider,
					AWSFetchersClients: &mockFetchersClients{
						AWSConfigProvider: fakeConfigProvider,
						eksClusters:       tt.eksClusters,
					},
					ClusterFeatures:    func() proto.Features { return proto.Features{} },
					AccessPoint:        mockAccessPoint,
					Matchers:           Matchers{},
					Emitter:            tt.emitter,
					DiscoveryGroup:     defaultDiscoveryGroup,
					Log:                logtest.NewLogger(),
					PublicProxyAddress: "proxy.example.com",
				})
				require.NoError(t, err)

				go func() {
					assert.NoError(t, server.Start())
				}()
				defer t.Cleanup(server.Stop)

				// Wait for the discovery server to complete one iteration of discovering resources
				synctest.Wait()

				// Discovery usage events are reported.
				require.NotEmpty(t, mockAccessPoint.usageEvents)

				// Check the UserTasks created by the discovery server.
				existingTasks := slices.Collect(maps.Values(mockAccessPoint.storeUserTasks))
				tt.userTasksDiscoverCheck(t, existingTasks)
			})
		})
	}
}

type mockAuthServer struct {
	authclient.DiscoveryAccessPoint

	storeDiscoveryConfigs map[string]*discoveryconfig.DiscoveryConfig
	storeUserTasks        map[string]*usertasksv1.UserTask

	events      types.Events
	usageEvents []*proto.SubmitUsageEventRequest

	enrollEKSClusters func(context.Context, *integrationpb.EnrollEKSClustersRequest, ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error)
}

func (m *mockAuthServer) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return m.events.NewWatcher(ctx, watch)
}

func (m *mockAuthServer) Ping(context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		ServerVersion: api.SemVer().String(),
	}, nil
}

func (m *mockAuthServer) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	m.usageEvents = append(m.usageEvents, req)
	return nil
}

func (m *mockAuthServer) ListDiscoveryConfigs(ctx context.Context, pageSize int, nextKey string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	return slices.Collect(maps.Values(m.storeDiscoveryConfigs)), "", nil
}

func (m *mockAuthServer) UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error) {
	dc, ok := m.storeDiscoveryConfigs[name]
	if !ok {
		return nil, trace.NotFound("discovery config %q not found", name)
	}

	dc.Status = status
	m.storeDiscoveryConfigs[name] = dc

	return dc, nil
}

func (m *mockAuthServer) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	return nil, nil
}

func (m *mockAuthServer) ListKubernetesClusters(ctx context.Context, limit int, start string) ([]types.KubeCluster, string, error) {
	return nil, "", nil
}

func (m *mockAuthServer) RangeKubernetesClusters(ctx context.Context, start, end string) iter.Seq2[types.KubeCluster, error] {
	return func(yield func(types.KubeCluster, error) bool) {}
}

func (m *mockAuthServer) GetKubernetesServers(context.Context) ([]types.KubeServer, error) {
	return nil, nil
}

func (m *mockAuthServer) GetDatabases(ctx context.Context) ([]types.Database, error) {
	return nil, nil
}

func (m *mockAuthServer) ListDatabases(ctx context.Context, limit int, startKey string) ([]types.Database, string, error) {
	return nil, "", nil
}

func (m *mockAuthServer) RangeDatabases(ctx context.Context, start, end string) iter.Seq2[types.Database, error] {
	return stream.Empty[types.Database]()
}

func (m *mockAuthServer) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	return nil, nil
}

func (m *mockAuthServer) EnrollEKSClusters(ctx context.Context, req *integrationpb.EnrollEKSClustersRequest, opts ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
	return m.enrollEKSClusters(ctx, req, opts...)
}

func (m *mockAuthServer) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return &types.SemaphoreLease{
		Expires: time.Now().Add(10 * time.Minute),
	}, nil
}

func (m *mockAuthServer) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}

func (m *mockAuthServer) GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error) {
	if task, ok := m.storeUserTasks[name]; ok {
		return task, nil
	}
	return nil, trace.NotFound("user task %q not found", name)
}

func (m *mockAuthServer) UpsertUserTask(ctx context.Context, req *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	m.storeUserTasks[req.GetMetadata().GetName()] = req
	return req, nil
}

type mockEKSClusterEnroller struct {
	resp *integrationpb.EnrollEKSClustersResponse
	err  error
}

func (m *mockEKSClusterEnroller) EnrollEKSClusters(ctx context.Context, req *integrationpb.EnrollEKSClustersRequest, opt ...grpc.CallOption) (*integrationpb.EnrollEKSClustersResponse, error) {
	ret := &integrationpb.EnrollEKSClustersResponse{
		Results: []*integrationpb.EnrollEKSClusterResult{},
	}
	// Filter out non-requested clusters.
	for _, clusterName := range req.EksClusterNames {
		for _, mockClusterResult := range m.resp.Results {
			if clusterName == mockClusterResult.EksClusterName {
				ret.Results = append(ret.Results, mockClusterResult)
			}
		}
	}
	return ret, m.err
}
