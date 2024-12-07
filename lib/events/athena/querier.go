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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/parquet-go/parquet-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/dynamoevents"
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
	s3Getter     s3Getter
}

type athenaClient interface {
	StartQueryExecution(ctx context.Context, params *athena.StartQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error)
	GetQueryExecution(ctx context.Context, params *athena.GetQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error)
	GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error)
}

type s3Getter interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type querierConfig struct {
	tablename               string
	database                string
	workgroup               string
	queryResultsS3          string
	locationS3Bucket        string
	locationS3Prefix        string
	getQueryResultsInterval time.Duration
	// getQueryResultsInitialDelay allows to set custom getQueryResultsInitialDelay.
	// If not provided, default will be used.
	getQueryResultsInitialDelay time.Duration

	disableQueryCostOptimization bool

	clock  clockwork.Clock
	awsCfg *aws.Config
	logger *slog.Logger

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

	if cfg.getQueryResultsInitialDelay == 0 {
		cfg.getQueryResultsInitialDelay = getQueryResultsInitialDelay
	}

	if cfg.logger == nil {
		cfg.logger = slog.With(teleport.ComponentKey, teleport.ComponentAthena)

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
		athenaClient: athena.NewFromConfig(*cfg.awsCfg, func(o *athena.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		}),
		s3Getter: s3.NewFromConfig(*cfg.awsCfg, func(o *s3.Options) {
			o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
		}),
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

	var startKeyset *keyset
	if req.StartKey != "" {
		var err error
		startKeyset, err = fromKey(req.StartKey, req.Order)
		if err != nil {
			return nil, "", trace.BadParameter("unsupported keyset format: %v", err)
		}
	}

	from := req.From
	to := req.To
	// If startKeyset is not nil, we can take shorten time range by taking value
	// from key set. From ASC order we modify 'from', for desc - 'to'.
	// This is useful because event-exporter plugin is using always quering with
	// the same from value, and it's using keyset to query for further data.
	// For example if exporter started 2023-01-01 and today is 2023-06-01, it will
	// call with value from=2023-01-01, to=2023-06-01 and ketset.T = 2023-06-01.
	// Values before 2023-06-01 will be filtered by athena engine, however
	// it will cause data scans on large timerange.
	if startKeyset != nil {
		if req.Order == types.EventOrderAscending && startKeyset.t.After(from) {
			from = startKeyset.t.UTC()
		}
		if req.Order == types.EventOrderDescending && startKeyset.t.Before(to) {
			to = startKeyset.t.UTC()
		}
	}

	// If pagination key was used and range is big, try to optimize costs by
	// doing queries on smaller range.
	// This is temporary workaround for polling of event exporter
	// until we have new API for exporting events.
	if q.canOptimizePaginatedSearchCosts(ctx, startKeyset, from, to) {
		events, keyset, err := q.costOptimizedPaginatedSearch(ctx, searchEventsRequest{
			fromUTC:   from.UTC(),
			toUTC:     to.UTC(),
			limit:     req.Limit,
			order:     req.Order,
			startKey:  startKeyset,
			filter:    searchEventsFilter{eventTypes: req.EventTypes},
			sessionID: "",
		})
		return events, keyset, trace.Wrap(err)
	}

	events, keyset, err := q.searchEvents(ctx, searchEventsRequest{
		fromUTC:   from.UTC(),
		toUTC:     to.UTC(),
		limit:     req.Limit,
		order:     req.Order,
		startKey:  startKeyset,
		filter:    searchEventsFilter{eventTypes: req.EventTypes},
		sessionID: "",
	})
	return events, keyset, trace.Wrap(err)
}

// ExportUnstructuredEvents exports events from a given event chunk returned by GetEventExportChunks. This API prioritizes
// performance over ordering and filtering, and is intended for bulk export of events.
func (q *querier) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	startTime := req.Date.AsTime()
	if startTime.IsZero() {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.BadParameter("missing required parameter 'date'"))
	}

	if req.Chunk == "" {
		return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.BadParameter("missing required parameter 'chunk'"))
	}

	date := startTime.Format(time.DateOnly)

	var cursor athenaExportCursor

	if req.Cursor != "" {
		if err := cursor.Decode(req.Cursor); err != nil {
			return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.Wrap(err))
		}
	}

	events := q.streamEventsFromChunk(ctx, date, req.Chunk)

	events = stream.Skip(events, int(cursor.pos))

	return stream.FilterMap(events, func(e eventParquet) (*auditlogpb.ExportEventUnstructured, bool) {
		cursor.pos++
		event, err := auditEventFromParquet(e)
		if err != nil {
			q.logger.WarnContext(ctx, "skipping export of audit event due to failed decoding",
				"error", err,
				"date", date,
				"chunk", req.Chunk,
				"pos", cursor.pos,
			)
			return nil, false
		}

		unstructuredEvent, err := apievents.ToUnstructured(event)
		if err != nil {
			q.logger.WarnContext(ctx, "skipping export of audit event due to failed conversion to unstructured event",
				"error", err,
				"date", date,
				"chunk", req.Chunk,
				"pos", cursor.pos,
			)

			return nil, false
		}

		return &auditlogpb.ExportEventUnstructured{
			Event:  unstructuredEvent,
			Cursor: cursor.Encode(),
		}, true
	})
}

