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
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	athenaTimestampFormat = "2006-01-02 15:04:05.999"
	// getQueryResultsInitialDelay defines how long querier will wait before asking
	// first time for status of execution query.
	getQueryResultsInitialDelay = 600 * time.Millisecond
	// getQueryResultsMaxTime defines what's maximum time for running a query.
	getQueryResultsMaxTime = 1 * time.Minute
)

// querier allows searching events on s3 using Athena engine.
// Data on s3 is stored in parquet files and partitioned by date using folders.
type querier struct {
	querierConfig

	athenaClient athenaClient
}

type athenaClient interface {
	StartQueryExecution(ctx context.Context, params *athena.StartQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error)
	GetQueryExecution(ctx context.Context, params *athena.GetQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error)
	GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error)
}

type querierConfig struct {
	tablename               string
	database                string
	workgroup               string
	queryResultsS3          string
	getQueryResultsInterval time.Duration

	clock  clockwork.Clock
	awsCfg *aws.Config
	logger log.FieldLogger

	// tracer is used to create spans
	tracer oteltrace.Tracer
}

func (cfg *querierConfig) CheckAndSetDefaults() error {
	// Proper format of those fields is already validated in athena.Config.
	// Here we just check if they were "wired" at all.
	switch {
	case cfg.tablename == "":
		return trace.BadParameter("empty tablename in athena querier")
	case cfg.database == "":
		return trace.BadParameter("empty database in athena querier")
	case cfg.queryResultsS3 == "":
		return trace.BadParameter("empty queryResultsS3 in athena querier")
	case cfg.getQueryResultsInterval == 0:
		return trace.BadParameter("empty getQueryResultsInterval in athena querier")
	case cfg.awsCfg == nil:
		return trace.BadParameter("empty awsCfg in athena querier")
	}

	if cfg.logger == nil {
		cfg.logger = log.WithFields(log.Fields{
			trace.Component: teleport.ComponentAthena,
		})
	}
	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}

	if cfg.tracer == nil {
		cfg.tracer = tracing.NoopTracer(teleport.ComponentAthena)
	}

	return nil
}

func newQuerier(cfg querierConfig) (*querier, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &querier{
		athenaClient:  athena.NewFromConfig(*cfg.awsCfg),
		querierConfig: cfg,
	}, nil
}

func (q *querier) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	ctx, span := q.tracer.Start(
		ctx,
		"audit/SearchEvents",
		oteltrace.WithAttributes(
			attribute.Int("limit", req.Limit),
			attribute.String("from", req.From.Format(time.RFC3339)),
			attribute.String("to", req.To.Format(time.RFC3339)),
		),
	)
	defer span.End()
	filter := searchEventsFilter{eventTypes: req.EventTypes}
	events, keyset, err := q.searchEvents(ctx, searchEventsRequest{
		fromUTC:   req.From.UTC(),
		toUTC:     req.To.UTC(),
		limit:     req.Limit,
		order:     req.Order,
		startKey:  req.StartKey,
		filter:    filter,
		sessionID: "",
	})
	return events, keyset, trace.Wrap(err)
}

func (q *querier) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	ctx, span := q.tracer.Start(
		ctx,
		"audit/SearchSessionEvents",
		oteltrace.WithAttributes(
			attribute.Int("limit", req.Limit),
			attribute.String("from", req.From.Format(time.RFC3339)),
			attribute.String("to", req.To.Format(time.RFC3339)),
		),
	)
	defer span.End()
	// TODO(tobiaszheller): maybe if fromUTC is 0000-00-00, ask first last 30days and fallback to -inf - now-30
	// for sessionID != "". This kind of call is done on RBAC to check if user can access that session.
	filter := searchEventsFilter{eventTypes: []string{events.SessionEndEvent, events.WindowsDesktopSessionEndEvent}}
	if req.Cond != nil {
		condFn, err := utils.ToFieldsCondition(req.Cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}
	events, keyset, err := q.searchEvents(ctx, searchEventsRequest{
		fromUTC:   req.From.UTC(),
		toUTC:     req.To.UTC(),
		limit:     req.Limit,
		order:     req.Order,
		startKey:  req.StartKey,
		filter:    filter,
		sessionID: req.SessionID,
	})
	return events, keyset, trace.Wrap(err)
}

