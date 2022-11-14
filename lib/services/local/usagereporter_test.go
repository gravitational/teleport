/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"fmt"
	"testing"
	"time"

	prehogapi "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testClusterName = "example.com"
	testClusterID   = "abcde"

	testMinBatchSize  = 3
	testMaxBatchSize  = 5
	testMaxBufferSize = 10
)

// newTestSubmitter creates a submitter that reports batches to a channel.
func newTestSubmitter(size int) (UsageSubmitFunc, chan []*prehogapi.SubmitEventRequest) {
	ch := make(chan []*prehogapi.SubmitEventRequest, size)

	return func(reporter *UsageReporter, batch []*prehogapi.SubmitEventRequest) error {
		ch <- batch
		return nil
	}, ch
}

// newFailingSubmitter creates a submitter function that always reports batches
// as failed. The current batch of events is written to the channel as usual
// for inspection.
func newFailingSubmitter(size int) (UsageSubmitFunc, chan []*prehogapi.SubmitEventRequest) {
	ch := make(chan []*prehogapi.SubmitEventRequest, size)

	return func(reporter *UsageReporter, batch []*prehogapi.SubmitEventRequest) error {
		ch <- batch
		return trace.BadParameter("testing error")
	}, ch
}

func newTestingUsageReporter(t *testing.T, clock clockwork.Clock, submitter UsageSubmitFunc) (*UsageReporter, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	l := log.WithFields(log.Fields{
		trace.Component: teleport.Component(teleport.ComponentUsageReporting),
	})

	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: testClusterName,
		ClusterID:   testClusterID,
	})
	require.NoError(t, err)

	anonymizer, err := utils.NewHMACAnonymizer(clusterName.GetClusterID())
	require.NoError(t, err)

	reporter := &UsageReporter{
		Entry:           l,
		ctx:             ctx,
		cancel:          cancel,
		anonymizer:      anonymizer,
		events:          make(chan []*prehogapi.SubmitEventRequest, 1),
		submissionQueue: make(chan []*prehogapi.SubmitEventRequest, 1),
		submit:          submitter,
		clock:           clock,
		clusterName:     clusterName,
		minBatchSize:    testMinBatchSize,
		maxBatchSize:    testMaxBatchSize,
		maxBatchAge:     usageReporterMaxBatchAge,
		maxBufferSize:   testMaxBufferSize,
		submitDelay:     usageReporterSubmitDelay,
		ready:           make(chan struct{}),
	}

	go reporter.Run()

	// Wait for timers to init.
	<-reporter.ready

	return reporter, cancel
}

// createDummyEvents creates a number of dummy events for testing
func createDummyEvents(start, count int) []services.UsageAnonymizable {
	var ret []services.UsageAnonymizable

	for i := 0; i < count; i++ {
		ret = append(ret, &services.UsageUserLogin{
			UserName:      fmt.Sprintf("%d", start+i),
			ConnectorType: types.KindGithubConnector,
		})
	}

	return ret
}

// compareUsageEvents ensures all given usage events
func compareUsageEvents(t *testing.T, reporter *UsageReporter, inputs []services.UsageAnonymizable, outputs []*prehogapi.SubmitEventRequest) {
	require.Len(t, outputs, len(inputs))

	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		anonymized := input.Anonymize(reporter.anonymizer)
		output := outputs[i]

		expectedClusterName := reporter.anonymizer.Anonymize([]byte(reporter.clusterName.GetClusterName()))
		require.Equal(t, expectedClusterName, output.ClusterName)

		switch e := output.GetEvent().(type) {
		case *prehogapi.SubmitEventRequest_UserLogin:
			userLogin, ok := anonymized.(*services.UsageUserLogin)
			require.True(t, ok)

			require.Equal(t, userLogin.UserName, e.UserLogin.UserName)
			require.Equal(t, userLogin.ConnectorType, e.UserLogin.ConnectorType)
		case *prehogapi.SubmitEventRequest_SsoCreate:
			ssoCreate, ok := anonymized.(*services.UsageSSOCreate)
			require.True(t, ok)

			require.Equal(t, ssoCreate.ConnectorType, e.SsoCreate.ConnectorType)
		case *prehogapi.SubmitEventRequest_SessionStart:
			sessionStart, ok := anonymized.(*services.UsageSessionStart)
			require.True(t, ok)

			require.Equal(t, sessionStart.UserName, e.SessionStart.UserName)
			require.Equal(t, sessionStart.SessionType, e.SessionStart.SessionType)
		case *prehogapi.SubmitEventRequest_ResourceCreate:
			resourceCreate, ok := anonymized.(*services.UsageResourceCreate)
			require.True(t, ok)

			require.Equal(t, resourceCreate.ResourceType, e.ResourceCreate.ResourceType)
		default:
			t.Fatalf("Unknown event type, can't validate anonymization: %T", output.GetEvent())
		}
	}
}

