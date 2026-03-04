package dynamo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

// TestShardSplitting is an integration test that simulates a high load scenario on a DynamoDB table to trigger shard splits in DynamoDB Streams.
// Given this requires putting a substancial write throughput and the nature of the race condition it is made opt in by setting both `TELEPORT_DYNAMODB_SHARD_SPLIT_TEST`
// and `TELEPORT_DYNAMODB_TEST` env vars.
func TestShardSplitting(t *testing.T) {
	ensureTestsEnabled(t)
	if os.Getenv("TELEPORT_DYNAMODB_SHARD_SPLIT_TEST") == "" {
		t.Skipf("DynamoDB TestShardSplitting test skipped.")
	}

	tests := []struct {
		name          string
		numEvents     int
		numWriters    int
		payloadSize   int
		eventTimeout  time.Duration
		backendConfig map[string]any
	}{
		{
			name:       "default shard split load",
			numEvents:  3000,
			numWriters: 20,
			// 300KB (safe margin below 400KB DynamoDB limit) large object are more offen triggering shard split.
			payloadSize:  300 * 1024,
			eventTimeout: time.Minute,
			backendConfig: map[string]any{
				"table_name":         dynamoDBTestTable(),
				"poll_stream_period": 50 * time.Millisecond,
				"read_min_capacity":  10,
				"read_max_capacity":  20,
				"read_target_value":  50.0,
				"write_min_capacity": 10,
				"write_max_capacity": 20,
				"write_target_value": 50.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			b := testCreateBackend(t, ctx, tt.backendConfig)
			w := testInitializeWatcher(t, ctx, b)

			monitor := newTestShardMonitor(ctx, b, t)
			go monitor.monitor(ctx)

			tracker := newEventTracker()
			go tracker.receiveEvents(ctx, t, w)

			writer := newEventWriter(
				b,
				tracker,
				tt.numEvents,
				tt.numWriters,
				tt.payloadSize,
			)

			numWritten := writer.run(t, ctx)
			t.Logf("All writers finished. Successfully wrote %d/%d events", numWritten, tt.numEvents)

			waitForEvents(t, tracker, numWritten, tt.eventTimeout)
			verifyNoEventLoss(t, tracker, numWritten)
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

		shards, err := b.collectActiveShards(ctx, streamArn)
		require.NoError(c, err)
		require.NotEmpty(c, shards)
	}, 2*time.Minute, 5*time.Second, "DynamoDB shards not ready in time")

	return b
}

// testInitializeWatcher sets up a backend.Watcher and waits for the initial OpInit event to confirm it's ready to receive events.
func testInitializeWatcher(t *testing.T, ctx context.Context, b *Backend) backend.Watcher {
	watcher, err := b.NewWatcher(ctx, backend.Watch{})
	require.NoError(t, err)
	t.Cleanup(func() {
		watcher.Close()
	})

	select {
	case event := <-watcher.Events():
		require.Equal(t, apitypes.OpInit, event.Type)

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for watcher init")
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
	lastKnownCounts testShardCounts
	t               *testing.T
}

// newTestShardMonitor creates an instance of [testShardMonitor] and captures the initial shard counts.
func newTestShardMonitor(ctx context.Context, b *Backend, t *testing.T) *testShardMonitor {
	startingCounts, err := fetchShardCounts(ctx, b)
	require.NoError(t, err)
	monitor := &testShardMonitor{
		backend:         b,
		t:               t,
		lastKnownCounts: startingCounts}
	return monitor
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
				m.t.Errorf("Error fetching shard counts: %v", err)
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
func fetchShardCounts(ctx context.Context, b *Backend) (testShardCounts, error) {
	status, err := b.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(b.TableName),
	})
	if err != nil {
		return testShardCounts{}, trace.Wrap(err)
	}

	streamInfo, err := b.streams.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: status.Table.LatestStreamArn,
	})
	if err != nil {
		return testShardCounts{}, trace.Wrap(err)
	}

	return countShards(streamInfo.StreamDescription.Shards), nil

}

// countShards iterates through the list of DynamoDB stream shards and counts how many are active, closed, and child shards.
func countShards(shards []streamtypes.Shard) testShardCounts {
	testShardCounts := testShardCounts{}
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
	writeMu       sync.Mutex
	writtenEvents map[string]bool

	receiveMu      sync.Mutex
	receivedEvents map[string]bool

	eventCount    int
	lastEventTime time.Time
}

func newEventTracker() *eventTracker {
	return &eventTracker{
		writtenEvents:  make(map[string]bool),
		receivedEvents: make(map[string]bool),
		lastEventTime:  time.Now(),
	}
}