type searchEventsRequest struct {
	fromUTC, toUTC time.Time
	limit          int
	order          types.EventOrder
	startKey       string
	filter         searchEventsFilter
	sessionID      string
}

func (q *querier) searchEvents(ctx context.Context, req searchEventsRequest) ([]apievents.AuditEvent, string, error) {
	limit := req.limit
	if limit <= 0 {
		limit = defaults.EventsIterationLimit
	}
	if limit > defaults.EventsMaxIterationLimit {
		return nil, "", trace.BadParameter("limit %v exceeds %v", limit, defaults.EventsMaxIterationLimit)
	}

	var startKeyset *keyset
	if req.startKey != "" {
		var err error
		startKeyset, err = fromKey(req.startKey)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	query, params := prepareQuery(searchParams{
		fromUTC:     req.fromUTC,
		toUTC:       req.toUTC,
		order:       req.order,
		limit:       limit,
		startKeyset: startKeyset,
		filter:      req.filter,
		sessionID:   req.sessionID,
		tablename:   q.tablename,
	})

	q.logger.WithField("query", query).
		WithField("params", params).
		WithField("startKey", req.startKey).
		Debug("Executing events query on Athena")

	queryId, err := q.startQueryExecution(ctx, query, params)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if err := q.waitForSuccess(ctx, queryId); err != nil {
		return nil, "", trace.Wrap(err)
	}

	output, nextKey, err := q.fetchResults(ctx, queryId, limit, req.filter.condition)
	return output, nextKey, trace.Wrap(err)
}

type searchEventsFilter struct {
	eventTypes []string
	condition  utils.FieldsCondition
}

type queryBuilder struct {
	builder strings.Builder
	args    []string
}

// withTicks wraps string with ticks.
// string params in athena need to be wrapped by "ticks".
func withTicks(in string) string {
	return fmt.Sprintf("'%s'", in)
}

func sliceWithTicks(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, withTicks(s))
	}
	return out
}

func (q *queryBuilder) Append(s string, args ...string) {
	q.builder.WriteString(s)
	q.args = append(q.args, args...)
}

func (q *queryBuilder) String() string {
	return q.builder.String()
}

func (q *queryBuilder) Args() []string {
	return q.args
}

type searchParams struct {
	fromUTC, toUTC time.Time
	limit          int
	order          types.EventOrder
	startKeyset    *keyset
	filter         searchEventsFilter
	sessionID      string
	tablename      string
}

// prepareQuery returns query string with parameter placeholders and execution parameters.
// To prevent SQL injection, Athena supports parametrized query.
// As parameter placeholder '?' should be used.
func prepareQuery(params searchParams) (query string, execParams []string) {
	qb := &queryBuilder{}
	qb.Append(`SELECT DISTINCT uid, event_time, event_data FROM `)
	// tablename is validated during config validation.
	// It can only contain characters defined by Athena, which are safe from SQL
	// Injection.
	// Athena does not support passing table name as query parameters.
	qb.Append(params.tablename)
	qb.Append(` WHERE event_date BETWEEN date(?) AND date(?)`, withTicks(params.fromUTC.Format(time.DateOnly)), withTicks(params.toUTC.Format(time.DateOnly)))
	qb.Append(` AND event_time BETWEEN ? and ?`,
		fmt.Sprintf("timestamp '%s'", params.fromUTC.Format(athenaTimestampFormat)), fmt.Sprintf("timestamp '%s'", params.toUTC.Format(athenaTimestampFormat)))

	if params.sessionID != "" {
		qb.Append(" AND session_id = ?", withTicks(params.sessionID))
	}

	if len(params.filter.eventTypes) > 0 {
		// Athena does not support IN with single `?` and multiple parameters.
		// Based on number of eventTypes, first query is prepared with defined
		// number of placeholders. It's safe because we just taken len of event
		// types to query, values of event types are passed as parameters.
		eventsTypesInQuery := fmt.Sprintf(" AND event_type IN (%s)",
			// Create following part: `?,?,?,?` based on len of eventTypes.
			strings.TrimSuffix(strings.Repeat("?,", len(params.filter.eventTypes)), ","))
		qb.Append(eventsTypesInQuery,
			sliceWithTicks(params.filter.eventTypes)...,
		)
	}

	if params.order == types.EventOrderAscending {
		if params.startKeyset != nil {
			qb.Append(` AND (event_time, uid) > (?,?)`,
				fmt.Sprintf("timestamp '%s'", params.startKeyset.t.Format(athenaTimestampFormat)), fmt.Sprintf("'%s'", params.startKeyset.uid.String()))
		}

		qb.Append(` ORDER BY event_time ASC, uid ASC`)
	} else {
		if params.startKeyset != nil {
			qb.Append(` AND (event_time, uid) < (?,?)`,
				fmt.Sprintf("timestamp '%s'", params.startKeyset.t.Format(athenaTimestampFormat)), fmt.Sprintf("'%s'", params.startKeyset.uid.String()))
		}
		qb.Append(` ORDER BY event_time DESC, uid DESC`)
	}

	// Athena engine v2 supports ? placeholders only in Where part.
	// To be compatible with v2, limit value is added as part of query.
	// It's safe because it was already validated and it's just int.
	qb.Append(` LIMIT ` + strconv.Itoa(params.limit) + `;`)

	return qb.String(), qb.Args()
}

