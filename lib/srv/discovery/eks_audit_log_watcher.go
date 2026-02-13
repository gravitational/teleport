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
	"time"

	"github.com/gravitational/trace"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
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

type eksAuditLogFetcherRunner interface {
	Run(context.Context) error
}

// eksAuditLogWatcher is a watcher that waits for notifications on a channel
// indicating what EKS clusters should have audit logs fetched, and reconciles
// that against what is currently being fetched. Fetchers are started and
// stopped in response to this reconcilliation.
type eksAuditLogWatcher struct {
	client             accessgraphv1alpha.AccessGraphServiceClient
	log                *slog.Logger
	auditLogClustersCh chan []eksAuditLogCluster

	// Fetchers tracks the cluster IDs (ARN) of the clusters for which we
	// have a fetcher running. The value is a CancelFunc that is called to
	// stop the fetcher.
	fetchers    map[string]context.CancelFunc
	completedCh chan fetcherCompleted

	// newFetcher is a function used to construct a new fetcher. It exists
	// so tests can override it to not create real fetchers.
	newFetcher func(
		*aws_sync.Fetcher,
		*accessgraphv1alpha.AWSEKSClusterV1,
		accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient,
		*slog.Logger,
	) eksAuditLogFetcherRunner
}

func newEKSAuditLogWatcher(
	client accessgraphv1alpha.AccessGraphServiceClient,
	logger *slog.Logger,
) *eksAuditLogWatcher {
	return &eksAuditLogWatcher{
		client:             client,
		log:                logger,
		auditLogClustersCh: make(chan []eksAuditLogCluster),
		fetchers:           make(map[string]context.CancelFunc),
		completedCh:        make(chan fetcherCompleted),
	}
}

// fetcherCompleted captures the result of a completed eksAuditLogFetcher.
type fetcherCompleted struct {
	clusterID string
	err       error
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
	w.log.InfoContext(ctx, "EKS Audit Log Watcher started")
	defer w.log.InfoContext(ctx, "EKS Audit Log Watcher completed")

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

// Reconcile triggers a reconcilliation of currently running fetchers against
// the given slice of clusters. The reconcilliation will stop any fetchers for
// clusters not in the slice and start any fetchers for clusters in the slice
// that are not running.
//
// If the given context is done before the clusters can be sent to the
// reconcilliation goroutine, the context's error will be returned.
func (w *eksAuditLogWatcher) Reconcile(ctx context.Context, clusters []eksAuditLogCluster) error {
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case w.auditLogClustersCh <- clusters:
	}

	return nil
}

// reconcile compares the given slice of clusters against the currently running
// log fetchers and stops any running fetchers not in the cluster slice and
// starts a log fetcher for any cluster in the slice that does not have a
// running log fetcher.
//
// Log fetchers that are started are initialized with the given grpc stream
// over which they should send their audit logs.
func (w *eksAuditLogWatcher) reconcile(ctx context.Context, clusters []eksAuditLogCluster, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient) {
	w.log.DebugContext(ctx, "Reconciling EKS audit log clusters", "new_count", len(clusters))

	// Make a map of the discovered clusters, keyed by ARN so we can compare against
	// the existing clusters we are fetching audit logs for.
	discoveredClusters := make(map[string]eksAuditLogCluster)
	for _, discovered := range clusters {
		discoveredClusters[discovered.cluster.Arn] = discovered
	}
	// Stop any fetchers for clusters we are running fetcher for that discovery did not return.
	for arn, fetcherCancel := range mapDifference(w.fetchers, discoveredClusters) {
		w.log.InfoContext(ctx, "Stopping eksKubeAuditLogFetcher", "cluster", arn)
		fetcherCancel()
		// cleanup will happen when the fetcher finishes and is put on the completed channel.
	}
	// Start any new fetchers for clusters we are not running that discovery returned.
	for arn, discovered := range mapDifference(discoveredClusters, w.fetchers) {
		w.log.InfoContext(ctx, "Starting eksKubeAuditLogFetcher", "cluster", arn)
		ctx, cancel := context.WithCancel(ctx)
		var logFetcher eksAuditLogFetcherRunner
		if w.newFetcher == nil {
			logFetcher = newEKSAuditLogFetcher(discovered.fetcher, discovered.cluster, stream, w.log)
		} else {
			// the pluggable newFetcher is for testing purposes
			logFetcher = w.newFetcher(discovered.fetcher, discovered.cluster, stream, w.log)
		}
		w.fetchers[arn] = cancel
		go func() {
			err := logFetcher.Run(ctx)
			select {
			case w.completedCh <- fetcherCompleted{arn, err}:
			case <-ctx.Done():
			}
		}()
	}
}

// complete cleans up the maintained list of running log fetchers, removing the
// given completed fetcher, and logs the completion status of the fetcher.
func (w *eksAuditLogWatcher) complete(ctx context.Context, completed fetcherCompleted) {
	arn := completed.clusterID
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
		return nil, trace.Wrap(err, "failed to receive KubeAuditLogStream config")
	}

	config := msg.GetConfig()
	if config == nil {
		return nil, trace.BadParameter("AccessGraphService.KubeAuditLogStream did not return KubeAuditLogConfig message")
	}

	return config, nil
}
