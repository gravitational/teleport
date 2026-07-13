/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package athena

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_consumer_sqsMessagesCollector(t *testing.T) {
	maxWaitTimeOnReceiveMessagesInFake := 5 * time.Millisecond

	t.Run("verify if events are sent over channel", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Given SqsMessagesCollector reading from fake sqs with a delayed ReceiveMessage call
			// When 3 messages are published
			// Then 3 messages can be received from eventsChan.

			// Given
			fq := &fakeSQS{maxWaitTime: maxWaitTimeOnReceiveMessagesInFake}
			cfg := validCollectCfgForTests(t)
			cfg.sqsReceiver = fq
			cfg.noOfWorkers = 1
			require.NoError(t, cfg.CheckAndSetDefaults())
			c := newSqsMessagesCollector(cfg)
			eventsChan := c.getEventsChan()

			ctx, readCancel := context.WithCancel(t.Context())
			defer readCancel()
			go c.fromSQS(ctx)

			// When
			wantEvents := []apievents.AuditEvent{
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app1"}},
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app2"}},
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app3"}},
			}
			fq.addEvents(wantEvents...)

			// Then
			got := receiveEvents(t, eventsChan, 3)
			readCancel()
			requireChannelClosed(t, eventsChan)
			requireEventsEqualInAnyOrder(t, wantEvents, eventAndAckIDToAuditEvents(got))
		})
	})

	t.Run("verify if collector finishes execution (via closing channel) upon ctx.Cancel", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Given SqsMessagesCollector reading from fake sqs with a delayed ReceiveMessage call
			// When ctx is canceled
			// Then reading chan is closed.

			// Given
			fq := &fakeSQS{maxWaitTime: maxWaitTimeOnReceiveMessagesInFake}
			cfg := validCollectCfgForTests(t)
			cfg.sqsReceiver = fq
			cfg.noOfWorkers = 1
			require.NoError(t, cfg.CheckAndSetDefaults())
			c := newSqsMessagesCollector(cfg)
			eventsChan := c.getEventsChan()

			readSQSCtx, readCancel := context.WithCancel(t.Context())
			go c.fromSQS(readSQSCtx)

			// When
			readCancel()

			// Then
			// Make sure that channel is closed.
			requireChannelClosed(t, eventsChan)
		})
	})

	t.Run("verify if collector finishes execution (via closing channel) upon reaching batchMaxItems", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Given SqsMessagesCollector reading from fake sqs with a delayed ReceiveMessage call
			// When batchMaxItems is reached.
			// Then reading chan is closed.

			// Given
			fq := &fakeSQS{maxWaitTime: maxWaitTimeOnReceiveMessagesInFake}
			cfg := validCollectCfgForTests(t)
			cfg.sqsReceiver = fq
			cfg.noOfWorkers = 1
			cfg.batchMaxItems = 3
			require.NoError(t, cfg.CheckAndSetDefaults())
			c := newSqsMessagesCollector(cfg)

			eventsChan := c.getEventsChan()

			go c.fromSQS(t.Context())

			// When
			wantEvents := []apievents.AuditEvent{
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app1"}},
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app2"}},
				&apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent}, AppMetadata: apievents.AppMetadata{AppName: "app3"}},
			}
			fq.addEvents(wantEvents...)
			got := receiveEvents(t, eventsChan, 3)

			// Then
			// Make sure that channel is closed.
			requireChannelClosed(t, eventsChan)
			requireEventsEqualInAnyOrder(t, wantEvents, eventAndAckIDToAuditEvents(got))
		})
	})
	t.Run("verify if collector finishes execution (via closing channel) upon reaching maxUniquePerDayEvents", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Given SqsMessagesCollector reading from fake sqs with a delayed ReceiveMessage call
			// When maxUniquePerDayEvents is reached.
			// Then reading chan is closed.

			// Given
			fq := &fakeSQS{maxWaitTime: maxWaitTimeOnReceiveMessagesInFake}
			cfg := validCollectCfgForTests(t)
			cfg.sqsReceiver = fq
			cfg.noOfWorkers = 1
			cfg.batchMaxItems = 1000
			require.NoError(t, cfg.CheckAndSetDefaults())
			c := newSqsMessagesCollector(cfg)

			eventsChan := c.getEventsChan()

			go c.fromSQS(t.Context())

			// When over 100 unique days are sent
			eventsToSend := make([]apievents.AuditEvent, 0, 101)
			for i := range 101 {
				day := time.Now().Add(time.Duration(i) * 24 * time.Hour)
				eventsToSend = append(eventsToSend, &apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent, Time: day}, AppMetadata: apievents.AppMetadata{AppName: "app1"}})
			}
			fq.addEvents(eventsToSend...)
			receiveEvents(t, eventsChan, 101)

			// Then
			// Make sure that channel is closed.
			requireChannelClosed(t, eventsChan)
		})
	})
}