func (q *querier) startQueryExecution(ctx context.Context, query string, params []string) (string, error) {
	ctx, span := q.tracer.Start(ctx, "athena/startQueryExecution")
	defer span.End()
	startQueryInput := &athena.StartQueryExecutionInput{
		QueryExecutionContext: &athenaTypes.QueryExecutionContext{
			Database: aws.String(q.database),
		},
		ExecutionParameters: params,
		QueryString:         aws.String(query),
	}
	if q.workgroup != "" {
		startQueryInput.WorkGroup = aws.String(q.workgroup)
	}

	if q.queryResultsS3 != "" {
		startQueryInput.ResultConfiguration = &athenaTypes.ResultConfiguration{
			OutputLocation: aws.String(q.queryResultsS3),
		}
	}

	startQueryOut, err := q.athenaClient.StartQueryExecution(ctx, startQueryInput)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return aws.ToString(startQueryOut.QueryExecutionId), nil
}

func (q *querier) waitForSuccess(ctx context.Context, queryId string) error {
	ctx, span := q.tracer.Start(
		ctx,
		"athena/waitForSuccess",
		oteltrace.WithAttributes(
			attribute.String("queryId", queryId),
		),
	)
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, getQueryResultsMaxTime)
	defer cancel()

	for i := 0; ; i++ {
		interval := q.getQueryResultsInterval
		if i == 0 {
			// we want a longer initial delay because processing execution on athena takes some time
			// and that's no real benefit to ask earlier.
			interval = getQueryResultsInitialDelay
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-q.clock.After(interval):
			// continue below
		}

		resp, err := q.athenaClient.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{QueryExecutionId: aws.String(queryId)})
		if err != nil {
			return trace.Wrap(err)
		}
		state := resp.QueryExecution.Status.State
		switch state {
		case athenaTypes.QueryExecutionStateSucceeded:
			return nil
		case athenaTypes.QueryExecutionStateCancelled, athenaTypes.QueryExecutionStateFailed:
			return trace.Errorf("got unexpected state: %s from queryID: %s", state, queryId)
		case athenaTypes.QueryExecutionStateQueued, athenaTypes.QueryExecutionStateRunning:
			continue
		default:
			return trace.Errorf("got unknown state: %s from queryID: %s", state, queryId)
		}
	}
}

