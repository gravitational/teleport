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
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/backend"
)

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

// Backend uses SQLite to implement storage interfaces
type Backend struct {
	Config
	*log.Entry
	db *sql.DB
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

// SetClock sets internal backend clock
func (l *Backend) SetClock(clock clockwork.Clock) {
	l.clock = clock
}

// Clock returns clock used by the backend
func (l *Backend) Clock() clockwork.Clock {
	return l.clock
}
