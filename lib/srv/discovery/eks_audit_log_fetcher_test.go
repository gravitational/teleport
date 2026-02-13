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
	"fmt"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/utils/testutils/grpctest"
)

type eksAuditLogFetcherFixture struct {
	ctx            context.Context
	cancel         context.CancelFunc
	server         kalsServer
	fetcherErr     error
	cluster        *accessgraphv1alpha.AWSEKSClusterV1
	fakeLogFetcher *fakeCloudWatchLogFetcher
}

// Start the fixture. Must be called inside synctest bubble.
func (f *eksAuditLogFetcherFixture) Start(t *testing.T) {
	t.Helper()

	f.ctx, f.cancel = context.WithCancel(t.Context())
	tester := grpctest.NewGRPCTester[kalsRequest, kalsResponse](f.ctx)
	f.server = tester.NewServerStream()
	logger := slog.New(slog.DiscardHandler)
	f.fakeLogFetcher = newFakeCloudWatchLogFetcher()
	f.cluster = &accessgraphv1alpha.AWSEKSClusterV1{
		Name: "cluster-name",
		Arn:  "cluster-arn",
	}
	logFetcher := newEKSAuditLogFetcher(f.fakeLogFetcher, f.cluster, tester.NewClientStream(), logger)
	go func() { f.fetcherErr = logFetcher.Run(f.ctx) }()
}

// End the fixture. Must be called inside synctest bubble.
func (f *eksAuditLogFetcherFixture) End(t *testing.T) {
	t.Helper()
	f.cancel()
	synctest.Wait()
	require.ErrorIs(t, f.fetcherErr, context.Canceled)
}

func (f *eksAuditLogFetcherFixture) testInitializeNewStream(t *testing.T) {
	t.Helper()

	// Wait for a NewStream action, and verify it contains what we expect
	msg, err := f.server.Recv()
	require.NoError(t, err)
	newStream := msg.GetNewStream()
	require.NotNil(t, newStream)
	cursor := newStream.GetInitial()
	require.NotNil(t, cursor)
	require.Equal(t, accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS, cursor.GetLogSource())
	require.Equal(t, f.cluster.GetArn(), cursor.GetClusterId())

	// Send back a ResumeState
	err = f.server.Send(newKubeAuditLogResponseResumeState(cursor))
	require.NoError(t, err)
}

// TestEKSAuditLogFetcher_NewStream_Unknown tests that when a new log stream
// is set up for a cluster, logs start being fetched from the cursor returned
// by the grpc service.
func TestEKSAuditLogFetcher_NewStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &eksAuditLogFetcherFixture{}
		f.Start(t)
		f.testInitializeNewStream(t)
		f.End(t)
	})
}

func TestEKSAuditLogFetcher_Batching(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		startTime := time.Now().UTC()
		logEpoch := startTime.Add(-7 * 24 * time.Hour)
		f := &eksAuditLogFetcherFixture{}
		f.Start(t)
		f.testInitializeNewStream(t)

		f.fakeLogFetcher.events <- nil
		// Wait for a polling loop to occur. As there are no logs left,
		// the time should now be the synctest epoch plus the poll interval
		time.Sleep(logPollInterval)
		synctest.Wait()
		require.Equal(t, startTime.Add(logPollInterval), time.Now().UTC())

		// Wait for an Events action with the log listed. Verify the log and cursor.
		f.fakeLogFetcher.events <- []cwltypes.FilteredLogEvent{
			makeEvent(logEpoch, 0, "{}"),
			makeEvent(logEpoch.Add(time.Second), 1, `{"log": "value"}`),
		}
		msg, err := f.server.Recv()
		require.NoError(t, err)
		events := msg.GetEvents()
		require.NotNil(t, events)
		require.Len(t, events.GetEvents(), 2)
		require.Empty(t, events.GetEvents()[0].GetFields())
		require.Len(t, events.GetEvents()[1].GetFields(), 1)
		cursor := events.GetCursor()
		require.NotNil(t, cursor)
		require.Equal(t, accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS, cursor.GetLogSource())
		require.Equal(t, f.cluster.GetArn(), cursor.GetClusterId())
		require.Equal(t, "event-id-1", cursor.GetEventId())
		require.Equal(t, logEpoch.Add(time.Second), cursor.GetLastEventTime().AsTime())

		f.fakeLogFetcher.events <- []cwltypes.FilteredLogEvent{
			makeEvent(logEpoch.Add(time.Second), 2, `{"log": "value2"}`),
			makeEvent(logEpoch.Add(2*time.Second), 3, `{}`),
		}
		msg, err = f.server.Recv()
		require.NoError(t, err)
		events = msg.GetEvents()
		require.NotNil(t, events)
		require.Len(t, events.GetEvents(), 2)
		require.Len(t, events.GetEvents()[0].GetFields(), 1)
		require.Empty(t, events.GetEvents()[1].GetFields())
		cursor = events.GetCursor()
		require.NotNil(t, cursor)
		require.Equal(t, accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS, cursor.GetLogSource())
		require.Equal(t, f.cluster.GetArn(), cursor.GetClusterId())
		require.Equal(t, "event-id-3", cursor.GetEventId())
		require.Equal(t, logEpoch.Add(2*time.Second), cursor.GetLastEventTime().AsTime())

		f.End(t)
	})
}

