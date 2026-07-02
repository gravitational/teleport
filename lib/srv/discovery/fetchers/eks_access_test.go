/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"net/http"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestEKSAccessRetriesIdentity verifies that a transient
// sts:GetCallerIdentity failure does not permanently disable access setup.
func TestEKSAccessRetriesIdentity(t *testing.T) {
	const resolvedARN = "arn:aws:iam::123456789012:role/discovery"
	sts := &mockSTSClient{arn: resolvedARN, failCalls: 1}
	mgr := newTestAccessManager(t, nil, sts)
	cluster := testDiscoveredEKSCluster(t, testEKSCluster(ekstypes.AuthenticationModeConfigMap), types.AWSMatcher{})

	require.Nil(t, mgr.Provision(context.Background(), cluster))

	status := mgr.Provision(context.Background(), cluster)
	require.NotNil(t, status)
	require.Equal(t, resolvedARN, status.Discovery.Aws.SetupAccessForArn)
	require.Equal(t, 2, sts.calls)
}

// TestEKSAccessReresolvesIdentity verifies ProvisionAll re-resolves the
// ambient caller identity each cycle instead of pinning it for the process lifetime, so a
// changed deployment identity is picked up on a later cycle.
func TestEKSAccessReresolvesIdentity(t *testing.T) {
	sts := &mockSTSClient{arn: "arn:aws:iam::123456789012:role/discovery"}
	mgr := newTestAccessManager(t, nil, sts)
	clusters := []*DiscoveredEKSCluster{
		testDiscoveredEKSCluster(t, testEKSCluster(ekstypes.AuthenticationModeConfigMap), types.AWSMatcher{}),
	}

	mgr.ProvisionAll(context.Background(), clusters)
	mgr.ProvisionAll(context.Background(), clusters)

	require.Equal(t, 2, sts.calls)
}

// TestEKSAccessIdentityLookupRegion verifies the identity lookup uses the
// cluster's region.
func TestEKSAccessIdentityLookupRegion(t *testing.T) {
	getter := &mockRegionalEKSClientGetterWithSTS{
		mockRegionalEKSClientGetter: mockRegionalEKSClientGetter{
			AWSConfigProvider: mocks.AWSConfigProvider{},
		},
		stsClient: &mockSTSClient{arn: "arn:aws:iam::123456789012:role/discovery"},
	}
	mgr, err := NewEKSAccessManager(getter, logtest.NewLogger())
	require.NoError(t, err)
	cluster := testDiscoveredEKSCluster(t, testEKSCluster(ekstypes.AuthenticationModeConfigMap), types.AWSMatcher{})

	require.NotNil(t, mgr.Provision(context.Background(), cluster))
	require.Equal(t, "eu-west-1", getter.stsRegion)
}

// TestEKSAccessSkipsWithoutBootstrap verifies that the manager
// inspects the access entry but does not provision admin access when the
// bootstrap ARN is unresolved.
func TestEKSAccessSkipsWithoutBootstrap(t *testing.T) {
	const principalARN = "arn:aws:iam::123456789012:role/operator"
	eksClient := &mockEKSAPI{accessEntryNotFound: true}
	mgr := newTestAccessManager(t,
		map[string]EKSClient{"eu-west-1": eksClient},
		&mockSTSClient{arn: "unused", failCalls: 100},
	)
	cluster := testDiscoveredEKSCluster(t, testEKSCluster(ekstypes.AuthenticationModeApi),
		types.AWSMatcher{SetupAccessForARN: principalARN})

	status := mgr.Provision(context.Background(), cluster)
	require.NotNil(t, status)
	require.Equal(t, principalARN, status.Discovery.Aws.SetupAccessForArn)

	require.Equal(t, 1, eksClient.describeAccessEntryCalls)
	require.Zero(t, eksClient.createAccessEntryCalls)
	require.Zero(t, eksClient.associateAccessPolicyCalls)
}

// TestEKSAccessDeletesTemporaryEntry verifies the access entry created for the
// bootstrap ARN is deleted afterwards, while a pre-existing entry is left intact.
func TestEKSAccessDeletesTemporaryEntry(t *testing.T) {
	cluster := &ekstypes.Cluster{Name: aws.String("test-cluster")}

	t.Run("created entry is deleted", func(t *testing.T) {
		client := &mockEKSAccessAPI{}
		setup := &eksAccessSetup{
			eks:          client,
			bootstrapARN: "arn:aws:iam::123456789012:role/bootstrap",
			logger:       logtest.NewLogger(),
		}
		// Ensure the deferred deletion runs after RBAC setup fails.
		err := setup.temporarilyGainAdminAccessAndCreateRole(context.Background(), cluster)
		require.Error(t, err)
		require.Equal(t, 1, client.deleteAccessEntryCalls)
	})

	t.Run("pre-existing entry is left intact", func(t *testing.T) {
		client := &mockEKSAccessAPI{createAlreadyExists: true}
		setup := &eksAccessSetup{
			eks:          client,
			bootstrapARN: "arn:aws:iam::123456789012:role/bootstrap",
			logger:       logtest.NewLogger(),
		}
		err := setup.temporarilyGainAdminAccessAndCreateRole(context.Background(), cluster)
		require.Error(t, err)
		require.Zero(t, client.deleteAccessEntryCalls)
	})
}