func (et *eventTracker) receiveEvents(ctx context.Context, t *testing.T, watcher backend.Watcher) {
	for {
		select {
		case event := <-watcher.Events():
			if event.Type == apitypes.OpPut {
				et.handleReceivedEvent(t, event)
			}
		case <-ctx.Done():
			return
		case <-watcher.Done():
			panic("watcher closed unexpectedly")
		}
	}
}

func (et *eventTracker) handleReceivedEvent(t *testing.T, event backend.Event) {
	keyStr := event.Item.Key.String()
	if _, ok := et.receivedEvents[keyStr]; ok {
		t.Fatalf("Received duplicate event: %s", keyStr)
	}

	et.receivedEvents[keyStr] = true
	et.eventCount++
	et.lastEventTime = time.Now()

	if et.eventCount%100 == 0 {
		t.Logf("Received %d events so far", et.eventCount)
	}
}

type eventWriter struct {
	backend      *Backend
	tracker      *eventTracker
	numEvents    int
	numWriters   int
	payloadSize  int
	seriesPrefix int64
}

func newEventWriter(
	b *Backend,
	tracker *eventTracker,
	numEvents int,
	numWriters int,
	payloadSize int,
) *eventWriter {
	return &eventWriter{
		backend:      b,
		tracker:      tracker,
		numEvents:    numEvents,
		numWriters:   numWriters,
		payloadSize:  payloadSize,
		seriesPrefix: time.Now().Unix(),
	}
}

func (w *eventWriter) run(t *testing.T, ctx context.Context) int {
	t.Logf(
		"Writing %d events with %d concurrent writers...",
		w.numEvents,
		w.numWriters,
	)

	payload := strings.Repeat("A", w.payloadSize)

	var wg sync.WaitGroup
	eventsPerWriter := w.numEvents / w.numWriters

	for writerID := range w.numWriters {
		wg.Go(func() {
			start := writerID * eventsPerWriter
			end := start + eventsPerWriter
			w.writeBatch(t, ctx, start, end, payload)
		})
	}
	wg.Wait()

	w.tracker.writeMu.Lock()
	defer w.tracker.writeMu.Unlock()

	return len(w.tracker.writtenEvents)
}

func (w *eventWriter) writeBatch(
	t *testing.T,
	ctx context.Context,
	start, end int,
	payload string,
) {
	for i := start; i < end; i++ {
		item := backend.Item{
			Key: backend.NewKey(
				"shard-split-test",
				"event",
				fmt.Sprintf("%d-%d", w.seriesPrefix, i),
			),
			Value: []byte(payload),
		}

		if err := writeWithRetry(ctx, t, w.backend, item); err != nil {
			continue
		}

		w.tracker.writeMu.Lock()
		w.tracker.writtenEvents[item.Key.String()] = true
		w.tracker.writeMu.Unlock()
	}
}

func writeWithRetry(ctx context.Context, t *testing.T, b *Backend, item backend.Item) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  1 * time.Second,
		Driver: retryutils.NewExponentialDriver(2 * time.Second),
		Max:    60 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	require.NoError(t, err)

	for {
		if _, err := b.Put(ctx, item); err != nil {
			// Log the error if it's not a throughput exceeded error,
			// which is expected for this test as we are forcing high load to trigger shard splits.
			var throttled *types.ThrottlingException
			if errors.As(err, &throttled) {
				// Write throttled, wait and retry.
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(retry.Duration()):
				}

				retry.Inc()
				continue
			}

			// No other write errors are expected
			require.NoError(t, err)
		}

		return nil
	}
}

func waitForEvents(t *testing.T, tracker *eventTracker, numWritten int, eventTimeout time.Duration) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tracker.receiveMu.Lock()
		timeSinceLastEvent := time.Since(tracker.lastEventTime)
		currentEventCount := tracker.eventCount
		tracker.receiveMu.Unlock()

		if currentEventCount >= numWritten {
			return
		}

		t.Logf("Waiting for events... received %d/%d (%.1f%%), last event %v ago",
			currentEventCount, numWritten,
			float64(currentEventCount)/float64(numWritten)*100,
			timeSinceLastEvent.Round(time.Second))

		if timeSinceLastEvent > eventTimeout {
			t.Logf("No events received for %v, stopping wait", eventTimeout)
			return
		}
	}
}

func verifyNoEventLoss(t *testing.T, tracker *eventTracker, numWritten int) {
	tracker.receiveMu.Lock()
	tracker.writeMu.Lock()
	defer tracker.receiveMu.Unlock()
	defer tracker.writeMu.Unlock()

	missing := make([]string, 0)
	for eventKey := range tracker.writtenEvents {
		if !tracker.receivedEvents[eventKey] {
			missing = append(missing, eventKey)
		}
	}

	t.Logf("Total events received: %d/%d written", tracker.eventCount, numWritten)
	require.Empty(t, missing, "Missing %d events that were successfully written. Missing event keys: %v", len(missing), missing)
}
