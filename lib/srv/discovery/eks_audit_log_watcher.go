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
	"errors"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
)

const (
	// initialLogBacklog is how far back to start retrieving EKS audit logs
	// for newly discovered clusters.
	// TODO(camscale): define in config.
	initialLogBacklog = 7 * 24 * time.Hour

	// logPollInterval is the amount of time to sleep between fetching audit
	// logs from Cloud Watch Logs for a cluster.
	// TODO(camscale): perhaps define in config.
	logPollInterval = 30 * time.Second
)

// eksAuditLogCluster is a cluster for which audit logs should be fetched and
// the fetcher to use to do that. It is sent over a channel as a slice from
// the AWS resource watcher (access_graph_aws.go) to the EKS audit log watcher
// created here. For each one of these received, an asynchronous log fetcher
// is spawned to fetch Kubernetes apiserver audit logs from Cloud Watch Logs
// and sent to the grpc AccessGraphService via the KubeAuditLogStream rpc.
type eksAuditLogCluster struct {
	fetcher *aws_sync.Fetcher
	cluster *accessgraphv1alpha.AWSEKSClusterV1
}

// initEKSAuditLogWatcher starts the EKS audit log watcher if there are any
// static or dynamic configurations specifying to collect EKS audit logs.
// If there are none, it waits for discover changes to the dynamic
// DiscoverConfig resources, and if any configurations for EKS audit logs
// appear, it then starts the EKS audit log watcher. If the EKS audit log
// watcher completes, we start it again if there is an active configuration.
func (s *Server) initEKSAuditLogWatcher(ctx context.Context, eksAuditLogClustersCh chan []eksAuditLogCluster) {
	reloadCh := s.newDiscoveryConfigChangedSub()

	for {
		if !s.hasAWSSyncEKSAuditLogFetcher() {
			s.Log.DebugContext(ctx, "No AWS sync fetchers with EKS Audit Logs configured. Access Graph EKS Audit Log sync will not be enabled.")
			select {
			case <-ctx.Done():
				return
			case <-reloadCh:
				// If the config changes, we need to re-evaluate the fetchers.
			}
			continue
		}

		s.Log.DebugContext(ctx, "EKS Audit Log Watcher started")
		err := s.startEKSAuditLogWatcher(ctx, eksAuditLogClustersCh)
		if errors.Is(err, errTAGFeatureNotEnabled) {
			break
		} else if err != nil {
			s.Log.WarnContext(ctx, "Error initializing and EKS Audit Log Watcher", "error", err)
		}
		s.Log.DebugContext(ctx, "EKS Audit Log Watcher stopped")

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		case <-reloadCh:
		}
	}
}

// hasAWSSyncEKSAuditLogFetcher returns true if there are any active
// configurations for EKS audit log syncing, either static or dynamic
// configurations.
func (s *Server) hasAWSSyncEKSAuditLogFetcher() bool {
	for _, fetcher := range s.staticTAGAWSFetchers {
		if fetcher.EKSAuditLogs != nil {
			return true
		}
	}

	s.muDynamicTAGAWSFetchers.RLock()
	defer s.muDynamicTAGAWSFetchers.RUnlock()
	for _, fetcherSet := range s.dynamicTAGAWSFetchers {
		for _, fetcher := range fetcherSet {
			if fetcher.EKSAuditLogs != nil {
				return true
			}
		}
	}
	return false
}