// TestEKSAccessNilCAGuard verifies that a cluster missing its certificate
// authority yields a clear error instead of panicking.
func TestEKSAccessNilCAGuard(t *testing.T) {
	setup := &eksAccessSetup{
		stsPresign: &fakeSTSPresignAPI{url: "https://sts.amazonaws.com/"},
		logger:     logtest.NewLogger(),
	}
	_, err := setup.createKubeClient(context.Background(), &ekstypes.Cluster{
		Name: aws.String("test-cluster"),
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %T: %v", err, err)
}

// TestEKSAccessProvisionConcurrency verifies ProvisionAll runs cluster setup
// concurrently up to provisionConcurrency, and that the concurrent ambient identity
// resolution it triggers is race-free under -race.
func TestEKSAccessProvisionConcurrency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// More clusters than the limit, so some must block on the limiter and the
		// observed peak is the cap rather than the cluster count.
		const clusterCount = provisionConcurrency + 2
		eksClient := &concurrencyRecordingEKSAPI{release: make(chan struct{})}
		mgr := newTestAccessManager(t,
			map[string]EKSClient{"eu-west-1": eksClient},
			&mockSTSClient{arn: "arn:aws:iam::123456789012:role/discovery"},
		)
		clusters := make([]*DiscoveredEKSCluster, clusterCount)
		for i := range clusters {
			clusters[i] = testDiscoveredEKSCluster(t, testEKSCluster(ekstypes.AuthenticationModeApi), types.AWSMatcher{})
		}

		var statuses []*types.KubernetesClusterStatus
		done := make(chan struct{})
		go func() {
			statuses = mgr.ProvisionAll(context.Background(), clusters)
			close(done)
		}()

		// Once every goroutine is durably blocked, provisionConcurrency workers are
		// parked in DescribeAccessEntry and the rest wait on the errgroup limiter.
		synctest.Wait()
		peak := int(eksClient.inFlight.Load())

		close(eksClient.release)
		<-done

		require.Equal(t, provisionConcurrency, peak)
		require.Len(t, statuses, clusterCount)
		for _, status := range statuses {
			require.NotNil(t, status)
		}
	})
}

// concurrencyRecordingEKSAPI blocks each DescribeAccessEntry on release so the test can
// observe how many run at once. It returns an entry that already grants
// teleportKubernetesGroup, so each Provision stops at the describe instead of running
// RBAC setup.
type concurrencyRecordingEKSAPI struct {
	EKSClient

	inFlight atomic.Int64
	release  chan struct{}
}

func (m *concurrencyRecordingEKSAPI) DescribeAccessEntry(_ context.Context, _ *eks.DescribeAccessEntryInput, _ ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error) {
	m.inFlight.Add(1)
	defer m.inFlight.Add(-1)
	<-m.release
	return &eks.DescribeAccessEntryOutput{
		AccessEntry: &ekstypes.AccessEntry{KubernetesGroups: []string{teleportKubernetesGroup}},
	}, nil
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

func newTestAccessManager(t *testing.T, clients map[string]EKSClient, sts *mockSTSClient) *EKSAccessManager {
	t.Helper()
	mgr, err := NewEKSAccessManager(&mockRegionalEKSClientGetterWithSTS{
		mockRegionalEKSClientGetter: mockRegionalEKSClientGetter{
			AWSConfigProvider: mocks.AWSConfigProvider{},
			clientsByRegion:   clients,
		},
		stsClient: sts,
	}, logtest.NewLogger())
	require.NoError(t, err)
	return mgr
}

// testDiscoveredEKSCluster builds the wrapper the fetcher would produce: a
// matched kube cluster, the describe output it was built from, and the matcher's
// access intent.
func testDiscoveredEKSCluster(t *testing.T, awsCluster *ekstypes.Cluster, matcher types.AWSMatcher) *DiscoveredEKSCluster {
	t.Helper()
	kube, err := common.NewKubeClusterFromAWSEKS(aws.ToString(awsCluster.Name), aws.ToString(awsCluster.Arn), awsCluster.Tags)
	require.NoError(t, err)
	return &DiscoveredEKSCluster{
		KubeCluster:       kube,
		awsCluster:        awsCluster,
		AssumeRole:        matcher.AssumeRole,
		SetupAccessForARN: matcher.SetupAccessForARN,
		Integration:       matcher.Integration,
	}
}

type mockRegionalEKSClientGetterWithSTS struct {
	mockRegionalEKSClientGetter
	stsClient STSClient
	stsRegion string
}

func (g *mockRegionalEKSClientGetterWithSTS) GetAWSSTSClient(cfg aws.Config) STSClient {
	g.stsRegion = cfg.Region
	return g.stsClient
}

// fakeSTSPresignAPI returns a presigned request with a fixed URL, so a
// Kubernetes auth token can be generated without contacting AWS.
type fakeSTSPresignAPI struct {
	url string
}

func (a *fakeSTSPresignAPI) PresignGetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{URL: a.url}, nil
}

// mockEKSAccessAPI records the access-entry deletions made while gaining
// temporary admin access.
type mockEKSAccessAPI struct {
	EKSClient

	createAlreadyExists bool

	deleteAccessEntryCalls int
}

func (m *mockEKSAccessAPI) CreateAccessEntry(_ context.Context, _ *eks.CreateAccessEntryInput, _ ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error) {
	if m.createAlreadyExists {
		return nil, awsResponseError(http.StatusConflict, "ResourceInUseException")
	}
	return &eks.CreateAccessEntryOutput{}, nil
}

func (m *mockEKSAccessAPI) AssociateAccessPolicy(_ context.Context, _ *eks.AssociateAccessPolicyInput, _ ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error) {
	return &eks.AssociateAccessPolicyOutput{}, nil
}

func (m *mockEKSAccessAPI) DeleteAccessEntry(_ context.Context, _ *eks.DeleteAccessEntryInput, _ ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error) {
	m.deleteAccessEntryCalls++
	return &eks.DeleteAccessEntryOutput{}, nil
}
