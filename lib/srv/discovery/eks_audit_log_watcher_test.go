/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
	"github.com/gravitational/teleport/lib/utils/testutils/grpctest"
)

type (
	kalsRequest  = accessgraphv1alpha.KubeAuditLogStreamRequest
	kalsResponse = accessgraphv1alpha.KubeAuditLogStreamResponse
	kalsClient   = grpc.BidiStreamingClient[kalsRequest, kalsResponse]
	kalsServer   = grpc.BidiStreamingServer[kalsRequest, kalsResponse]
)

func TestEKSAuditLogWatcher_Init(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		client := newFakeKubeAuditLogClient(ctx)
		watcher := newEKSAuditLogWatcher(client, slog.New(slog.DiscardHandler))
		var err error
		go func() { err = watcher.Run(ctx) }()

		// Receive a config request.
		req, err := client.serverStream.Recv()
		require.NoError(t, err)
		require.NotNil(t, req.GetConfig())

		// Send back a config response with the config we received.
		err = client.serverStream.Send(newKubeAuditLogResponseConfig(req.GetConfig()))
		require.NoError(t, err)

		cancel()
		synctest.Wait()
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestEKSAuditLogWatcher_Reconcile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		fetcherTracker := newFakeFetcherTracker()
		client := newFakeKubeAuditLogClient(ctx)
		watcher := newEKSAuditLogWatcher(client, slog.New(slog.DiscardHandler))
		watcher.newFetcher = fetcherTracker.newFetcher
		var err error
		go func() { err = watcher.Run(ctx) }()

		// Receive a config request.
		req, err := client.serverStream.Recv()
		require.NoError(t, err)
		require.NotNil(t, req.GetConfig())

		// Send back a config response with the config we received.
		err = client.serverStream.Send(newKubeAuditLogResponseConfig(req.GetConfig()))
		require.NoError(t, err)

		// Send a single cluster1 to the watcher to reconcile
		cluster1 := &accessgraphv1alpha.AWSEKSClusterV1{Arn: "test-arn"}
		fetcher1 := &aws_sync.Fetcher{}
		watcher.Reconcile(ctx, []eksAuditLogCluster{{fetcher1, cluster1}})
		synctest.Wait()

		// Verify that a fetcher was started.
		f1, ok := fetcherTracker.fetchers["test-arn"]
		require.True(t, ok, "eksAuditLogFetcher not in fetcherTracker")
		require.True(t, f1.runCalled, "fetcher Run() was not called")
		require.Same(t, fetcher1, f1.fetcher)
		require.Same(t, cluster1, f1.cluster)
		require.Len(t, fetcherTracker.fetchers, 1)
		require.Equal(t, 1, fetcherTracker.newCount)

		// Add another cluster
		cluster2 := &accessgraphv1alpha.AWSEKSClusterV1{Arn: "test-arn2"}
		fetcher2 := &aws_sync.Fetcher{}
		watcher.Reconcile(ctx, []eksAuditLogCluster{{fetcher1, cluster1}, {fetcher2, cluster2}})
		synctest.Wait()

		// Verify that a fetcher was started.
		f2, ok := fetcherTracker.fetchers["test-arn2"]
		require.True(t, ok, "eksAuditLogFetcher not in fetcherTracker")
		require.True(t, f2.runCalled, "fetcher Run() was not called")
		require.Same(t, fetcher2, f2.fetcher)
		require.Same(t, cluster2, f2.cluster)
		require.Len(t, fetcherTracker.fetchers, 2)
		require.Equal(t, 2, fetcherTracker.newCount)

		// Drop back to a single cluster
		watcher.Reconcile(ctx, []eksAuditLogCluster{{fetcher1, cluster1}})
		synctest.Wait()
		require.Len(t, fetcherTracker.fetchers, 1)
		require.Equal(t, 2, fetcherTracker.newCount)
		require.True(t, f2.done)

		// Send an empty cluster list. Should stop last fetcher
		watcher.Reconcile(ctx, []eksAuditLogCluster{})
		synctest.Wait()
		require.Empty(t, fetcherTracker.fetchers)
		require.Equal(t, 2, fetcherTracker.newCount)
		require.True(t, f1.done)

		cancel()
		synctest.Wait()
		require.ErrorIs(t, err, context.Canceled)
	})
}

// fakeFetcherTracker keeps track of the fetchers created by an
// eksAuditLogWatcher. It has a newFetcher method that can plug into a watcher
// so that real fetchers are not created, and returns a fake fetcher for
// testing purposes.
type fakeFetcherTracker struct {
	fetchers map[string]*fakeEksAuditLogFetcher
	newCount int
}

func newFakeFetcherTracker() *fakeFetcherTracker {
	return &fakeFetcherTracker{fetchers: make(map[string]*fakeEksAuditLogFetcher)}
}

// newFetcher plugs into eksAuditLogWatcher.newFetcher so that it creates fake
// fetchers for testing purposes. We do not need real fetchers to test the
// watcher.
func (fft *fakeFetcherTracker) newFetcher(
	fetcher *aws_sync.Fetcher,
	cluster *accessgraphv1alpha.AWSEKSClusterV1,
	stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient,
	log *slog.Logger,
) eksAuditLogFetcherRunner {
	f := &fakeEksAuditLogFetcher{
		fetcher: fetcher,
		cluster: cluster,
		cleanup: func() { delete(fft.fetchers, cluster.Arn) },
	}
	fft.fetchers[cluster.Arn] = f
	fft.newCount++
	return f
}

type fakeEksAuditLogFetcher struct {
	fetcher   *aws_sync.Fetcher
	cluster   *accessgraphv1alpha.AWSEKSClusterV1
	cleanup   func()
	runCalled bool
	done      bool
}

func (f *fakeEksAuditLogFetcher) Run(ctx context.Context) error {
	f.runCalled = true // used in synctest bubble, no race
	<-ctx.Done()
	f.done = true
	f.cleanup()
	return ctx.Err()
}

func newKubeAuditLogResponseConfig(cfg *accessgraphv1alpha.KubeAuditLogConfig) *kalsResponse {
	return &kalsResponse{
		State: &accessgraphv1alpha.KubeAuditLogStreamResponse_Config{
			Config: cfg,
		},
	}
}

func newFakeKubeAuditLogClient(ctx context.Context) *fakeKubeAuditLogClient {
	tester := grpctest.NewGRPCTester[kalsRequest, kalsResponse](ctx)
	return &fakeKubeAuditLogClient{
		clientStream: tester.NewClientStream(),
		serverStream: tester.NewServerStream(),
	}
}

type fakeKubeAuditLogClient struct {
	accessgraphv1alpha.AccessGraphServiceClient

	clientStream kalsClient
	serverStream kalsServer
}

// Implements KubeAuditLogStream grpc method on the client
func (c *fakeKubeAuditLogClient) KubeAuditLogStream(ctx context.Context, opts ...grpc.CallOption) (kalsClient, error) {
	return c.clientStream, nil
}