// fetchResults returns query results for given queryID.
// Athena API allows only fetch 1000 results, so if client asks for more, multiple
// calls to GetQueryResults will be necessary.
func (q *querier) fetchResults(ctx context.Context, queryId string, limit int, condition utils.FieldsCondition) ([]apievents.AuditEvent, string, error) {
	ctx, span := q.tracer.Start(
		ctx,
		"athena/fetchResults",
		oteltrace.WithAttributes(
			attribute.String("queryId", queryId),
		),
	)
	defer span.End()
	rb := &responseBuilder{}
	// nextToken is used as offset to next calls for GetQueryResults.
	var nextToken string
	for {
		var nextTokenPtr *string
		if nextToken != "" {
			nextTokenPtr = aws.String(nextToken)
		}
		resultResp, err := q.athenaClient.GetQueryResults(ctx, &athena.GetQueryResultsInput{
			// AWS SDK allows only 1000 results.
			MaxResults:       aws.Int32(1000),
			QueryExecutionId: aws.String(queryId),
			NextToken:        nextTokenPtr,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		sizeLimit, err := rb.appendUntilSizeLimit(resultResp, condition)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		if sizeLimit {
			endkeySet, err := rb.endKeyset()
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return rb.output, endkeySet.ToKey(), nil
		}

		// It means that there are no more results to fetch from athena results
		// output location.
		if resultResp.NextToken == nil {
			output := rb.output
			// We have the same amount of results as requested, return keyset
			// because there could be more results.
			if len(output) >= limit {
				endkeySet, err := rb.endKeyset()
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return output, endkeySet.ToKey(), nil
			}
			// output is smaller then limit, no keyset needed.
			return output, "", nil
		}
		nextToken = *resultResp.NextToken

	}
}

type responseBuilder struct {
	output []apievents.AuditEvent
	// totalSize is used to track size of output
	totalSize int
}

func (r *responseBuilder) endKeyset() (*keyset, error) {
	if len(r.output) < 1 {
		// Search can return 0 events, it means we don't have keyset to return
		// but it is also not an error.
		return nil, nil
	}
	lastEvent := r.output[len(r.output)-1]

	endKeyset, err := eventToKeyset(lastEvent)
	return endKeyset, trace.Wrap(err)
}

func eventToKeyset(in apievents.AuditEvent) (*keyset, error) {
	var out keyset
	var err error
	out.t = in.GetTime()
	out.uid, err = uuid.Parse(in.GetID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// appendUntilSizeLimit converts events from json blob to apievents.AuditEvent.
// It stops if events.MaxEventBytesInResponse is reached or if there are no more
// events. It returns true if size limit was reached.
func (rb *responseBuilder) appendUntilSizeLimit(resultResp *athena.GetQueryResultsOutput, condition utils.FieldsCondition) (bool, error) {
	if resultResp == nil || resultResp.ResultSet == nil {
		return false, nil
	}
	for i, row := range resultResp.ResultSet.Rows {
		if len(row.Data) != 3 {
			return false, trace.BadParameter("invalid number of row at response, got %d", len(row.Data))
		}
		// GetQueryResults returns as first row header from CSV.
		// We don't need it, so we will just ignore first row if it contains
		// header.
		if i == 0 && aws.ToString(row.Data[0].VarCharValue) == "uid" {
			continue
		}
		eventData := aws.ToString(row.Data[2].VarCharValue)

		var fields events.EventFields
		if err := utils.FastUnmarshal([]byte(eventData), &fields); err != nil {
			return false, trace.Wrap(err, "failed to unmarshal event, %s", eventData)
		}
		event, err := events.FromEventFields(fields)
		if err != nil {
			return false, trace.Wrap(err)
		}
		// TODO(tobiaszheller): encode filter as query params and remove it in next PRs.
		if condition != nil && !condition(utils.Fields(fields)) {
			continue
		}

		if len(eventData)+rb.totalSize > events.MaxEventBytesInResponse {
			return true, nil
		}
		rb.totalSize += len(eventData)
		rb.output = append(rb.output, event)
	}
	return false, nil
}

// keyset is a point at which the searchEvents pagination ended, and can be
// resumed from.
type keyset struct {
	t   time.Time
	uid uuid.UUID
}

// keySetLen defines len of keyset. 8 bytes from timestamp + 16 for UUID.
const keySetLen = 24

// FromKey attempts to parse a keyset from a string. The string is a URL-safe
// base64 encoding of the time in microseconds as an int64, the event UUID;
// numbers are encoded in little-endian.
func fromKey(key string) (*keyset, error) {
	if key == "" {
		return nil, trace.BadParameter("missing key")
	}

	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(b) != keySetLen {
		return nil, trace.BadParameter("malformed pagination key")
	}
	ks := &keyset{
		t: time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC(),
	}
	ks.uid, err = uuid.FromBytes(b[8:24])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ks, nil
}

// ToKey converts the keyset into a URL-safe string.
func (ks *keyset) ToKey() string {
	if ks == nil {
		return ""
	}
	var b [keySetLen]byte
	binary.LittleEndian.PutUint64(b[0:8], uint64(ks.t.UnixMicro()))
	copy(b[8:24], ks.uid[:])
	return base64.URLEncoding.EncodeToString(b[:])
}
