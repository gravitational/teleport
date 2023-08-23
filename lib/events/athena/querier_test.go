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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
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

func Test_querier_prepareQuery(t *testing.T) {
	const (
		tablename        = "test_table"
		selectFromPrefix = `SELECT DISTINCT uid, event_time, event_data FROM test_table`
		whereTimeRange   = ` WHERE event_date BETWEEN date(?) AND date(?) AND event_time BETWEEN ? and ?`
	)
	fromTimeUTC := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
	toTimeUTC := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	fromDateParam := "'2023-02-01'"
	fromTimestampParam := "timestamp '2023-02-01 00:00:00'"
	toDateParam := "'2023-03-01'"
	toTimestampParam := "timestamp '2023-03-01 00:00:00'"
	timeRangeParams := []string{fromDateParam, toDateParam, fromTimestampParam, toTimestampParam}

	otherTimeUTC := time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC)
	otherTimestampParam := "timestamp '2023-02-15 00:00:00'"

	keySetFrom9762a4fe := &keyset{
		t:   otherTimeUTC,
		uid: uuid.MustParse("9762a4fe-ac4b-47b5-ba4f-5f70d065849a"),
	}

	tests := []struct {
		name         string
		searchParams searchParams
		wantQuery    string
		wantParams   []string
		wantErr      string
	}{
		{
			name: "query on time range",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				limit:     100,
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` ORDER BY event_time ASC, uid ASC LIMIT 100;`,
			wantParams: timeRangeParams,
		},
		{
			name: "query on time range order DESC",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				limit:     100,
				order:     types.EventOrderDescending,
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` ORDER BY event_time DESC, uid DESC LIMIT 100;`,
			wantParams: timeRangeParams,
		},
		{
			name: "query with event types",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				filter:    searchEventsFilter{eventTypes: []string{"app.create", "app.delete"}},
				limit:     100,
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND event_type IN (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`,
			wantParams: append(timeRangeParams, "'app.create'", "'app.delete'"),
		},
		{
			name: "session id",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				sessionID: "9762a4fe-ac4b-47b5-ba4f-5f70d065849a",
				limit:     100,
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND session_id = ? ORDER BY event_time ASC, uid ASC LIMIT 100;`,
			wantParams: append(timeRangeParams, "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'"),
		},
		{
			name: "query on time range with keyset",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				limit:     100,
				startKey:  keySetFrom9762a4fe.ToKey(),
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`,
			wantParams: append(timeRangeParams, otherTimestampParam, "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'"),
		},
		{
			name: "query on time range DESC with keyset",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				limit:     100,
				order:     types.EventOrderDescending,
				startKey:  keySetFrom9762a4fe.ToKey(),
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`,
			wantParams: append(timeRangeParams, otherTimestampParam, "'9762a4fe-ac4b-47b5-ba4f-5f70d065849a'"),
		},
		{
			name: "query on time range with keyset from dynamo",
			searchParams: searchParams{
				fromUTC: fromTimeUTC,
				toUTC:   toTimeUTC,
				limit:   100,
				order:   types.EventOrderAscending,
				// startKey generated by dynamo which points to Apr 27 2023 08:22:58 UTC
				startKey:  `{"date":"2023-04-27","iterator":{"CreatedAt":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"1682583778","NS":null,"NULL":null,"S":null,"SS":null},"CreatedAtDate":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"2023-04-27","SS":null},"EventIndex":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"0","NS":null,"NULL":null,"S":null,"SS":null},"SessionID":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb","SS":null}}}`,
				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND (event_time, uid) > (?,?) ORDER BY event_time ASC, uid ASC LIMIT 100;`,
			wantParams: append(timeRangeParams, `timestamp '2023-04-27 08:22:58'`, "'00000000-0000-0000-0000-000000000000'"),
		},
		{
			name: "query on time range with keyset from dynamo and desc order",
			searchParams: searchParams{
				fromUTC: fromTimeUTC,
				toUTC:   toTimeUTC,
				limit:   100,
				order:   types.EventOrderDescending,
				// startKey generated by dynamo which points to Apr 27 2023 08:22:58 UTC
				startKey: `{"date":"2023-04-27","iterator":{"CreatedAt":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"1682583778","NS":null,"NULL":null,"S":null,"SS":null},"CreatedAtDate":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"2023-04-27","SS":null},"EventIndex":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":"0","NS":null,"NULL":null,"S":null,"SS":null},"SessionID":{"B":null,"BOOL":null,"BS":null,"L":null,"M":null,"N":null,"NS":null,"NULL":null,"S":"4bc51fd7-4f0c-47ee-b9a5-da621fbdbabb","SS":null}}}`,

				tablename: tablename,
			},
			wantQuery: selectFromPrefix + whereTimeRange +
				` AND (event_time, uid) < (?,?) ORDER BY event_time DESC, uid DESC LIMIT 100;`,
			wantParams: append(timeRangeParams, `timestamp '2023-04-27 08:22:58'`, "'ffffffff-ffff-ffff-ffff-ffffffffffff'"),
		},
		{
			name: "invalid keyset",
			searchParams: searchParams{
				fromUTC:   fromTimeUTC,
				toUTC:     toTimeUTC,
				limit:     100,
				order:     types.EventOrderDescending,
				startKey:  "invalid-keyset",
				tablename: tablename,
			},
			wantErr: "unsupported keyset format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotParams, err := prepareQuery(tt.searchParams)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(gotQuery, tt.wantQuery), "query")
				require.Empty(t, cmp.Diff(gotParams, tt.wantParams), "params")
			}
		})
	}
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
	veryBigEvent := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: strings.Repeat("aaaaa", events.MaxEventBytesInResponse),
		},
	}
	tests := []struct {
		name      string
		limit     int
		condition utils.FieldsCondition
		// fakeResp defines responses which will be returned based on given
		// input token to GetQueryResults. Note that due to limit of GetQueryResults
		// we are doing multiple calls, first always with empty token.
		fakeResp   map[string]eventsWithToken
		wantEvents []apievents.AuditEvent
		wantKeyset string
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
			name: "events with veryBigEvent exceeding > MaxEventBytesInResponse",
			fakeResp: map[string]eventsWithToken{
				"":       {returnToken: "token1", events: []apievents.AuditEvent{event1}},
				"token1": {returnToken: "", events: []apievents.AuditEvent{event2, event3, veryBigEvent}},
			},
			limit: 10,
			// we don't expect veryBigEvent because it should go to next batch
			wantEvents: []apievents.AuditEvent{event1, event2, event3},
			wantKeyset: mustEventToKey(t, event3),
		},
		{
			// TODO(tobiaszheller): right now if we have event that's > 1 MiB, it will be silently ignored (due to gRPC unary limit).
			// Come back later when we have decision what to do with it.
			name: "only 1 very big event",
			fakeResp: map[string]eventsWithToken{
				"": {returnToken: "", events: []apievents.AuditEvent{veryBigEvent}},
			},
			limit:      10,
			wantEvents: []apievents.AuditEvent{},
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
					logger:    utils.NewLoggerForTests(),
					tracer:    tracing.NoopTracer(teleport.ComponentAthena),
				},
				athenaClient: &fakeAthenaResultsGetter{
					resp: tt.fakeResp,
				},
			}
			gotEvents, gotKeyset, err := q.fetchResults(context.Background(), "queryid", tt.limit, tt.condition)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.wantEvents, gotEvents, cmpopts.EquateEmpty()))
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
		fields, err := events.ToEventFields(event)
		if err != nil {
			return nil, err
		}
		marshaled, err := utils.FastMarshal(fields)
		if err != nil {
			return nil, err
		}
		rows = append(rows, athenaTypes.Row{
			Data: []athenaTypes.Datum{
				// The first 2 fields are ignored in our code, they are returned only because Athena requires
				// to return parameters used in ordering.
				{VarCharValue: aws.String("ignored")},
				{VarCharValue: aws.String("ignored")},
				{VarCharValue: aws.String(string(marshaled))},
			},
		})
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
