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

package dynamoevents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const dynamoDBLargeQueryRetries int = 10

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type dynamoContext struct {
	log   *Log
	suite test.EventsSuite
}

func setupDynamoContext(t *testing.T) *dynamoContext {
	testEnabled := os.Getenv(teleport.AWSRunTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping AWS-dependent test suite.")
	}
	fakeClock := clockwork.NewFakeClockAt(time.Now().UTC())

	log, err := New(context.Background(), Config{
		Tablename:          fmt.Sprintf("teleport-test-%v", uuid.New().String()),
		Clock:              fakeClock,
		UIDGenerator:       utils.NewFakeUID(),
		ReadCapacityUnits:  100,
		WriteCapacityUnits: 100,
	})
	require.NoError(t, err)

	// Clear all items in table.
	err = log.deleteAllItems(context.Background())
	require.NoError(t, err)

	tt := &dynamoContext{
		log: log,
		suite: test.EventsSuite{
			Log:        log,
			Clock:      fakeClock,
			QueryDelay: time.Second * 5,
		},
	}

	t.Cleanup(func() {
		tt.Close(t)
	})

	return tt
}

func (tt *dynamoContext) Close(t *testing.T) {
	if tt.log != nil {
		err := tt.log.deleteTable(context.Background(), tt.log.Tablename, true)
		require.NoError(t, err)
	}
}

func TestPagination(t *testing.T) {
	tt := setupDynamoContext(t)

	tt.suite.EventPagination(t)
}

func TestSessionEventsCRUD(t *testing.T) {
	tt := setupDynamoContext(t)

	tt.suite.SessionEventsCRUD(t)
}

func TestSearchSessionEvensBySessionID(t *testing.T) {
	tt := setupDynamoContext(t)

	tt.suite.SearchSessionEventsBySessionID(t)
}

// TestCheckpointOutsideOfWindow tests if [Log] doesn't panic
// if checkpoint date is outside of the window [fromUTC,toUTC].
func TestCheckpointOutsideOfWindow(t *testing.T) {
	tt := &Log{
		logger: slog.With(teleport.ComponentKey, teleport.ComponentDynamoDB),
	}

	key := checkpointKey{
		Date: "2022-10-02",
	}
	keyB, err := json.Marshal(key)
	require.NoError(t, err)

	results, nextKey, err := tt.SearchEvents(
		context.Background(),
		events.SearchEventsRequest{
			From:     time.Date(2021, 10, 10, 0, 0, 0, 0, time.UTC),
			To:       time.Date(2021, 11, 10, 0, 0, 0, 0, time.UTC),
			Limit:    100,
			StartKey: string(keyB),
			Order:    types.EventOrderAscending,
		},
	)
	require.NoError(t, err)
	require.Empty(t, results)
	require.Empty(t, nextKey)
}

func TestSizeBreak(t *testing.T) {
	tt := setupDynamoContext(t)

	const eventSize = 50 * 1024
	blob := randStringAlpha(eventSize)

	const eventCount int = 10
	for i := range eventCount {
		err := tt.suite.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: "bob"},
			Metadata: apievents.Metadata{
				Type: events.UserLoginEvent,
				Time: tt.suite.Clock.Now().UTC().Add(time.Second * time.Duration(i)),
			},
			IdentityAttributes: apievents.MustEncodeMap(map[string]any{"test.data": blob}),
		})
		require.NoError(t, err)
	}

	var checkpoint string
	gotEvents := make([]apievents.AuditEvent, 0)
	ctx := context.Background()
	for {
		fetched, lCheckpoint, err := tt.log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     tt.suite.Clock.Now().UTC().Add(-time.Hour),
			To:       tt.suite.Clock.Now().UTC().Add(time.Hour),
			Limit:    eventCount,
			Order:    types.EventOrderDescending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		checkpoint = lCheckpoint
		gotEvents = append(gotEvents, fetched...)

		if checkpoint == "" {
			break
		}
	}

	lastTime := tt.suite.Clock.Now().UTC().Add(time.Hour)

	for _, event := range gotEvents {
		require.True(t, event.GetTime().Before(lastTime))
		lastTime = event.GetTime()
	}
}

// TestIndexExists tests functionality of the `Log.indexExists` function.
func TestIndexExists(t *testing.T) {
	tt := setupDynamoContext(t)

	hasIndex, err := tt.log.indexExists(context.Background(), tt.log.Tablename, indexTimeSearchV2)
	require.NoError(t, err)
	require.True(t, hasIndex)
}

