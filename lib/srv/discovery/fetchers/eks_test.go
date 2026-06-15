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

package fetchers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awsregions "github.com/gravitational/teleport/lib/cloud/aws/regions"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestEKSFetcher(t *testing.T) {
	region1Clusters := []*ekstypes.Cluster{{
		Name:   aws.String("us-east-1-cluster"),
		Arn:    aws.String("arn:aws:eks:us-east-1:accountID:cluster/us-east-1-cluster"),
		Status: ekstypes.ClusterStatusActive,
		Tags:   map[string]string{"env": "prod"},
	}}
	region2Clusters := []*ekstypes.Cluster{{
		Name:   aws.String("eu-west-1-cluster"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/eu-west-1-cluster"),
		Status: ekstypes.ClusterStatusActive,
		Tags:   map[string]string{"env": "prod"},
	}}
	denied := awsResponseError(http.StatusForbidden, "AccessDenied")
	twoRegionLister := func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error) {
		return &mockAccountClient{
			output: &account.ListRegionsOutput{
				Regions: []accounttypes.Region{
					{RegionName: aws.String("us-east-1")},
					{RegionName: aws.String("eu-west-1")},
				},
			},
		}, nil
	}
	deniedLister := func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error) {
		return &mockAccountClient{err: denied}, nil
	}
	// Single-region shorthand: dispatch the stock populated mock for one region.
	single := func(region string) map[string]EKSClient {
		return map[string]EKSClient{region: &mockEKSAPI{clusters: eksMockClusters}}
	}

	type args struct {
		regions         []string
		filterLabels    types.Labels
		clientsByRegion map[string]EKSClient
		regionsLister   awsregions.ListerGetter
		assumeRole      types.AssumeRole
	}
	tests := []struct {
		name    string
		args    args
		want    types.ResourcesWithLabels
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "list everything",
			args: args{
				regions:         []string{"eu-west-1"},
				filterLabels:    types.Labels{types.Wildcard: []string{types.Wildcard}},
				clientsByRegion: single("eu-west-1"),
			},
			want:    eksClustersToResources(t, eksMockClusters...),
			wantErr: require.NoError,
		},
		{
			name: "list everything with assumed role",
			args: args{
				regions:         []string{"eu-west-1"},
				filterLabels:    types.Labels{types.Wildcard: []string{types.Wildcard}},
				clientsByRegion: single("eu-west-1"),
				assumeRole:      types.AssumeRole{RoleARN: "arn:aws:iam::123456789012:role/test-role", ExternalID: "extID123"},
			},
			want:    eksClustersToResources(t, eksMockClusters...),
			wantErr: require.NoError,
		},
		{
			name: "list prod clusters",
			args: args{
				regions:         []string{"eu-west-1"},
				filterLabels:    types.Labels{"env": []string{"prod"}},
				clientsByRegion: single("eu-west-1"),
			},
			want:    eksClustersToResources(t, eksMockClusters[:2]...),
			wantErr: require.NoError,
		},
		{
			name: "list stg clusters from eu-west-1",
			args: args{
				regions: []string{"uswest2"},
				filterLabels: types.Labels{
					"env":      []string{"stg"},
					"location": []string{"eu-west-1"},
				},
				clientsByRegion: single("uswest2"),
			},
			want:    eksClustersToResources(t, eksMockClusters[2:]...),
			wantErr: require.NoError,
		},
		{
			name: "filter not found",
			args: args{
				regions:         []string{"uswest2"},
				filterLabels:    types.Labels{"env": []string{"none"}},
				clientsByRegion: single("uswest2"),
			},
			want:    eksClustersToResources(t),
			wantErr: require.NoError,
		},
		{
			name: "list everything with specified values",
			args: args{
				regions:         []string{"uswest2"},
				filterLabels:    types.Labels{"env": []string{"prod", "stg"}},
				clientsByRegion: single("uswest2"),
			},
			want:    eksClustersToResources(t, eksMockClusters...),
			wantErr: require.NoError,
		},
		{
			name: "list across explicit regions",
			args: args{
				regions:      []string{"us-east-1", "eu-west-1"},
				filterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				clientsByRegion: map[string]EKSClient{
					"us-east-1": &mockEKSAPI{clusters: region1Clusters},
					"eu-west-1": &mockEKSAPI{clusters: region2Clusters},
				},
			},
			want:    eksClustersToResources(t, append(region1Clusters, region2Clusters...)...),
			wantErr: require.NoError,
		},
		{
			name: "wildcard expands to enabled regions",
			args: args{
				regions:      []string{types.Wildcard},
				filterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				clientsByRegion: map[string]EKSClient{
					"us-east-1": &mockEKSAPI{clusters: region1Clusters},
					"eu-west-1": &mockEKSAPI{clusters: region2Clusters},
				},
				regionsLister: twoRegionLister,
			},
			want:    eksClustersToResources(t, append(region1Clusters, region2Clusters...)...),
			wantErr: require.NoError,
		},
		{
			name: "per-region failure does not abort matcher",
			args: args{
				regions:      []string{"us-east-1", "eu-west-1"},
				filterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				clientsByRegion: map[string]EKSClient{
					"us-east-1": &mockEKSAPI{listErr: denied},
					"eu-west-1": &mockEKSAPI{clusters: region2Clusters},
				},
			},
			want:    eksClustersToResources(t, region2Clusters...),
			wantErr: require.NoError,
		},
		{
			name: "wildcard account:ListRegions denied",
			args: args{
				regions:       []string{types.Wildcard},
				filterLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
				regionsLister: deniedLister,
			},
			want: nil,
			wantErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %T: %v", err, err)
				require.Contains(t, err.Error(), "account:ListRegions")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stsClt := &mocks.STSClient{}
			matcher := types.AWSMatcher{
				Regions: tt.args.regions,
				Tags:    tt.args.filterLabels,
			}
			if tt.args.assumeRole.RoleARN != "" {
				matcher.AssumeRole = &tt.args.assumeRole
			}
			cfg := EKSFetcherConfig{
				ClientGetter: &mockRegionalEKSClientGetter{
					AWSConfigProvider: mocks.AWSConfigProvider{
						STSClient: stsClt,
					},
					clientsByRegion: tt.args.clientsByRegion,
				},
				RegionsListerGetter: tt.args.regionsLister,
				Matcher:             matcher,
				Logger:              logtest.NewLogger(),
			}
			fetcher, err := NewEKSFetcher(cfg)
			require.NoError(t, err)
			resources, err := fetcher.Get(context.Background())
			tt.wantErr(t, err)
			if err != nil {
				return
			}

			clusters := types.ResourcesWithLabels{}
			for _, r := range resources {
				if e, ok := r.(*DiscoveredEKSCluster); ok {
					clusters = append(clusters, e.GetKubeCluster())
				} else {
					clusters = append(clusters, r)
				}
			}

			require.Equal(t, tt.want.ToMap(), clusters.ToMap())
			if tt.args.assumeRole.RoleARN != "" {
				require.Contains(t, stsClt.GetAssumedRoleARNs(), tt.args.assumeRole.RoleARN)
			}
		})
	}
}

