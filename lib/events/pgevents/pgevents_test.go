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

package pgevents

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

// TELEPORT_TEST_PGEVENTS_URL is a connection string similar to the one used by
// "audit_events_uri" (in teleport.yaml).
// For example: "postgresql://teleport@localhost:5432/teleport_test1?sslcert=/path/to/cert.pem&sslkey=/path/to/key.pem&sslrootcert=/path/to/ca.pem&sslmode=verify-full"
const urlEnvVar = "TELEPORT_TEST_PGEVENTS_URL"

func TestPostgresEvents(t *testing.T) {
	// Don't t.Parallel(), relies on the database backed by urlEnvVar.
	log := newLogForTesting(t)

	ctx := context.Background()

	truncateEvents := func(t *testing.T) {
		_, err := log.pool.Exec(ctx, "TRUNCATE events")
		require.NoError(t, err, "truncate events")
	}

	suite := test.EventsSuite{
		Log:   log,
		Clock: clockwork.NewRealClock(),
	}

	t.Run("SessionEventsCRUD", func(t *testing.T) {
		// The tests in the suite expect a blank slate each time.
		truncateEvents(t)
		suite.SessionEventsCRUD(t)
	})
	t.Run("EventPagination", func(t *testing.T) {
		truncateEvents(t)
		suite.EventPagination(t)
	})
	t.Run("SearchSessionEventsBySessionID", func(t *testing.T) {
		truncateEvents(t)
		suite.SearchSessionEventsBySessionID(t)
	})
	t.Run("SearchEventsBySearchTerm", func(t *testing.T) {
		truncateEvents(t)
		suite.SearchEventsBySearchTerm(t)
	})
}

// TestLog_nonStandardSessionID tests for
// https://github.com/gravitational/teleport/issues/46207.
func TestLog_nonStandardSessionID(t *testing.T) {
	// Don't t.Parallel(), relies on the database backed by urlEnvVar.
	eventsLog := newLogForTesting(t)

	// Example app event. Only the session ID matters for the test, everything
	// else is realistic but irrelevant here.
	eventTime := time.Now()
	appStartEvent := &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: "zarq",
			Time:        eventTime,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   "17.2.2",
			ServerID:        "18d877c6-c8ab-46fc-9806-b638c0d6c556",
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			// IMPORTANT: not an UUID!
			SessionID: "f8571503d72f35938ce5001b792baebcce3183719ae947fde1ed685f7848facc",
		},
		UserMetadata: apievents.UserMetadata{
			User:     "alpaca",
			UserKind: apievents.UserKind_USER_KIND_HUMAN,
		},
		PublicAddr: "dumper.zarq.dev",
		AppMetadata: apievents.AppMetadata{
			AppURI:        "http://127.0.0.1:52932",
			AppPublicAddr: "dumper.zarq.dev",
			AppName:       "dumper",
		},
	}

	ctx := context.Background()

	// Emit event with non-standard session ID.
	require.NoError(t,
		eventsLog.EmitAuditEvent(ctx, appStartEvent),
		"emit audit event",
	)

	// Search event by the same (non-standard) session ID.
	// SearchSessionEvents has a hard-coded list of eventTypes that excludes App
	// events, so we must use searchEvents instead.
	before := eventTime.Add(-1 * time.Second)
	after := eventTime.Add(1 * time.Second)
	appEvents, _, err := eventsLog.searchEvents(ctx,
		before,                                // fromTime
		after,                                 // toTime
		[]string{appStartEvent.Metadata.Type}, // eventTypes
		nil,                                   // cond
		appStartEvent.SessionID,
		"", // search
		2,  // limit
		types.EventOrderAscending,
		"", // startKey
	)
	require.NoError(t, err, "search session events")
	wantFields, err := events.ToEventFields(appStartEvent)
	require.NoError(t, err, "convert event to fields")
	want := []events.EventFields{wantFields}
	if diff := cmp.Diff(want, appEvents); diff != "" {
		t.Errorf("searchEvents mismatch (-want +got)\n%s", diff)
	}
}

