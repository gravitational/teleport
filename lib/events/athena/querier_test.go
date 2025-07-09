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
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/dustin/go-humanize"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSearchEvents(t *testing.T) {
	toDateParam := func(in time.Time) string {
		return fmt.Sprintf("'%s'", in.Format(time.DateOnly))
	}

	toAthenaTimestampParam := func(in time.Time) string {
		return fmt.Sprintf("timestamp '%s'", in.Format(athenaTimestampFormat))
	}

	const (
		tablename        = "test_table"
		selectFromPrefix = `SELECT DISTINCT uid, event_time, event_data FROM test_table`
		whereTimeRange   = ` WHERE event_date BETWEEN date(?) AND date(?) AND event_time BETWEEN ? and ?`
	)

	fromUTC := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	toUTC := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)

	uuidUsedInKeyset := "9762a4fe-ac4b-47b5-ba4f-5f70d065849a"
	keysetNearTo := &keyset{
		t:   toUTC.Add(-15 * time.Minute),
		uid: uuid.MustParse(uuidUsedInKeyset),
	}
	keysetNearFrom := &keyset{
		t:   fromUTC.Add(15 * time.Minute),
		uid: uuid.MustParse(uuidUsedInKeyset),
	}

	dynamoKeysetTimestamp := time.Date(2023, 4, 27, 8, 22, 58, 0, time.UTC)
	keysetMiddleOfRange := &keyset{
		t:   time.Date(2023, 4, 1, 12, 55, 0, 0, time.UTC),
		uid: uuid.MustParse(uuidUsedInKeyset),
	}

	keysetFromDate := func(t time.Time) *keyset {
		return &keyset{
			t:   t,
			uid: uuid.MustParse(uuidUsedInKeyset),
		}
	}

	timeRangeParams := []string{toDateParam(fromUTC), toDateParam(toUTC), toAthenaTimestampParam(fromUTC), toAthenaTimestampParam(toUTC)}

	sliceOfDummyEvents := func(noOfEvents int) []apievents.AuditEvent {
		out := make([]apievents.AuditEvent, 0, noOfEvents)
		for i := 0; i < noOfEvents; i++ {
			out = append(out, &apievents.AppCreate{
				Metadata: apievents.Metadata{
					ID:   uuid.NewString(),
					Time: time.Now().UTC(),
					Type: events.AppCreateEvent,
				},
				AppMetadata: apievents.AppMetadata{
					AppName: "app-1",
				},
			})
		}
		return out
	}

	singleCallResults := func(noOfEvents int) [][]apievents.AuditEvent {
		return [][]apievents.AuditEvent{sliceOfDummyEvents(noOfEvents)}
	}

	wantCallsToAthena := func(t *testing.T, mock *mockAthenaExecutor, want int) {
		require.Len(t, mock.startQueryReqs, want)
	}
	wantSingleCallToAthena := func(t *testing.T, mock *mockAthenaExecutor) {
		wantCallsToAthena(t, mock, 1)
	}
	wantQuery := func(t *testing.T, mock *mockAthenaExecutor, want string) {
		require.Empty(t, cmp.Diff(want, aws.ToString(mock.startQueryReqs[0].QueryString)), "query")
	}
	wantQueryParamsInCallNo := func(t *testing.T, mock *mockAthenaExecutor, call int, want ...string) {
		require.Empty(t, cmp.Diff(want, mock.startQueryReqs[call].ExecutionParameters), "params")
	}
	wantQueryParams := func(t *testing.T, mock *mockAthenaExecutor, want ...string) {
		wantQueryParamsInCallNo(t, mock, 0, want...)
	}
	tests := []struct {
		name                string
		searchParams        *events.SearchEventsRequest
		searchSessionParams *events.SearchSessionEventsRequest
		queryResultsResps   [][]apievents.AuditEvent
		disableQueryCostOpt bool
		wantErr             string
		check               func(t *testing.T, m *mockAthenaExecutor, paginationKey string)
	}{
		{
			name: "query on time range",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock, timeRangeParams...)
			},
		},
		{
			name: "query on time range order DESC",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
				Order: types.EventOrderDescending,
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParams(t, mock, timeRangeParams...)
			},
		},
		{
			name: "query with event types",
			searchParams: &events.SearchEventsRequest{
				From:       fromUTC,
				To:         toUTC,
				Limit:      100,
				EventTypes: []string{"app.create", "app.delete"},
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND event_type IN (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock, append(timeRangeParams, "'app.create'", "'app.delete'")...)
			},
		},
		{
			name: "session id",
			searchSessionParams: &events.SearchSessionEventsRequest{
				From:      fromUTC,
				To:        toUTC,
				SessionID: "9762a4fe-ac4b-47b5-ba4f-5f70d065849a",
				Limit:     100,
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND session_id = ? AND event_type IN (?,?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock, append(timeRangeParams, "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", "'session.end'", "'windows.desktop.session.end'", "'db.session.end'")...)
			},
		},
		{
			name: "query on time range with keyset",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
				// keyset with timestamp near original 'To' value is used to
				// test case when we are close to end of time range (no cost optimiation).
				StartKey: keysetNearTo.ToKey(),
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in ASC order and valid value in keyset, instead of from, use value from keyset.
					toDateParam(keysetNearTo.t), toDateParam(toUTC), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetNearTo.t), toAthenaTimestampParam(toUTC), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetNearTo.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query disabled",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
			},
			disableQueryCostOpt: true,
			queryResultsResps:   singleCallResults(30),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock,
					toDateParam(keysetMiddleOfRange.t), toDateParam(toUTC), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(toUTC), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query, 1h extended range returns enough results, asc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
			},
			// we ask for 100 using limit and returned is 100. It means we have
			// enough data and can return without extending range.
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock,
					toDateParam(keysetMiddleOfRange.t), toDateParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query, 1d extended range returns enough results, asc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
			},
			// we ask for 100 using limit and in 2nd extended call no of returned
			// is 100. It means we have enough data and can return without extending range.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(100)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantCallsToAthena(t, mock, 2)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keysetMiddleOfRange.t), toDateParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 1,
					toDateParam(keysetMiddleOfRange.t), toDateParam(keysetMiddleOfRange.t.Add(24*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(keysetMiddleOfRange.t.Add(24*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query, fallback to full range after not enough results in 5 calls, asc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
			},
			// we ask for 100 using limit and 100 is never returned. We do full
			// 5 calls with last one being on full range.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(36), sliceOfDummyEvents(37), sliceOfDummyEvents(38), sliceOfDummyEvents(39)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantCallsToAthena(t, mock, 5)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keysetMiddleOfRange.t), toDateParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(keysetMiddleOfRange.t.Add(1*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 4,
					toDateParam(keysetMiddleOfRange.t), toDateParam(toUTC), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t), toAthenaTimestampParam(toUTC), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query should not be triggered if time between ketset and end of range is < 24h, asc",
			searchParams: &events.SearchEventsRequest{
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetNearTo.ToKey(),
			},
			// Not full results to make sure fallback is not triggered.
			queryResultsResps: singleCallResults(50),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in ASC order and valid value in keyset, instead of from, use value from ketset.
					toDateParam(keysetNearTo.t), toDateParam(toUTC), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetNearTo.t), toAthenaTimestampParam(toUTC), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetNearTo.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query should not extend over initial end of range, asc",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
				// cost optimized ranges are 1h, 1d, 7d, 30d, max.
				// if we are -26h from end of range,
				// it should call +1h, +1d and max (7d and 30d extends max range).
				StartKey: keysetFromDate(toUTC.Add(-26 * time.Hour)).ToKey(),
			},
			// Not full results to make sure fallback is not triggered.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(36), sliceOfDummyEvents(37)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				keyset := keysetFromDate(toUTC.Add(-26 * time.Hour))
				wantCallsToAthena(t, mock, 3)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keyset.t), toDateParam(keyset.t.Add(1*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keyset.t), toAthenaTimestampParam(keyset.t.Add(1*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 1,
					toDateParam(keyset.t), toDateParam(keyset.t.Add(24*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keyset.t), toAthenaTimestampParam(keyset.t.Add(24*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 2,
					toDateParam(keyset.t), toDateParam(toUTC), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keyset.t), toAthenaTimestampParam(toUTC), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "query on time range DESC with keyset",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
				Order: types.EventOrderDescending,
				// keyset with timestamp near original 'From' value is used to
				// test case when we are close to end of time range (no cost optimiation).
				StartKey: keysetNearFrom.ToKey(),
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in DESC order and valid value in keyset, instead of from, use value from keyset.
					toDateParam(fromUTC), toDateParam(keysetNearFrom.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(fromUTC), toAthenaTimestampParam(keysetNearFrom.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetNearFrom.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", // AND (event_time, uid) < (?,?)
				)
			},
		},
		{
			name: "cost optimized query, 1h extended range returns enough results, desc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
				Order:    types.EventOrderDescending,
			},
			// we ask for 100 using limit and returned is 100. It means we have
			// enough data and can return without extending range.
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParams(t, mock,
					toDateParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toDateParam(keysetMiddleOfRange.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toAthenaTimestampParam(keysetMiddleOfRange.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query, 1d extended range returns enough results, desc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
				Order:    types.EventOrderDescending,
			},
			// we ask for 100 using limit and in 2nd extended call no of returned
			// is 100. It means we have enough data and can return without extending range.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(100)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantCallsToAthena(t, mock, 2)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toDateParam(keysetMiddleOfRange.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toAthenaTimestampParam(keysetMiddleOfRange.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 1,
					toDateParam(keysetMiddleOfRange.t.Add(-24*time.Hour)), toDateParam(keysetMiddleOfRange.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t.Add(-24*time.Hour)), toAthenaTimestampParam(keysetMiddleOfRange.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query, fallback to full range after not enough results in 5 calls, desc",
			searchParams: &events.SearchEventsRequest{
				// range over 4 month, keyset in the middle of range,
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetMiddleOfRange.ToKey(),
				Order:    types.EventOrderDescending,
			},
			// we ask for 100 using limit and 100 is never returned. We do full
			// 5 calls with last one being on full range.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(36), sliceOfDummyEvents(37), sliceOfDummyEvents(38), sliceOfDummyEvents(39)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantCallsToAthena(t, mock, 5)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toDateParam(keysetMiddleOfRange.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keysetMiddleOfRange.t.Add(-1*time.Hour)), toAthenaTimestampParam(keysetMiddleOfRange.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 4,
					toDateParam(fromUTC), toDateParam(keysetMiddleOfRange.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(fromUTC), toAthenaTimestampParam(keysetMiddleOfRange.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetMiddleOfRange.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "cost optimized query should not be triggered if time between ketset and end of range is < 24h, desc",
			searchParams: &events.SearchEventsRequest{
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				StartKey: keysetNearFrom.ToKey(),
				Order:    types.EventOrderDescending,
			},
			// Not full results to make sure fallback is not triggered.
			queryResultsResps: singleCallResults(50),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in DESC order and valid value in keyset, instead of from, use value from ketset.
					toDateParam(fromUTC), toDateParam(keysetNearFrom.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(fromUTC), toAthenaTimestampParam(keysetNearFrom.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keysetNearFrom.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", // AND (event_time, uid) < (?,?)
				)
			},
		},
		{
			name: "cost optimized query should not extend over initial start of range, desc",
			searchParams: &events.SearchEventsRequest{
				From:  fromUTC,
				To:    toUTC,
				Limit: 100,
				// cost optimized ranges are 1h, 1d, 7d, 30d, max.
				// if we are 26h from start of range,
				// it should call -1h, -1d and max (7d and 30d extends max range).
				StartKey: keysetFromDate(fromUTC.Add(26 * time.Hour)).ToKey(),
				Order:    types.EventOrderDescending,
			},
			// Not full results to make sure fallback is not triggered.
			queryResultsResps: [][]apievents.AuditEvent{sliceOfDummyEvents(35), sliceOfDummyEvents(36), sliceOfDummyEvents(37)},
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				keyset := keysetFromDate(fromUTC.Add(26 * time.Hour))
				wantCallsToAthena(t, mock, 3)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParamsInCallNo(t, mock, 0,
					toDateParam(keyset.t.Add(-1*time.Hour)), toDateParam(keyset.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keyset.t.Add(-1*time.Hour)), toAthenaTimestampParam(keyset.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 1,
					toDateParam(keyset.t.Add(-24*time.Hour)), toDateParam(keyset.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(keyset.t.Add(-24*time.Hour)), toAthenaTimestampParam(keyset.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
				wantQueryParamsInCallNo(t, mock, 2,
					toDateParam(fromUTC), toDateParam(keyset.t), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(fromUTC), toAthenaTimestampParam(keyset.t), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(keyset.t), "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'", //  AND (event_time, uid) > (?,?))
				)
			},
		},
		{
			name: "query on time range with keyset from dynamo",
			searchParams: &events.SearchEventsRequest{
				From: fromUTC,
				// To is set here as value from keyset +5h to test case
				// when cost optimized search should not be used.
				To:    dynamoKeysetTimestamp.Add(5 * time.Hour),
				Limit: 100,
				Order: types.EventOrderAscending,
				// startKey generated by dynamo which points to Apr 27 2023 08:22:58 UTC
				StartKey: `{"date":"2023-04-27","iterator":"{\"CreatedAt\":1682583778,\"CreatedAtDate\":\"2023-04-27\",\"EventIndex\":0,\"SessionID\":\"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb\"}"}`,
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				dynamoKeysetTimestamp := time.Date(2023, 4, 27, 8, 22, 58, 0, time.UTC)
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in ASC order and valid value in keyset, instead of from, use value from ketset.
					toDateParam(dynamoKeysetTimestamp), toDateParam(dynamoKeysetTimestamp.Add(5*time.Hour)), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(dynamoKeysetTimestamp), toAthenaTimestampParam(dynamoKeysetTimestamp.Add(5*time.Hour)), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(dynamoKeysetTimestamp), "'00000000-0000-0000-0000-000000000000'", // AND (event_time, uid) > (?,?)
				)
			},
		},
		{
			name: "query on time range with keyset from dynamo and desc order",
			searchParams: &events.SearchEventsRequest{
				// To is set here as value from keyset -5h to test case
				// when cost optimized search should not be used.
				From:     dynamoKeysetTimestamp.Add(-5 * time.Hour),
				To:       toUTC,
				Limit:    100,
				Order:    types.EventOrderDescending,
				StartKey: `{"date":"2023-04-27","iterator":"{\"CreatedAt\":1682583778,\"CreatedAtDate\":\"2023-04-27\",\"EventIndex\":0,\"SessionID\":\"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb\"}"}`,
			},
			queryResultsResps: singleCallResults(100),
			check: func(t *testing.T, mock *mockAthenaExecutor, paginationKey string) {
				dynamoKeysetTimestamp := time.Date(2023, 4, 27, 8, 22, 58, 0, time.UTC)
				wantSingleCallToAthena(t, mock)
				wantQuery(t, mock, selectFromPrefix+whereTimeRange+
					` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`)
				wantQueryParams(t, mock,
					// in DESC order and valid value in keyset, instead of from, use value from ketset.
					toDateParam(dynamoKeysetTimestamp.Add(-5*time.Hour)), toDateParam(dynamoKeysetTimestamp), // event_date BETWEEN date(?) AND date(?)
					toAthenaTimestampParam(dynamoKeysetTimestamp.Add(-5*time.Hour)), toAthenaTimestampParam(dynamoKeysetTimestamp), // event_time BETWEEN ? and ?`
					toAthenaTimestampParam(dynamoKeysetTimestamp), "'ffffffff-ffff-ffff-ffff-ffffffffffff'", // AND (event_time, uid) < (?,?)
				)
			},
		},
		{
			name: "invalid keyset",
			searchParams: &events.SearchEventsRequest{
				From:     fromUTC,
				To:       toUTC,
				Limit:    100,
				Order:    types.EventOrderDescending,
				StartKey: "invalid-keyset",
			},
			wantErr: "unsupported keyset format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAthenaExecutor{
				getQueryResults: tt.queryResultsResps,
			}
			q := querier{
				athenaClient: mock,
				querierConfig: querierConfig{
					tablename: tablename,
					clock:     clockwork.NewRealClock(),
					logger:    slog.Default(),
					tracer:    tracing.NoopTracer(teleport.ComponentAthena),
					// Use something > 0, to avoid default 600ms delay.
					getQueryResultsInitialDelay:  1 * time.Microsecond,
					disableQueryCostOptimization: tt.disableQueryCostOpt,
				},
			}
			var err error
			var paginationKey string
			if tt.searchParams != nil {
				_, paginationKey, err = q.SearchEvents(context.Background(), *tt.searchParams)
			} else {
				_, paginationKey, err = q.SearchSessionEvents(context.Background(), *tt.searchSessionParams)
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			if tt.check != nil {
				tt.check(t, mock, paginationKey)
			}
		})
	}
}

type mockAthenaExecutor struct {
	startQueryReqs []*athena.StartQueryExecutionInput

	// getQueryResults defines slice of responses returned in each consecutive call.
	getQueryResults [][]apievents.AuditEvent
	getQueryCall    int
}

func (m *mockAthenaExecutor) StartQueryExecution(ctx context.Context, params *athena.StartQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error) {
	m.startQueryReqs = append(m.startQueryReqs, params)
	return &athena.StartQueryExecutionOutput{QueryExecutionId: aws.String(uuid.NewString())}, nil
}

func (m *mockAthenaExecutor) GetQueryExecution(ctx context.Context, params *athena.GetQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error) {
	return &athena.GetQueryExecutionOutput{QueryExecution: &athenaTypes.QueryExecution{Status: &athenaTypes.QueryExecutionStatus{State: athenaTypes.QueryExecutionStateSucceeded}}}, nil
}

func (m *mockAthenaExecutor) GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error) {
	callNo := m.getQueryCall
	m.getQueryCall++
	if len(m.getQueryResults) > callNo {
		out := []athenaTypes.Row{}
		for _, event := range m.getQueryResults[callNo] {
			row, err := apiEventToAthenaRow(event)
			if err != nil {
				return nil, err
			}
			out = append(out, row)
		}
		return &athena.GetQueryResultsOutput{ResultSet: &athenaTypes.ResultSet{Rows: out}}, nil
	}
	return nil, errors.New("not defined mock response")
}

func Test_keyset(t *testing.T) {
	// keyset using microseconds precision,that's why truncate is needed.
	wantT := clockwork.NewFakeClock().Now().UTC().Truncate(time.Microsecond)
	wantUID := uuid.New()
	ks := &keyset{
		t:   wantT,
		uid: wantUID,
	}
	key := ks.ToKey()
	fromKs, err := fromAthenaKey(key)
	require.NoError(t, err)
	require.Equal(t, wantT, fromKs.t)
	require.Equal(t, wantUID, fromKs.uid)
}

func Test_querier_fetchResults(t *testing.T) {
	const tableName = "test_table"
	event1 := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-1",
		},
	}
	event2 := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-2",
		},
	}
	event3 := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-3",
		},
	}
	event4 := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-4",
		},
	}
	bigUntrimmableEvent := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: strings.Repeat("aaaaa", events.MaxEventBytesInResponse),
		},
	}
	bigTrimmableEvent := &apievents.DatabaseSessionQuery{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.DatabaseSessionQueryEvent,
		},
		DatabaseQuery: strings.Repeat("aaaaa", events.MaxEventBytesInResponse),
	}
	bigTrimmedEvent := bigTrimmableEvent.TrimToMaxSize(events.MaxEventBytesInResponse)
	tests := []struct {
		name      string
		limit     int
		condition utils.FieldsCondition
		// fakeResp defines responses which will be returned based on given
		// input token to GetQueryResults. Note that due to limit of GetQueryResults
		// we are doing multiple calls, first always with empty token.
		fakeResp     map[string]eventsWithToken
		wantEvents   []apievents.AuditEvent
		wantKeyset   string
		wantErrorMsg string
	}{
		{
			name:  "no data returned from query, return empty results",
			limit: 10,
		},
		{
			name: "events < then limit, mock returns data in multiple calls",
			fakeResp: map[string]eventsWithToken{
				// empty means what is returned in first call.
				"":       {returnToken: "token1", events: []apievents.AuditEvent{event1}},
				"token1": {returnToken: "", events: []apievents.AuditEvent{event2, event3, event4}},
			},
			limit:      10,
			wantEvents: []apievents.AuditEvent{event1, event2, event3, event4},
		},
		{
			name: "events with untrimmable event exceeding > MaxEventBytesInResponse",
			fakeResp: map[string]eventsWithToken{
				"":       {returnToken: "token1", events: []apievents.AuditEvent{event1}},
				"token1": {returnToken: "", events: []apievents.AuditEvent{event2, event3, bigUntrimmableEvent}},
			},
			limit: 10,
			// we don't expect bigUntrimmableEvent because it should go to next batch
			wantEvents: []apievents.AuditEvent{event1, event2, event3},
			wantKeyset: mustEventToKey(t, event3),
		},
		{
			name: "only 1 very big untrimmable event",
			fakeResp: map[string]eventsWithToken{
				"": {returnToken: "", events: []apievents.AuditEvent{bigUntrimmableEvent}},
			},
			limit: 10,
			wantErrorMsg: fmt.Sprintf(
				"app.create event %s is 5.0 MiB and cannot be returned because it exceeds the maximum response size of %s",
				bigUntrimmableEvent.Metadata.ID, humanize.IBytes(events.MaxEventBytesInResponse)),
		},
		{
			name: "events with trimmable event exceeding > MaxEventBytesInResponse",
			fakeResp: map[string]eventsWithToken{
				"":       {returnToken: "token1", events: []apievents.AuditEvent{event1}},
				"token1": {returnToken: "", events: []apievents.AuditEvent{event2, event3, bigTrimmableEvent}},
			},
			limit: 10,
			// we don't expect bigTrimmableEvent because it should go to next batch
			wantEvents: []apievents.AuditEvent{event1, event2, event3},
			wantKeyset: mustEventToKey(t, event3),
		},
		{
			name: "only 1 very big trimmable event",
			fakeResp: map[string]eventsWithToken{
				"": {returnToken: "", events: []apievents.AuditEvent{bigTrimmableEvent}},
			},
			limit:      10,
			wantEvents: []apievents.AuditEvent{bigTrimmedEvent},
			wantKeyset: mustEventToKey(t, bigTrimmableEvent),
		},
		{
			name: "number of events equals limit in req, make sure that pagination keyset is returned",
			fakeResp: map[string]eventsWithToken{
				"":       {returnToken: "token1", events: []apievents.AuditEvent{event1}},
				"token1": {returnToken: "", events: []apievents.AuditEvent{event2, event3}},
			},
			limit:      3,
			wantEvents: []apievents.AuditEvent{event1, event2, event3},
			wantKeyset: mustEventToKey(t, event3),
		},
		{
			name: "filter events based on condition",
			fakeResp: map[string]eventsWithToken{
				"": {returnToken: "", events: []apievents.AuditEvent{event1, event2, event3, event4}},
			},
			condition: func(f utils.Fields) bool {
				return f.GetString("app_name") != event3.AppName
			},
			limit:      10,
			wantEvents: []apievents.AuditEvent{event1, event2, event4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &querier{
				querierConfig: querierConfig{
					tablename: tableName,
					logger:    slog.Default(),
					tracer:    tracing.NoopTracer(teleport.ComponentAthena),
				},
				athenaClient: &fakeAthenaResultsGetter{
					resp: tt.fakeResp,
				},
			}
			gotEvents, gotKeyset, err := q.fetchResults(context.Background(), "queryid", tt.limit, tt.condition)
			if tt.wantErrorMsg != "" {
				require.ErrorContains(t, err, tt.wantErrorMsg)
				return
			}
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.wantEvents, gotEvents, cmpopts.EquateEmpty(),
				// Expect the database query to be trimmed
				cmpopts.IgnoreFields(apievents.DatabaseSessionQuery{}, "DatabaseQuery")))
			require.Equal(t, tt.wantKeyset, gotKeyset)
		})
	}
}

