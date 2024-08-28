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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
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

	eventStream := auditLogger.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
		StartDate: timestamppb.New(time.Now()),
	})

	var n int
	var cursor string
	for eventStream.Next() {
		n++
		if n == 2 {
			cursor = eventStream.Item().Cursor
		}
	}

	require.NoError(t, eventStream.Done())
	require.Equal(t, 4, n)

	eventStream = auditLogger.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
		StartDate: timestamppb.New(time.Now()),
		Cursor:    cursor,
	})

	n = 0
	for eventStream.Next() {
		n++
	}

	require.NoError(t, eventStream.Done())
	require.Equal(t, 2, n)

	eventStream = auditLogger.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
		StartDate: timestamppb.New(time.Now().Add(48 * time.Hour)),
	})

	for eventStream.Next() {
		require.FailNowf(t, "unexpected event from future date", "event: %+v", eventStream.Item())
	}

	require.NoError(t, eventStream.Done())
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