// TestEKSFetcherRetriesCallerIdentity verifies that a transient
// sts:GetCallerIdentity failure does not permanently disable access setup.
func TestEKSFetcherRetriesCallerIdentity(t *testing.T) {
	const resolvedARN = "arn:aws:iam::123456789012:role/discovery"
	sts := &mockSTSClient{arn: resolvedARN, failCalls: 1}
	fetcher := newTestEKSFetcher(t,
		types.AWSMatcher{
			Regions: []string{"eu-west-1"},
			Tags:    types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		map[string]EKSClient{"eu-west-1": &mockEKSAPI{
			clusters: []*ekstypes.Cluster{testEKSCluster(ekstypes.AuthenticationModeConfigMap)},
		}},
		sts,
	)

	resources, err := fetcher.Get(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Nil(t, awsDiscoveryStatus(resources[0].(*DiscoveredEKSCluster)))

	resources, err = fetcher.Get(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	awsStatus := awsDiscoveryStatus(resources[0].(*DiscoveredEKSCluster))
	require.NotNil(t, awsStatus)
	require.Equal(t, resolvedARN, awsStatus.SetupAccessForArn)
	require.Equal(t, 2, sts.calls)
}

// TestEKSFetcherFallsBackToSetupAccessForARN verifies that
// status is recorded when STS fails but Matcher.SetupAccessForARN supplies
// the principal ARN, so cleanup can later find the access entry to remove.
func TestEKSFetcherFallsBackToSetupAccessForARN(t *testing.T) {
	const principalARN = "arn:aws:iam::123456789012:role/operator"
	fetcher := newTestEKSFetcher(t,
		types.AWSMatcher{
			Regions:           []string{"eu-west-1"},
			Tags:              types.Labels{types.Wildcard: []string{types.Wildcard}},
			SetupAccessForARN: principalARN,
		},
		map[string]EKSClient{"eu-west-1": &mockEKSAPI{
			clusters: []*ekstypes.Cluster{testEKSCluster(ekstypes.AuthenticationModeConfigMap)},
		}},
		&mockSTSClient{arn: "unused", failCalls: 100},
	)

	resources, err := fetcher.Get(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	awsStatus := awsDiscoveryStatus(resources[0].(*DiscoveredEKSCluster))
	require.NotNil(t, awsStatus)
	require.Equal(t, principalARN, awsStatus.SetupAccessForArn)
}

// TestEKSFetcherSkipsAccessSetupWithoutBootstrap verifies that
// the fetcher inspects the access entry but does not attempt to provision
// admin access when the bootstrap ARN is unresolved.
func TestEKSFetcherSkipsAccessSetupWithoutBootstrap(t *testing.T) {
	const principalARN = "arn:aws:iam::123456789012:role/operator"
	eksClient := &mockEKSAPI{
		clusters:            []*ekstypes.Cluster{testEKSCluster(ekstypes.AuthenticationModeApi)},
		accessEntryNotFound: true,
	}
	fetcher := newTestEKSFetcher(t,
		types.AWSMatcher{
			Regions:           []string{"eu-west-1"},
			Tags:              types.Labels{types.Wildcard: []string{types.Wildcard}},
			SetupAccessForARN: principalARN,
		},
		map[string]EKSClient{"eu-west-1": eksClient},
		&mockSTSClient{arn: "unused", failCalls: 100},
	)

	resources, err := fetcher.Get(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	awsStatus := awsDiscoveryStatus(resources[0].(*DiscoveredEKSCluster))
	require.NotNil(t, awsStatus)
	require.Equal(t, principalARN, awsStatus.SetupAccessForArn)

	require.Equal(t, 1, eksClient.describeAccessEntryCalls)
	require.Zero(t, eksClient.createAccessEntryCalls)
	require.Zero(t, eksClient.associateAccessPolicyCalls)
}

func testEKSCluster(authMode ekstypes.AuthenticationMode) *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:         aws.String("test-cluster"),
		Arn:          aws.String("arn:aws:eks:eu-west-1:123456789012:cluster/test-cluster"),
		Status:       ekstypes.ClusterStatusActive,
		Tags:         map[string]string{"env": "prod"},
		AccessConfig: &ekstypes.AccessConfigResponse{AuthenticationMode: authMode},
	}
}

func newTestEKSFetcher(t *testing.T, matcher types.AWSMatcher, clients map[string]EKSClient, sts *mockSTSClient) common.Fetcher {
	t.Helper()
	fetcher, err := NewEKSFetcher(EKSFetcherConfig{
		ClientGetter: &mockRegionalEKSClientGetterWithSTS{
			mockRegionalEKSClientGetter: mockRegionalEKSClientGetter{
				AWSConfigProvider: mocks.AWSConfigProvider{},
				clientsByRegion:   clients,
			},
			stsClient: sts,
		},
		Matcher: matcher,
		Logger:  logtest.NewLogger(),
	})
	require.NoError(t, err)
	return fetcher
}

// mockSTSClient records how many times GetCallerIdentity is called and
// can be configured to fail the first failCalls attempts.
type mockSTSClient struct {
	arn       string
	failCalls int
	calls     int
}

func (m *mockSTSClient) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	m.calls++
	if m.calls <= m.failCalls {
		return nil, errors.New("transient sts error")
	}
	return &sts.GetCallerIdentityOutput{Arn: aws.String(m.arn)}, nil
}

func (*mockSTSClient) AssumeRole(context.Context, *sts.AssumeRoleInput, ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	panic("not implemented")
}

func awsDiscoveryStatus(c *DiscoveredEKSCluster) *types.KubernetesClusterAWSStatus {
	status := c.GetStatus()
	if status == nil || status.Discovery == nil {
		return nil
	}
	return status.Discovery.Aws
}

type mockRegionalEKSClientGetterWithSTS struct {
	mockRegionalEKSClientGetter
	stsClient STSClient
}

func (g *mockRegionalEKSClientGetterWithSTS) GetAWSSTSClient(aws.Config) STSClient {
	return g.stsClient
}

type mockSTSPresignAPI struct{}

func (a *mockSTSPresignAPI) PresignGetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	panic("not implemented")
}

type mockEKSAPI struct {
	EKSClient

	clusters            []*ekstypes.Cluster
	listErr             error
	accessEntryNotFound bool

	describeAccessEntryCalls   int
	createAccessEntryCalls     int
	associateAccessPolicyCalls int
}

func (m *mockEKSAPI) DescribeAccessEntry(_ context.Context, _ *eks.DescribeAccessEntryInput, _ ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error) {
	m.describeAccessEntryCalls++
	if m.accessEntryNotFound {
		return nil, awsResponseError(http.StatusNotFound, "ResourceNotFoundException")
	}
	return &eks.DescribeAccessEntryOutput{AccessEntry: &ekstypes.AccessEntry{}}, nil
}

func (m *mockEKSAPI) CreateAccessEntry(_ context.Context, _ *eks.CreateAccessEntryInput, _ ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error) {
	m.createAccessEntryCalls++
	return &eks.CreateAccessEntryOutput{}, nil
}

func (m *mockEKSAPI) AssociateAccessPolicy(_ context.Context, _ *eks.AssociateAccessPolicyInput, _ ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error) {
	m.associateAccessPolicyCalls++
	return &eks.AssociateAccessPolicyOutput{}, nil
}

func (m *mockEKSAPI) ListClusters(ctx context.Context, req *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var names []string
	for _, cluster := range m.clusters {
		names = append(names, aws.ToString(cluster.Name))
	}
	return &eks.ListClustersOutput{
		Clusters: names,
	}, nil
}

func (m *mockEKSAPI) DescribeCluster(_ context.Context, req *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	for _, cluster := range m.clusters {
		if aws.ToString(cluster.Name) == aws.ToString(req.Name) {
			return &eks.DescribeClusterOutput{
				Cluster: cluster,
			}, nil
		}
	}
	return nil, errors.New("cluster not found")
}

var eksMockClusters = []*ekstypes.Cluster{
	{
		Name:   aws.String("cluster1"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "prod",
			"location": "eu-west-1",
		},
	},
	{
		Name:   aws.String("cluster2"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster2"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "prod",
			"location": "eu-west-1",
		},
	},

	{
		Name:   aws.String("cluster3"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster3"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "stg",
			"location": "eu-west-1",
		},
	},
	{
		Name:   aws.String("cluster4"),
		Arn:    aws.String("arn:aws:eks:eu-west-1:accountID:cluster/cluster1"),
		Status: ekstypes.ClusterStatusActive,
		Tags: map[string]string{
			"env":      "stg",
			"location": "eu-west-1",
		},
	},
}

func eksClustersToResources(t *testing.T, clusters ...*ekstypes.Cluster) types.ResourcesWithLabels {
	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		kubeCluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(cluster.Name), aws.ToString(cluster.Arn), cluster.Tags)
		require.NoError(t, err)
		require.True(t, kubeCluster.IsAWS())
		common.ApplyEKSNameSuffix(kubeCluster)
		kubeClusters = append(kubeClusters, kubeCluster)
	}
	return kubeClusters.AsResources()
}

