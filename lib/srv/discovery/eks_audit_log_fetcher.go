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
	"log/slog"
	"time"

	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// cloudwatchlogFetcher fetches cloudwatch logs for a given cluster, starting
// at the given cursur position. This interface exists so tests can plug in a
// fake fetcher and not need to stub out deeper AWS interfaces.
type cloudwatchlogFetcher interface {
	FetchEKSAuditLogs(
		ctx context.Context,
		cluster *accessgraphv1alpha.AWSEKSClusterV1,
		cursor *accessgraphv1alpha.KubeAuditLogCursor,
	) ([]cwltypes.FilteredLogEvent, error)
}

// eksAuditLogFetcher is a fetcher for EKS audit logs for a single cluster,
// fetching the logs from AWS Cloud Watch Logs. It uses the grpc stream
// to initiate the stream and possibly receive a resume state used to
// synchronize the start point with a previous run fetching the logs.
type eksAuditLogFetcher struct {
	fetcher cloudwatchlogFetcher
	cluster *accessgraphv1alpha.AWSEKSClusterV1
	stream  accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient
	log     *slog.Logger
}

func newEKSAuditLogFetcher(
	fetcher cloudwatchlogFetcher,
	cluster *accessgraphv1alpha.AWSEKSClusterV1,
	stream accessgraphv1alpha.AccessGraphService_KubeAuditLogStreamClient,
	log *slog.Logger,
) *eksAuditLogFetcher {
	return &eksAuditLogFetcher{
		fetcher: fetcher,
		cluster: cluster,
		stream:  stream,
		log:     log,
	}
}

// Run continuously polls AWS Cloud Watch Logs for Kubernetes apiserver
// audit logs for the configured cluster. It feeds the logs retrieved to the
// configured grpc stream, running until the given context is canceled.
func (f *eksAuditLogFetcher) Run(ctx context.Context) error {
	f.log = f.log.With("cluster", f.cluster.Arn)

	cursor := initialCursor(f.cluster)
	if err := f.sendTAGKubeAuditLogNewStream(ctx, cursor); err != nil {
		return trace.Wrap(err)
	}

	cursor, err := f.receiveTAGKubeAuditLogResume(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for ctx.Err() == nil {
		var events []*structpb.Struct
		events, cursor = f.fetchLogs(ctx, cursor)

		if len(events) == 0 {
			select {
			case <-ctx.Done():
			case <-time.After(logPollInterval):
			}
			continue
		}

		if err := f.sendTAGKubeAuditLogEvents(ctx, events, cursor); err != nil {
			return trace.Wrap(err)
		}

		f.log.DebugContext(ctx, "Sent KubeAuditLogEvents", "count", len(events),
			"cursor_time", cursor.GetLastEventTime().AsTime())
	}
	return trace.Wrap(ctx.Err())
}

// fetchLogs fetches a batch of logs from AWS Cloud Watch Logs after the given
// cursor position and unmarshals them into the protobuf Struct well-known
// type.
//
// It returns the fetched log entries and a new cursor for the next call. If an
// error occurs, it is logged, and the function returns nil logs and the
// original input cursor. This allows the caller to retry the operation.
func (f *eksAuditLogFetcher) fetchLogs(ctx context.Context, cursor *accessgraphv1alpha.KubeAuditLogCursor) ([]*structpb.Struct, *accessgraphv1alpha.KubeAuditLogCursor) {
	awsEvents, err := f.fetcher.FetchEKSAuditLogs(ctx, f.cluster, cursor)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			f.log.ErrorContext(ctx, "Failed to fetch EKS audit logs", "error", err)
		}
		return nil, cursor
	}

	if len(awsEvents) == 0 {
		return nil, cursor
	}

	events := []*structpb.Struct{}
	var awsEvent cwltypes.FilteredLogEvent
	for _, awsEvent = range awsEvents {
		// TODO(camscale): Track event sizes and don't go over protobuf message
		// limit. newAccessGraphClient() sets the limit to 50MB
		event := &structpb.Struct{}
		m := protojson.UnmarshalOptions{}
		err = m.Unmarshal([]byte(*awsEvent.Message), event)
		if err != nil {
			f.log.ErrorContext(ctx, "failed to protojson.Unmarshal", "error", err)
			continue
		}
		events = append(events, event)
	}
	cursor = cursorFromEvent(f.cluster, awsEvent)

	return events, cursor
}

func (f *eksAuditLogFetcher) sendTAGKubeAuditLogNewStream(ctx context.Context, cursor *accessgraphv1alpha.KubeAuditLogCursor) error {
	err := f.stream.Send(
		&accessgraphv1alpha.KubeAuditLogStreamRequest{
			Action: &accessgraphv1alpha.KubeAuditLogStreamRequest_NewStream{
				NewStream: &accessgraphv1alpha.KubeAuditLogNewStream{Initial: cursor},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(f.stream)
		f.log.ErrorContext(ctx, "Failed to send accessgraph.KubeAuditLogNewStream", "error", err)
		return trace.Wrap(err)
	}
	return nil
}

func (f *eksAuditLogFetcher) receiveTAGKubeAuditLogResume(ctx context.Context) (*accessgraphv1alpha.KubeAuditLogCursor, error) {
	msg, err := f.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, "failed to receive KubeAuditLogStream resume state")
	}

	state := msg.GetResumeState()
	if state == nil {
		return nil, trace.BadParameter("AccessGraphService.KubeAuditLogStream did not return KubeAuditLogResumeState message")
	}

	f.log.InfoContext(ctx, "KubeAuditLogResumeState received", "state", state)
	return state.Cursor, nil
}

func (f *eksAuditLogFetcher) sendTAGKubeAuditLogEvents(ctx context.Context, events []*structpb.Struct, cursor *accessgraphv1alpha.KubeAuditLogCursor) error {
	err := f.stream.Send(
		&accessgraphv1alpha.KubeAuditLogStreamRequest{
			Action: &accessgraphv1alpha.KubeAuditLogStreamRequest_Events{
				Events: &accessgraphv1alpha.KubeAuditLogEvents{Events: events, Cursor: cursor},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(f.stream)
		f.log.ErrorContext(ctx, "Failed to send accessgraph.KubeAuditLogEvents", "error", err)
		return trace.Wrap(err)
	}
	return nil
}

func cursorFromEvent(cluster *accessgraphv1alpha.AWSEKSClusterV1, event cwltypes.FilteredLogEvent) *accessgraphv1alpha.KubeAuditLogCursor {
	return &accessgraphv1alpha.KubeAuditLogCursor{
		LogSource:     accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS,
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
		LogSource:     accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS,
		ClusterId:     cluster.Arn,
		LastEventTime: timestamppb.New(time.Now().UTC().Add(-initialLogBacklog)),
	}
}