func receiveEvents(t *testing.T, ch <-chan eventAndAckID, n int) []eventAndAckID {
	t.Helper()

	out := make([]eventAndAckID, 0, n)
	for len(out) < n {
		select {
		case event, ok := <-ch:
			require.True(t, ok, "events channel closed after receiving %d of %d events", len(out), n)
			out = append(out, event)
		case <-time.After(15 * time.Second):
			t.Fatalf("timed out waiting for event %d of %d", len(out)+1, n)
		}
	}
	return out
}

func requireChannelClosed(t *testing.T, ch <-chan eventAndAckID) {
	t.Helper()

	select {
	case event, ok := <-ch:
		require.False(t, ok, "expected events channel to be closed, received %v", event)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for events channel to close")
	}
}

func validCollectCfgForTests(t *testing.T) sqsCollectConfig {
	metrics, err := newAthenaMetrics(athenaMetricsConfig{
		batchInterval:        defaultBatchInterval,
		externalAuditStorage: false,
	})
	require.NoError(t, err)
	return sqsCollectConfig{
		sqsReceiver:       &mockReceiver{},
		queueURL:          "test-queue",
		payloadBucket:     "bucket",
		payloadDownloader: &fakeS3manager{},
		logger:            slog.Default(),
		errHandlingFn: func(ctx context.Context, errC chan error) {
			err, ok := <-errC
			if ok && err != nil {
				// we don't expect error in that test case.
				t.Log("Unexpected error", err)
				t.Fail()
			}
		},
		metrics: metrics,
	}
}

type fakeSQS struct {
	mu          sync.Mutex
	msgs        []sqsTypes.Message
	maxWaitTime time.Duration
}

func (f *fakeSQS) addEvents(events ...apievents.AuditEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range events {
		f.msgs = append(f.msgs, rawProtoMessage(e))
	}
}

func (f *fakeSQS) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(f.maxWaitTime):
		// continue below
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.msgs) > 0 {
		out := &sqs.ReceiveMessageOutput{
			Messages: f.msgs,
		}
		f.msgs = nil
		return out, nil
	}
	return &sqs.ReceiveMessageOutput{}, nil
}

type receiver struct {
	mu   sync.Mutex
	msgs []eventAndAckID
}

func (f *receiver) Do(eventsChan <-chan eventAndAckID) {
	for e := range eventsChan {
		f.mu.Lock()
		f.msgs = append(f.msgs, e)
		f.mu.Unlock()
	}
}

func (f *receiver) GetMsgs() []eventAndAckID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.msgs
}

func eventAndAckIDToAuditEvents(in []eventAndAckID) []apievents.AuditEvent {
	var out []apievents.AuditEvent
	for _, eventAndAckID := range in {
		out = append(out, eventAndAckID.event)
	}
	return out
}