func newLogForTesting(t *testing.T) *Log {
	t.Helper()

	connString, ok := os.LookupEnv(urlEnvVar)
	if !ok {
		t.Skipf("Missing %v environment variable.", urlEnvVar)
	}
	u, err := url.Parse(connString)
	require.NoError(t, err, "parse Postgres connString from %s", urlEnvVar)

	var cfg Config
	require.NoError(t, cfg.SetFromURL(u), "cfg.SetFromURL")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	eventsLog, err := New(ctx, cfg)
	require.NoError(t, err, "create new Log")
	t.Cleanup(func() { assert.NoError(t, eventsLog.Close(), "close events log") })

	return eventsLog
}

func TestConfig(t *testing.T) {
	configs := map[string]*Config{
		"postgres://foo#auth_mode=azure": {
			AuthConfig: pgcommon.AuthConfig{
				AuthMode: pgcommon.AzureADAuth,
			},
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#auth_mode=gcp-cloudsql&gcp_connection_name=project:location:instance&gcp_ip_type=private": {
			AuthConfig: pgcommon.AuthConfig{
				AuthMode:          pgcommon.GCPCloudSQLIAMAuth,
				GCPConnectionName: "project:location:instance",
				GCPIPType:         pgcommon.GCPIPTypePrivateIP,
			},
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo": {
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#retention_period=2160h": {
			RetentionPeriod: 2160 * time.Hour,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#disable_cleanup=true": {
			DisableCleanup:  true,
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#cert_reload_interval=1h": {
			RetentionPeriod:    defaultRetentionPeriod,
			CleanupInterval:    defaultCleanupInterval,
			CertReloadInterval: time.Hour,
		},

		"postgres://foo#auth_mode=invalid-auth-mode": nil,
	}

	for u, expectedConfig := range configs {
		u, err := url.Parse(u)
		require.NoError(t, err)
		var actualConfig Config
		require.NoError(t, actualConfig.SetFromURL(u))

		if expectedConfig == nil {
			require.Error(t, actualConfig.CheckAndSetDefaults())
			continue
		}

		require.NoError(t, actualConfig.CheckAndSetDefaults())
		actualConfig.Log = nil
		actualConfig.PoolConfig = nil

		require.Equal(t, expectedConfig, &actualConfig)
	}
}

func TestBuildSchema(t *testing.T) {
	testLog := logtest.NewLogger()

	testConfig := &Config{
		Log: testLog,
	}

	hasDateIndex := func(t require.TestingT, schemasRaw any, args ...any) {
		require.IsType(t, []string(nil), schemasRaw)
		schemas := schemasRaw.([]string)
		require.NotEmpty(t, schemas)
		require.Contains(t, schemas[0], dateIndex, args...)
	}
	hasNoDateIndex := func(t require.TestingT, schemasRaw any, args ...any) {
		require.IsType(t, []string(nil), schemasRaw)
		schemas := schemasRaw.([]string)
		require.NotContains(t, schemas[0], dateIndex, args...)
	}

	tests := []struct {
		name         string
		isCockroach  bool
		assertSchema require.ValueAssertionFunc
	}{
		{
			name:         "postgres",
			isCockroach:  false,
			assertSchema: hasDateIndex,
		},
		{
			name:         "cockroach",
			isCockroach:  true,
			assertSchema: hasNoDateIndex,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemas, _ := buildSchema(tt.isCockroach, testConfig)
			tt.assertSchema(t, schemas)
		})
	}
}