func TestEKSAuditLogFetcher_ContinueOnError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		startTime := time.Now().UTC()
		logEpoch := startTime.Add(-7 * 24 * time.Hour)
		f := &eksAuditLogFetcherFixture{}
		f.Start(t)
		f.testInitializeNewStream(t)

		f.fakeLogFetcher.err <- errors.New("oh noes. something went wrong")
		// Wait for a polling loop to occur. As there are no logs left,
		// the time should now be the synctest epoch plus the poll interval
		time.Sleep(logPollInterval)
		synctest.Wait()
		require.Equal(t, startTime.Add(logPollInterval), time.Now().UTC())

		// Wait for an Events action with the log listed. Verify the log and cursor.
		f.fakeLogFetcher.events <- []cwltypes.FilteredLogEvent{
			makeEvent(logEpoch, 0, "{}"),
			makeEvent(logEpoch.Add(time.Second), 1, `{"log": "value"}`),
		}
		msg, err := f.server.Recv()
		require.NoError(t, err)
		events := msg.GetEvents()
		require.NotNil(t, events)
		require.Len(t, events.GetEvents(), 2)
		require.Empty(t, events.GetEvents()[0].GetFields())
		require.Len(t, events.GetEvents()[1].GetFields(), 1)
		cursor := events.GetCursor()
		require.NotNil(t, cursor)
		require.Equal(t, accessgraphv1alpha.KubeAuditLogCursor_KUBE_AUDIT_LOG_SOURCE_EKS, cursor.GetLogSource())
		require.Equal(t, f.cluster.GetArn(), cursor.GetClusterId())
		require.Equal(t, "event-id-1", cursor.GetEventId())
		require.Equal(t, logEpoch.Add(time.Second), cursor.GetLastEventTime().AsTime())

		f.End(t)
	})
}

func newKubeAuditLogResponseResumeState(cursor *accessgraphv1alpha.KubeAuditLogCursor) *kalsResponse {
	return &kalsResponse{
		State: &accessgraphv1alpha.KubeAuditLogStreamResponse_ResumeState{
			ResumeState: &accessgraphv1alpha.KubeAuditLogResumeState{
				Cursor: cursor,
			},
		},
	}
}

func makeEvent(t time.Time, id int, msg string) cwltypes.FilteredLogEvent {
	return cwltypes.FilteredLogEvent{
		EventId:       aws.String(fmt.Sprintf("event-id-%d", id)),
		IngestionTime: aws.Int64(t.UnixMilli()),
		Timestamp:     aws.Int64(t.UnixMilli()),
		LogStreamName: aws.String("kube-apiserver-audit-12345678"),
		Message:       aws.String(msg),
	}
}

func newFakeCloudWatchLogFetcher() *fakeCloudWatchLogFetcher {
	return &fakeCloudWatchLogFetcher{
		events: make(chan []cwltypes.FilteredLogEvent),
		err:    make(chan error),
	}
}

// fakeCloudWatchLogFetcher is a cloudwatch log fetcher that waits on channels
// for the data to return. This allows the unit under test to rendezvous with
// the tests, allowing the tests to advance the state of the fetcher as it
// needs.
type fakeCloudWatchLogFetcher struct {
	events chan []cwltypes.FilteredLogEvent
	err    chan error
}

func (f *fakeCloudWatchLogFetcher) FetchEKSAuditLogs(
	ctx context.Context,
	cluster *accessgraphv1alpha.AWSEKSClusterV1,
	cursor *accessgraphv1alpha.KubeAuditLogCursor,
) ([]cwltypes.FilteredLogEvent, error) {
	select {
	case events := <-f.events:
		return events, nil
	case err := <-f.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
