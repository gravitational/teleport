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
	"cmp"
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

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

		s.Log.DebugContext(ctx, "EKS Audit Log Fetcher started")
		err := s.startEKSAuditLogFetchers(ctx, reloadCh, eksAuditLogClustersCh)
		if errors.Is(err, errTAGFeatureNotEnabled) {
			break
		} else if err != nil {
			s.Log.WarnContext(ctx, "Error initializing and watching access graph eks fetchers", "error", err)
		}
		s.Log.DebugContext(ctx, "EKS Audit Log Fetcher stopped")

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

// startEKSAuditLogFetchers
func (s *Server) startEKSAuditLogFetchers(ctx context.Context, reloadCh <-chan struct{}, eksAuditLogClustersCh <-chan []eksAuditLogCluster) error {
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
	// This prevents multiple discovery services to push AWS resources in parallel.
	// lease must be released to cleanup the resource in auth server.
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

	config := s.Config.AccessGraphConfig

	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.GetClientCert,
		config,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer accessGraphConn.Close()

	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	stream, err := client.KubeAuditLogStream(ctx)
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get access graph service KubeAuditLogStream", "error", err)
		return trace.Wrap(err)
	}
	err = stream.Send(
		&accessgraphv1alpha.KubeAuditLogStreamRequest{
			Action: &accessgraphv1alpha.KubeAuditLogStreamRequest_Config{
				Config: &accessgraphv1alpha.KubeAuditLogConfig{},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(stream)
		s.Log.ErrorContext(ctx, "Failed to send access graph config", "error", err)
		return trace.Wrap(err)
	}

	if err := s.receiveTAGKubeAuditLogConfig(ctx, stream); err != nil {
		return trace.Wrap(err)
	}

	resumeCursors, err := s.receiveTAGKubeAuditLogResume(ctx, stream)
	if err != nil {
		return trace.Wrap(err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.InfoContext(ctx, "Access graph service connection was closed")
		}
	}()

	auditLogClusters := make([]eksAuditLogCluster, 0)

	type fetcherTracker struct {
		fetcher    *aws_sync.Fetcher
		cancelFunc context.CancelFunc
	}
	fetchers := make(map[string]fetcherTracker)
	startEKSAuditLogFetcher := func(ctx context.Context, e eksAuditLogCluster) {
		ctx, cancel := context.WithCancel(ctx)
		fetchers[e.cluster.Arn] = fetcherTracker{e.fetcher, cancel}
		cursor := cmp.Or(resumeCursors[e.cluster.Arn], initialCursor(e.cluster))
		go s.runKubeAuditLogFetcher(ctx, stream, e.fetcher, e.cluster, cursor)
	}

	// Loop waiting for EKS clusters we need to fetch audit logs for on
	// s.awsKubeAuditLogClustersCh channel (from the resource syncer).
	// Reconcile that list of clusters against what we know and start/stop
	// any log fetchers necessary.
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-reloadCh:
			// Do nothing I think. If a reload adds or removes a dynamic config
			// for collecting audit logs, we'll get that via the awsAuditLogClustersCh
			// channel when the cluster is discovered.
			// TODO(camscale): Get rid of this - we don't need it. Or perhaps if
			// there are no fetchers with EKS logs fetching configured, we just
			// return and let the outer loop in initEKSAuditLogFetchers take over.
		case clusters := <-eksAuditLogClustersCh:
			// TODO(camscale): Reconcile clusters against previously received
			// clusters, starting new fetchers and stopping old ones.
			// This is needed for dynamic discovery resources.
			// For now, we just accept the first set of clusters only, as the
			// static config should not change.
			logDiscoveredClusters(ctx, s.Log, clusters)
			if len(auditLogClusters) == 0 {
				for _, cluster := range clusters {
					startEKSAuditLogFetcher(ctx, cluster)
				}
				auditLogClusters = clusters
			}
		}
	}
}

// logDiscoveredClusters is temporary until cluster reconciliation is done.
func logDiscoveredClusters(ctx context.Context, l *slog.Logger, clusters []eksAuditLogCluster) {
	l.InfoContext(ctx, "Received clusters from resource syncer")
	for _, cluster := range clusters {
		l.InfoContext(ctx, "Discovered cluster", "cluster", cluster.cluster.Arn)
	}
}

func (s *Server) receiveTAGKubeAuditLogConfig(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient) error {
	msg, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, " failed to receive KubeAuditLogStream config")
	}

	config := msg.GetConfig()
	if config == nil {
		return trace.BadParameter("AccessGraphService.KubeAuditLogStream did not return KubeAuditLogConfig message")
	}

	s.Log.InfoContext(ctx, "KubeAuditLogConfig received", "config", config)
	return nil
}

