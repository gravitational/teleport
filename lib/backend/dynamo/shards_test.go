// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dynamo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	iterstream "github.com/gravitational/teleport/lib/itertools/stream"
)

// TestShardSplitting is an integration test that simulates a high load scenario on a DynamoDB table to trigger shard splits in DynamoDB Streams.
// Given this requires putting a substantial write throughput and the nature of the race condition it is made opt in by setting both `TELEPORT_DYNAMODB_SHARD_SPLIT_TEST`
// and `TELEPORT_DYNAMODB_TEST` env vars.
// Ideally this test should be ran from a ec2 instance in the same region as the DynamoDB table to avoid any throttling or latency issues from hitting the AWS API from a local machine.
// The test expects a fresh empty table on each run, and each event is written conditionally only once.
func TestShardSplitting(t *testing.T) {
	ensureTestsEnabled(t)
	if os.Getenv("TELEPORT_DYNAMODB_SHARD_SPLIT_TEST") == "" {
		t.Skip("DynamoDB TestShardSplitting test skipped.")
	}

	tests := []struct {
		name                string
		partitionCount      int
		writersPerPartition int
		writesPerWriter     int
		payloadSize         int
		eventTimeout        time.Duration
		backendConfig       map[string]any
		precondition        func(t *testing.T)
	}{
		// Each individual partition is limited to 1,000 write units per second and 3,000 read units per second.
		// Each WCU is 1 write per second for items up to 1KB in size, so to trigger shard splits with items of 350kB each partion
		// should trigger splits at around ~3 writes per second.
		{
			// Configuration for running the shard split test locally.
			// With a single partition and 50 writers writing 100 events each the test will write a total of 5,000 events
			// which should be sufficient to trigger at least 1 shard split. Note that due to uncontrolled latency
			// between local clients and AWS API this test may be flaky. For best results run the 'ec2' configuration.
			name:                "local",
			partitionCount:      1,
			writersPerPartition: 20,
			writesPerWriter:     100,
			payloadSize:         300 * 1024, // below 400KB DynamoDB limit
			eventTimeout:        20 * time.Minute,
			backendConfig: map[string]any{
				"table_name":         dynamoDBTestTable(),
				"poll_stream_period": 500 * time.Millisecond,
				"retry_period":       250 * time.Millisecond,
			},
		},
		{
			// On a fresh table and running in a dedicated ec2 instance this
			// test is expected to produce approximately 50 shard splits in ~10 minutes.
			name:                "ec2",
			partitionCount:      4,
			writersPerPartition: 200,
			writesPerWriter:     15,
			payloadSize:         350 * 1024, // below 400KB DynamoDB limit
			eventTimeout:        time.Minute,
			backendConfig: map[string]any{
				"table_name":         dynamoDBTestTable(),
				"poll_stream_period": 250 * time.Millisecond,
				"retry_period":       250 * time.Millisecond,
			},
			precondition: func(t *testing.T) {
				// This configuration is expected to be more stable when run from an EC2 instance in the same region as the DynamoDB table.
				if os.Getenv("TELEPORT_TEST_EC2") == "" {
					t.Skip("skipping ec2 test, not running on EC2")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.precondition != nil {
				tt.precondition(t)
			}
			ctx := t.Context()

			b := testCreateBackend(t, ctx, tt.backendConfig)
			w := testInitializeWatcher(t, ctx, b)

			monitor := newTestShardMonitor(ctx, b, t)
			go monitor.monitor(ctx)

			tracker := newEventTracker(t)
			go tracker.receiveEvents(ctx, w)

			writer := newEventWriter(
				t,
				b,
				tracker,
				tt.partitionCount,
				tt.writersPerPartition,
				tt.writesPerWriter,
				tt.payloadSize,
			)

			writer.launchWorkers(ctx)

			numEvents := tt.writersPerPartition * tt.partitionCount * tt.writesPerWriter
			// Start monitoring as soon as we start writing.
			tracker.waitForEvents(ctx, numEvents, tt.eventTimeout)

			// Ensure all writers have finished, this may not be the case in a case where unexpected events are
			// received the the target event count is reached before all writes have completed.
			writer.wg.Wait()

			t.Logf("Starting shard status: %d active, %d closed, %d child shards ",
				monitor.initialCounts.active, monitor.initialCounts.closed, monitor.initialCounts.child)
			t.Logf("Ending shard status: %d active, %d closed, %d child shards ",
				monitor.lastKnownCounts.active, monitor.lastKnownCounts.closed, monitor.lastKnownCounts.child)

			// This test relies on at least 1 shard split happening.
			require.Less(t, monitor.initialCounts.child, monitor.lastKnownCounts.child, "Expected last known child shard count to be larger than initial!")

			verifyNoEventLoss(t, tracker)
		})
	}
}

func testCreateBackend(t *testing.T, ctx context.Context, config map[string]any) *Backend {
	t.Helper()
	b, err := New(ctx, config)
	require.NoError(t, err)

	t.Cleanup(func() {
		b.Close()
	})

	// Wait for table and streams to be ready.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ts, _, err := b.getTableStatus(ctx, aws.String(b.TableName))
		require.Equal(c, tableStatus(tableStatusOK), ts)
		require.NoError(c, err)
	}, 2*time.Minute, 5*time.Second, "DynamoDB table did not become ready in time")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		streamArn, err := b.findStream(ctx)
		require.NoError(c, err)
		require.NotEmpty(c, streamArn)

		shards, err := iterstream.Collect(b.fetchShards(ctx, streamArn))
		require.NoError(c, err)
		require.NotEmpty(c, shards)
	}, 2*time.Minute, 5*time.Second, "DynamoDB shards not ready in time")
	return b
}

