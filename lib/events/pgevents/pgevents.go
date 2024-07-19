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
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	Schema    = "postgres"
	AltSchema = "postgresql"

	// componentName is the component name used for logging.
	componentName = "pgevents"
)

const (
	defaultRetentionPeriod = 8766 * time.Hour // 365.25 days, i.e. one year
	defaultCleanupInterval = time.Hour
)

// URL parameters for configuration.
const (
	authModeParam          = "auth_mode"
	gcpConnectionNameParam = "gcp_connection_name"
	gcpIPTypeParam         = "gcp_ip_type"

	disableCleanupParam  = "disable_cleanup"
	cleanupIntervalParam = "cleanup_interval"
	retentionPeriodParam = "retention_period"
)

const (
	schemaV1Table = `CREATE TABLE events (
		event_time timestamptz NOT NULL,
		event_id uuid NOT NULL,
		event_type text NOT NULL,
		session_id uuid NOT NULL,
		event_data json NOT NULL,
		creation_time timestamptz NOT NULL DEFAULT now(),
		CONSTRAINT events_pkey PRIMARY KEY (event_time, event_id)
	);
	CREATE INDEX events_search_session_events_idx ON events (session_id, event_time, event_id)
		WHERE session_id != '00000000-0000-0000-0000-000000000000';`
	dateIndex                       = "CREATE INDEX events_creation_time_idx ON events USING brin (creation_time);"
	schemaV1TableWithDateIndex      = schemaV1Table + "\n" + dateIndex
	schemaV1CockroachSetRowExpiry   = "ALTER TABLE events SET (ttl_expiration_expression = '((creation_time AT TIME ZONE ''UTC'') + (%d * INTERVAL ''1 microsecond'')) AT TIME ZONE ''UTC'' ');"
	schemaV1CockroachUnsetRowExpiry = "ALTER TABLE events RESET (ttl_expiration_expression);"
)

// Config is the configuration struct to pass to New.
type Config struct {
	pgcommon.AuthConfig

	Log        *slog.Logger
	PoolConfig *pgxpool.Config

	DisableCleanup  bool
	RetentionPeriod time.Duration
	CleanupInterval time.Duration
}

// SetFromURL sets config params from the URL, as per [pgxpool.ParseConfig]
// (with some additional query params in the fragment for our own options).
func (c *Config) SetFromURL(u *url.URL) error {
	if u == nil {
		return nil
	}

	configURL := *u

	params, err := url.ParseQuery(configURL.EscapedFragment())
	if err != nil {
		return trace.Wrap(err)
	}
	configURL.Fragment = ""
	configURL.RawFragment = ""

	poolConfig, err := pgxpool.ParseConfig(configURL.String())
	if err != nil {
		return trace.Wrap(err)
	}
	c.PoolConfig = poolConfig

	c.AuthMode = pgcommon.AuthMode(params.Get(authModeParam))
	c.GCPConnectionName = params.Get(gcpConnectionNameParam)
	c.GCPIPType = pgcommon.GCPIPType(params.Get(gcpIPTypeParam))

	if s := params.Get(disableCleanupParam); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return trace.Wrap(err)
		}
		c.DisableCleanup = b
	}

	if s := params.Get(cleanupIntervalParam); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return trace.Wrap(err)
		}
		c.CleanupInterval = d
	}

	if s := params.Get(retentionPeriodParam); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return trace.Wrap(err)
		}
		c.RetentionPeriod = d
	}

	return nil
}

// CheckAndSetDefaults checks if the Config is valid, setting default parameters
// where they're unset. PoolConfig is only checked for presence.
func (c *Config) CheckAndSetDefaults() error {
	if c.PoolConfig == nil || c.PoolConfig.ConnConfig == nil {
		return trace.BadParameter("missing pool config")
	}

	if err := c.AuthConfig.Check(); err != nil {
		return trace.Wrap(err)
	}

	if c.RetentionPeriod < 0 {
		return trace.BadParameter("retention period must be non-negative")
	}
	if c.RetentionPeriod == 0 {
		c.RetentionPeriod = defaultRetentionPeriod
	}

	if c.CleanupInterval < 0 {
		return trace.BadParameter("cleanup interval must be non-negative")
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = defaultCleanupInterval
	}

	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, componentName)
	}

	return nil
}