// TestUsageReporterTimeSubmit verifies event submission due to elapsed time.
func TestUsageReporterTimeSubmit(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	submitter, batchChan := newTestSubmitter(2)

	reporter, cancel := newTestingUsageReporter(t, fakeClock, submitter)
	defer cancel()

	// Create a few events, bot not enough to exceed minBatchSize.
	events := createDummyEvents(0, 2)
	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(events...))

	// Advance a bit, but not enough to trigger a time-based submission.
	fakeClock.Advance(usageReporterSubmitDelay / 2)

	// Make sure no events show up.
	select {
	case e := <-batchChan:
		t.Fatalf("Received events too early: %+v", e)
	default:
		// Nothing to see yet.
	}

	// Advance enough to trigger a submission.
	fakeClock.Advance(2 * usageReporterMaxBatchAge)

	select {
	case e := <-batchChan:
		require.Len(t, e, len(events))
		compareUsageEvents(t, reporter, events, e)
	case <-time.After(2 * time.Second):
		t.Fatalf("Did not receive expected events.")
	}
}

// TestUsageReporterBatchSubmit ensures batch size-based submission works as
// expected.
func TestUsageReporterBatchSubmit(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	submitter, batchChan := newTestSubmitter(2)

	reporter, cancel := newTestingUsageReporter(t, fakeClock, submitter)
	defer cancel()

	// Create enough events to fill a batch and then some.
	events := createDummyEvents(0, 10)
	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(events...))

	// Receive the first batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[:5], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	// Submit an extra event to trigger an early send
	extra := createDummyEvents(9, 1)
	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(extra...))
	events = append(events, extra...)

	// Make sure the minimum delay is enforced for the subsequent batch.
	select {
	case e := <-batchChan:
		t.Fatalf("Received events too early: %+v", e)
	default:
		// Nothing to see yet.
	}

	fakeClock.Advance(usageReporterSubmitDelay)

	// Receive the 2nd batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[5:10], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	// With no new events, the final (added) event will be sent after the
	// regular interval.
	select {
	case e := <-batchChan:
		t.Fatalf("Received final event too early: %+v", e)
	default:
		// Nothing to see yet.
	}

	fakeClock.Advance(usageReporterMaxBatchAge)

	select {
	case e := <-batchChan:
		require.Len(t, e, 1)
		compareUsageEvents(t, reporter, events[10:], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}
}

// TestUsageReporterDiscard validates that events are discarded when the buffer
// is full.
func TestUsageReporterDiscard(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	submitter, batchChan := newTestSubmitter(2)

	reporter, cancel := newTestingUsageReporter(t, fakeClock, submitter)
	defer cancel()

	// Create enough events to fill the buffer and then some.
	events := createDummyEvents(0, 12)
	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(events...))

	// Receive the first batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[:5], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	// Wait for the regular submit interval.
	fakeClock.Advance(usageReporterMaxBatchAge)

	// Receive the final batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[5:10], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	// Wait again.
	fakeClock.Advance(usageReporterMaxBatchAge * 2)

	// Try to receive again. These events should have been discarded.
	select {
	case e := <-batchChan:
		t.Fatalf("Received unexpected events: %+v", e)
	default:
		// Nothing to see yet.
	}
}

// TestUsageReporterErrorReenqueue ensures failed events are added back to the
// queue and eventually dropped.
func TestUsageReporterErrorReenqueue(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	submitter, batchChan := newFailingSubmitter(2)

	reporter, cancel := newTestingUsageReporter(t, fakeClock, submitter)
	defer cancel()

	// Create enough events to fill the buffer and then some.
	events := createDummyEvents(0, 10)
	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(events...))

	// Receive the first batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[:5], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	// The submission fails, so events are reenqueued. This triggers an early
	// send at the submit delay rather than the full batch send interval.
	fakeClock.Advance(usageReporterSubmitDelay)

	// Receive the second batch.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[5:10], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	fakeClock.Advance(usageReporterSubmitDelay)

	// Receive the first batch again, since it was reenqueued.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[:5], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	fakeClock.Advance(usageReporterSubmitDelay)

	// Receive the second batch again, since it was reenqueued.
	select {
	case e := <-batchChan:
		require.Len(t, e, testMaxBatchSize)
		compareUsageEvents(t, reporter, events[5:10], e)
	case <-time.After(time.Second):
		t.Fatalf("Did not receive expected events.")
	}
}