// testInitializeWatcher sets up a backend.Watcher and waits for the initial OpInit event to confirm it's ready to receive events.
func testInitializeWatcher(t *testing.T, ctx context.Context, b *Backend) backend.Watcher {
	const initTimeout = 30 * time.Second

	watcher, err := b.NewWatcher(ctx, backend.Watch{})
	require.NoError(t, err)

	t.Cleanup(func() {
		watcher.Close()
	})

	select {
	case event := <-watcher.Events():
		require.Equal(t, apitypes.OpInit, event.Type)
	case <-time.After(initTimeout):
		t.Fatal("Timeout waiting for watcher init")
	case <-ctx.Done():
		return nil
	}

	return watcher
}

// testShardCounts is a simple struct to hold counts of active, closed, and child shards for easier comparison and logging.
type testShardCounts struct {
	active int
	closed int
	child  int
}

// testShardMonitor tracks DynamoDB shard topology changes
type testShardMonitor struct {
	backend         *Backend
	lastKnownCounts *testShardCounts
	initialCounts   *testShardCounts
	t               *testing.T
}

// newTestShardMonitor creates an instance of [testShardMonitor] and captures the initial shard counts.
func newTestShardMonitor(ctx context.Context, b *Backend, t *testing.T) *testShardMonitor {
	startingCounts, err := fetchShardCounts(ctx, b)
	require.NoError(t, err)
	require.NotNil(t, startingCounts)

	t.Logf("Starting shard status: %d active, %d closed, %d child shards ",
		startingCounts.active, startingCounts.closed, startingCounts.child)

	return &testShardMonitor{
		backend:         b,
		t:               t,
		lastKnownCounts: startingCounts,
		initialCounts:   startingCounts,
	}
}

