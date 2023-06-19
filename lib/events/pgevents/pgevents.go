// Copyright 2022 Gravitational, Inc
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

package pgevents

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/backend/pgbk"
	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
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
	authModeParam        = "auth_mode"
	azureClientIDParam   = "azure_client_id"
	disableCleanupParam  = "disable_cleanup"
	cleanupIntervalParam = "cleanup_interval"
)

var (
	txReadWrite = pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}
	txReadOnly = pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadOnly,
	}
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

// SetFromURL sets config params from the URL, as per pgxpool.ParseConfig (with
// some additional query params for our own options).
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

	return nil
}

// SetFromAuditConfig sets parameters of the Config based on a
// ClusterAuditConfig. For now, it only sets RetentionPeriod.
func (c *Config) SetFromAuditConfig(auditConfig types.ClusterAuditConfig) {
	if auditConfig == nil {
		return
	}

	if d := auditConfig.RetentionPeriod(); d != nil {
		c.RetentionPeriod = d.Duration()
	}
}

// SetLogFromParent sets the Log, from a parent log, giving it a component name.
func (c *Config) SetLogFromParent(parent logrus.FieldLogger) {
	c.Log = parent.WithField(trace.Component, teleport.Component(componentName))
}

// CheckAndSetDefaults checks if the Config is valid, setting default parameters
// where they're unset. PoolConfig is only checked for presence.
func (c *Config) CheckAndSetDefaults() error {
	if c.PoolConfig == nil || c.PoolConfig.ConnConfig == nil {
		return trace.BadParameter("missing pool config")
	}

	if err := pgbk.ValidateDatabaseName(c.PoolConfig.ConnConfig.Database); err != nil {
		return trace.Wrap(err)
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
		c.SetLogFromParent(logrus.StandardLogger())
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
		bc, err := AzureBeforeConnect(cfg.AzureClientID, cfg.Log)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.PoolConfig.BeforeConnect = bc
	}

	cfg.Log.Info("Setting up events backend.")

	if pgConn, err := connectPostgres(ctx, cfg.PoolConfig); err != nil {
		cfg.Log.WithError(err).Warn("Failed to connect to the \"postgres\" database.")
	} else {
		// this will error out if the encoding of template1 is not UTF8; in such
		// cases, the database creation should probably be done manually anyway
		createDB := fmt.Sprintf("CREATE DATABASE \"%v\" ENCODING UTF8", cfg.PoolConfig.ConnConfig.Database)
		if _, err := pgConn.Exec(ctx, createDB); err != nil && !isCode(err, pgerrcode.DuplicateDatabase) {
			// CREATE will check permissions first and we may not have CREATEDB
			// privileges in more hardened setups; the subsequent connection
			// will fail immediately if we can't connect, anyway, so we can log
			// permission errors at debug level here.
			if isCode(err, pgerrcode.InsufficientPrivilege) {
				cfg.Log.WithError(err).Debug("Error creating database.")
			} else {
				cfg.Log.WithError(err).Warn("Error creating database.")
			}
		}
		if err := pgConn.Close(ctx); err != nil {
			cfg.Log.WithError(err).Warn("Error closing connection to the \"postgres\" database.")
		}
	}

	pool, err := pgxpool.ConnectConfig(ctx, cfg.PoolConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := &Log{
		cfg:  cfg,
		pool: pool,
	}

	if err := l.setupAndMigrate(ctx); err != nil {
		l.Close()
		return nil, trace.Wrap(err)
	}

	if !cfg.DisableCleanup {
		periodicCtx, periodicCancel := context.WithCancel(context.Background())
		go l.periodicCleanup(periodicCtx)
		l.periodicCancel = periodicCancel
	}

	cfg.Log.Info("Started events backend.")

	return l, nil
}

// Log is an external events.IAuditLog backed by a PostgreSQL or CockroachDB
// database.
type Log struct {
	cfg  Config
	pool *pgxpool.Pool

	// periodicCancel cancels the context in which the periodic cleanup runs.
	// Can be nil.
	periodicCancel context.CancelFunc
}

var _ events.AuditLogSessionStreamer = (*Log)(nil)

// Close stops the potential background cleanup task and closes the connection
// pool.
func (l *Log) Close() error {
	l.cfg.Log.Info("Closing events backend.")
	if l.periodicCancel != nil {
		l.periodicCancel()
	}
	l.pool.Close()
	l.cfg.Log.Info("Closed events backend.")
	return nil
}

// beginTxFunc wraps the pool's BeginTxFunc with automatic retries. It's
// important that f resets any shared state at the beginning, and that any
// exfiltrated data is only used after the transaction commits cleanly.
func (l *Log) beginTxFunc(ctx context.Context, txOptions pgx.TxOptions, f func(pgx.Tx) error) error {
	retrySerialization := func() error {
		for i := 0; i < 20; i++ {
			err := l.pool.BeginTxFunc(ctx, txOptions, f)
			if err == nil || !isCode(err, pgerrcode.SerializationFailure) || !isCode(err, pgerrcode.DeadlockDetected) {
				return trace.Wrap(err)
			}
			l.cfg.Log.WithError(err).WithField("attempt", i).Debug("Serialization failure.")
		}
		return trace.LimitExceeded("too many serialization failures")
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.NewHalfJitter(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for i := 1; i < 20; i++ {
		err = retrySerialization()
		if err == nil {
			return nil
		}

		select {
		case <-retry.After():
			l.cfg.Log.WithError(err).WithField("attempt", i).Debug("Retrying transaction.")
			retry.Inc()
		case <-ctx.Done():
			return ctx.Err()
		}

	}

	return trace.LimitExceeded("too many retries, last error: %v", err)
}

var schemas = []string{`
CREATE TABLE audit (
	event_time timestamp NOT NULL,
	event_type text NOT NULL,
	session_id uuid NOT NULL,
	event_index bigint NOT NULL,
	audit_id uuid PRIMARY KEY,
	event_data json NOT NULL,
	creation_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX audit_search_events_idx ON audit (event_time, session_id, event_index, audit_id);
CREATE INDEX audit_creation_time_idx ON audit (creation_time);
`}

// setupAndMigrate will create the required tables and indices in the database,
// by applying the SQL in the schemas slice consecutively. Stores the versions
// of the applied schemas in the audit_version table.
func (l *Log) setupAndMigrate(ctx context.Context) error {
	var currentVersion int
	if err := l.beginTxFunc(ctx, txReadWrite, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS audit_version (
				version int PRIMARY KEY CHECK (version > 0),
				creation_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
		); err != nil && !isCode(err, pgerrcode.InsufficientPrivilege) {
			return trace.Wrap(err)
		}

		if err := tx.QueryRow(ctx,
			"SELECT coalesce(max(version), 0) FROM audit_version",
		).Scan(&currentVersion); err != nil {
			return trace.Wrap(err)
		}

		if currentVersion > len(schemas) {
			return trace.BadParameter("database schema version greater than maximum supported")
		}

		for i := currentVersion; i < len(schemas); i++ {
			if _, err := tx.Exec(ctx, schemas[i]); err != nil {
				return trace.Wrap(err)
			}
			if _, err := tx.Exec(ctx,
				"INSERT INTO audit_version ( version ) VALUES ( $1 )",
				i+1,
			); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if currentVersion != len(schemas) {
		l.cfg.Log.WithFields(logrus.Fields{
			"prev_version": currentVersion,
			"cur_version":  len(schemas),
		}).Info("Migrated database schema.")
	}

	return nil
}

// periodicCleanup removes events past their retention period from the table,
// periodically. Returns after the context is done.
func (l *Log) periodicCleanup(ctx context.Context) {
	tk := time.NewTicker(l.cfg.CleanupInterval)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			l.cfg.Log.Debug("Executing periodic cleanup.")
			var deletedRows int64
			if err := l.beginTxFunc(ctx, txReadWrite, func(tx pgx.Tx) error {
				// the cast to interval is required, even though RetentionPeriod is
				// a time.Duration
				tag, err := tx.Exec(ctx,
					"DELETE FROM audit WHERE creation_time < (current_timestamp - $1::interval)",
					l.cfg.RetentionPeriod)
				if err != nil {
					return trace.Wrap(err)
				}

				deletedRows = tag.RowsAffected()
				return nil
			}); err != nil {
				l.cfg.Log.WithError(err).Warn("Failed to execute periodic cleanup.")
				continue
			}
			l.cfg.Log.WithField("deleted_rows", deletedRows).Debug("Executed periodic cleanup.")
		}
	}
}

// EmitAuditEvent implements events.IAuditLog
func (l *Log) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	var sid uuid.UUID
	if s := events.GetSessionID(event); s != "" {
		u, err := uuid.Parse(s)
		if err != nil {
			return trace.Wrap(err)
		}
		sid = u
	}

	err := l.beginTxFunc(ctx, txReadWrite, func(tx pgx.Tx) error {
		// there's no "generate a random UUID" polyglot that works across
		// versions of postgres and in crdb, so we generate the id ourselves
		_, err := tx.Exec(ctx, `
			INSERT INTO audit ( event_time, event_type, session_id, event_index, audit_id, event_data )
			VALUES ( $1, $2, $3, $4, $5, $6 )`,
			event.GetTime(), event.GetType(), sid, event.GetIndex(), uuid.New(), event,
		)
		return trace.Wrap(err)
	})

	return trace.Wrap(err)
}

// GetSessionEvents implements events.IAuditLog
func (l *Log) GetSessionEvents(_ string, sid session.ID, after int) ([]events.EventFields, error) {
	ctx := context.TODO()

	usid, err := uuid.Parse(string(sid))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if usid == uuid.Nil {
		return nil, trace.BadParameter("invalid all-zero session id")
	}

	var evs []events.EventFields
	if err := l.beginTxFunc(ctx, txReadOnly, func(tx pgx.Tx) error {
		evs = evs[:0]

		var ev events.EventFields
		_, err := tx.QueryFunc(ctx, `
			SELECT event_data
			FROM audit
			WHERE session_id = $1 AND event_index >= $2
			ORDER BY event_time, event_index, audit_id
			LIMIT $3`,
			[]any{usid, after, defaults.EventsMaxIterationLimit}, []any{&ev},
			func(pgx.QueryFuncRow) error { evs = append(evs, ev); ev = nil; return nil },
		)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return evs, nil
}


// searchEvents returns events within the time range, filtering (optionally) by
// event types, session id, and a generic condition, limiting results by a count
// and by a maximum size of the underlying json data of
// events.MaxEventBytesInResponse, sorting by time, session id and event index
// either ascending or descending, and returning an opaque, URL-safe string that
// can be passed to the same function to continue fetching data.
func (l *Log) searchEvents(
	ctx context.Context, fromUTC, toUTC time.Time,
	eventTypes []string, cond *types.WhereExpr, sessionID string,
	limit int, order types.EventOrder, startKey string,
) ([]apievents.AuditEvent, string, error) {
	if limit <= 0 {
		limit = defaults.EventsIterationLimit
	}
	if limit > defaults.EventsMaxIterationLimit {
		return nil, "", trace.BadParameter("limit %v exceeds %v", limit, defaults.MaxIterationLimit)
	}

	var startKeyset keyset
	if startKey != "" {
		if err := startKeyset.FromKey(startKey); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var condFn utils.FieldsCondition
	if cond != nil {
		f, err := utils.ToFieldsCondition(cond)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		condFn = f
	}

	var q queryBuilder
	q.Append(`
		DECLARE cur CURSOR FOR SELECT
			event_data,
			event_time,
			session_id,
			event_index,
			audit_id
		FROM audit
		WHERE event_time BETWEEN %v AND %v`,
		fromUTC.UTC(), toUTC.UTC())
	if len(eventTypes) > 0 {
		q.Append(" AND event_type = ANY (%v)", eventTypes)
	}
	if len(sessionID) > 0 {
		q.Append(" AND session_id = %v", sessionID)
	}
	if order != types.EventOrderDescending {
		if len(startKey) > 0 {
			q.Append(" AND (event_time, session_id, event_index, audit_id) > (%v, %v, %v, %v)",
				startKeyset.t, startKeyset.sid, startKeyset.ei, startKeyset.id)
		}
		q.Append(" ORDER BY event_time, session_id, event_index, audit_id")
	} else {
		if len(startKey) > 0 {
			q.Append(" AND (event_time, session_id, event_index, audit_id) < (%v, %v, %v, %v)",
				startKeyset.t, startKeyset.sid, startKeyset.ei, startKeyset.id)
		}
		q.Append(" ORDER BY event_time DESC, session_id DESC, event_index DESC, audit_id DESC")
	}

	var evs []apievents.AuditEvent
	var sizeLimit bool
	var endKeyset keyset

	const fetchSize = defaults.EventsIterationLimit
	fetchQuery := fmt.Sprintf("FETCH %d FROM cur", fetchSize)

	if err := l.beginTxFunc(ctx, txReadOnly, func(tx pgx.Tx) error {
		evs = evs[:0]
		sizeLimit = false

		if _, err := tx.Exec(ctx, q.String(), q.Args()...); err != nil {
			return trace.Wrap(err)
		}

		totalSize := 0
		for len(evs) < limit && !sizeLimit {
			rows, err := tx.Query(ctx, fetchQuery)
			if err != nil {
				return trace.Wrap(err)
			}

			var data []byte
			var ks keyset
			for rows.Next() {
				if err := rows.Scan(&data, &ks.t, &ks.sid, &ks.ei, &ks.id); err != nil {
					rows.Close()
					return trace.Wrap(err)
				}

				var evf events.EventFields
				if err := utils.FastUnmarshal(data, &evf); err != nil {
					rows.Close()
					return trace.Wrap(err)
				}

				// TODO(espadolini): encode cond as a condition in the query
				if condFn != nil && !condFn(utils.Fields(evf)) {
					continue
				}

				if len(data)+totalSize > events.MaxEventBytesInResponse {
					sizeLimit = true
					rows.Close()
					break
				}
				totalSize += len(data)

				ev, err := events.FromEventFields(evf)
				if err != nil {
					rows.Close()
					return trace.Wrap(err)
				}

				evs = append(evs, ev)
				endKeyset = ks

				if len(evs) >= limit {
					rows.Close()
					break
				}
			}
			if err := rows.Err(); err != nil {
				return trace.Wrap(err)
			}

			if rows.CommandTag().RowsAffected() < fetchSize {
				break
			}
		}
		return nil
	}); err != nil {
		return nil, "", trace.Wrap(err)
	}

	nextKey := ""
	if len(evs) > 0 && (len(evs) >= limit || sizeLimit) {
		nextKey = endKeyset.ToKey()
	}

	return evs, nextKey, nil
}

// SearchEvents implements events.IAuditLog
func (l *Log) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	var emptyCond *types.WhereExpr
	const emptySessionID = ""
	return l.searchEvents(context.TODO(), req.From, req.To, req.EventTypes, emptyCond, emptySessionID, req.Limit, req.Order, req.StartKey)
}

// SearchSessionEvents implements events.IAuditLog
func (l *Log) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	sessionEndTypes := []string{events.SessionEndEvent, events.WindowsDesktopSessionEndEvent}
	return l.searchEvents(context.TODO(), req.From, req.To, sessionEndTypes, req.Cond, req.SessionID, req.Limit, req.Order, req.StartKey)
}

// GetSessionChunk does not implement events.IAuditLog
func (*Log) GetSessionChunk(string, session.ID, int, int) ([]byte, error) {
	return nil, trace.NotImplemented("not implemented by external log pgevents")
}

// StreamSessionEvents does not implement events.IAuditLog
func (*Log) StreamSessionEvents(context.Context, session.ID, int64) (chan apievents.AuditEvent, chan error) {
	errC := make(chan error, 1)
	errC <- trace.NotImplemented("not implemented by external log pgevents")
	return nil, errC
}