// TestDateRangeGenerator tests the `daysBetween` function which generates ISO
// 6801 date strings for every day between two points in time.
func TestDateRangeGenerator(t *testing.T) {
	// date range within a month
	start := time.Date(2021, 4, 10, 8, 5, 0, 0, time.UTC)
	end := start.Add(time.Hour * time.Duration(24*4))
	days := daysBetween(start, end)
	require.Equal(t, []string{"2021-04-10", "2021-04-11", "2021-04-12", "2021-04-13", "2021-04-14"}, days)

	// date range transitioning between two months
	start = time.Date(2021, 8, 30, 8, 5, 0, 0, time.UTC)
	end = start.Add(time.Hour * time.Duration(24*2))
	days = daysBetween(start, end)
	require.Equal(t, []string{"2021-08-30", "2021-08-31", "2021-09-01"}, days)
}

// TestLargeTableRetrieve checks that we can retrieve all items from a large
// table at once. It is run in a separate suite with its own table to avoid the
// prolonged table clearing and the consequent 'test timed out' errors.
func TestLargeTableRetrieve(t *testing.T) {
	tt := setupDynamoContext(t)

	const eventCount = 4000
	for range eventCount {
		err := tt.suite.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: "bob"},
			Metadata: apievents.Metadata{
				Type: events.UserLoginEvent,
				Time: tt.suite.Clock.Now().UTC(),
			},
		})
		require.NoError(t, err)
	}

	var (
		history []apievents.AuditEvent
		err     error
	)
	ctx := context.Background()
	for range dynamoDBLargeQueryRetries {
		time.Sleep(tt.suite.QueryDelay)

		history, _, err = tt.suite.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:  tt.suite.Clock.Now().Add(-1 * time.Hour),
			To:    tt.suite.Clock.Now().Add(time.Hour),
			Order: types.EventOrderAscending,
		})
		require.NoError(t, err)

		if len(history) == eventCount {
			break
		}
	}

	// `check.HasLen` prints the entire array on failure, which pollutes the output.
	require.Len(t, history, eventCount)
}

func TestFromWhereExpr(t *testing.T) {
	t.Parallel()

	t.Run("sid", func(t *testing.T) {
		cond := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{Field: events.SessionEventID},
			R: &types.WhereExpr{Literal: "test-session-id"},
		}}
		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "FieldsMap.#condName0 = :condValue0", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "sid"},
			attrValues: map[string]any{":condValue0": "test-session-id"},
		}, params)
	})

	t.Run("contains", func(t *testing.T) {
		// !(equals(login, "root") || equals(login, "admin")) && contains(participants, "test-user")
		cond := &types.WhereExpr{And: types.WhereExpr2{
			L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
			}}},
			R: &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "test-user"}}},
		}}

		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "(NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND (contains(FieldsMap.#condName1, :condValue2))", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
			attrValues: map[string]any{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user"},
		}, params)
	})

	t.Run("contains_any", func(t *testing.T) {
		// !(equals(login, "root") || equals(login, "admin")) && contains_any(participants, set("test-user","other-user"))
		cond := &types.WhereExpr{And: types.WhereExpr2{
			L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
			}}},
			R: &types.WhereExpr{ContainsAny: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: []string{"test-user", "other-user"}}}},
		}}

		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "(NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND ((contains(FieldsMap.#condName1, :condValue2) OR contains(FieldsMap.#condName1, :condValue3)))", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
			attrValues: map[string]any{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user", ":condValue3": "other-user"},
		}, params)
	})

	t.Run("contains_all", func(t *testing.T) {
		// !(equals(login, "root") || equals(login, "admin")) && contains_all(participants, set("test-user","other-user"))
		cond := &types.WhereExpr{And: types.WhereExpr2{
			L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
				L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
				R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
			}}},
			R: &types.WhereExpr{ContainsAll: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: []string{"test-user", "other-user"}}}},
		}}

		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "(NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND ((contains(FieldsMap.#condName1, :condValue2) AND contains(FieldsMap.#condName1, :condValue3)))", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
			attrValues: map[string]any{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user", ":condValue3": "other-user"},
		}, params)
	})

	t.Run("can_view AND", func(t *testing.T) {
		// !(equals(login, "root") || equals(login, "admin")) && contains_all(participants, set("test-user","other-user")) && can_view()
		cond := &types.WhereExpr{
			And: types.WhereExpr2{
				L: &types.WhereExpr{And: types.WhereExpr2{
					L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
						L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
						R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
					}}},
					R: &types.WhereExpr{ContainsAll: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: []string{"test-user", "other-user"}}}},
				}},
				R: &types.WhereExpr{CanView: &types.WhereNoExpr{}},
			},
		}

		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "((NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND ((contains(FieldsMap.#condName1, :condValue2) AND contains(FieldsMap.#condName1, :condValue3)))) AND (attribute_exists(SessionID))", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
			attrValues: map[string]any{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user", ":condValue3": "other-user"},
		}, params)
	})
	t.Run("can_view OR ", func(t *testing.T) {
		// !(equals(login, "root") || equals(login, "admin")) && contains_all(participants, set("test-user","other-user")) || can_view()
		cond := &types.WhereExpr{
			Or: types.WhereExpr2{
				L: &types.WhereExpr{And: types.WhereExpr2{
					L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
						L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
						R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
					}}},
					R: &types.WhereExpr{ContainsAll: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: []string{"test-user", "other-user"}}}},
				}},
				R: &types.WhereExpr{CanView: &types.WhereNoExpr{}},
			},
		}

		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)

		require.Equal(t, "((NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND ((contains(FieldsMap.#condName1, :condValue2) AND contains(FieldsMap.#condName1, :condValue3)))) OR (attribute_exists(SessionID))", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
			attrValues: map[string]any{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user", ":condValue3": "other-user"},
		}, params)
	})

	t.Run("map_ref", func(t *testing.T) {
		// session.server_labels["env"] = "prod"
		cond := &types.WhereExpr{Equals: types.WhereExpr2{
			L: &types.WhereExpr{MapRef: &types.WhereExpr2{
				L: &types.WhereExpr{Field: "server_labels"},
				R: &types.WhereExpr{Literal: "env"},
			}},
			R: &types.WhereExpr{Literal: "prod"},
		}}
		params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]any{}}
		expr, err := fromWhereExpr(cond, &params)
		require.NoError(t, err)
		require.Equal(t, "FieldsMap.#condName0.#condName1 = :condValue0", expr)
		require.Equal(t, condFilterParams{
			attrNames:  map[string]string{"#condName0": "server_labels", "#condName1": "env"},
			attrValues: map[string]any{":condValue0": "prod"},
		}, params)
	})
}