// athenaExportCursors follow the format a1:<pos>.
type athenaExportCursor struct {
	pos int64
}

func (c *athenaExportCursor) Encode() string {
	return fmt.Sprintf("a1:%d", c.pos)
}

func (c *athenaExportCursor) Decode(key string) error {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return trace.BadParameter("invalid key format")
	}
	if parts[0] != "a1" {
		return trace.BadParameter("unsupported cursor format (expected a1, got %q)", parts[0])
	}
	pos, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return trace.Wrap(err)
	}
	c.pos = pos
	return nil
}

// GetEventExportChunks returns a stream of event chunks that can be exported via ExportUnstructuredEvents. The returned
// list isn't ordered and polling for new chunks requires re-consuming the entire stream from the beginning.
func (q *querier) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	dt := req.Date.AsTime()
	if dt.IsZero() {
		return stream.Fail[*auditlogpb.EventExportChunk](trace.BadParameter("missing required parameter 'date'"))
	}

	date := dt.Format(time.DateOnly)

	prefix := fmt.Sprintf("%s/%s/", q.locationS3Prefix, date)

	var continuationToken *string
	firstPage := true

	return stream.PageFunc(func() ([]*auditlogpb.EventExportChunk, error) {
		if !firstPage && continuationToken == nil {
			// no more pages available.
			return nil, io.EOF
		}

		firstPage = false

		rsp, err := q.s3Getter.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(q.locationS3Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			var nsk *s3types.NoSuchKey
			if continuationToken == nil && errors.As(err, &nsk) {
				q.logger.DebugContext(ctx, "no event chunks found for date",
					"date", date,
					"error", err,
				)
				// no pages available
				return nil, io.EOF
			}
			q.logger.ErrorContext(ctx, "failed to list event chunk objects in S3",
				"error", err,
				"date", date,
			)
			return nil, trace.Wrap(err)
		}

		continuationToken = rsp.NextContinuationToken

		chunks := make([]*auditlogpb.EventExportChunk, 0, len(rsp.Contents))

		for _, obj := range rsp.Contents {
			fullKey := aws.ToString(obj.Key)

			if !strings.HasSuffix(fullKey, ".parquet") {
				q.logger.DebugContext(ctx, "skipping non-parquet s3 file",
					"key", fullKey,
					"date", date,
				)
				continue
			}

			chunkID := strings.TrimSuffix(strings.TrimPrefix(fullKey, prefix), ".parquet")
			if chunkID == "" {
				q.logger.WarnContext(ctx, "skipping empty parquet file name",
					"key", fullKey,
					"date", date,
				)
				continue
			}

			chunks = append(chunks, &auditlogpb.EventExportChunk{
				Chunk: chunkID,
			})
		}

		return chunks, nil
	})
}