func rawProtoMessage(in apievents.AuditEvent) sqsTypes.Message {
	oneOf := apievents.MustToOneOf(in)
	bb, err := oneOf.Marshal()
	if err != nil {
		panic(err)
	}
	return sqsTypes.Message{
		Body: aws.String(base64.StdEncoding.EncodeToString(bb)),
		MessageAttributes: map[string]sqsTypes.MessageAttributeValue{
			payloadTypeAttr: {StringValue: aws.String(payloadTypeRawProtoEvent)},
		},
		ReceiptHandle: aws.String(uuid.NewString()),
	}
}

// TestSQSMessagesCollectorErrorsOnReceive verifies that workers fetching events
// from ReceiveMessage endpoint, will wait specified interval before retrying
// after receiving error from API call.
func TestSQSMessagesCollectorErrorsOnReceive(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Minute)
	defer cancel()

	mockReceiver := &mockReceiver{
		receiveMessageRespFn: func() (*sqs.ReceiveMessageOutput, error) {
			return nil, errors.New("aws error")
		},
	}

	errHandlingFn := func(ctx context.Context, errC chan error) {
		require.ErrorContains(t, trace.NewAggregateFromChannel(errC, ctx), "aws error")
	}
	waitIntervalOnReceiveError := 5 * time.Millisecond
	noOfWorker := 2
	iterationsToWait := 4
	expectedNoOfCalls := noOfWorker * iterationsToWait

	cfg := validCollectCfgForTests(t)
	cfg.sqsReceiver = mockReceiver
	cfg.noOfWorkers = noOfWorker
	cfg.waitOnReceiveError = waitIntervalOnReceiveError
	cfg.errHandlingFn = errHandlingFn
	require.NoError(t, cfg.CheckAndSetDefaults())
	c := newSqsMessagesCollector(cfg)

	eventsChan := c.getEventsChan()
	sqsCtx, sqsCancel := context.WithCancel(ctx)
	go c.fromSQS(sqsCtx)

	<-time.After(time.Duration(iterationsToWait) * waitIntervalOnReceiveError)
	sqsCancel()
	select {
	case <-ctx.Done():
		t.Fatal("Collector never finished")
	case _, ok := <-eventsChan:
		require.False(t, ok, "No data should be sent on events channel")
	}

	gotNoOfCalls := mockReceiver.getNoOfCalls()
	// We can't be sure that there will be equaly noOfCalls as expected,
	// because they are process in async way, but anything within range x>= 0 && x< 1.5*expected is valid.
	require.LessOrEqual(t, float64(gotNoOfCalls), 1.5*float64(expectedNoOfCalls), "receiveMessage got too many calls")
	require.Greater(t, gotNoOfCalls, 0, "receiveMessage was not called at all")
}

type mockReceiver struct {
	receiveMessageRespFn  func() (*sqs.ReceiveMessageOutput, error)
	receiveMessageCountMu sync.Mutex
	receiveMessageCount   int
}

func (m *mockReceiver) getNoOfCalls() int {
	m.receiveMessageCountMu.Lock()
	defer m.receiveMessageCountMu.Unlock()
	return m.receiveMessageCount
}

func (m *mockReceiver) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	m.receiveMessageCountMu.Lock()
	m.receiveMessageCount++
	m.receiveMessageCountMu.Unlock()
	return m.receiveMessageRespFn()
}