// TestEmitAuditEventForLargeEvents tries to emit large audit events to
// DynamoDB backend.
func TestEmitAuditEventForLargeEvents(t *testing.T) {
	tt := setupDynamoContext(t)

	ctx := context.Background()
	now := tt.suite.Clock.Now().UTC()
	dbQueryEvent := &apievents.DatabaseSessionQuery{
		Metadata: apievents.Metadata{
			Time: tt.suite.Clock.Now().UTC(),
			Type: events.DatabaseSessionQueryEvent,
		},
		DatabaseQuery: strings.Repeat("A", maxItemSize),
	}
	err := tt.suite.Log.EmitAuditEvent(ctx, dbQueryEvent)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		result, _, err := tt.suite.Log.SearchEvents(ctx, events.SearchEventsRequest{
			From:       now.Add(-1 * time.Hour),
			To:         now.Add(time.Hour),
			EventTypes: []string{events.DatabaseSessionQueryEvent},
			Order:      types.EventOrderAscending,
		})
		require.NoError(t, err)
		require.Len(t, result, 1)
	}, 10*time.Second, 500*time.Millisecond)

	appReqEvent := &testAuditEvent{
		AppSessionRequest: apievents.AppSessionRequest{
			Metadata: apievents.Metadata{
				Time: tt.suite.Clock.Now().UTC(),
				Type: events.AppSessionRequestEvent,
			},
			Path: strings.Repeat("A", maxItemSize),
		},
	}
	err = tt.suite.Log.EmitAuditEvent(ctx, appReqEvent)
	require.ErrorContains(t, err, "ValidationException: Item size has exceeded the maximum allowed size")
}

// testAuditEvent wraps an existing AuditEvent, but overrides
// the TrimToMaxSize to be a noop so that functionality can
// be tested if an event exceeds the size limits.
type testAuditEvent struct {
	apievents.AppSessionRequest
}

func (t *testAuditEvent) TrimToMaxSize(maxSizeBytes int) apievents.AuditEvent {
	return t
}

