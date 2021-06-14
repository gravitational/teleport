/*
Copyright 2018-2019 Gravitational, Inc.

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

package lite

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	sqlite3 "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

const (
	// BackendName is the name of this backend
	BackendName = "sqlite"
	// AlternativeName is another name of this backend.
	AlternativeName                      = "dir"
	defaultDirMode           os.FileMode = 0770
	defaultDBFile                        = "sqlite.db"
	slowTransactionThreshold             = time.Second
	syncOFF                              = "OFF"
	busyTimeout                          = 10000
)

// GetName is a part of backend API and it returns SQLite backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

func init() {
	sql.Register(BackendName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return nil
		},
	})
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
	// Sync sets synchronous pragrma
	Sync string `json:"sync,omitempty"`
	// BusyTimeout sets busy timeout in milliseconds
	BusyTimeout int `json:"busy_timeout,omitempty"`
	// Memory turns memory mode of the database
	Memory bool `json:"memory"`
	// MemoryName sets the name of the database,
	// set to "sqlite.db" by default
	MemoryName string `json:"memory_name"`
	// Mirror turns on mirror mode for the backend,
	// which will use record IDs for Put and PutRange passed from
	// the resources, not generate a new one
	Mirror bool `json:"mirror"`
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to sqlite
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Path == "" && !cfg.Memory {
		return trace.BadParameter("specify directory path to the database using 'path' parameter")
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferSize
	}
	if cfg.PollStreamPeriod == 0 {
		cfg.PollStreamPeriod = backend.DefaultPollStreamPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Sync == "" {
		cfg.Sync = syncOFF
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = busyTimeout
	}
	if cfg.MemoryName == "" {
		cfg.MemoryName = defaultDBFile
	}
	return nil
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
	var connectorURL string
	if !cfg.Memory {
		// Ensure that the path to the root directory exists.
		err := os.MkdirAll(cfg.Path, defaultDirMode)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		fullPath := filepath.Join(cfg.Path, defaultDBFile)
		connectorURL = fmt.Sprintf("file:%v?_busy_timeout=%v&_sync=%v", fullPath, cfg.BusyTimeout, cfg.Sync)
	} else {
		connectorURL = fmt.Sprintf("file:%v?mode=memory", cfg.MemoryName)
	}
	db, err := sql.Open(BackendName, connectorURL)
	if err != nil {
		return nil, trace.Wrap(err, "error opening URI: %v", connectorURL)
	}
	// serialize access to sqlite to avoid database is locked errors
	db.SetMaxOpenConns(1)
	buf, err := backend.NewCircularBuffer(ctx, cfg.BufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	closeCtx, cancel := context.WithCancel(ctx)
	watchStarted, signalWatchStart := context.WithCancel(ctx)
	l := &Backend{
		Config:           cfg,
		db:               db,
		Entry:            log.WithFields(log.Fields{trace.Component: BackendName}),
		clock:            cfg.Clock,
		buf:              buf,
		ctx:              closeCtx,
		cancel:           cancel,
		watchStarted:     watchStarted,
		signalWatchStart: signalWatchStart,
	}
	l.Debugf("Connected to: %v, poll stream period: %v", connectorURL, cfg.PollStreamPeriod)
	if err := l.createSchema(); err != nil {
		return nil, trace.Wrap(err, "error creating schema: %v", connectorURL)
	}
	if err := l.showPragmas(); err != nil {
		l.Warningf("Failed to show pragma settings: %v.", err)
	}
	go l.runPeriodicOperations()
	return l, nil
}

// Backend uses SQLite to implement storage interfaces
type Backend struct {
	Config
	*log.Entry
	backend.NoMigrations
	db *sql.DB
	// clock is used to generate time,
	// could be swapped in tests for fixed time
	clock clockwork.Clock

	buf              *backend.CircularBuffer
	ctx              context.Context
	cancel           context.CancelFunc
	watchStarted     context.Context
	signalWatchStart context.CancelFunc

	// closedFlag is set to indicate that the database is closed
	closedFlag int32
}

// showPragmas is used to debug SQLite database connection
// parameters, when called, logs some key PRAGMA values
func (l *Backend) showPragmas() error {
	return l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(l.ctx, "PRAGMA synchronous;")
		var syncValue string
		if err := row.Scan(&syncValue); err != nil {
			return trace.Wrap(err)
		}
		var timeoutValue string
		row = tx.QueryRowContext(l.ctx, "PRAGMA busy_timeout;")
		if err := row.Scan(&timeoutValue); err != nil {
			return trace.Wrap(err)
		}
		l.Debugf("Synchronous: %v, busy timeout: %v", syncValue, timeoutValue)
		return nil
	})
}

func (l *Backend) createSchema() error {
	schemas := []string{

		`CREATE TABLE IF NOT EXISTS kv (
           key TEXT NOT NULL PRIMARY KEY,
           modified INTEGER NOT NULL,
           expires DATETIME,
           value BLOB);
        CREATE INDEX IF NOT EXISTS kv_expires ON kv (expires);`,

		`CREATE TABLE IF NOT EXISTS events (
           id INTEGER PRIMARY KEY,
           type TEXT NOT NULL,
           created INTEGER NOT NULL,
           kv_key TEXT NOT NULL,
           kv_modified INTEGER NOT NULL,
           kv_expires DATETIME,
           kv_value BLOB
         );
        CREATE INDEX IF NOT EXISTS events_created ON events (created);`,

		`CREATE TABLE IF NOT EXISTS meta (
           version INTEGER NOT NULL,
           imported BOOLEAN NOT NULL
         );`,
	}

	for _, schema := range schemas {
		if _, err := l.db.ExecContext(l.ctx, schema); err != nil {
			l.Errorf("Failing schema step: %v, %v.", schema, err)
			return trace.Wrap(err)
		}
	}

	return nil
}

func (l *Backend) newLease(item backend.Item) *backend.Lease {
	var lease backend.Lease
	if item.Expires.IsZero() {
		return &lease
	}
	lease.Key = item.Key
	return &lease
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
	if len(i.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		created := l.clock.Now().UTC()
		if !l.EventsOff {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := stmt.ExecContext(ctx, types.OpPut, created, string(i.Key), id(created), expires(i.Expires), i.Value); err != nil {
				return trace.Wrap(err)
			}
		}
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO kv(key, modified, expires, value) values(?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := stmt.ExecContext(ctx, string(i.Key), id(created), expires(i.Expires), i.Value); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.newLease(i), nil
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (l *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}
	now := l.clock.Now().UTC()
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT value FROM kv WHERE key = ? AND (expires IS NULL OR expires > ?) LIMIT 1")
		if err != nil {
			return trace.Wrap(err)
		}
		row := q.QueryRowContext(ctx, string(expected.Key), now)
		var value []byte
		if err := row.Scan(&value); err != nil {
			if err == sql.ErrNoRows {
				return trace.CompareFailed("key %v is not found", string(expected.Key))
			}
			return trace.Wrap(err)
		}

		if !bytes.Equal(value, expected.Value) {
			return trace.CompareFailed("current value does not match expected for %v", string(expected.Key))
		}

		created := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET value = ?, expires = ?, modified = ? WHERE key = ?")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = stmt.ExecContext(ctx, replaceWith.Value, expires(replaceWith.Expires), id(created), string(replaceWith.Key))
		if err != nil {
			return trace.Wrap(err)
		}
		if !l.EventsOff {
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := stmt.ExecContext(ctx, types.OpPut, created, string(replaceWith.Key), id(created), expires(replaceWith.Expires), replaceWith.Value); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.newLease(replaceWith), nil
}

// id converts time to ID
func id(t time.Time) int64 {
	return t.UTC().UnixNano()
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (l *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key == nil {
		return nil, trace.BadParameter("missing parameter key")
	}
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		created := l.clock.Now().UTC()
		recordID := i.ID
		if !l.Mirror {
			recordID = id(created)
		}
		if !l.EventsOff {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := stmt.ExecContext(ctx, types.OpPut, created, string(i.Key), recordID, expires(i.Expires), i.Value); err != nil {
				return trace.Wrap(err)
			}
		}
		stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO kv(key, modified, expires, value) values(?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := stmt.ExecContext(ctx, string(i.Key), recordID, expires(i.Expires), i.Value); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.newLease(i), nil
}

const (
	schemaVersion = 1
)

// Imported returns true if backend already imported data from another backend
func (l *Backend) Imported(ctx context.Context) (imported bool, err error) {
	err = l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT imported from meta LIMIT 1")
		if err != nil {
			return trace.Wrap(err)
		}
		row := q.QueryRowContext(ctx)
		if err := row.Scan(&imported); err != nil {
			if err != sql.ErrNoRows {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	return imported, err
}

// Import imports elements, makes sure elements are imported only once
// returns trace.AlreadyExists if elements have been imported
func (l *Backend) Import(ctx context.Context, items []backend.Item) error {
	for i := range items {
		if items[i].Key == nil {
			return trace.BadParameter("missing parameter key in item %v", i)
		}
	}
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT imported from meta LIMIT 1")
		if err != nil {
			return trace.Wrap(err)
		}
		var imported bool
		row := q.QueryRowContext(ctx)
		if err := row.Scan(&imported); err != nil {
			if err != sql.ErrNoRows {
				return trace.Wrap(err)
			}
		}
		if imported {
			return trace.AlreadyExists("database has been already imported")
		}

		if err := l.putRangeInTransaction(ctx, tx, items, true); err != nil {
			return trace.Wrap(err)
		}

		stmt, err := tx.PrepareContext(ctx, "INSERT INTO meta(version, imported) values(?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := stmt.ExecContext(ctx, schemaVersion, true); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PutRange puts range of items into backend (creates if items doe not
// exists, updates it otherwise)
func (l *Backend) PutRange(ctx context.Context, items []backend.Item) error {
	for i := range items {
		if items[i].Key == nil {
			return trace.BadParameter("missing parameter key in item %v", i)
		}
	}
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.putRangeInTransaction(ctx, tx, items, false)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (l *Backend) putRangeInTransaction(ctx context.Context, tx *sql.Tx, items []backend.Item, forceEventsOff bool) error {
	var eventsStmt *sql.Stmt
	var err error
	if !l.EventsOff {
		eventsStmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO kv(key, modified, expires, value) values(?, ?, ?, ?)")
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range items {
		created := l.clock.Now().UTC()
		recordID := id(created)
		if !l.Mirror {
			recordID = items[i].ID
		}
		if !l.EventsOff && !forceEventsOff {
			if _, err := eventsStmt.ExecContext(ctx, types.OpPut, created, string(items[i].Key), recordID, expires(items[i].Expires), items[i].Value); err != nil {
				return trace.Wrap(err)
			}
		}
		if _, err := stmt.ExecContext(ctx, string(items[i].Key), recordID, expires(items[i].Expires), items[i].Value); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Update updates value in the backend
func (l *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if i.Key == nil {
		return nil, trace.BadParameter("missing parameter key")
	}
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		created := l.clock.Now().UTC()
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET value = ?, expires = ?, modified = ? WHERE key = ?")
		if err != nil {
			return trace.Wrap(err)
		}
		result, err := stmt.ExecContext(ctx, i.Value, expires(i.Expires), id(created), string(i.Key))
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.NotFound("key %v is not found", string(i.Key))
		}
		if !l.EventsOff {
			stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := stmt.ExecContext(ctx, types.OpPut, created, string(i.Key), id(created), expires(i.Expires), i.Value); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.newLease(i), nil
}

// Get returns a single item or not found error
func (l *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	if len(key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	var item backend.Item
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.getInTransaction(ctx, key, tx, &item)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &item, nil
}

// getInTransaction returns an item, works in transaction
func (l *Backend) getInTransaction(ctx context.Context, key []byte, tx *sql.Tx, item *backend.Item) error {
	// When in mirror mode, don't set the current time so the SELECT query
	// returns expired items.
	var now time.Time
	if !l.Mirror {
		now = l.clock.Now().UTC()
	}

	q, err := tx.PrepareContext(ctx,
		"SELECT key, value, expires, modified FROM kv WHERE key = ? AND (expires IS NULL OR expires > ?) LIMIT 1")
	if err != nil {
		return trace.Wrap(err)
	}
	row := q.QueryRowContext(ctx, string(key), now)
	var expires NullTime
	if err := row.Scan(&item.Key, &item.Value, &expires, &item.ID); err != nil {
		if err == sql.ErrNoRows {
			return trace.NotFound("key %v is not found", string(key))
		}
		return trace.Wrap(err)
	}
	item.Expires = expires.Time
	return nil
}

// GetRange returns query range
func (l *Backend) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultLargeLimit
	}

	// When in mirror mode, don't set the current time so the SELECT query
	// returns expired items.
	var now time.Time
	if !l.Mirror {
		now = l.clock.Now().UTC()
	}

	var result backend.GetResult
	err := l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT key, value, expires, modified FROM kv WHERE (key >= ? and key <= ?) AND (expires is NULL or expires > ?) ORDER BY key LIMIT ?")
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := q.QueryContext(ctx, string(startKey), string(endKey), now, limit)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()

		for rows.Next() {
			var i backend.Item
			var expires NullTime
			if err := rows.Scan(&i.Key, &i.Value, &expires, &i.ID); err != nil {
				return trace.Wrap(err)
			}
			i.Expires = expires.Time
			result.Items = append(result.Items, i)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

// KeepAlive updates TTL on the lease
func (l *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if len(lease.Key) == 0 {
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
		if !l.EventsOff {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified, kv_expires, kv_value) values(?, ?, ?, ?, ?, ?)")
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := stmt.ExecContext(ctx, types.OpPut, created, string(item.Key), id(created), expires.UTC(), item.Value); err != nil {
				return trace.Wrap(err)
			}
		}
		stmt, err := tx.PrepareContext(ctx, "UPDATE kv SET expires = ?, modified = ? WHERE key = ?")
		if err != nil {
			return trace.Wrap(err)
		}
		result, err := stmt.ExecContext(ctx, expires.UTC(), id(now), string(lease.Key))
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return trace.Wrap(err)
		}
		if rows == 0 {
			return trace.NotFound("key %v is not found", string(lease.Key))
		}
		return nil
	})
}

func (l *Backend) deleteInTransaction(ctx context.Context, key []byte, tx *sql.Tx) error {
	stmt, err := tx.PrepareContext(ctx, "DELETE FROM kv WHERE key = ?")
	if err != nil {
		return trace.Wrap(err)
	}
	result, err := stmt.ExecContext(ctx, string(key))
	if err != nil {
		return trace.Wrap(err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return trace.Wrap(err)
	}
	if rows == 0 {
		return trace.NotFound("key %v is not found", string(key))
	}
	if !l.EventsOff {
		created := l.clock.Now().UTC()
		stmt, err = tx.PrepareContext(ctx, "INSERT INTO events(type, created, kv_key, kv_modified) values(?, ?, ?, ?)")
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := stmt.ExecContext(ctx, types.OpDelete, created, string(key), created.UnixNano()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (l *Backend) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		return l.deleteInTransaction(ctx, key, tx)
	})
}

// DeleteRange deletes range of items with keys between startKey and endKey
// Note that elements deleted by range do not produce any events
func (l *Backend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}
	return l.inTransaction(ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(ctx,
			"SELECT key FROM kv WHERE key >= ? and key <= ?")
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := q.QueryContext(ctx, string(startKey), string(endKey))
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()
		var keys [][]byte
		for rows.Next() {
			var key []byte
			if err := rows.Scan(&key); err != nil {
				return trace.Wrap(err)
			}
			keys = append(keys, key)
		}

		for i := range keys {
			if err := l.deleteInTransaction(l.ctx, keys[i], tx); err != nil {
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
	select {
	case <-l.watchStarted.Done():
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(ctx.Err(), "context is closing")
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
	l.buf.Reset()
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
			l.Warningf("SLOW TRANSACTION: %v, %v.", diff, string(debug.Stack()))
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
			l.Errorf("Unexpected panic in inTransaction: %v, trying to rollback.", r)
			err = trace.BadParameter("panic: %v", r)
			if e2 := rollback(); e2 != nil {
				l.Errorf("Failed to rollback: %v.", e2)
			}
			return
		}
		if err != nil && !trace.IsNotFound(err) {
			if isConstraintError(trace.Unwrap(err)) {
				err = trace.AlreadyExists(err.Error())
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
					l.Warningf("Unexpected error in inTransaction: %v, rolling back.", trace.DebugReport(err))
				}
				if e2 := rollback(); e2 != nil {
					l.Errorf("Failed to rollback too: %v.", e2)
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

func expires(t time.Time) interface{} {
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
	e, ok := trace.Unwrap(err).(sqlite3.Error)
	if !ok {
		return false
	}
	return e.Code == sqlite3.ErrConstraint
}

func isLockedError(err error) bool {
	e, ok := trace.Unwrap(err).(sqlite3.Error)
	if !ok {
		return false
	}
	return e.Code == sqlite3.ErrBusy
}

func isInterrupt(err error) bool {
	e, ok := trace.Unwrap(err).(sqlite3.Error)
	if !ok {
		return false
	}
	return e.Code == sqlite3.ErrInterrupt
}

func isReadonlyError(err error) bool {
	e, ok := trace.Unwrap(err).(sqlite3.Error)
	if !ok {
		return false
	}
	return e.Code == sqlite3.ErrReadonly
}

// NullTime represents a time.Time that may be null. NullTime implements the
// sql.Scanner interface so it can be used as a scan destination, similar to
// sql.NullString.
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}
