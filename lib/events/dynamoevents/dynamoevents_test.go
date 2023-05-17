/*
Copyright 2018 Gravitational, Inc.

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

package dynamoevents

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

const dynamoDBLargeQueryRetries int = 10

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
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
		Region:       "eu-north-1",
		Tablename:    fmt.Sprintf("teleport-test-%v", uuid.New().String()),
		Clock:        fakeClock,
		UIDGenerator: utils.NewFakeUID(),
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

func TestSizeBreak(t *testing.T) {
	tt := setupDynamoContext(t)

	const eventSize = 50 * 1024
	blob := randStringAlpha(eventSize)

	const eventCount int = 10
	for i := 0; i < eventCount; i++ {
		err := tt.suite.Log.EmitAuditEvent(context.Background(), &apievents.UserLogin{
			Method:       events.LoginMethodSAML,
			Status:       apievents.Status{Success: true},
			UserMetadata: apievents.UserMetadata{User: "bob"},
			Metadata: apievents.Metadata{
				Type: events.UserLoginEvent,
				Time: tt.suite.Clock.Now().UTC().Add(time.Second * time.Duration(i)),
			},
			IdentityAttributes: apievents.MustEncodeMap(map[string]interface{}{"test.data": blob}),
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
	for i := 0; i < eventCount; i++ {
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
	for i := 0; i < dynamoDBLargeQueryRetries; i++ {
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

	// !(equals(login, "root") || equals(login, "admin")) && contains(participants, "test-user")
	cond := &types.WhereExpr{And: types.WhereExpr2{
		L: &types.WhereExpr{Not: &types.WhereExpr{Or: types.WhereExpr2{
			L: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}},
			R: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "admin"}}},
		}}},
		R: &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "test-user"}}},
	}}

	params := condFilterParams{attrNames: map[string]string{}, attrValues: map[string]interface{}{}}
	expr, err := fromWhereExpr(cond, &params)
	require.NoError(t, err)

	require.Equal(t, "(NOT ((FieldsMap.#condName0 = :condValue0) OR (FieldsMap.#condName0 = :condValue1))) AND (contains(FieldsMap.#condName1, :condValue2))", expr)
	require.Equal(t, condFilterParams{
		attrNames:  map[string]string{"#condName0": "login", "#condName1": "participants"},
		attrValues: map[string]interface{}{":condValue0": "root", ":condValue1": "admin", ":condValue2": "test-user"},
	}, params)
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

	result, _, err := tt.suite.Log.SearchEvents(ctx, events.SearchEventsRequest{
		From:       now.Add(-1 * time.Hour),
		To:         now.Add(time.Hour),
		EventTypes: []string{events.DatabaseSessionQueryEvent},
		Order:      types.EventOrderAscending,
	})
	require.NoError(t, err)
	require.Len(t, result, 1)

	appReqEvent := &apievents.AppSessionRequest{
		Metadata: apievents.Metadata{
			Time: tt.suite.Clock.Now().UTC(),
			Type: events.AppSessionRequestEvent,
		},
		Path: strings.Repeat("A", maxItemSize),
	}
	err = tt.suite.Log.EmitAuditEvent(ctx, appReqEvent)
	require.ErrorContains(t, err, "ValidationException: Item size has exceeded the maximum allowed size")
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringAlpha(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