func (q *querier) streamEventsFromChunk(ctx context.Context, date, chunk string) stream.Stream[eventParquet] {
	data, err := q.readEventChunk(ctx, date, chunk)
	if err != nil {
		return stream.Fail[eventParquet](err)
	}

	reader := parquet.NewGenericReader[eventParquet](bytes.NewReader(data))

	closer := func() {
		reader.Close()
	}

	return stream.Func(func() (eventParquet, error) {
		// conventional wisdom says that we should use a larger persistent buffer here
		// but in loadtesting this API was abserved having almost twice the throughput
		// with a single element local buf variable instead.
		var buf [1]eventParquet
		_, err := reader.Read(buf[:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				return eventParquet{}, io.EOF
			}
			return eventParquet{}, trace.Wrap(err)
		}
		return buf[0], nil
	}, closer)
}

func (q *querier) readEventChunk(ctx context.Context, date, chunk string) ([]byte, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(q.locationS3Bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s/%s.parquet", q.locationS3Prefix, date, chunk)),
	}
	getObjectOutput, err := q.s3Getter.GetObject(ctx, getObjectInput)
	if err != nil {
		var nsk *s3types.NoSuchKey
		if errors.As(err, &nsk) {
			q.logger.DebugContext(ctx, "event chunk not found",
				"date", date,
				"chunk", chunk,
				"error", err,
			)
			return nil, trace.NotFound("event chunk %q not found", chunk)
		}
		q.logger.ErrorContext(ctx, "failed to get event chunk",
			"error", err,
			"date", date,
		)

		return nil, trace.Wrap(err)
	}

	defer getObjectOutput.Body.Close()

	// ideally we'd start streaming events immediately without waiting for the read to
	// complete. in practice thats tricky since the parquet reader wants methods that aren't
	// typically available on lazy readers. we may be able to eek out a bit more performance by
	// implementing a custom wrapper that lazily loads all bytes into an unlimited size buffer
	// so that we can support methods like Seek and ReadAt, which aren't available on buffered
	// readers with fixed sizes.
	return io.ReadAll(getObjectOutput.Body)
}

func (q *querier) canOptimizePaginatedSearchCosts(ctx context.Context, startKey *keyset, from, to time.Time) bool {
	return !q.disableQueryCostOptimization && startKey != nil && to.Sub(from) > 24*time.Hour
}

