// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgevents

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	pgcommon "github.com/gravitational/teleport/lib/events/pgevents/common"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	Schema    = "postgres"
	AltSchema = "postgresql"

	// componentName is the component name used for logging.
	componentName = "pgevents"
)

const (
	defaultRetentionPeriod = 8766 * time.Hour // 365.25 days
	defaultCleanupInterval = time.Hour
)

// URL parameters for configuration.
const (
	authModeParam      = "auth_mode"
	azureClientIDParam = "azure_client_id"

	disableCleanupParam  = "disable_cleanup"
	cleanupIntervalParam = "cleanup_interval"
	retentionPeriodParam = "retention_period"
)

// AuthMode determines if we should use some environment-specific authentication
// mechanism or credentials.
type AuthMode string

const (
	// FixedAuth uses the static credentials as defined in the connection
	// string.
	FixedAuth AuthMode = ""
	// AzureADAuth gets a connection token from Azure and uses it as the
	// password when connecting.
	AzureADAuth AuthMode = "azure"
)

// Check returns an error if the AuthMode is invalid.
func (a AuthMode) Check() error {
	switch a {
	case FixedAuth, AzureADAuth:
		return nil
	default:
		return trace.BadParameter("invalid authentication mode %q", a)
	}
}

// Config is the configuration struct to pass to New.
type Config struct {
	Log        logrus.FieldLogger
	PoolConfig *pgxpool.Config

	AuthMode      AuthMode
	AzureClientID string

	DisableCleanup  bool
	RetentionPeriod time.Duration
	CleanupInterval time.Duration
}

// SetFromURL sets config params from the URL, as per [pgxpool.ParseConfig]
// (with some additional query params for our own options).
func (c *Config) SetFromURL(u *url.URL) error {
	if u == nil {
		return nil
	}

	poolConfig, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return trace.Wrap(err)
	}
	c.PoolConfig = poolConfig

	popParam := func(k string) (string, bool) {
		if v, ok := poolConfig.ConnConfig.RuntimeParams[k]; ok {
			delete(poolConfig.ConnConfig.RuntimeParams, k)
			return v, true
		}
		return "", false
	}

	if s, ok := popParam(authModeParam); ok {
		c.AuthMode = AuthMode(s)
	}

	if s, ok := popParam(azureClientIDParam); ok {
		c.AzureClientID = s
	}

	if s, ok := popParam(disableCleanupParam); ok {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return trace.Wrap(err)
		}
		c.DisableCleanup = b
	}

	if s, ok := popParam(cleanupIntervalParam); ok {
		d, err := time.ParseDuration(s)
		if err != nil {
			return trace.Wrap(err)
		}
		c.CleanupInterval = d
	}

	if s, ok := popParam(retentionPeriodParam); ok {
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

	if err := c.AuthMode.Check(); err != nil {
		return trace.Wrap(err)
	}

	if c.AzureClientID != "" && c.AuthMode != AzureADAuth {
		return trace.BadParameter("azure client ID requires azure auth mode")
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
		c.Log = logrus.WithField(trace.Component, componentName)
	}

	return nil
}

