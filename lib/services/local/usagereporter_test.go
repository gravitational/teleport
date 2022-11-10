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
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	prehogapi "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
)

const (
	testClusterName = "example.com"
	testClusterID   = "abcde"

	testMinBatchSize  = 3
	testMaxBatchSize  = 5
	testMaxBufferSize = 10
)

func newTestSubmitter(t *testing.T, size int) (UsageSubmitFunc, chan []*prehogapi.SubmitEventRequest) {
	ch := make(chan []*prehogapi.SubmitEventRequest, size)

	return func(reporter *UsageReporter, batch []*prehogapi.SubmitEventRequest) error {
		ch <- batch
		t.Logf("test submitter received: %+v\n", batch)
		return nil
	}, ch
}

func newFailingSubmitter() UsageSubmitFunc {
	return func(reporter *UsageReporter, batch []*prehogapi.SubmitEventRequest) error {
		return trace.BadParameter("testing error")
	}
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

	return &UsageReporter{
		Entry:           l,
		ctx:             ctx,
		cancel:          cancel,
		anonymizer:      anonymizer,
		events:          make(chan []*prehogapi.SubmitEventRequest),
		submissionQueue: make(chan []*prehogapi.SubmitEventRequest),
		submit:          submitter,
		clock:           clock,
		clusterName:     clusterName,
		minBatchSize:    testMinBatchSize,
		maxBatchSize:    testMaxBatchSize,
		maxBatchAge:     usageReporterMaxBatchAge,
		maxBufferSize:   testMaxBufferSize,
	}, cancel
}

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

func TestUsageReporter(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	submitter, batchChan := newTestSubmitter(t, 2)

	reporter, cancel := newTestingUsageReporter(t, fakeClock, submitter)
	go reporter.Run()
	defer cancel()

	events := []services.UsageAnonymizable{
		&services.UsageUserLogin{
			UserName: "alice@example.com",
		},
		&services.UsageSessionStart{
			UserName:    "alice@example.com",
			SessionType: string(types.SSHSessionKind),
		},
	}

	require.NoError(t, reporter.SubmitAnonymizedUsageEvents(events...))

	// Advance a bit, but not enough to trigger a submission.
	fakeClock.Advance(usageReporterMaxBatchAge / 2)

	select {
	case e := <-batchChan:
		t.Fatalf("Received events too early: %+v", e)
	default:
		// Nothing to see yet.
	}

	// Advance enough to trigger a submission.
	fakeClock.Advance(usageReporterMaxBatchAge)

	var submitted []*prehogapi.SubmitEventRequest
	select {
	case e := <-batchChan:
		t.Logf("Received: %+v", e)
		submitted = e
	case <-time.After(2 * time.Second):
		t.Fatalf("Did not receive expected events.")
	}

	require.Len(t, submitted, len(events))

	compareUsageEvents(t, reporter, events, submitted)
}