// monitor continuously polls DynamoDB shard topology and logs any changes in shard counts.
func (m *testShardMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			shardCounts, err := fetchShardCounts(ctx, m.backend)
			if err != nil {
				m.t.Logf("Error fetching shard counts: %v", err)
				continue
			}

			if shardCounts.active != m.lastKnownCounts.active ||
				shardCounts.closed != m.lastKnownCounts.closed ||
				shardCounts.child != m.lastKnownCounts.child {
				m.t.Logf("Shard status: %d active, %d closed, %d child shards ",
					shardCounts.active, shardCounts.closed, shardCounts.child)

				// Strictly speaking we only ever expect the child count to grow. Log all changes
				// in case that assumption is incorrect.
				if shardCounts.child != m.lastKnownCounts.child {
					m.t.Logf("SHARD SPLIT DETECTED! Child shards changed from %d to %d",
						m.lastKnownCounts.child, shardCounts.child)
				}

				m.lastKnownCounts = shardCounts
			}
		case <-ctx.Done():
			return
		}
	}
}

// fetchShardCounts retrieves the current counts of active, closed, and child shards for the backend's DynamoDB table.
func fetchShardCounts(ctx context.Context, b *Backend) (*testShardCounts, error) {
	status, err := b.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(b.TableName),
	})
	if err != nil {
		return nil, trace.Wrap(err, "describing table")
	}

	streamInfo, err := b.streams.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: status.Table.LatestStreamArn,
	})
	if err != nil {
		return nil, trace.Wrap(err, "describing stream")
	}

	return countShards(streamInfo.StreamDescription.Shards), nil

}

// countShards iterates through the list of DynamoDB stream shards and counts how many are active, closed, and child shards.
func countShards(shards []streamtypes.Shard) *testShardCounts {
	testShardCounts := &testShardCounts{}
	for _, shard := range shards {
		if shard.SequenceNumberRange.EndingSequenceNumber == nil {
			testShardCounts.active++
		} else {
			testShardCounts.closed++
		}
		if shard.ParentShardId != nil {
			testShardCounts.child++
		}
	}
	return testShardCounts
}

type eventTracker struct {
	t         *testing.T
	startTime time.Time

	writeMu       sync.Mutex
	writtenEvents map[string]bool

	receiveMu      sync.Mutex
	receivedEvents map[string]int
	lastEventTime  time.Time
}

func newEventTracker(t *testing.T) *eventTracker {
	now := time.Now()
	return &eventTracker{
		writtenEvents:  make(map[string]bool),
		receivedEvents: make(map[string]int),
		lastEventTime:  now,
		startTime:      now,
		t:              t,
	}
}

func (et *eventTracker) receiveEvents(ctx context.Context, watcher backend.Watcher) {
	for {
		select {
		case event := <-watcher.Events():
			if event.Type == apitypes.OpPut {
				et.handleReceivedEvent(event)
			}
		case <-ctx.Done():
			return
		case <-watcher.Done():
			panic("watcher closed unexpectedly, this is likely caused by a network failure or an issue with fetching records from DynamoDB streams.")
		}
	}
}

func (et *eventTracker) handleReceivedEvent(event backend.Event) {
	keyStr := event.Item.Key.String()
	et.receiveMu.Lock()
	et.receivedEvents[keyStr]++
	et.lastEventTime = time.Now()
	et.receiveMu.Unlock()
}

func (e *eventTracker) waitForEvents(ctx context.Context, targetCount int, eventTimeout time.Duration) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	rxPercent := 0.0
	donePercent := 0.0

	for {
		select {
		case <-ticker.C:
			e.receiveMu.Lock()
			timeSinceLastEvent := time.Since(e.lastEventTime)
			numReceived := len(e.receivedEvents)
			e.receiveMu.Unlock()

			e.writeMu.Lock()
			numWritten := len(e.writtenEvents)
			e.writeMu.Unlock()

			if numWritten > 0 {
				rxPercent = float64(numReceived) / float64(numWritten) * 100
			}
			if numWritten > 0 {
				donePercent = float64(numWritten) / float64(targetCount) * 100
			}

			e.t.Logf(
				"TX: %-8d RX: %-8d Captured: %-6.1f%% Written: %-6.1f%% Last Event: %-10v",
				numWritten,
				numReceived,
				rxPercent,
				donePercent,
				timeSinceLastEvent.Round(time.Second),
			)

			if numReceived >= targetCount {
				e.t.Logf("Received target of %d events, proceeding with verification", targetCount)
				return
			}
			if timeSinceLastEvent > eventTimeout {
				e.t.Logf("No events received for %v, stopping wait", eventTimeout)
				return
			}

		case <-ctx.Done():
			return

		}
	}
}

