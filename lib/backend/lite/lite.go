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

package lite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mattn/go-sqlite3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
)

func init() {
	backend.MustRegister(GetName(), func(ctx context.Context, p backend.Params) (backend.Backend, error) {
		return New(ctx, p)
	})
}

const (
	// BackendName is the name of this backend.
	BackendName = "sqlite"
	// AlternativeName is another name of this backend.
	AlternativeName = "dir"

	// SyncFull fsyncs the database file on disk after every write.
	SyncFull = "FULL"

	// JournalMemory keeps the rollback journal in memory instead of storing it
	// on disk.
	JournalMemory = "MEMORY"
)

const (
	// defaultDirMode is the mode of the newly created directories that are part
	// of the Path
	defaultDirMode os.FileMode = 0700

	// dbMode is the mode set on sqlite database files
	dbMode os.FileMode = 0600

	// defaultDBFile is the file name of the sqlite db in the directory
	// specified by Path
	defaultDBFile            = "sqlite.db"
	slowTransactionThreshold = time.Second

	// defaultSync is the default value for Sync
	defaultSync = SyncFull

	// defaultBusyTimeout is the default value for BusyTimeout, in ms
	defaultBusyTimeout = 10000
)

// GetName is a part of backend API and it returns SQLite backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

// Config structure represents configuration section
type Config struct {
	// Path is a path to the database directory
	Path string `json:"path,omitempty"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// PollStreamPeriod is a polling period for event stream
	PollStreamPeriod time.Duration `json:"poll_stream_period,omitempty"`
	// EventsOff turns events off
	EventsOff bool `json:"events_off,omitempty"`
	// Clock allows to override clock used in the backend
	Clock clockwork.Clock `json:"-"`
	// Sync sets the synchronous pragma
	Sync string `json:"sync,omitempty"`
	// BusyTimeout sets busy timeout in milliseconds
	BusyTimeout int `json:"busy_timeout,omitempty"`
	// Journal sets the journal_mode pragma
	Journal string `json:"journal,omitempty"`
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to sqlite
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Path == "" {
		return trace.BadParameter("specify directory path to the database using 'path' parameter")
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	if cfg.PollStreamPeriod == 0 {
		cfg.PollStreamPeriod = backend.DefaultPollStreamPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Sync == "" {
		cfg.Sync = defaultSync
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = defaultBusyTimeout
	}
	return nil
}

// ConnectionURI returns a connection string usable with sqlite according to the
// Config.
func (cfg *Config) ConnectionURI() string {
	params := url.Values{}
	params.Set("_busy_timeout", strconv.Itoa(cfg.BusyTimeout))
	// The _txlock parameter is parsed by go-sqlite to determine if (all)
	// transactions should be started with `BEGIN DEFERRED` (the default, same
	// as `BEGIN`), `BEGIN IMMEDIATE` or `BEGIN EXCLUSIVE`.
	//
	// The way we use sqlite relies entirely on the busy timeout handler (also
	// configured through the connection URL, with the _busy_timeout parameter)
	// to address concurrency problems, and treats any SQLITE_BUSY errors as a
	// fatal issue with the database; however, in scenarios with multiple
	// readwriters it is possible to still get a busy error even with a generous
	// busy timeout handler configured, as two transactions that both start off
	// with a SELECT - thus acquiring a SHARED lock, see
	// https://www.sqlite.org/lockingv3.html#transaction_control - then attempt
	// to upgrade to a RESERVED lock to upsert or delete something can end up
	// requiring one of the two transactions to forcibly rollback to avoid a
	// deadlock, which is signaled by the sqlite engine with a SQLITE_BUSY error
	// returned to one of the two. When that happens, a concurrent-aware program
	// can just try the transaction again a few times - making sure to disregard
	// what was read before the transaction actually committed.
	//
	// As we're not really interested in concurrent sqlite access (process
	// storage has very little written to, sharing a sqlite database as the
	// backend between two auths is not really supported, and caches shouldn't
	// ever run on the same underlying sqlite backend) we instead start every
	// transaction with `BEGIN IMMEDIATE`, which grabs a RESERVED lock
	// immediately (waiting for the busy timeout in case some other connection
	// to the database has the lock) at the beginning of the transaction, thus
	// avoiding any spurious SQLITE_BUSY error that can happen halfway through a
	// transaction.
	//
	// If we end up requiring better concurrent access to sqlite in the future
	// we should consider enabling Write-Ahead Logging mode, to actually allow
	// for reads to happen at the same time as writes, adding some amount of
	// retries to inTransaction, and double-checking that all uses of it
	// correctly handle the possibility of the transaction being restarted.
	params.Set("_txlock", "immediate")
	if cfg.Sync != "" {
		params.Set("_sync", cfg.Sync)
	}
	if cfg.Journal != "" {
		params.Set("_journal", cfg.Journal)
	}

	u := url.URL{
		Scheme:   "file",
		OmitHost: true,
		Path:     filepath.Join(cfg.Path, defaultDBFile),
		RawQuery: params.Encode(),
	}
	return u.String()
}

// New returns a new instance of sqlite backend
func New(ctx context.Context, params backend.Params) (*Backend, error) {
	var cfg *Config
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("SQLite configuration is invalid: %v", err)
	}
	return NewWithConfig(ctx, *cfg)
}

// NewWithConfig returns a new instance of lite backend using
// configuration struct as a parameter
func NewWithConfig(ctx context.Context, cfg Config) (*Backend, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	connectionURI := cfg.ConnectionURI()
	path := filepath.Join(cfg.Path, defaultDBFile)
	// Ensure that the path to the root directory exists.
	err := os.MkdirAll(cfg.Path, os.ModeDir|defaultDirMode)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, trace.AccessDenied("Teleport does not have permission to write to %v", path)
		}
		return nil, trace.ConvertSystemError(err)
	}

	setPermissions := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		setPermissions = true
	}

	db, err := sql.Open("sqlite3", cfg.ConnectionURI())
	if err != nil {
		return nil, trace.Wrap(err, "error opening URI: %v", connectionURI)
	}

	if setPermissions {
		// Ensure the database has restrictive access permissions.
		err = db.PingContext(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = os.Chmod(path, dbMode)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	// serialize access to sqlite, as we're using immediate transactions anyway,
	// and in-memory go locks are faster than sqlite locks
	db.SetMaxOpenConns(1)
	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)
	closeCtx, cancel := context.WithCancel(ctx)
	l := &Backend{
		Config: cfg,
		db:     db,
		logger: slog.With(teleport.ComponentKey, BackendName),
		clock:  cfg.Clock,
		buf:    buf,
		ctx:    closeCtx,
		cancel: cancel,
	}
	l.logger.DebugContext(ctx, "Connected to database", "database", connectionURI, "poll_stream_period", cfg.PollStreamPeriod)
	if err := l.createSchema(); err != nil {
		return nil, trace.Wrap(err, "error creating schema: %v", connectionURI)
	}
	if err := l.showPragmas(); err != nil {
		l.logger.WarnContext(ctx, "Failed to show pragma settings", "error", err)
	}
	go l.runPeriodicOperations()
	return l, nil
}

// Backend uses SQLite to implement storage interfaces
type Backend struct {
	Config
	logger *slog.Logger
	db     *sql.DB
	// clock is used to generate time,
	// could be swapped in tests for fixed time
	clock clockwork.Clock

	buf    *backend.CircularBuffer
	ctx    context.Context
	cancel context.CancelFunc

	// closedFlag is set to indicate that the database is closed
	closedFlag int32
}

func (l *Backend) GetName() string {
	return GetName()
}

// showPragmas is used to debug SQLite database connection
// parameters, when called, logs some key PRAGMA values
func (l *Backend) showPragmas() error {
	return l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		var journalMode string
		row := tx.QueryRowContext(l.ctx, "PRAGMA journal_mode;")
		if err := row.Scan(&journalMode); err != nil {
			return trace.Wrap(err)
		}
		row = tx.QueryRowContext(l.ctx, "PRAGMA synchronous;")
		var synchronous string
		if err := row.Scan(&synchronous); err != nil {
			return trace.Wrap(err)
		}
		var busyTimeout string
		row = tx.QueryRowContext(l.ctx, "PRAGMA busy_timeout;")
		if err := row.Scan(&busyTimeout); err != nil {
			return trace.Wrap(err)
		}
		l.logger.DebugContext(l.ctx, "retrieved pragma values", "journal_mode", journalMode, "synchronous", synchronous, "busy_timeout", busyTimeout)
		return nil
	})
}

func (l *Backend) createSchema() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS kv (
           key TEXT NOT NULL PRIMARY KEY,
           modified INTEGER NOT NULL,
           expires DATETIME,
           value BLOB,
           revision TEXT NOT NULL DEFAULT ""
		);
        CREATE INDEX IF NOT EXISTS kv_expires ON kv (expires);`,

		`CREATE TABLE IF NOT EXISTS events (
           id INTEGER PRIMARY KEY,
           type TEXT NOT NULL,
           created INTEGER NOT NULL,
           kv_key TEXT NOT NULL,
           kv_modified INTEGER NOT NULL,
           kv_expires DATETIME,
           kv_value BLOB,
           kv_revision TEXT NOT NULL DEFAULT ""
         );
        CREATE INDEX IF NOT EXISTS events_created ON events (created);`,

		`DROP TABLE IF EXISTS meta;`,
	}

	for _, schema := range schemas {
		if _, err := l.db.ExecContext(l.ctx, schema); err != nil {
			l.logger.ErrorContext(l.ctx, "Failed schema step", "step", schema, "error", err)
			return trace.Wrap(err)
		}
	}

	for table, column := range map[string]string{"kv": "revision", "events": "kv_revision"} {
		if err := l.migrateRevision(table, column); err != nil {
			l.logger.WarnContext(l.ctx, "Failed schema migration", "table", table, "column", column, "error", err)
			return trace.Wrap(err)
		}
	}

	return nil
}