func (s *Server) receiveTAGKubeAuditLogResume(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient) (map[string]*accessgraphv1alpha.KubeAuditLogCursor, error) {
	msg, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, " failed to receive KubeAuditLogStream resume state")
	}

	state := msg.GetResumeState()
	if state == nil {
		return nil, trace.BadParameter("AccessGraphService.KubeAuditLogStream did not return KubeAuditLogResumeState message")
	}

	resumeCursors := make(map[string]*accessgraphv1alpha.KubeAuditLogCursor)
	for _, cursor := range state.Cursors {
		if cursor.LogSource != accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_AWS_CLOUDWATCH {
			continue
		}
		if resumeCursors[cursor.ClusterId] != nil {
			s.Log.WarnContext(ctx, "Duplicate resume cursor. Ignoring", "cluster_id", cursor.ClusterId)
			continue
		}
		resumeCursors[cursor.ClusterId] = cursor
	}

	s.Log.InfoContext(ctx, "KubeAuditLogResumeState received", "resume_state", state)
	return resumeCursors, nil
}

func (s *Server) runKubeAuditLogFetcher(
	ctx context.Context,
	stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient,
	fetcher *aws_sync.Fetcher,
	cluster *accessgraphv1alpha.AWSEKSClusterV1,
	cursor *accessgraphv1alpha.KubeAuditLogCursor,
) error {
	s.Log.InfoContext(ctx, "Starting EKS KubeAuditLog fetcher", "cluster", cluster.Arn)

	for {
		var events []*structpb.Struct
		events, cursor = s.fetchEKSAuditLogs(ctx, fetcher, cluster, cursor)

		if len(events) == 0 {
			select {
			case <-ctx.Done():
				if !errors.Is(ctx.Err(), context.Canceled) {
					return trace.Wrap(ctx.Err())
				}
				return nil
			case <-time.After(logPollInterval):
			}
			continue
		}

		err := stream.Send(
			&accessgraphv1alpha.KubeAuditLogStreamRequest{
				Action: &accessgraphv1alpha.KubeAuditLogStreamRequest_Events{
					Events: &accessgraphv1alpha.KubeAuditLogEvents{
						Events: events,
						Cursor: cursor,
					},
				},
			},
		)
		if err != nil {
			err = consumeTillErr(stream)
			if !errors.Is(err, context.Canceled) {
				s.Log.ErrorContext(ctx, "failed to send KubeAuditLogEvents", "error", err)
				return trace.Wrap(err)
			}
			return nil
		}
		s.Log.DebugContext(ctx, "Sent KubeAuditLogEvents", "count", len(events),
			"cursor_time", cursor.LastEventTime.AsTime().Format(time.RFC3339))
	}
}

func (s *Server) fetchEKSAuditLogs(
	ctx context.Context,
	fetcher *aws_sync.Fetcher,
	cluster *accessgraphv1alpha.AWSEKSClusterV1,
	cursor *accessgraphv1alpha.KubeAuditLogCursor,
) ([]*structpb.Struct, *accessgraphv1alpha.KubeAuditLogCursor) {
	awsEvents, err := fetcher.FetchEKSAuditLogs(ctx, cluster, cursor)
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to fetch EKS audit logs", "error", err)
		return nil, cursor
	}

	if len(awsEvents) == 0 {
		return nil, cursor
	}

	events := []*structpb.Struct{}
	var awsEvent cwltypes.FilteredLogEvent
	for _, awsEvent = range awsEvents {
		event := &structpb.Struct{}
		m := protojson.UnmarshalOptions{}
		err = m.Unmarshal([]byte(*awsEvent.Message), event)
		if err != nil {
			s.Log.ErrorContext(ctx, "failed to protojson.Unmarshal", "error", err)
			continue
		}
		events = append(events, event)
	}
	cursor = cursorFromEvent(cluster, awsEvent)

	return events, cursor
}

func cursorFromEvent(cluster *accessgraphv1alpha.AWSEKSClusterV1, event cwltypes.FilteredLogEvent) *accessgraphv1alpha.KubeAuditLogCursor {
	return &accessgraphv1alpha.KubeAuditLogCursor{
		LogSource:     accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_AWS_CLOUDWATCH,
		ClusterId:     cluster.Arn,
		EventId:       *event.EventId,
		LastEventTime: timestamppb.New(time.UnixMilli(*event.Timestamp)),
	}
}

// initialCursor returns a cursor for a EKS cluster that we have not previously
// retrieved logs from, so there is no resume state. The cursor is set to
// have logs retrieved back a standard amount of time.
func initialCursor(cluster *accessgraphv1alpha.AWSEKSClusterV1) *accessgraphv1alpha.KubeAuditLogCursor {
	return &accessgraphv1alpha.KubeAuditLogCursor{
		LogSource:     accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_AWS_CLOUDWATCH,
		ClusterId:     cluster.Arn,
		LastEventTime: timestamppb.New(time.Now().UTC().Add(-initialLogBacklog)),
	}
}
