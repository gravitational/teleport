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
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TELEPORT_TEST_PGEVENTS_URL is a connection string similar to the one used by
// "audit_events_uri" (in teleport.yaml).
// For example: "postgresql://teleport@localhost:5432/teleport_test1?sslcert=/path/to/cert.pem&sslkey=/path/to/key.pem&sslrootcert=/path/to/ca.pem&sslmode=verify-full"
const urlEnvVar = "TELEPORT_TEST_PGEVENTS_URL"

func TestPostgresEvents(t *testing.T) {
	// Don't t.Parallel(), relies on the database backed by urlEnvVar.
	log, truncateEvents := newLog(t)

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
}

// TestLog_nonStandardSessionID tests for
// https://github.com/gravitational/teleport/issues/46207.
func TestLog_nonStandardSessionID(t *testing.T) {
	// Don't t.Parallel(), relies on the database backed by urlEnvVar.
	eventsLog, _ := newLog(t)

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
		2, // limit
		types.EventOrderAscending,
		"", // startKey
	)
	require.NoError(t, err, "search session events")
	want := []apievents.AuditEvent{appStartEvent}
	if diff := cmp.Diff(want, appEvents, protocmp.Transform()); diff != "" {
		t.Errorf("searchEvents mismatch (-want +got)\n%s", diff)
	}
}

func newLog(t *testing.T) (eventsLog *Log, truncateEvents func(*testing.T)) {
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

	eventsLog, err = New(ctx, cfg)
	require.NoError(t, err, "create new Log")
	t.Cleanup(func() { assert.NoError(t, eventsLog.Close(), "close events log") })

	truncateEvents = func(t *testing.T) {
		_, err := eventsLog.pool.Exec(ctx, "TRUNCATE events")
		require.NoError(t, err, "truncate events")
	}
	return eventsLog, truncateEvents
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
	testLog := utils.NewSlogLoggerForTests()

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