// Returns a new Log given a Config. Starts a background cleanup task unless
// disabled in the Config.
func New(ctx context.Context, cfg Config) (*Log, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := metrics.RegisterPrometheusCollectors(prometheusCollectors...); err != nil {
		return nil, trace.Wrap(err, "registering prometheus collectors")
	}

	if err := cfg.AuthConfig.ApplyToPoolConfigs(ctx, cfg.Log, cfg.PoolConfig); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.Log.InfoContext(ctx, "Setting up events backend.")

	pgcommon.TryEnsureDatabase(ctx, cfg.PoolConfig, cfg.Log)

	pool, err := pgxpool.NewWithConfig(ctx, cfg.PoolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var isCockroach bool

	// We're a bit hacky here, we must not start the cleanup job if we're
	// running on cockroach (rows have TTLs). To avoid running another query,
	// the builder function detects and reports if it's a cockroach via a shared
	// variable. This works because everything is synchronous.
	schemaBuilder := func(conn *pgx.Conn) ([]string, error) {
		isCockroach = conn.PgConn().ParameterStatus("crdb_version") != ""

		return buildSchema(isCockroach, &cfg)
	}

	if err := pgcommon.SetupAndMigrateDynamic(ctx, cfg.Log, pool, "audit_version", schemaBuilder); err != nil {
		pool.Close()
		return nil, trace.Wrap(err)
	}

	periodicCtx, cancel := context.WithCancel(context.Background())
	l := &Log{
		log:    cfg.Log,
		pool:   pool,
		cancel: cancel,
	}

	if isCockroach {
		err = configureCockroachDBRetention(ctx, &cfg, pool)
		if err != nil {
			return nil, trace.Wrap(err, "configuring CockroachDB retention")
		}
	} else {
		// Regular PostgreSQL that doesn't support expiring rows, we must run a
		// periodic cleanup job.
		if !cfg.DisableCleanup {
			cfg.Log.DebugContext(
				ctx, "Starting periodic cleanup background worker.",
				"retention", cfg.RetentionPeriod.String(),
				"cleanup_interval", cfg.CleanupInterval)
			l.wg.Add(1)
			go l.periodicCleanup(periodicCtx, cfg.CleanupInterval, cfg.RetentionPeriod)
		}
	}

	l.log.InfoContext(ctx, "Started events backend.")

	return l, nil
}

func configureCockroachDBRetention(ctx context.Context, cfg *Config, pool *pgxpool.Pool) error {
	// Arbitrary timeout to make sure we don't end up hanging for some reason
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var expiryQuery string
	if cfg.DisableCleanup {
		cfg.Log.DebugContext(ctx, "Disabling CockroachDB native row expiry")
		expiryQuery = schemaV1CockroachUnsetRowExpiry
	} else {
		cfg.Log.DebugContext(ctx, "Configuring CockroachDB native row expiry")
		expiryQuery = fmt.Sprintf(schemaV1CockroachSetRowExpiry, cfg.RetentionPeriod)
	}
	_, err := pool.Exec(ctx, expiryQuery, pgx.QueryExecModeExec)
	return trace.Wrap(err)

}

func buildSchema(isCockroach bool, cfg *Config) (schemas []string, err error) {
	// If this is a real postgres, we cannot use self-expiring rows and we need
	// to create an index for the deletion job to run. This index type is not
	// supported by CockroachDB at the time of writing
	// (see https://github.com/cockroachdb/cockroach/issues/41293)
	if !isCockroach {
		return []string{schemaV1TableWithDateIndex}, nil
	}

	cfg.Log.DebugContext(context.TODO(), "CockroachDB detected.")
	return []string{schemaV1Table}, nil
}

// Log is an external [events.AuditLogger] backed by a PostgreSQL database.
type Log struct {
	log  *slog.Logger
	pool *pgxpool.Pool

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Close stops all background tasks and closes the connection pool.
func (l *Log) Close() error {
	l.cancel()
	l.wg.Wait()
	l.pool.Close()
	return nil
}

// periodicCleanup removes events past the retention period from the table,
// periodically. Returns after the context is done.
func (l *Log) periodicCleanup(ctx context.Context, cleanupInterval, retentionPeriod time.Duration) {
	defer l.wg.Done()

	tk := time.NewTicker(cleanupInterval)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}

		l.log.DebugContext(ctx, "Executing periodic cleanup.")
		start := time.Now()
		deleted, err := pgcommon.RetryIdempotent(ctx, l.log, func() (int64, error) {
			tag, err := l.pool.Exec(ctx,
				"DELETE FROM events WHERE creation_time < (now() - $1::interval)",
				retentionPeriod,
			)
			if err != nil {
				return 0, trace.Wrap(err)
			}

			return tag.RowsAffected(), nil
		})
		batchDeleteLatencies.Observe(time.Since(start).Seconds())

		if err != nil {
			batchDeleteRequestsFailure.Inc()
			l.log.ErrorContext(ctx, "Failed to execute periodic cleanup.", "error", err)
		} else {
			batchDeleteRequestsSuccess.Inc()
			l.log.DebugContext(ctx, "Executed periodic cleanup.", "deleted", deleted)
		}
	}
}