func TestConfig_SetFromURL(t *testing.T) {
	useFipsCfg := Config{
		UseFIPSEndpoint: types.ClusterAuditConfigSpecV2_FIPS_ENABLED,
	}
	cases := []struct {
		name         string
		url          string
		cfg          Config
		cfgAssertion func(*testing.T, Config)
	}{
		{
			name: "fips enabled via url",
			url:  "dynamodb://event_table_name?use_fips_endpoint=true",
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_ENABLED, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips disabled via url",
			url:  "dynamodb://event_table_name?use_fips_endpoint=false&endpoint=dynamo.example.com",
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_DISABLED, config.UseFIPSEndpoint)
				require.Equal(t, "dynamo.example.com", config.Endpoint)
			},
		},
		{
			name: "fips mode not set",
			url:  "dynamodb://event_table_name",
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_UNSET, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips mode enabled by default",
			url:  "dynamodb://event_table_name",
			cfg:  useFipsCfg,
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_ENABLED, config.UseFIPSEndpoint)
			},
		},
		{
			name: "fips mode can be overridden",
			url:  "dynamodb://event_table_name?use_fips_endpoint=false",
			cfg:  useFipsCfg,
			cfgAssertion: func(t *testing.T, config Config) {
				require.Equal(t, types.ClusterAuditConfigSpecV2_FIPS_DISABLED, config.UseFIPSEndpoint)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := url.Parse(tt.url)
			require.NoError(t, err)
			require.NoError(t, tt.cfg.SetFromURL(uri))

			tt.cfgAssertion(t, tt.cfg)
		})
	}
}

func TestConfig_CheckAndSetDefaults(t *testing.T) {
	zero := types.NewDuration(0)
	hour := types.NewDuration(time.Hour)

	tests := []struct {
		name        string
		config      *Config
		assertionFn func(t *testing.T, cfg *Config, err error)
	}{
		{
			name:   "table name required",
			config: &Config{},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %T", err)
			},
		},
		{
			name: "nil retention period uses default",
			config: &Config{
				Tablename:       "test",
				RetentionPeriod: nil,
			},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.NoError(t, err)
				require.Equal(t, DefaultRetentionPeriod.Duration(), cfg.RetentionPeriod.Duration())
			},
		},
		{
			name: "zero retention period uses default",
			config: &Config{
				Tablename:       "test",
				RetentionPeriod: &zero,
			},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.NoError(t, err)
				require.Equal(t, DefaultRetentionPeriod.Duration(), cfg.RetentionPeriod.Duration())
			},
		},
		{
			name: "supplied retention period is used",
			config: &Config{
				Tablename:       "test",
				RetentionPeriod: &hour,
			},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.NoError(t, err)
				require.Equal(t, hour.Duration(), cfg.RetentionPeriod.Duration())
			},
		},
		{
			name:   "zero capacity uses defaults",
			config: &Config{Tablename: "test"},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(DefaultReadCapacityUnits), cfg.ReadCapacityUnits)
				require.Equal(t, int64(DefaultWriteCapacityUnits), cfg.WriteCapacityUnits)
			},
		},
		{
			name: "supplied capacity is used",
			config: &Config{
				Tablename:          "test",
				ReadCapacityUnits:  1,
				WriteCapacityUnits: 7,
			},
			assertionFn: func(t *testing.T, cfg *Config, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(1), cfg.ReadCapacityUnits)
				require.Equal(t, int64(7), cfg.WriteCapacityUnits)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.CheckAndSetDefaults()
			test.assertionFn(t, test.config, err)
		})
	}
}

// TestEmitSessionEventsSameIndex given events that share the same session ID
// and index, the emit should succeed.
func TestEmitSessionEventsSameIndex(t *testing.T) {
	ctx := context.Background()
	tt := setupDynamoContext(t)
	sessionID := session.NewID()

	require.NoError(t, tt.log.EmitAuditEvent(ctx, generateEvent(sessionID, 0, "")))
	require.NoError(t, tt.log.EmitAuditEvent(ctx, generateEvent(sessionID, 1, "")))
	require.NoError(t, tt.log.EmitAuditEvent(ctx, generateEvent(sessionID, 1, "")))
}