// mockRegionalEKSClientGetter dispatches EKS clients keyed by the region
// supplied to GetConfig.
type mockRegionalEKSClientGetter struct {
	mocks.AWSConfigProvider

	clientsByRegion map[string]EKSClient
}

func (g *mockRegionalEKSClientGetter) GetAWSEKSClient(cfg aws.Config) EKSClient {
	if c, ok := g.clientsByRegion[cfg.Region]; ok {
		return c
	}
	return &mockEKSAPI{}
}

func (g *mockRegionalEKSClientGetter) GetAWSSTSClient(aws.Config) STSClient {
	return &mockSTSClient{}
}

func (g *mockRegionalEKSClientGetter) GetAWSSTSPresignClient(aws.Config) kubeutils.STSPresignClient {
	return &mockSTSPresignAPI{}
}

// mockAccountClient implements account.ListRegionsAPIClient.
type mockAccountClient struct {
	output *account.ListRegionsOutput
	err    error
}

func (m *mockAccountClient) ListRegions(context.Context, *account.ListRegionsInput, ...func(*account.Options)) (*account.ListRegionsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// awsResponseError builds an AWS SDK v2 response error with the given HTTP
// status so libcloudaws.ConvertRequestFailureError translates it to the
// matching trace error.
func awsResponseError(status int, msg string) error {
	return &awshttp.ResponseError{
		RequestID: "test-request-id",
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
			Err:      errors.New(msg),
		},
	}
}