func TestConsumerRunContinuouslyOnSingleAuth(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		log := slog.New(slog.DiscardHandler)
		backend, err := memory.New(memory.Config{})
		require.NoError(t, err)
		defer backend.Close()

		batchInterval := 20 * time.Millisecond

		c1 := consumer{
			logger:           log,
			backend:          backend,
			batchMaxInterval: batchInterval,
		}
		c2 := consumer{
			logger:           log,
			backend:          backend,
			batchMaxInterval: batchInterval,
		}
		m1 := newMockEventsProcessor()
		m2 := newMockEventsProcessor()

		ctx1, cancel1 := context.WithCancel(t.Context())
		defer cancel1()
		ctx2, cancel2 := context.WithCancel(t.Context())
		defer cancel2()

		// Start two consumers with different mocks in background. Backend locking
		// should allow only one mock to start processing.
		go c1.runContinuouslyOnSingleAuth(ctx1, m1.Run)
		go c2.runContinuouslyOnSingleAuth(ctx2, m2.Run)

		started := waitForProcessorStart(t, map[*mockEventsProcessor]context.CancelFunc{
			m1: cancel1,
			m2: cancel2,
		})
		if started.mockEventsProcessor == m1 {
			requireProcessorNotStarted(t, m2)
		} else {
			requireProcessorNotStarted(t, m1)
		}

		// Cancel the consumer that holds the lock and verify the other one takes over.
		started.cancel()
		if started.mockEventsProcessor == m1 {
			waitForProcessorStart(t, map[*mockEventsProcessor]context.CancelFunc{m2: cancel2})
		} else {
			waitForProcessorStart(t, map[*mockEventsProcessor]context.CancelFunc{m1: cancel1})
		}
	})
}

type mockEventsProcessor struct {
	started chan struct{}
}

func newMockEventsProcessor() *mockEventsProcessor {
	return &mockEventsProcessor{started: make(chan struct{}, 1)}
}