var _ events.AuditLogger = (*Log)(nil)

// EmitAuditEvent implements [events.AuditLogger].
func (l *Log) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	var sessionID uuid.UUID
	if s := events.GetSessionID(event); s != "" {
		u, err := uuid.Parse(s)
		if err != nil {
			return trace.Wrap(err)
		}
		sessionID = u
	}

	eventJSON, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}

	eventID := uuid.New()

	start := time.Now()
	// if an event with the same event_id exists, it means that we inserted it
	// and then failed to receive the success reply from the commit
	_, err = pgcommon.RetryIdempotent(ctx, l.log, func() (struct{}, error) {
		_, err := l.pool.Exec(ctx,
			"INSERT INTO events (event_time, event_id, event_type, session_id, event_data)"+
				" VALUES ($1, $2, $3, $4, $5)"+
				" ON CONFLICT DO NOTHING",
			event.GetTime().UTC(), eventID, event.GetType(), sessionID, eventJSON,
		)
		return struct{}{}, trace.Wrap(err)
	})

	writeLatencies.Observe(time.Since(start).Seconds())

	if err != nil {
		writeRequestsFailure.Inc()
		return trace.Wrap(err)
	}
	writeRequestsSuccess.Inc()

	return nil
}

// searchEvents returns events within the time range, filtering (optionally) by
// event types, session id, and a generic condition, limiting results by a count
// and by a maximum size of the underlying json data of
// events.MaxEventBytesInResponse, sorting by time, session id and event index
// either ascending or descending, and returning an opaque, URL-safe string that
// can be passed to the same function to continue fetching data.
func (l *Log) searchEvents(
	ctx context.Context,
	fromTime, toTime time.Time,
	eventTypes []string, cond *types.WhereExpr, sessionID string,
	limit int, order types.EventOrder, startKey string,
) ([]apievents.AuditEvent, string, error) {
	var sessionUUID uuid.UUID
	if sessionID != "" {
		var err error
		sessionUUID, err = uuid.Parse(sessionID)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	if limit <= 0 {
		limit = defaults.EventsIterationLimit
	}
	if limit > defaults.EventsMaxIterationLimit {
		return nil, "", trace.BadParameter("limit %v exceeds %v", limit, defaults.MaxIterationLimit)
	}

	var startTime time.Time
	var startID uuid.UUID
	if startKey != "" {
		var err error
		startTime, startID, err = fromStartKey(startKey)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var condFn utils.FieldsCondition
	if cond != nil {
		var err error
		condFn, err = utils.ToFieldsCondition(cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var qb strings.Builder
	qb.WriteString("DECLARE cur CURSOR FOR SELECT" +
		" events.event_time, events.event_id, events.event_data" +
		" FROM events" +
		" WHERE events.event_time BETWEEN @from_time AND @to_time")

	if len(eventTypes) > 0 {
		qb.WriteString(" AND events.event_type = ANY(@event_types)")
	}
	if sessionID != "" {
		// hint to the query planner, it can use the partial index on session_id
		// no matter what the argument is
		qb.WriteString(" AND events.session_id != '00000000-0000-0000-0000-000000000000' AND events.session_id = @session_id")
	}
	if order != types.EventOrderDescending {
		if startKey != "" {
			qb.WriteString(" AND (events.event_time, events.event_id) > (@start_time, @start_id)")
		}
		qb.WriteString(" ORDER BY events.event_time, events.event_id")
	} else {
		if startKey != "" {
			qb.WriteString(" AND (events.event_time, events.event_id) < (@start_time, @start_id)")
		}
		qb.WriteString(" ORDER BY events.event_time DESC, events.event_id DESC")
	}

	queryString := qb.String()
	queryArgs := pgx.NamedArgs{
		"from_time":   fromTime,
		"to_time":     toTime,
		"event_types": eventTypes,
		"session_id":  sessionUUID,
		"start_time":  startTime,
		"start_id":    startID,
	}

	const fetchSize = defaults.EventsIterationLimit
	fetchQuery := fmt.Sprintf("FETCH %d FROM cur", fetchSize)

	var evs []apievents.AuditEvent
	var sizeLimit bool
	var endTime time.Time
	var endID uuid.UUID

	transactionStart := time.Now()
	const idempotent = true
	err := pgcommon.RetryTx(ctx, l.log, l.pool, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	}, idempotent, func(tx pgx.Tx) error {
		evs = nil
		sizeLimit = false
		endTime = time.Time{}
		endID = uuid.Nil

		// the query already has 16 options; if we were to add more - by adding
		// server-side filtering, for instance - we should consider not
		// preparing them, by passing pgx.QueryExecModeExec here, to avoid
		// preparing a bunch of statements that might only get executed once
		if _, err := tx.Exec(ctx, queryString, queryArgs); err != nil {
			return trace.Wrap(err)
		}

		totalSize := 0
		for len(evs) < limit && !sizeLimit {
			rows, _ := tx.Query(ctx, fetchQuery)

			var t time.Time
			var id uuid.UUID
			var data []byte
			tag, err := pgx.ForEachRow(rows, []any{&t, &id, &data}, func() error {
				var evf events.EventFields
				if err := utils.FastUnmarshal(data, &evf); err != nil {
					return trace.Wrap(err)
				}

				// TODO(espadolini): encode cond as a condition in the query
				if condFn != nil && !condFn(utils.Fields(evf)) {
					return nil
				}

				if len(data)+totalSize > events.MaxEventBytesInResponse {
					sizeLimit = true
					rows.Close()
					return nil
				}
				totalSize += len(data)

				ev, err := events.FromEventFields(evf)
				if err != nil {
					return trace.Wrap(err)
				}

				evs = append(evs, ev)
				endTime = t
				endID = id

				if len(evs) >= limit {
					rows.Close()
					return nil
				}

				return nil
			})
			if err != nil {
				return trace.Wrap(err)
			}

			if tag.RowsAffected() < fetchSize {
				break
			}
		}
		return nil
	})
	batchReadLatencies.Observe(time.Since(transactionStart).Seconds())

	if err != nil {
		batchReadRequestsFailure.Inc()
		return nil, "", trace.Wrap(err)
	}
	batchReadRequestsSuccess.Inc()

	var nextKey string
	if len(evs) > 0 && (len(evs) >= limit || sizeLimit) {
		nextKey = toNextKey(endTime, endID)
	}

	return evs, nextKey, nil
}

// SearchEvents implements [events.AuditLogger].
func (l *Log) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	var emptyCond *types.WhereExpr
	const emptySessionID = ""
	return l.searchEvents(ctx, req.From, req.To, req.EventTypes, emptyCond, emptySessionID, req.Limit, req.Order, req.StartKey)
}

// SearchSessionEvents implements [events.AuditLogger].
func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	return l.searchEvents(ctx, req.From, req.To, events.SessionRecordingEvents, req.Cond, req.SessionID, req.Limit, req.Order, req.StartKey)
}
