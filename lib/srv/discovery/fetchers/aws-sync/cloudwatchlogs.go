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

package aws_sync

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/gravitational/trace"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// FetchEKSAuditLogs returns a slice of audit log events for the given cluster
// starting from the given cursor.
func (a *Fetcher) FetchEKSAuditLogs(ctx context.Context, cluster *accessgraphv1alpha.AWSEKSClusterV1, cursor *accessgraphv1alpha.KubeAuditLogCursor) ([]cwltypes.FilteredLogEvent, error) {
	cfg, err := a.AWSConfigProvider.GetConfig(ctx, cluster.GetRegion(), a.getAWSOptions()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := a.awsClients.getCloudWatchLogsClient(cfg)

	// limit is not a hard limit - we may exceed it but won't get any more pages
	// once reached.
	var limit int32 = 500 // TODO(camscale): Consider making this a parameter
	startTime := cursor.GetLastEventTime().AsTime().UTC()
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        aws.String("/aws/eks/" + cluster.GetName() + "/cluster"),
		LogStreamNamePrefix: aws.String("kube-apiserver-audit-"),
		StartTime:           aws.Int64(startTime.UnixMilli()),
		Limit:               aws.Int32(limit),
	}

	var result []cwltypes.FilteredLogEvent
	for p := cloudwatchlogs.NewFilterLogEventsPaginator(client, input); p.HasMorePages(); {
		output, err := p.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		eventsAfterCursor := cwlEventsAfterCursor(output.Events, cursor)
		if eventsAfterCursor != nil {
			cursor = nil
			result = append(result, eventsAfterCursor...)
			if len(result) >= int(limit) {
				break
			}
		}
	}

	return result, nil
}

// cwlEventsAfterCursor returns the events from the given events after the
// cursor. If the cursor was not found, but the timestamp of the cursor
// was not passed, then nil is returned. In this case, the cursor is still
// valid and should continue to be used to find the next event. Otherwise
// a slice (possibly empty) is returned which means the cursor was consumed;
// either the event ID in the cursor was found, or we passed the timestamp
// of the cursor.
// If the cursor is nil, the events are returned unfiltered.
func cwlEventsAfterCursor(events []cwltypes.FilteredLogEvent, cursor *accessgraphv1alpha.KubeAuditLogCursor) []cwltypes.FilteredLogEvent {
	// If we're not looking for events from a cursor position, just return all events.
	if cursor == nil || cursor.GetEventId() == "" {
		return events
	}

	startTime := cursor.GetLastEventTime().AsTime().UTC()
	for i, event := range events {
		// If we never saw cursor.EventId with the given timestamp,
		// just return all the events.
		if time.UnixMilli(*event.Timestamp).UTC().After(startTime) {
			return events
		}
		if *event.EventId == cursor.GetEventId() {
			return events[i+1:]
		}
	}
	// The cursor was not found in the events, but it was not discarded as
	// the timestamp on the events did not move past the cursor timestamp.
	// A nil slice (as opposed to an empty slice) indicates this.
	return nil
}