// TestSearchEventsLimitEndOfDay tests if the search events function can handle
// moving the cursor to the next day when the limit is reached exactly at the
// end of the day.
// This only works if tests run against a real DynamoDB instance.
func TestSearchEventsLimitEndOfDay(t *testing.T) {

	ctx := context.Background()
	tt := setupDynamoContext(t)
	blob := "data"
	const eventCount int = 10

	// create events for two days
	for dayDiff := range 2 {
		for i := range eventCount {
			err := tt.suite.Log.EmitAuditEvent(ctx, &apievents.UserLogin{
				Method:       events.LoginMethodSAML,
				Status:       apievents.Status{Success: true},
				UserMetadata: apievents.UserMetadata{User: "bob"},
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Time: tt.suite.Clock.Now().UTC().Add(time.Hour*24*time.Duration(dayDiff) + time.Second*time.Duration(i)),
				},
				IdentityAttributes: apievents.MustEncodeMap(map[string]any{"test.data": blob}),
			})
			require.NoError(t, err)
		}
	}

	windowStart := time.Date(
		tt.suite.Clock.Now().UTC().Year(),
		tt.suite.Clock.Now().UTC().Month(),
		tt.suite.Clock.Now().UTC().Day(),
		0, /* hour */
		0, /* minute */
		0, /* second */
		0, /* nanosecond */
		time.UTC)
	windowEnd := windowStart.Add(time.Hour * 24)

	data, err := json.Marshal(checkpointKey{
		Date: windowStart.Format("2006-01-02"),
	})
	require.NoError(t, err)
	checkpoint := string(data)

	var gotEvents []apievents.AuditEvent
	for {
		fetched, lCheckpoint, err := tt.log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     windowStart,
			To:       windowEnd,
			Limit:    eventCount,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		checkpoint = lCheckpoint
		gotEvents = append(gotEvents, fetched...)

		if checkpoint == "" {
			break
		}
	}

	require.Len(t, gotEvents, eventCount)
	lastTime := tt.suite.Clock.Now().UTC().Add(-time.Hour)

	for _, event := range gotEvents {
		require.True(t, event.GetTime().After(lastTime))
		lastTime = event.GetTime()
	}
}

// TestValidationErrorsHandling given events that return validation
// errors (large event size and already exists), the emit should handle them
// and succeed on emitting the event when it does support trimming.
func TestValidationErrorsHandling(t *testing.T) {
	ctx := context.Background()
	tt := setupDynamoContext(t)
	sessionID := session.NewID()
	largeQuery := strings.Repeat("A", maxItemSize)

	// First write should only trigger the large event size
	require.NoError(t, tt.log.EmitAuditEvent(ctx, generateEvent(sessionID, 0, largeQuery)))
	// Second should trigger both errors.
	require.NoError(t, tt.log.EmitAuditEvent(ctx, generateEvent(sessionID, 0, largeQuery)))
}

func generateEvent(sessionID session.ID, index int64, query string) apievents.AuditEvent {
	return &apievents.DatabaseSessionQuery{
		Metadata: apievents.Metadata{
			Type:        events.DatabaseSessionQueryEvent,
			ClusterName: "root",
			Index:       index,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sessionID.String(),
		},
		DatabaseQuery: query,
	}
}

func randStringAlpha(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.N(len(letters))]
	}
	return string(b)
}

func TestEndpoints(t *testing.T) {
	// Don't t.Parallel(), uses t.Setenv and modules.SetTestModules.

	tests := []struct {
		name          string
		fips          bool
		envVarValue   string // value for the _DISABLE_FIPS environment variable
		wantFIPSError bool
	}{
		{
			name:          "fips",
			fips:          true,
			wantFIPSError: true,
		},
		{
			name:          "fips with env skip",
			fips:          true,
			envVarValue:   "yes",
			wantFIPSError: false,
		},
		{
			name:          "without fips",
			wantFIPSError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_AWS_FIPS", tt.envVarValue)

			fips := types.ClusterAuditConfigSpecV2_FIPS_DISABLED
			if tt.fips {
				fips = types.ClusterAuditConfigSpecV2_FIPS_ENABLED
				modulestest.SetTestModules(t, modulestest.Modules{
					FIPS: true,
				})
			}

			mux := http.NewServeMux()
			mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}))

			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			b, err := New(context.Background(), Config{
				Region:       "us-west-1",
				Tablename:    "teleport-test",
				UIDGenerator: utils.NewFakeUID(),
				// The prefix is intentionally removed to validate that a scheme
				// is applied automatically. This validates backwards compatible behavior
				// with existing configurations and the behavior change from aws-sdk-go to aws-sdk-go-v2.
				Endpoint:        strings.TrimPrefix(server.URL, "http://"),
				Insecure:        true,
				UseFIPSEndpoint: fips,
				CredentialsProvider: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{}, nil
				}),
			})
			// FIPS mode should fail because it is a violation to enable FIPS
			// while also setting a custom endpoint.
			if tt.wantFIPSError {
				assert.ErrorContains(t, err, "FIPS")
				return
			}

			assert.ErrorContains(t, err, fmt.Sprintf("StatusCode: %d", http.StatusTeapot))
			assert.Nil(t, b, "backend not nil")
		})
	}
}