// Returns a new Log given a Config. Starts a background cleanup task unless
// disabled in the Config.
func New(ctx context.Context, cfg Config) (*Log, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.AuthMode == AzureADAuth {
		bc, err := pgcommon.AzureBeforeConnect(cfg.AzureClientID, cfg.Log)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.PoolConfig.BeforeConnect = bc
	}

	cfg.Log.Info("Setting up events backend.")

	pgcommon.TryEnsureDatabase(ctx, cfg.PoolConfig, cfg.Log)

	pool, err := pgxpool.NewWithConfig(ctx, cfg.PoolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	periodicCtx, cancel := context.WithCancel(context.Background())
	l := &Log{
		log:    cfg.Log,
		pool:   pool,
		cancel: cancel,
	}

	if err := l.setupAndMigrate(ctx); err != nil {
		l.Close()
		return nil, trace.Wrap(err)
	}

	if !cfg.DisableCleanup {
		l.wg.Add(1)
		go l.periodicCleanup(periodicCtx, cfg.CleanupInterval, cfg.RetentionPeriod)
	}

	l.log.Info("Started events backend.")

	return l, nil
}

// Log is an external [events.AuditLogger] backed by a PostgreSQL database.
type Log struct {
	log  logrus.FieldLogger
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

var schemas = []string{
	`CREATE TABLE events (
		event_time timestamptz NOT NULL,
		event_id uuid NOT NULL,
		event_type text NOT NULL,
		session_id uuid NOT NULL,
		event_data json NOT NULL,
		creation_time timestamptz NOT NULL DEFAULT now(),
		CONSTRAINT events_pkey PRIMARY KEY (event_time, event_id)
	);
	CREATE INDEX events_creation_time_idx ON events USING brin (creation_time);
	CREATE INDEX events_search_session_events_idx ON events (session_id, event_time, event_id)
		WHERE session_id != '00000000-0000-0000-0000-000000000000';`,
}

// setupAndMigrate sets up the database schema, applying migrations in order if
// necessary.
func (l *Log) setupAndMigrate(ctx context.Context) error {
	var version int32
	var migrateErr error

	if err := pgcommon.Retry0(ctx, l.log, func() error {
		_, err := l.pool.Exec(ctx,
			`CREATE TABLE IF NOT EXISTS audit_version (
				version integer PRIMARY KEY CHECK (version > 0),
				created timestamptz NOT NULL DEFAULT now()
			)`, pgx.QueryExecModeExec,
		)
		return trace.Wrap(err)
	}); err != nil {
		// the very first SELECT in the next transaction will fail, we don't
		// need anything higher than debug here
		l.log.WithError(err).Debug("Failed to confirm the existence of the audit_version table.")
	}

	if err := pgcommon.RetryTx(ctx, l.log, l.pool, pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			"SELECT COALESCE(max(version), 0) FROM audit_version",
			pgx.QueryExecModeExec,
		).Scan(&version); err != nil {
			return trace.Wrap(err)
		}

		if int(version) > len(schemas) {
			migrateErr = trace.BadParameter("unsupported schema version %v", version)
			return nil
		}

		if int(version) == len(schemas) {
			return nil
		}

		for _, s := range schemas[version:] {
			if _, err := tx.Exec(ctx, s, pgx.QueryExecModeExec); err != nil {
				return trace.Wrap(err)
			}
		}

		if _, err := tx.Exec(ctx,
			"INSERT INTO audit_version (version) VALUES ($1)",
			pgx.QueryExecModeExec, len(schemas),
		); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if migrateErr != nil {
		return migrateErr
	}

	if int(version) != len(schemas) {
		l.log.WithFields(logrus.Fields{
			"previous_version": version,
			"current_version":  len(schemas),
		}).Info("Migrated database schema.")
	}

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

		l.log.Debug("Executing periodic cleanup.")
		deleted, err := pgcommon.Retry(ctx, l.log, func() (int64, error) {
			tag, err := l.pool.Exec(ctx,
				"DELETE FROM events WHERE creation_time < (now() - $1::interval)",
				retentionPeriod,
			)
			if err != nil {
				return 0, trace.Wrap(err)
			}

			return tag.RowsAffected(), nil
		})
		if err != nil {
			l.log.WithError(err).Warn("Failed to execute periodic cleanup.")
		} else {
			l.log.WithField("deleted_rows", deleted).Debug("Executed periodic cleanup.")
		}
	}
}

var _ events.AuditLogger = (*Log)(nil)

// EmitAuditEvent implements [events.AuditLogger].
func (l *Log) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
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

	if err := pgcommon.Retry0(ctx, l.log, func() error {
		_, err := l.pool.Exec(ctx,
			"INSERT INTO events (event_time, event_id, event_type, session_id, event_data)"+
				" VALUES ($1, $2, $3, $4, $5)",
			event.GetTime().UTC(), eventID, event.GetType(), sessionID, eventJSON,
		)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

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
		" event_time, event_id, event_data" +
		" FROM events" +
		" WHERE event_time BETWEEN @from_time AND @to_time")

	if len(eventTypes) > 0 {
		qb.WriteString(" AND event_type = ANY(@event_types)")
	}
	if sessionID != "" {
		// hint to the query planner, it can use the partial index on session_id
		// no matter what the argument is
		qb.WriteString(" AND session_id != '00000000-0000-0000-0000-000000000000' AND session_id = @session_id")
	}
	if order != types.EventOrderDescending {
		if startKey != "" {
			qb.WriteString(" AND (event_time, event_id) > (@start_time, @start_id)")
		}
		qb.WriteString(" ORDER BY event_time, event_id")
	} else {
		if startKey != "" {
			qb.WriteString(" AND (event_time, event_id) < (@start_time, @start_id)")
		}
		qb.WriteString(" ORDER BY event_time DESC, event_id DESC")
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

	if err := pgcommon.RetryTx(ctx, l.log, l.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		evs = nil
		sizeLimit = false
		endTime = time.Time{}
		endID = uuid.Nil

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
	}); err != nil {
		return nil, "", trace.Wrap(err)
	}

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
	sessionEndTypes := []string{events.SessionEndEvent, events.WindowsDesktopSessionEndEvent}
	return l.searchEvents(ctx, req.From, req.To, sessionEndTypes, req.Cond, req.SessionID, req.Limit, req.Order, req.StartKey)
}