func (m *mockEventsProcessor) Run(ctx context.Context) {
	select {
	case m.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
}

type startedProcessor struct {
	*mockEventsProcessor
	cancel context.CancelFunc
}

func waitForProcessorStart(t *testing.T, processors map[*mockEventsProcessor]context.CancelFunc) startedProcessor {
	t.Helper()

	timeout := time.After(15 * time.Second)
	for {
		for processor, cancel := range processors {
			select {
			case <-processor.started:
				return startedProcessor{mockEventsProcessor: processor, cancel: cancel}
			default:
			}
		}

		select {
		case <-time.After(time.Millisecond):
		case <-timeout:
			t.Fatal("timed out waiting for processor to start")
		}
	}
}

func requireProcessorNotStarted(t *testing.T, processor *mockEventsProcessor) {
	t.Helper()

	synctest.Wait()
	select {
	case <-processor.started:
		t.Fatal("processor started while another consumer should hold the lock")
	default:
	}
}

func TestRunWithMinInterval(t *testing.T) {
	t.Run("function returns earlier than minInterval, wait should happen", func(t *testing.T) {
		fn := func(ctx context.Context) bool {
			// did not reached max size
			return false
		}
		minInterval := 5 * time.Millisecond
		start := time.Now()
		stop := runWithMinInterval(t.Context(), fn, minInterval)
		elapsed := time.Since(start)
		require.False(t, stop)
		require.GreaterOrEqual(t, elapsed, minInterval)
	})

	t.Run("function takes longer than minInterval, noting more should happen", func(t *testing.T) {
		minInterval := 5 * time.Millisecond
		fn := func(ctx context.Context) bool {
			// did not reached max size
			select {
			case <-time.After(2 * minInterval):
				return false
			case <-ctx.Done():
				return false
			}
		}
		start := time.Now()
		stop := runWithMinInterval(t.Context(), fn, minInterval)
		elapsed := time.Since(start)
		require.False(t, stop)
		require.GreaterOrEqual(t, elapsed, 2*minInterval)
	})

	t.Run("reached maxBatchSize, wait should not happen", func(t *testing.T) {
		fn := func(ctx context.Context) bool {
			return true
		}
		minInterval := 5 * time.Millisecond
		start := time.Now()
		stop := runWithMinInterval(t.Context(), fn, minInterval)
		elapsed := time.Since(start)
		require.False(t, stop)
		require.Less(t, elapsed, minInterval)
	})

	t.Run("context is canceled, make sure that stop is returned.", func(t *testing.T) {
		minInterval := 5 * time.Millisecond
		fn := func(ctx context.Context) bool {
			// did not reached max size
			select {
			case <-time.After(minInterval):
				return false
			case <-ctx.Done():
				return false
			}
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		stop := runWithMinInterval(ctx, fn, minInterval)
		require.True(t, stop)
	})
}

// TestConsumerWriteToS3 checks if writing parquet files per date works.
// It receives events from different dates and make sure that multiple
// files are created and compare it against file in testdata.
// Testdata files should be verified with "parquet tools" cli after changing.
func TestConsumerWriteToS3(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Minute)
	defer cancel()

	tmp := t.TempDir()
	localWriter := func(ctx context.Context, date string) (io.WriteCloser, error) {
		err := os.MkdirAll(filepath.Join(tmp, date), 0o777)
		if err != nil {
			return nil, err
		}
		localW, err := os.Create(filepath.Join(tmp, date, "test.parquet"))
		return localW, err
	}

	april1st2023AfternoonStr := "2023-04-01T16:20:50.52Z"
	april1st2023Afternoon, err := time.Parse(time.RFC3339, april1st2023AfternoonStr)
	require.NoError(t, err)

	makeAppCreateEventWithTime := func(t time.Time, name string) apievents.AuditEvent {
		return &apievents.AppCreate{Metadata: apievents.Metadata{Type: events.AppCreateEvent, Time: t}, AppMetadata: apievents.AppMetadata{AppName: name}}
	}

	eventR1 := makeAppCreateEventWithTime(april1st2023Afternoon, "app-1")
	eventR2 := makeAppCreateEventWithTime(april1st2023Afternoon.Add(10*time.Second), "app-2")
	// r3 date is next date, so it should be written as separate file.
	eventR3 := makeAppCreateEventWithTime(april1st2023Afternoon.Add(18*time.Hour), "app3")

	events := []eventAndAckID{
		{receiptHandle: "r1", event: eventR1},
		{receiptHandle: "r2", event: eventR2},
		{receiptHandle: "r3", event: eventR3},
	}

	eventsC := make(chan eventAndAckID, 100)
	go func() {
		for _, e := range events {
			eventsC <- e
		}
		close(eventsC)
	}()

	c := &consumer{
		collectConfig: validCollectCfgForTests(t),
	}
	gotHandlesToDelete, err := c.writeToS3(ctx, eventsC, localWriter)
	require.NoError(t, err)
	// Make sure that all events are marked to delete.
	require.Equal(t, []string{"r1", "r2", "r3"}, gotHandlesToDelete)

	// verify that both files for 2023-04-01 and 2023-04-02 were written and
	// if they contain audit events.
	type wantGot struct {
		name       string
		wantEvents []apievents.AuditEvent
		gotFile    string
	}
	toCheck := []wantGot{
		{
			name:       "2023-04-01 should contain 2 events",
			wantEvents: []apievents.AuditEvent{eventR1, eventR2},
			gotFile:    filepath.Join(tmp, "2023-04-01", "test.parquet"),
		},
		{
			name:       "2023-04-02 should contain 1 events",
			wantEvents: []apievents.AuditEvent{eventR3},
			gotFile:    filepath.Join(tmp, "2023-04-02", "test.parquet"),
		},
	}

	for _, v := range toCheck {
		t.Run("Checking "+v.name, func(t *testing.T) {
			rows, err := parquet.ReadFile[eventParquet](v.gotFile)
			require.NoError(t, err)
			gotEvents, err := parquetRowsToAuditEvents(rows)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(gotEvents, v.wantEvents))
		})
	}
}

func parquetRowsToAuditEvents(in []eventParquet) ([]apievents.AuditEvent, error) {
	out := make([]apievents.AuditEvent, 0, len(in))
	for _, p := range in {
		var fields events.EventFields
		if err := utils.FastUnmarshal([]byte(p.EventData), &fields); err != nil {
			return nil, trace.Wrap(err, "failed to unmarshal event, %s", p.EventData)
		}
		event, err := events.FromEventFields(fields)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, event)
	}
	return out, nil
}