func mustEventToKey(t *testing.T, in apievents.AuditEvent) string {
	ks, err := eventToKeyset(in)
	if err != nil {
		t.Fatal(err)
	}
	return ks.ToKey()
}

type fakeAthenaResultsGetter struct {
	athenaClient
	iteration int
	resp      map[string]eventsWithToken
}

type eventsWithToken struct {
	events      []apievents.AuditEvent
	returnToken string
}

func apiEventToAthenaRow(event apievents.AuditEvent) (athenaTypes.Row, error) {
	fields, err := events.ToEventFields(event)
	if err != nil {
		return athenaTypes.Row{}, err
	}
	marshaled, err := utils.FastMarshal(fields)
	if err != nil {
		return athenaTypes.Row{}, err
	}
	return athenaTypes.Row{
		Data: []athenaTypes.Datum{
			// The first 2 fields are ignored in our code, they are returned only because Athena requires
			// to return parameters used in ordering.
			{VarCharValue: aws.String("ignored")},
			{VarCharValue: aws.String("ignored")},
			{VarCharValue: aws.String(string(marshaled))},
		},
	}, nil
}

func (f *fakeAthenaResultsGetter) GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error) {
	if f.resp == nil {
		return &athena.GetQueryResultsOutput{}, nil
	}

	eventsWithToken, ok := f.resp[aws.ToString(params.NextToken)]
	if !ok {
		return nil, errors.New("not defined return param in fake")
	}

	var rows []athenaTypes.Row
	if f.iteration == 0 {
		// That's what AWS API does, always adds header on first call.
		rows = append(rows, athenaTypes.Row{
			Data: []athenaTypes.Datum{{VarCharValue: aws.String("uid")}, {VarCharValue: aws.String("event_time")}, {VarCharValue: aws.String("event_data")}},
		})
	}

	for _, event := range eventsWithToken.events {
		row, err := apiEventToAthenaRow(event)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	f.iteration++

	var nextToken *string
	if eventsWithToken.returnToken != "" {
		nextToken = aws.String(eventsWithToken.returnToken)
	}

	return &athena.GetQueryResultsOutput{
		NextToken: nextToken,
		ResultSet: &athenaTypes.ResultSet{
			Rows: rows,
		},
	}, nil
}