// costOptimizedPaginatedSearch instead of scanning data on big time range from request
// do scans on smaller ranges first and if there are not enough results,
// it extends range using steps, up to original time range.
// It's temporary workaround to reduce costs when event exporter is executing
// search events endpoint using big time range and requesting only small amount
// of data.
// Ex. For timerange (2023-04-01 12:00, 2023-08-01 12:00) we will do following calls:
// - 1. (2023-04-01 12:00, 2023-04-01 13:00) - 1h increase
// - 2. (2023-04-01 12:00, 2023-04-02 12:00) - 24h increase
// - 3. (2023-04-01 12:00, 2023-04-08 12:00) - 24*7h increase
// - 4. (2023-04-01 12:00, 2023-05-01 12:00) - 24*30h increase
// - 5. (2023-04-01 12:00, 2023-08-01 12:00) - original range.
// If any of steps returns enough data based on limit, we return immediately.
func (q *querier) costOptimizedPaginatedSearch(ctx context.Context, req searchEventsRequest) ([]apievents.AuditEvent, string, error) {
	var events []apievents.AuditEvent
	var err error
	var keyset string

	toUTC := req.toUTC
	fromUTC := req.fromUTC

	for _, dateToMod := range prepareTimeRangesForCostOptimizedSearch(req.fromUTC, req.toUTC, req.order) {
		if req.order == types.EventOrderAscending {
			toUTC = dateToMod.UTC()
			q.logger.DebugContext(ctx, "Doing cost optimized query by modifying to date", "requested_dated", req.toUTC, "modified_date", toUTC)
		} else {
			fromUTC = dateToMod.UTC()
			q.logger.DebugContext(ctx, "Doing cost optimized query by modifying from date", "requested_date", req.fromUTC, "modified_date", fromUTC)

		}
		events, keyset, err = q.searchEvents(ctx, searchEventsRequest{
			fromUTC:   fromUTC,
			toUTC:     toUTC,
			limit:     req.limit,
			order:     req.order,
			startKey:  req.startKey,
			filter:    req.filter,
			sessionID: req.sessionID,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		if keyset != "" {
			// means limit is reached, we can return now.
			return events, keyset, nil
		}
	}
	// if we never had non empty keyset, we return just last response
	// which was on original range.
	return events, keyset, nil
}

// prepareTimeRangesForCostOptimizedSearch based on order, prepare slice of timestamps
// which should be used for modification of to/from in searchEvents call.
func prepareTimeRangesForCostOptimizedSearch(from, to time.Time, order types.EventOrder) []time.Time {
	stepsToIncrease := []time.Duration{
		1 * time.Hour,
		24 * time.Hour,
		7 * 24 * time.Hour,
		30 * 24 * time.Hour,
	}
	var out []time.Time

	if order == types.EventOrderAscending {
		for _, durationToAdd := range stepsToIncrease {
			if newTo := from.Add(durationToAdd); newTo.Before(to) {
				out = append(out, newTo)
			}
		}
		// at the end add original range.
		out = append(out, to)
	} else {
		for _, durationToAdd := range stepsToIncrease {
			if newFrom := to.Add(-1 * durationToAdd); newFrom.After(from) {
				out = append(out, newFrom)
			}
		}
		// at the end add original range.
		out = append(out, from)
	}
	return out
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
	filter := searchEventsFilter{eventTypes: events.SessionRecordingEvents}
	if req.Cond != nil {
		condFn, err := utils.ToFieldsCondition(req.Cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		filter.condition = condFn
	}

	var startKeyset *keyset
	if req.StartKey != "" {
		var err error
		startKeyset, err = fromKey(req.StartKey, req.Order)
		if err != nil {
			return nil, "", trace.BadParameter("unsupported keyset format: %v", err)
		}
	}

	from := req.From
	to := req.To
	// If startKeyset is not nil, we can take shorten time range by taking value
	// from key set. From ASC order we modify 'from', for desc - 'to'.
	// This is useful because event-exporter plugin is using always quering with
	// the same from value, and it's using keyset to query for further data.
	// For example if exporter started 2023-01-01 and today is 2023-06-01, it will
	// call with value from=2023-01-01, to=2023-06-01 and ketset.T = 2023-06-01.
	// Values before 2023-06-01 will be filtered by athena engine, however
	// it will cause data scans on large timerange.
	if startKeyset != nil {
		if req.Order == types.EventOrderAscending && startKeyset.t.After(from) {
			from = startKeyset.t.UTC()
		}
		if req.Order == types.EventOrderDescending && startKeyset.t.Before(to) {
			to = startKeyset.t.UTC()
		}
	}

	events, keyset, err := q.searchEvents(ctx, searchEventsRequest{
		fromUTC:   from,
		toUTC:     to,
		limit:     req.Limit,
		order:     req.Order,
		startKey:  startKeyset,
		filter:    filter,
		sessionID: req.SessionID,
	})
	return events, keyset, trace.Wrap(err)
}

type searchEventsRequest struct {
	fromUTC, toUTC time.Time
	limit          int
	order          types.EventOrder
	startKey       *keyset
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

	query, params, err := prepareQuery(searchParams{
		fromUTC:     req.fromUTC,
		toUTC:       req.toUTC,
		order:       req.order,
		limit:       limit,
		startKeyset: req.startKey,
		filter:      req.filter,
		sessionID:   req.sessionID,
		tablename:   q.tablename,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	startTime := time.Now()
	defer func() {
		q.logger.DebugContext(ctx, "Executed events query on Athena",
			"query", query,
			"params", params,
			"start_key", req.startKey,
			"duration", time.Since(startTime),
		)
	}()
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
func prepareQuery(params searchParams) (query string, execParams []string, err error) {
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

	return qb.String(), qb.Args(), nil
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
			interval = q.getQueryResultsInitialDelay
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
	rb := &responseBuilder{
		logger: q.logger,
	}
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

	logger *slog.Logger
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
			// Encountered an event that would push the total page over the size
			// limit.
			if len(rb.output) > 0 {
				// There are already one or more full events to return, just
				// return them and the next event will be picked up on the next
				// page.
				return true, nil
			}
			// A single event is larger than the max page size - the best we can
			// do is try to trim it.
			event = event.TrimToMaxSize(events.MaxEventBytesInResponse)

			// Check to make sure the trimmed event is small enough.
			fields, err = events.ToEventFields(event)
			if err != nil {
				return false, trace.Wrap(err)
			}
			marshalledEvent, err := utils.FastMarshal(&fields)
			if err != nil {
				return false, trace.Wrap(err, "failed to marshal event, %s", eventData)
			}
			if len(marshalledEvent)+rb.totalSize <= events.MaxEventBytesInResponse {
				events.MetricQueriedTrimmedEvents.Inc()
				// Exact rb.totalSize doesn't really matter since the response is
				// already size limited.
				rb.totalSize += events.MaxEventBytesInResponse
				rb.output = append(rb.output, event)
				return true, nil
			}

			// Failed to trim the event to size. The only options are to return
			// a response with 0 events, skip this event, or return an error.
			//
			// Silently skipping events is a terrible option, it's better for
			// the client to get an error.
			//
			// Returning 0 events amounts to either skipping the event or
			// getting the client stuck in a paging loop depending on what would
			// be returned for the next page token.
			//
			// Returning a descriptive error should at least give the client a
			// hint as to what has gone wrong so that an attempt can be made to
			// fix it.
			//
			// If this condition is reached it should be considered a bug, any
			// event that can possibly exceed the maximum size should implement
			// TrimToMaxSize (until we can one day implement an API for storing
			// and retrieving large events).
			rb.logger.ErrorContext(context.Background(), "Failed to query event exceeding maximum response size.",
				"event_type", event.GetType(),
				"event_id", event.GetID(),
				"event_size", len(eventData),
			)
			return true, trace.Errorf(
				"%s event %s is %s and cannot be returned because it exceeds the maximum response size of %s",
				event.GetType(), event.GetID(), humanize.IBytes(uint64(len(eventData))), humanize.IBytes(events.MaxEventBytesInResponse))
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

// fromKey parses startKey used for query pagination.
// It supports also startKey in format used from dynamoevent, to provide
// smooth migration between dynamo <-> athena backend when event exporter is running.
func fromKey(startKey string, order types.EventOrder) (*keyset, error) {
	startKeyset, athenaErr := fromAthenaKey(startKey)
	if athenaErr == nil {
		return startKeyset, nil
	}
	startKeyset, err := fromDynamoKey(startKey, order)
	if err != nil {
		// can't process it as athena keyset or dynamo, return top level err
		return nil, trace.Wrap(athenaErr)
	}
	return startKeyset, nil
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

var maxUUID = uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

// fromAthenaKey attempts to parse a keyset from a string. The string is a URL-safe
// base64 encoding of the time in microseconds as an int64, the event UUID;
// numbers are encoded in little-endian.
func fromAthenaKey(key string) (*keyset, error) {
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

// fromDynamoKey attempts to parse a keyset from a string as a Dynamo key.
func fromDynamoKey(startKey string, order types.EventOrder) (*keyset, error) {
	// check if it's dynamoDB startKey to be backward compatible.
	createdAt, err := dynamoevents.GetCreatedAtFromStartKey(startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// createdAt is returned from dynamo startKey, however uid is not stored
	// there. On athena side for pagination we use following syntax:
	// (event_time, uid) > (?,?) for ASC order, so let's used 0000 uid there.
	// In worst case it will resut in few duplicate events.
	if order == types.EventOrderAscending {
		return &keyset{
			t:   createdAt.UTC(),
			uid: uuid.Nil,
		}, nil
	}
	// For DESC order we use (event_time, uid) < (?,?), so use FFFF as uuid.
	return &keyset{
		t:   createdAt.UTC(),
		uid: maxUUID,
	}, nil
}