func (l *Backend) migrateRevision(table, column string) (err error) {
	return trace.Wrap(l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(l.ctx, "SELECT EXISTS( SELECT 1 FROM pragma_table_info(?) WHERE name = ? )", table, column).Scan(&exists); err != nil {
			return trace.Wrap(err)
		}

		if exists {
			return nil
		}

		query := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s TEXT NOT NULL DEFAULT "";`, table, column)
		_, err = tx.ExecContext(l.ctx, query)
		return trace.Wrap(err)
	}))
}

// SetClock sets internal backend clock
func (l *Backend) SetClock(clock clockwork.Clock) {
	l.clock = clock
}

// Clock returns clock used by the backend
func (l *Backend) Clock() clockwork.Clock {
	return l.clock
}

// Create creates item if it does not exist
func (l *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}

	i.Revision = backend.CreateRevision()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		created := l.clock.Now().UTC()
		if !l.EventsOff {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpPut, created, i.Key.String(), id(created), expires(i.Expires), i.Value, i.Revision); err != nil {
				return trace.Wrap(err)
			}
		}

		rows, err := tx.QueryContext(ctx, "SELECT key, value, expires, modified FROM kv WHERE key = ? AND expires <= ? LIMIT 1", i.Key.String(), created)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()

		if rows.Next() {
			err = l.deleteInTransaction(ctx, i.Key, tx)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO kv(key, modified, expires, value, revision) values(?, ?, ?, ?, ?)", i.Key.String(), id(created), expires(i.Expires), i.Value, i.Revision); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return backend.NewLease(i), nil
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (l *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if expected.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if replaceWith.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if expected.Key.Compare(replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	now := l.clock.Now().UTC()
	replaceWith.Revision = backend.CreateRevision()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx, "SELECT value FROM kv WHERE key = ? AND (expires IS NULL OR expires > ?) LIMIT 1")
		if err != nil {
			return trace.Wrap(err)
		}
		defer q.Close()

		row := q.QueryRowContext(ctx, expected.Key.String(), now)
		var value []byte
		if err := row.Scan(&value); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return trace.CompareFailed("key %q is not found", expected.Key.String())
			}
			return trace.Wrap(err)
		}

		if !bytes.Equal(value, expected.Value) {
			return trace.CompareFailed("current value does not match expected for %v", expected.Key)
		}

		created := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET value = ?, expires = ?, modified = ?, revision = ? WHERE key = ?")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		_, err = stmt.ExecContext(ctx, replaceWith.Value, expires(replaceWith.Expires), id(created), replaceWith.Revision, replaceWith.Key.String())
		if err != nil {
			return trace.Wrap(err)
		}
		if !l.EventsOff {
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpPut, created, replaceWith.Key.String(), id(created), expires(replaceWith.Expires), replaceWith.Value, replaceWith.Revision); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return backend.NewLease(replaceWith), nil
}

// id converts time to ID
func id(t time.Time) int64 {
	return t.UTC().UnixNano()
}

// Put puts value into backend (creates if it does not
// exist, updates it otherwise)
func (l *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}

	i.Revision = backend.CreateRevision()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.putInTransaction(ctx, i, tx)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return backend.NewLease(i), nil
}

func (l *Backend) putInTransaction(ctx context.Context, i backend.Item, tx *sql.Tx) error {
	created := l.clock.Now().UTC()
	recordID := id(created)
	if !l.EventsOff {
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		if _, err := stmt.ExecContext(ctx, types.OpPut, created, i.Key.String(), recordID, expires(i.Expires), i.Value, i.Revision); err != nil {
			return trace.Wrap(err)
		}
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO kv(key, modified, expires, value, revision) values(?, ?, ?, ?, ?)")
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, i.Key.String(), recordID, expires(i.Expires), i.Value, i.Revision); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Update updates value in the backend
func (l *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}

	i.Revision = backend.CreateRevision()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		created := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET value = ?, expires = ?, modified = ?, revision = ? WHERE kv.key = ? AND (expires IS NULL OR expires > ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		result, err := stmt.ExecContext(ctx, i.Value, expires(i.Expires), id(created), i.Revision, i.Key.String(), created)
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.NotFound("key %q is not found", i.Key.String())
		}
		if !l.EventsOff {
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpPut, created, i.Key.String(), id(created), expires(i.Expires), i.Value, i.Revision); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return backend.NewLease(i), nil
}

// Get returns a single item or not found error
func (l *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	if key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}
	var item backend.Item
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.getInTransaction(ctx, key, tx, &item)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if item.Revision == "" {
		item.Revision = backend.BlankRevision
	}
	return &item, nil
}

// getInTransaction returns an item, works in transaction
func (l *Backend) getInTransaction(ctx context.Context, key backend.Key, tx *sql.Tx, item *backend.Item) error {
	q, err := tx.PrepareContext(ctx,
		"SELECT key, value, expires, revision FROM kv WHERE key = ? AND (expires IS NULL OR expires > ?) LIMIT 1")
	if err != nil {
		return trace.Wrap(err)
	}
	defer q.Close()

	row := q.QueryRowContext(ctx, key.String(), l.clock.Now().UTC())
	var expires sql.NullTime
	if err := row.Scan(&item.Key, &item.Value, &expires, &item.Revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return trace.NotFound("key %q is not found", key.String())
		}
		return trace.Wrap(err)
	}
	item.Expires = expires.Time
	return nil
}

func (l *Backend) Items(ctx context.Context, params backend.ItemsParams) iter.Seq2[backend.Item, error] {
	if params.StartKey.IsZero() {
		err := trace.BadParameter("missing parameter startKey")
		return func(yield func(backend.Item, error) bool) { yield(backend.Item{}, err) }
	}
	if params.EndKey.IsZero() {
		err := trace.BadParameter("missing parameter endKey")
		return func(yield func(backend.Item, error) bool) { yield(backend.Item{}, err) }
	}

	const (
		queryAsc        = "SELECT key, value, expires, revision FROM kv WHERE (key BETWEEN ? AND ?) AND (? == '' OR key > ?) AND (expires IS NULL OR expires > ?) ORDER BY key ASC LIMIT ?"
		queryDesc       = "SELECT key, value, expires, revision FROM kv WHERE (key BETWEEN ? AND ?) AND (? == '' OR key < ?) AND (expires IS NULL OR expires > ?) ORDER BY key DESC LIMIT ?"
		defaultPageSize = 1000
	)
	return func(yield func(backend.Item, error) bool) {
		limit := params.Limit
		if limit <= 0 {
			limit = backend.DefaultRangeLimit
		}

		var exclusiveStartKey string
		startKey := params.StartKey.String()
		endKey := params.EndKey.String()

		query := queryAsc
		if params.Descending {
			query = queryDesc
		}

		var pageLimit, totalCount int
		items := make([]backend.Item, 0, min(limit, defaultPageSize))
		for {
			items = items[:0]
			pageLimit = min(limit-totalCount, defaultPageSize)
			if err := l.inTransaction(ctx, func(tx *sql.Tx) error {
				q, err := tx.PrepareContext(ctx, query)
				if err != nil {
					return trace.Wrap(err)
				}
				defer q.Close()

				rows, err := q.QueryContext(ctx, startKey, endKey, exclusiveStartKey, exclusiveStartKey, l.clock.Now().UTC(), pageLimit)
				if err != nil {
					return trace.Wrap(err)
				}
				defer rows.Close()

				for rows.Next() {
					var item backend.Item
					var expires sql.NullTime
					if err := rows.Scan(&item.Key, &item.Value, &expires, &item.Revision); err != nil {
						return trace.Wrap(err)
					}
					item.Expires = expires.Time
					if item.Revision == "" {
						item.Revision = backend.BlankRevision
					}

					items = append(items, item)
				}

				// Explicitly call rows.Close() to return the error instead of
				// it being ignored in the defer above.
				return trace.Wrap(rows.Close())
			}); err != nil {
				yield(backend.Item{}, trace.Wrap(err))
				return
			}

			if len(items) >= pageLimit {
				exclusiveStartKey = items[len(items)-1].Key.String()
			}

			for _, item := range items {
				if !yield(item, nil) {
					return
				}

				totalCount++
				if limit != backend.NoLimit && totalCount >= limit {
					return
				}
			}

			if len(items) < pageLimit {
				return
			}
		}
	}
}

// GetRange returns query range
func (l *Backend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	if startKey.IsZero() {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}

	var result backend.GetResult
	for item, err := range l.Items(ctx, backend.ItemsParams{StartKey: startKey, EndKey: endKey, Limit: limit}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.Items = append(result.Items, item)
	}

	if len(result.Items) == backend.DefaultRangeLimit {
		l.logger.WarnContext(ctx, "Range query hit backend limit. (this is a bug!)", "start_key", startKey, "limit", backend.DefaultRangeLimit)
	}
	return &result, nil
}

// KeepAlive updates TTL on the lease
func (l *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if lease.Key.IsZero() {
		return trace.BadParameter("lease key is not specified")
	}
	now := l.clock.Now().UTC()
	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		var item backend.Item
		err := l.getInTransaction(ctx, lease.Key, tx, &item)
		if err != nil {
			return trace.Wrap(err)
		}
		created := l.clock.Now().UTC()
		item.Revision = backend.CreateRevision()
		if !l.EventsOff {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpPut, created, item.Key.String(), id(created), expires.UTC(), item.Value, item.Revision); err != nil {
				return trace.Wrap(err)
			}
		}
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET expires = ?, modified = ?, revision = ? WHERE key = ?")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		result, err := stmt.ExecContext(ctx, expires.UTC(), id(now), backend.CreateRevision(), lease.Key.String())
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.NotFound("key %q is not found", lease.Key.String())
		}
		return nil
	})
}

func (l *Backend) deleteInTransaction(ctx context.Context, key backend.Key, tx *sql.Tx) error {
	stmt, err := tx.PrepareContext(ctx, "DELETE FROM kv WHERE key = ?")
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()

	result, err := stmt.ExecContext(ctx, key.String())
	if err != nil {
		return trace.Wrap(err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return trace.Wrap(err)
	}
	if rows == 0 {
		return trace.NotFound("key %q is not found", key.String())
	}
	if !l.EventsOff {
		created := l.clock.Now().UTC()
		stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified) values(?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		if _, err := stmt.ExecContext(ctx, types.OpDelete, created, key.String(), created.UnixNano()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (l *Backend) Delete(ctx context.Context, key backend.Key) error {
	if key.IsZero() {
		return trace.BadParameter("missing parameter key")
	}
	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.deleteInTransaction(ctx, key, tx)
	})
}

// DeleteRange deletes range of items with keys between startKey and endKey
// Note that elements deleted by range do not produce any events
func (l *Backend) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	if startKey.IsZero() {
		return trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return trace.BadParameter("missing parameter endKey")
	}
	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT key FROM kv WHERE key >= ? and key <= ?")
		if err != nil {
			return trace.Wrap(err)
		}
		defer q.Close()

		rows, err := q.QueryContext(ctx, startKey.String(), endKey.String())
		if err != nil {
			return trace.Wrap(err)
		}

		var keys []backend.Key
		defer rows.Close()
		for rows.Next() {
			var key backend.Key
			if err := rows.Scan(&key); err != nil {
				return trace.Wrap(err)
			}

			keys = append(keys, key)
		}

		// Close rows early before any deletions occur.
		if err := rows.Close(); err != nil {
			return trace.Wrap(err)
		}

		for _, key := range keys {
			if err := l.deleteInTransaction(l.ctx, key, tx); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	})
}

func (l *Backend) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key.IsZero() || i.Revision == "" {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	if i.Revision == backend.BlankRevision {
		i.Revision = ""
	}

	rev := backend.CreateRevision()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		now := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET value = ?, expires = ?, modified = ?, revision = ? WHERE key = ? AND revision = ? AND (expires IS NULL OR expires > ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		result, err := stmt.ExecContext(ctx, i.Value, expires(i.Expires), id(now), rev, i.Key.String(), i.Revision, now)
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.Wrap(backend.ErrIncorrectRevision)
		}
		if !l.EventsOff {
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value, kv_revision) values(?, ?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpPut, now, i.Key.String(), id(now), expires(i.Expires), i.Value, rev); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Revision = rev
	return backend.NewLease(i), nil
}

func (l *Backend) ConditionalDelete(ctx context.Context, key backend.Key, revision string) error {
	if key.IsZero() || revision == "" {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	if revision == backend.BlankRevision {
		revision = ""
	}

	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		now := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "DELETE FROM kv WHERE key = ? AND revision = ? AND (expires IS NULL OR expires > ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		defer stmt.Close()

		result, err := stmt.ExecContext(ctx, key.String(), revision, now)
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.Wrap(backend.ErrIncorrectRevision)
		}
		if !l.EventsOff {
			created := l.clock.Now().UTC()
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_revision) values(?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			defer stmt.Close()

			if _, err := stmt.ExecContext(ctx, types.OpDelete, created, key.String(), created.UnixNano(), revision); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
}

// NewWatcher returns a new event watcher
func (l *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	if l.EventsOff {
		return nil, trace.BadParameter("events are turned off for this backend")
	}
	return l.buf.NewWatcher(ctx, watch)
}

// Close closes all associated resources
func (l *Backend) Close() error {
	l.cancel()
	return l.closeDatabase()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (l *Backend) CloseWatchers() {
	l.buf.Clear()
}

func (l *Backend) isClosed() bool {
	return atomic.LoadInt32(&l.closedFlag) == 1
}

func (l *Backend) setClosed() {
	atomic.StoreInt32(&l.closedFlag, 1)
}

func (l *Backend) closeDatabase() error {
	l.setClosed()
	l.buf.Close()
	return l.db.Close()
}

func (l *Backend) inTransaction(ctx context.Context, f func(tx *sql.Tx) error) (err error) {
	start := time.Now()
	defer func() {
		diff := time.Since(start)
		if diff > slowTransactionThreshold {
			l.logger.WarnContext(ctx, "SLOW TRANSACTION", "duration", diff, "stack", string(debug.Stack()))
		}
	}()
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	commit := func() error {
		return tx.Commit()
	}
	rollback := func() error {
		return tx.Rollback()
	}
	defer func() {
		if r := recover(); r != nil {
			l.logger.ErrorContext(ctx, "Unexpected panic in inTransaction, trying to rollback.", "error", r)
			err = trace.BadParameter("panic: %v", r)
			if e2 := rollback(); e2 != nil {
				l.logger.ErrorContext(ctx, "Failed to rollback", "error", e2)
			}
			return
		}
		if err != nil && !trace.IsNotFound(err) {
			if isConstraintError(trace.Unwrap(err)) {
				err = trace.AlreadyExists("%s", err)
			}
			// transaction aborted by interrupt, no action needed
			if isInterrupt(trace.Unwrap(err)) {
				return
			}
			if isLockedError(trace.Unwrap(err)) {
				err = trace.ConnectionProblem(err, "database is locked")
			}
			if isReadonlyError(trace.Unwrap(err)) {
				err = trace.ConnectionProblem(err, "database is in readonly mode")
			}
			if !l.isClosed() {
				if !trace.IsCompareFailed(err) && !trace.IsAlreadyExists(err) && !trace.IsConnectionProblem(err) {
					l.logger.WarnContext(ctx, "Unexpected error in inTransaction, rolling back.", "error", err)
				}
				if e2 := rollback(); e2 != nil {
					l.logger.ErrorContext(ctx, "Failed to rollback too", "error", e2)
				}
			}
			return
		}
		if err2 := commit(); err2 != nil {
			err = trace.Wrap(err2)
		}
	}()
	err = f(tx)
	return
}

func expires(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func convertError(err error) error {
	origError := trace.Unwrap(err)
	if isClosedError(origError) {
		return trace.ConnectionProblem(err, "database is closed")
	}
	return err
}

func isClosedError(err error) bool {
	return err.Error() == "sql: database is closed"
}

func isConstraintError(err error) bool {
	var e sqlite3.Error
	if ok := errors.As(err, &e); !ok {
		return false
	}
	return e.Code == sqlite3.ErrConstraint
}

func isLockedError(err error) bool {
	var e sqlite3.Error
	if ok := errors.As(err, &e); !ok {
		return false
	}
	return e.Code == sqlite3.ErrBusy
}

func isInterrupt(err error) bool {
	var e sqlite3.Error
	if ok := errors.As(err, &e); !ok {
		return false
	}
	return e.Code == sqlite3.ErrInterrupt
}

func isReadonlyError(err error) bool {
	var e sqlite3.Error
	if ok := errors.As(err, &e); !ok {
		return false
	}
	return e.Code == sqlite3.ErrReadonly
}