type eventWriter struct {
	backend *Backend
	tracker *eventTracker

	partitionCount      int
	writersPerPartition int
	writesPerWriter     int
	payloadSize         int
	seedStr             string

	wg sync.WaitGroup
	t  *testing.T
}

func newEventWriter(
	t *testing.T,
	b *Backend,
	tracker *eventTracker,
	partitionCount int,
	writersPerPartition int,
	writesPerWriter int,
	payloadSize int,
) *eventWriter {

	seedStr := os.Getenv("TEST_SEED")
	if seedStr == "" {
		seedStr = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	t.Logf("using test seed: TEST_SEED=%q", seedStr)

	return &eventWriter{
		backend:             b,
		tracker:             tracker,
		partitionCount:      partitionCount,
		writersPerPartition: writersPerPartition,
		writesPerWriter:     writesPerWriter,
		payloadSize:         payloadSize,
		seedStr:             seedStr,
		t:                   t,
	}
}

func (w *eventWriter) launchWorkers(ctx context.Context) {
	payload := strings.Repeat("A", w.payloadSize)
	for partition := range w.partitionCount {
		for writer := range w.writersPerPartition {
			w.wg.Go(func() {
				w.writeBatch(ctx, partition, writer, w.writesPerWriter, payload)
			})
		}
	}
}

// Writes a batch of events for a given partition and writer.
// The key is for each event is /{partition}/{writer}/{eventNumber} for easier tracking.
func (w *eventWriter) writeBatch(ctx context.Context, partition, writer, numEvents int, payload string) {
	for i := range numEvents {
		item := backend.Item{
			Key:   backend.NewKey(strconv.Itoa(partition), w.seedStr, strconv.Itoa(writer), strconv.Itoa(i)),
			Value: []byte(payload),
		}

		if err := w.writeWithRetry(ctx, item, partition); err != nil {
			// Skip events that fail to write after retries.
			w.t.Logf("Failed to write item %q after retries: %v, skipping", item.Key.String(), err)
			continue
		}

		w.tracker.writeMu.Lock()
		w.tracker.writtenEvents[item.Key.String()] = true
		w.tracker.writeMu.Unlock()
	}
}

// putOnce attempts to write an item to DynamoDB exactly once.
// The partition argument is used to determine the hash key for the item, which is used to
// distribute writes across partitions and trigger shard splits.
func (w *eventWriter) putOnce(ctx context.Context, item backend.Item, partition int) error {
	r := record{
		HashKey:   strconv.Itoa(partition),
		FullPath:  item.Key.String(),
		Value:     item.Value,
		Timestamp: time.Now().UTC().Unix(),
	}

	av, err := attributevalue.MarshalMap(r)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:                av,
		TableName:           aws.String(w.backend.TableName),
		ConditionExpression: aws.String("attribute_not_exists(FullPath)"),
	}

	_, err = w.backend.svc.PutItem(ctx, &input)
	return convertError(err)
}

