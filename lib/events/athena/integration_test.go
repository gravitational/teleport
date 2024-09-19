// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package athena

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
)

func TestIntegrationAthenaSearchSessionEventsBySessionID(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaSearchSessionEventsBySessionID(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaSearchSessionEventsBySessionID(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaSearchSessionEventsBySessionID(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:                                  auditLogger,
		Clock:                                ac.clock,
		SearchSessionEvensBySessionIDTimeout: ac.batcherInterval + 10*time.Second,
	}

	eventsSuite.SearchSessionEventsBySessionID(t)
}

func TestIntegrationAthenaSessionEventsCRUD(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaSessionEventsCRUD(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaSessionEventsCRUD(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaSessionEventsCRUD(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.SessionEventsCRUD(t)
}

func TestIntegrationAthenaEventExport(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaEventExport(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaEventExport(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaEventExport(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.EventExport(t)
}

func TestIntegrationAthenaEventPagination(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaEventPagination(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaEventPagination(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaEventPagination(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{BypassSNS: bypassSNS})
	auditLogger := &EventuallyConsistentAuditLogger{
		Inner: ac.log,
		// Additional 5s is used to compensate for uploading parquet on s3.
		QueryDelay: ac.batcherInterval + 5*time.Second,
	}
	eventsSuite := test.EventsSuite{
		Log:   auditLogger,
		Clock: ac.clock,
	}

	eventsSuite.EventPagination(t)
}

func TestIntegrationAthenaLargeEvents(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		const bypassSNSFalse = false
		testIntegrationAthenaLargeEvents(t, bypassSNSFalse)
	})
	t.Run("sqs", func(t *testing.T) {
		const bypassSNSTrue = true
		testIntegrationAthenaLargeEvents(t, bypassSNSTrue)
	})
}

func testIntegrationAthenaLargeEvents(t *testing.T, bypassSNS bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ac := SetupAthenaContext(t, ctx, AthenaContextConfig{
		MaxBatchSize: 1,
		BypassSNS:    bypassSNS,
	})
	in := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Index: 2,
			Type:  events.SessionStartEvent,
			ID:    uuid.NewString(),
			Code:  strings.Repeat("d", 200000),
			Time:  ac.clock.Now().UTC(),
		},
	}
	err := ac.log.EmitAuditEvent(ctx, in)
	require.NoError(t, err)

	var history []apievents.AuditEvent
	// We have batch time 10s, and 5s for upload and additional buffer for s3 download
	err = retryutils.RetryStaticFor(time.Second*20, time.Second*2, func() error {
		history, _, err = ac.log.SearchEvents(ctx, events.SearchEventsRequest{
			From:  ac.clock.Now().UTC().Add(-1 * time.Minute),
			To:    ac.clock.Now().UTC(),
			Limit: 10,
			Order: types.EventOrderDescending,
		})
		if err != nil {
			return err
		}
		if len(history) == 0 {
			return errors.New("events not propagated yet")
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Empty(t, cmp.Diff(in, history[0]))
}