// startEKSAuditLogWatcher starts the watcher for EKS audit logs. It ensures
// that it can obtain a semaphore so multiple auth servers do not try to
// collect logs concurrently.
func (s *Server) startEKSAuditLogWatcher(ctx context.Context, eksAuditLogClustersCh <-chan []eksAuditLogCluster) error {
	clusterFeatures := s.Config.ClusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(errTAGFeatureNotEnabled)
	}

	// aws discovery semaphore lock.
	const (
		semaphoreName       = "access_graph_aws_eks_audit_log_sync"
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLock will retry until the semaphore is acquired.
	// This prevents multiple discovery services from collecting EKS
	// logs concurrently, duplicating the logs.
	// The lease must be released to cleanup the resource in auth server.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.AccessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAccessGraph,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.Config.ServerID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// once the lease parent context is canceled, the lease will be released.
	// this will stop the access graph sync.
	ctx, cancel := context.WithCancel(lease)
	defer cancel()

	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up semaphore", "error", err)
		}
	}()

	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.GetClientCert,
		s.Config.AccessGraphConfig,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer accessGraphConn.Close()

	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	// First wait for the connection to leave the Connecting state.

	if !accessGraphConn.WaitForStateChange(ctx, connectivity.Connecting) {
		return trace.Wrap(ctx.Err())
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.InfoContext(ctx, "Access graph service connection was closed")
		}
	}()

	watcher := newEksAuditLogWatcher(client, s.Log, eksAuditLogClustersCh)
	err = watcher.Run(ctx)
	return trace.Wrap(err)
}

// eksAuditLogWatcher is a watcher that waits for notifications on a channel
// indicating what EKS clusters should have audit logs fetched, and reconciles
// that against what is currently being fetched. Fetchers are started and
// stopped in response to this reconcilliation.
type eksAuditLogWatcher struct {
	client             accessgraphv1alpha.AccessGraphServiceClient
	log                *slog.Logger
	auditLogClustersCh <-chan []eksAuditLogCluster

	fetchers    map[string]*eksAuditLogFetcher
	completedCh chan fetcherCompleted
}

func newEksAuditLogWatcher(
	client accessgraphv1alpha.AccessGraphServiceClient,
	logger *slog.Logger,
	auditLogClustersCh <-chan []eksAuditLogCluster,
) *eksAuditLogWatcher {
	return &eksAuditLogWatcher{
		client:             client,
		log:                logger,
		auditLogClustersCh: auditLogClustersCh,
		fetchers:           make(map[string]*eksAuditLogFetcher),
		completedCh:        make(chan fetcherCompleted),
	}
}

// fetcherCompleted captures the result of a completed eksAuditLogFetcher.
type fetcherCompleted struct {
	fetcher *eksAuditLogFetcher
	err     error
}