func TestDeleteMessagesFromQueue(t *testing.T) {
	t.Parallel()

	handlesGen := func(n int) []string {
		out := make([]string, 0, n)
		for i := range n {
			out = append(out, fmt.Sprintf("handle-%d", i))
		}
		return out
	}
	noOfHandles := 18
	handles := handlesGen(noOfHandles)

	collectConfig := validCollectCfgForTests(t)

	tests := []struct {
		name       string
		mockRespFn func(ctx context.Context, params *sqs.DeleteMessageBatchInput) (*sqs.DeleteMessageBatchOutput, error)
		wantCheck  func(t *testing.T, err error, mock *mockSQSDeleter)
	}{
		{
			name: "delete returns no error, expect 2 calls to delete",
			mockRespFn: func(ctx context.Context, params *sqs.DeleteMessageBatchInput) (*sqs.DeleteMessageBatchOutput, error) {
				if aws.ToString(params.QueueUrl) == "" {
					return nil, errors.New("mock called with empty QueueUrl")
				}
				if noOfEntries := len(params.Entries); noOfEntries > 10 || noOfEntries == 0 {
					return nil, fmt.Errorf("mock called with invalid number of entries %d", noOfEntries)
				}
				return &sqs.DeleteMessageBatchOutput{}, nil
			},
			wantCheck: func(t *testing.T, err error, mock *mockSQSDeleter) {
				require.NoError(t, err)
				require.Equal(t, 2, mock.calls)
				require.Equal(t, noOfHandles, mock.noOfEntries)
			},
		},
		{
			name: "delete returns top level error, make sure it's returned",
			mockRespFn: func(ctx context.Context, params *sqs.DeleteMessageBatchInput) (*sqs.DeleteMessageBatchOutput, error) {
				if aws.ToString(params.QueueUrl) == "" {
					return nil, errors.New("mock called with empty QueueUrl")
				}
				if noOfEntries := len(params.Entries); noOfEntries > 10 || noOfEntries == 0 {
					return nil, fmt.Errorf("mock called with invalid number of entries %d", noOfEntries)
				}
				return nil, errors.New("AWS API err")
			},
			wantCheck: func(t *testing.T, err error, _ *mockSQSDeleter) {
				require.ErrorContains(t, err, "AWS API err")
			},
		},
		{
			name: "half of entries returns error",
			mockRespFn: func(ctx context.Context, params *sqs.DeleteMessageBatchInput) (*sqs.DeleteMessageBatchOutput, error) {
				success := make([]sqsTypes.DeleteMessageBatchResultEntry, 0)
				failed := make([]sqsTypes.BatchResultErrorEntry, 0)
				for i, e := range params.Entries {
					if i%2 == 0 {
						success = append(success, sqsTypes.DeleteMessageBatchResultEntry{
							Id: e.Id,
						})
					} else {
						failed = append(failed, sqsTypes.BatchResultErrorEntry{
							Id:      e.Id,
							Message: aws.String("entry failed"),
						})
					}
				}
				return &sqs.DeleteMessageBatchOutput{
					Failed:     failed,
					Successful: success,
				}, nil
			},
			wantCheck: func(t *testing.T, err error, mock *mockSQSDeleter) {
				require.Error(t, err)
				var agg trace.Aggregate
				require.ErrorAs(t, trace.Unwrap(err), &agg)
				for _, errFromAgg := range agg.Errors() {
					require.ErrorContains(t, errFromAgg, "entry failed")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSQSDeleter{
				respFn: tt.mockRespFn,
			}
			c := consumer{
				sqsDeleter:    mock,
				queueURL:      "queue-url",
				collectConfig: collectConfig,
			}
			err := c.deleteMessagesFromQueue(t.Context(), handles)
			tt.wantCheck(t, err, mock)
		})
	}
}

type mockSQSDeleter struct {
	respFn func(ctx context.Context, params *sqs.DeleteMessageBatchInput) (*sqs.DeleteMessageBatchOutput, error)

	// mu protects fields below
	mu          sync.Mutex
	calls       int
	noOfEntries int
}

func (m *mockSQSDeleter) DeleteMessageBatch(ctx context.Context, params *sqs.DeleteMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.noOfEntries += len(params.Entries)
	return m.respFn(ctx, params)
}

func TestCollectedEventsMetadataMerge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		a        collectedEventsMetadata
		b        collectedEventsMetadata
		expected collectedEventsMetadata
	}{
		{
			name: "Merge with empty a",
			a: collectedEventsMetadata{
				Size:            0,
				Count:           0,
				OldestTimestamp: time.Time{},
			},
			b: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now,
				UniqueDays:      map[string]struct{}{now.Format(time.DateOnly): {}},
			},
			expected: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now,
				UniqueDays:      map[string]struct{}{now.Format(time.DateOnly): {}},
			},
		},
		{
			name: "Merge with empty b",
			a: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now,
				UniqueDays:      map[string]struct{}{now.Format(time.DateOnly): {}},
			},
			b: collectedEventsMetadata{
				Size:            0,
				Count:           0,
				OldestTimestamp: time.Time{},
			},
			expected: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now,
				UniqueDays:      map[string]struct{}{now.Format(time.DateOnly): {}},
			},
		},
		{
			name: "Merge with non-empty metadata",
			a: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now.Add(-time.Hour),
			},
			b: collectedEventsMetadata{
				Size:            15,
				Count:           7,
				OldestTimestamp: now,
			},
			expected: collectedEventsMetadata{
				Size:            25,
				Count:           12,
				OldestTimestamp: now.Add(-time.Hour),
			},
		},
		{
			name: "Merge with two different days",
			a: collectedEventsMetadata{
				Size:            10,
				Count:           5,
				OldestTimestamp: now.Add(-36 * time.Hour),
				UniqueDays:      map[string]struct{}{now.Add(-36 * time.Hour).Format(time.DateOnly): {}},
			},
			b: collectedEventsMetadata{
				Size:            15,
				Count:           7,
				OldestTimestamp: now,
				UniqueDays:      map[string]struct{}{now.Format(time.DateOnly): {}},
			},
			expected: collectedEventsMetadata{
				Size:            25,
				Count:           12,
				OldestTimestamp: now.Add(-36 * time.Hour),
				UniqueDays: map[string]struct{}{
					now.Add(-36 * time.Hour).Format(time.DateOnly): {},
					now.Format(time.DateOnly):                      {},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.a.Merge(tt.b)
			require.Empty(t, cmp.Diff(tt.a, tt.expected))
		})
	}
}

func Test_getMessageSentTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		msg     sqsTypes.Message
		want    time.Time
		wantErr string
	}{
		{
			name: "valid value sentTimestamp",
			msg:  sqsTypes.Message{Attributes: map[string]string{"SentTimestamp": "1687183084420"}},
			want: time.Date(2023, time.June, 19, 13, 58, 4, 420000000, time.UTC),
		},
		{
			name: "empty map",
			msg:  sqsTypes.Message{},
			want: time.Time{},
		},
		{
			name: "missing attribute",
			msg:  sqsTypes.Message{Attributes: map[string]string{"abc": "def"}},
			want: time.Time{},
		},
		{
			name:    "wrong format of sentTimestamp",
			msg:     sqsTypes.Message{Attributes: map[string]string{"SentTimestamp": "def"}},
			wantErr: "invalid syntax",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMessageSentTimestamp(tt.msg)
			if tt.wantErr == "" {
				require.NoError(t, err, "getMessageSentTimestamp return unexpected err")
				require.Equal(t, tt.want, got)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