// writeWithRetry attempts to write an item to DynamoDB with retries on failure.
func (w *eventWriter) writeWithRetry(ctx context.Context, item backend.Item, partition int) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  1 * time.Second,
		Driver: retryutils.NewExponentialDriver(500 * time.Millisecond),
		Max:    10 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		panic("unexpected error creating retry strategy (this is a bug): " + err.Error())
	}

	const maxRetry = 5
	retryCount := 0

	for retryCount < maxRetry {
		if err := w.putOnce(ctx, item, partition); err != nil {
			var throttled *types.ThrottlingException
			if errors.As(err, &throttled) {
				// Do not backoff on throttling errors, we want to retry as fast as possible to trigger shard splits.
				continue
			}

			if trace.IsCompareFailed(err) {
				// Already exists, retrying would not help, skip this item.
				return err
			}

			// When heavily throttled this operation can return [types.InternalServerError] with "Internal Server Error" message.
			// These errors are not marked as throttling errors by the AWS SDK but they are retryable. Avoid logging to reduce the noise.
			if !trace.IsBadParameter(err) {
				w.t.Logf("unexpected write error: %s, retrying...", trace.DebugReport(err))
			}

			retryCount++

			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-time.After(retry.Duration()):
				retry.Inc()
			}
			continue
		}

		return nil
	}

	return trace.LimitExceeded("failed to write item %q after %d retries", item.Key.String(), maxRetry)
}

func diffKeys[K comparable, V1 any, V2 any](a map[K]V1, b map[K]V2) (onlyInA, onlyInB []K) {
	for k := range a {
		if _, ok := b[k]; !ok {
			onlyInA = append(onlyInA, k)
		}
	}

	for k := range b {
		if _, ok := a[k]; !ok {
			onlyInB = append(onlyInB, k)
		}
	}

	return
}

func verifyNoEventLoss(t *testing.T, tracker *eventTracker) {
	tracker.receiveMu.Lock()
	tracker.writeMu.Lock()
	defer tracker.receiveMu.Unlock()
	defer tracker.writeMu.Unlock()

	t.Logf("Total events received: %d/%d written", len(tracker.receivedEvents), len(tracker.writtenEvents))

	for key, count := range tracker.receivedEvents {
		assert.Equal(t, 1, count, "Duplicate events (%v) received for key %q", count, key)
	}

	missingRecived, unexpectedReceived := diffKeys(tracker.writtenEvents, tracker.receivedEvents)
	require.Empty(t, missingRecived, "Missing %d events that were successfully written. Missing event keys: %v", len(missingRecived), missingRecived)
	require.Empty(t, unexpectedReceived, "Received %d unexpected events that were not written. Unexpected event keys: %v", len(unexpectedReceived), unexpectedReceived)
}

func TestBackend_deleteShardsWithParents(t *testing.T) {
	tests := []struct {
		name   string
		params backend.Params
		shards []streamtypes.Shard
		want   []streamtypes.Shard
	}{
		{
			name: "no parents",
			shards: []streamtypes.Shard{
				{ShardId: aws.String("shardId-orphan1")},
				{ShardId: aws.String("shardId-orphan2")},
			},
			want: []streamtypes.Shard{
				{ShardId: aws.String("shardId-orphan1")},
				{ShardId: aws.String("shardId-orphan2")},
			},
		},

		{
			name: "parents in the list",
			shards: []streamtypes.Shard{
				{ShardId: aws.String("shardId-parent1")},
				{
					ShardId:       aws.String("shardId-child1"),
					ParentShardId: aws.String("shardId-parent1"),
				},
				{ShardId: aws.String("shardId-orphan1")},
			},
			want: []streamtypes.Shard{
				{ShardId: aws.String("shardId-parent1")},
				{ShardId: aws.String("shardId-orphan1")},
			},
		},

		{
			name: "parents outside the list",
			shards: []streamtypes.Shard{
				{
					ShardId:       aws.String("shardId-child1"),
					ParentShardId: aws.String("shardId-parent1"),
				},
				{ShardId: aws.String("shardId-orphan1")},
			},
			want: []streamtypes.Shard{
				{
					ShardId:       aws.String("shardId-child1"),
					ParentShardId: aws.String("shardId-parent1"),
				},
				{ShardId: aws.String("shardId-orphan1")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Dummy backend instance just to call the method under test.
			b := &Backend{
				logger: slog.With(teleport.ComponentKey, BackendName),
			}
			got := b.deleteShardsWithParents(context.Background(), tt.shards)
			require.Equal(t, tt.want, got)
		})
	}
}