// Run starts a watcher by creating a KubeAuditLogStream on its grpc client. It
// negotiates a configuration (currently a no-op) and starts a main loop
// waiting for a list of clusters that it should run log fetchers for. As these
// lists of clusters arrives, it reconciles it against the running log fetchers
// and starts/stops log fetchers as required to match the given list. It
// completes when the given context is done.
//
// If any errors occur initializing the grpc stream, it is returned and the
// main loop is not run.
func (w *eksAuditLogWatcher) Run(ctx context.Context) error {
	stream, err := w.client.KubeAuditLogStream(ctx)
	if err != nil {
		w.log.ErrorContext(ctx, "Failed to get access graph service KubeAuditLogStream", "error", err)
		return trace.Wrap(err)
	}

	config := &accessgraphv1alpha.KubeAuditLogConfig{}
	if err := sendTAGKubeAuditLogConfig(ctx, stream, config); err != nil {
		w.log.ErrorContext(ctx, "Failed to send access graph config", "error", err)
		return trace.Wrap(err)
	}

	config, err = receiveTAGKubeAuditLogConfig(ctx, stream)
	if err != nil {
		w.log.ErrorContext(ctx, "Failed to receive access graph config", "error", err)
		return trace.Wrap(err)
	}
	w.log.InfoContext(ctx, "KubeAuditLogConfig received", "config", config)

	// Loop waiting for EKS clusters we need to fetch audit logs for on
	// s.awsKubeAuditLogClustersCh channel (from the resource syncer).
	// Reconcile that list of clusters against what we know and start/stop
	// any log fetchers necessary.
	for {
		select {
		case clusters := <-w.auditLogClustersCh:
			w.reconcile(ctx, clusters, stream)
		case completed := <-w.completedCh:
			w.complete(ctx, completed)
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// reconcile compares the given slice of clusters against the currently running
// log fetchers and stops any running fetchers not in the cluster slice and
// starts a log fetcher for any cluster in the slice that does not have a
// running log fetcher.
//
// Log fetchers that are started are initialized with the given grpc stream
// over which they should send their audit logs.
func (w *eksAuditLogWatcher) reconcile(ctx context.Context, clusters []eksAuditLogCluster, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient) {
	// Make a map of the discovered clusters, keyed by ARN so we can compare against
	// the existing clusters we are fetching audit logs for.
	discoveredClusters := make(map[string]eksAuditLogCluster)
	for _, discovered := range clusters {
		discoveredClusters[discovered.cluster.Arn] = discovered
	}
	// Stop any fetchers for clusters we are running fetcher for that discovery did not return.
	for arn, existing := range mapDifference(w.fetchers, discoveredClusters) {
		w.log.InfoContext(ctx, "Stopping eksKubeAuditLogFetcher", "cluster", arn)
		existing.cancel()
		// cleanup will happen when the fetcher finishes and is put on the completed channel.
	}
	// Start any new fetchers for clusters we are not running that discovery returned.
	for arn, discovered := range mapDifference(discoveredClusters, w.fetchers) {
		w.log.InfoContext(ctx, "Starting eksKubeAuditLogFetcher", "cluster", arn)
		ctx, cancel := context.WithCancel(ctx)
		logFetcher := &eksAuditLogFetcher{
			fetcher: discovered.fetcher,
			cluster: discovered.cluster,
			stream:  stream,
			log:     w.log,
			cancel:  cancel,
		}
		w.fetchers[arn] = logFetcher
		go func() {
			err := logFetcher.Run(ctx)
			w.completedCh <- fetcherCompleted{logFetcher, err}
		}()
	}
}

// complete cleans up the maintained list of running log fetchers, removing the
// given completed fetcher, and logs the completion status of the fetcher.
func (w *eksAuditLogWatcher) complete(ctx context.Context, completed fetcherCompleted) {
	arn := completed.fetcher.cluster.Arn
	if completed.err != nil && !errors.Is(completed.err, context.Canceled) {
		w.log.ErrorContext(ctx, "eksKubeAuditLogFetcher completed with error", "cluster", arn, "error", completed.err)
	} else {
		w.log.InfoContext(ctx, "eksKubeAuditLogFetcher completed", "cluster", arn)
	}
	delete(w.fetchers, arn)
}

// mapDifference yields all keys and values of m1 where the key is not in m2. It
// can be considered the set operation "mapDifference" - m1-m2, yielding all
// elements of m1 not in m2.
func mapDifference[K comparable, V1 any, V2 any](m1 map[K]V1, m2 map[K]V2) iter.Seq2[K, V1] {
	return func(yield func(K, V1) bool) {
		for k, v := range m1 {
			if _, ok := m2[k]; !ok {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

func sendTAGKubeAuditLogConfig(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient, config *accessgraphv1alpha.KubeAuditLogConfig) error {
	err := stream.Send(
		&accessgraphv1alpha.KubeAuditLogStreamRequest{
			Action: &accessgraphv1alpha.KubeAuditLogStreamRequest_Config{Config: config},
		},
	)
	if err != nil {
		err = consumeTillErr(stream)
		return trace.Wrap(err)
	}
	return nil
}

func receiveTAGKubeAuditLogConfig(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient) (*accessgraphv1alpha.KubeAuditLogConfig, error) {
	msg, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, " failed to receive KubeAuditLogStream config")
	}

	config := msg.GetConfig()
	if config == nil {
		return nil, trace.BadParameter("AccessGraphService.KubeAuditLogStream did not return KubeAuditLogConfig message")
	}

	return config, nil
}