func TestStartKeyBackCompat(t *testing.T) {
	const (
		oldStartKey = `{"date":"2023-04-27","iterator":{"CreatedAt":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"1682583778","NS":null,"NULL":null,"S":null,"SS":null},"CreatedAtDate":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"2023-04-27","SS":null},"EventIndex":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"0","NS":null,"NULL":null,"S":null,"SS":null},"SessionID":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb","SS":null}}}`
		newStartKey = `{"date":"2023-04-27","iterator":"{\"CreatedAt\":1682583778,\"CreatedAtDate\":\"2023-04-27\",\"EventIndex\":0,\"SessionID\":\"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb\"}"}`
	)

	oldCP, err := getCheckpointFromStartKey(oldStartKey)
	require.NoError(t, err)

	newCP, err := getCheckpointFromStartKey(newStartKey)
	require.NoError(t, err)

	// we must check the iterator field equality separately because it's a string
	// containing a JSON-encoded event and field ordering might not be consistent.
	require.Equal(t, oldCP.EventKey, newCP.EventKey)
	require.Equal(t, oldCP.Date, newCP.Date)

	var oldIterator, newIterator event
	require.NoError(t, json.Unmarshal([]byte(oldCP.Iterator), &oldIterator))
	require.NoError(t, json.Unmarshal([]byte(newCP.Iterator), &newIterator))
	require.Equal(t, oldIterator, newIterator)
}

// TestCursorIteratorPrecision exists because cursors are sensitive to data-loss
// and we had a bug where we would unmarshall a cursor into `map[string]any`,
// causing all int64 to do a round-trip through float64 and losing precision.
// The precision loss would cause the cursor hash to be in te past.
// If the cursor shifts by more events than the page size, this creates a
// livelock and the query cannot proceed.
// This test creates events with very close EventIndex (1 nanosecond diff),
// reads the events 1 by one, and makes sure the reader is not stuck reading the
// same event over and over.
func TestCursorIteratorPrecision(t *testing.T) {
	tt := setupDynamoContext(t)
	clock, ok := tt.log.Clock.(*clockwork.FakeClock)
	require.True(t, ok, "this test requires a FakeClock")
	baseTime := clock.Now().UTC()

	// Test Setup: creating fixtures really close in the dynamo index.

	// For this test to work, we need the same session ID for all events
	sessionId := uuid.NewString()
	numEvents := 5
	testEvents := make(map[string]struct{}, numEvents)

	for range numEvents {
		id := uuid.NewString()
		// For the first event, EventIndex will be zero, for the next ones it
		// will be the unix nanosecond timestamp.
		clock.Advance(time.Nanosecond)
		err := tt.log.EmitAuditEvent(context.Background(), &apievents.Exec{
			UserMetadata: apievents.UserMetadata{User: "test-user"},
			Metadata: apievents.Metadata{
				ID:   id,
				Type: events.UserLoginEvent,
				Time: clock.Now().UTC(),
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: sessionId,
			},
		})
		testEvents[id] = struct{}{}
		require.NoError(t, err)
	}

	// Test execution: do paginated queries to read all the fixtures.
	eventsSeen := make(map[string]apievents.AuditEvent, numEvents)
	toTime := baseTime.Add(time.Hour)
	var arr []apievents.AuditEvent
	var err error
	var checkpoint string

	for range testEvents {
		arr, checkpoint, err = tt.log.SearchEvents(t.Context(), events.SearchEventsRequest{
			From:     baseTime,
			To:       toTime,
			Limit:    1,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		require.NoError(t, err)
		require.Len(t, arr, 1)

		id := arr[0].GetID()
		var c checkpointKey
		require.NoError(t, json.Unmarshal([]byte(checkpoint), &c), "event %s", id)
		require.NotEmpty(t, c.Iterator, "event %s", id)

		var e EventKey
		require.NoError(t, json.Unmarshal([]byte(c.Iterator), &e), "event %s", id)
		eventsSeen[id] = arr[0]
	}

	// Test validation: make sure that all fixtures were read (as opposed to
	// some event being returned several times because of a cursor issue).
	for id := range testEvents {
		require.Contains(t, eventsSeen, id, "eventsSeen should contain %q", id)
	}

}