func TestEmitAuditEvent_NoLossDeduplication(t *testing.T) {
	// Don't t.Parallel(), relies on the database backed by urlEnvVar.
	log := newLogForTesting(t)
	ctx := t.Context()

	truncate := func(t *testing.T) {
		t.Helper()
		_, err := log.pool.Exec(ctx, "TRUNCATE events")
		require.NoError(t, err, "truncate events")
	}

	eventTime := time.Now().UTC().Truncate(time.Microsecond)
	from := eventTime.Add(-time.Minute)
	to := eventTime.Add(time.Minute)

	t.Run("IdenticalReDeliveryIsDeduplicated", func(t *testing.T) {
		truncate(t)
		id := uuid.NewString()
		ev := makeDedupTestEvent(id, eventTime, "alpaca")

		before := testutil.ToFloat64(writeRequestsDeduped)
		require.NoError(t, log.EmitAuditEvent(ctx, ev), "first delivery")
		require.NoError(t, log.EmitAuditEvent(ctx, ev), "at-least-once re-delivery")

		stored := fetchEventData(ctx, t, log, from, to)
		require.Len(t, stored, 1, "an identical re-delivery should be stored exactly once")
		require.InDelta(t, before+1, testutil.ToFloat64(writeRequestsDeduped), 0.0001,
			"the duplicate delivery should increment the deduped counter exactly once")
	})

	t.Run("DistinctEventsSharingIDAreBothStored", func(t *testing.T) {
		truncate(t)
		id := uuid.NewString()
		alice := makeDedupTestEvent(id, eventTime, "alice")
		bob := makeDedupTestEvent(id, eventTime, "bob") // same (id, time), different payload

		before := testutil.ToFloat64(eventIDCollisions)
		require.NoError(t, log.EmitAuditEvent(ctx, alice))
		require.NoError(t, log.EmitAuditEvent(ctx, bob))

		stored := fetchEventData(ctx, t, log, from, to)
		require.Len(t, stored, 2, "two distinct events sharing an id must both be stored")
		require.True(t, containsMarker(stored, "alice"), "the event for alice must not be dropped")
		require.True(t, containsMarker(stored, "bob"), "the event for bob must not be dropped")
		require.InDelta(t, before+1, testutil.ToFloat64(eventIDCollisions), 0.0001,
			"the colliding distinct event should increment the collision counter exactly once")
	})

	t.Run("ConcurrentDistinctEventsSharingIDAreNotLost", func(t *testing.T) {
		truncate(t)
		id := uuid.NewString()
		const n = 20

		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() {
				ev := makeDedupTestEvent(id, eventTime, fmt.Sprintf("user-%02d", i))
				errs[i] = log.EmitAuditEvent(ctx, ev)
			})
		}
		wg.Wait()

		for i, err := range errs {
			require.NoError(t, err, "concurrent emit %d", i)
		}

		stored := fetchEventData(ctx, t, log, from, to)
		for i := range n {
			marker := fmt.Sprintf("user-%02d", i)
			require.True(t, containsMarker(stored, marker),
				"distinct event %q must not be lost under concurrent delivery", marker)
		}
	})
}

func makeDedupTestEvent(id string, eventTime time.Time, marker string) *apievents.AppSessionStart {
	return &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			ID:          id,
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: "mycluster",
			Time:        eventTime,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        "18d877c6-c8ab-46fc-9806-b638c0d6c556",
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: "11111111-1111-1111-1111-111111111111",
		},
		UserMetadata: apievents.UserMetadata{
			User:     marker,
			UserKind: apievents.UserKind_USER_KIND_HUMAN,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        "http://127.0.0.1:52932",
			AppPublicAddr: "dumper.zarq.dev",
			AppName:       "dumper",
		},
	}
}

func fetchEventData(ctx context.Context, t *testing.T, log *Log, from, to time.Time) []string {
	t.Helper()
	rows, err := log.pool.Query(ctx,
		"SELECT event_data::text FROM events WHERE event_time BETWEEN $1 AND $2",
		from, to,
	)
	require.NoError(t, err, "query events")
	defer rows.Close()

	var out []string
	for rows.Next() {
		var data string
		require.NoError(t, rows.Scan(&data), "scan event_data")
		out = append(out, data)
	}
	require.NoError(t, rows.Err(), "iterate event rows")
	return out
}

func containsMarker(data []string, marker string) bool {
	for _, d := range data {
		if strings.Contains(d, marker) {
			return true
		}
	}
	return false
}
