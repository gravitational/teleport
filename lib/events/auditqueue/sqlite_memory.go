/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package auditqueue

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
)

const (
	defaultInMemoryMaxBytes = 100 * 1024 * 1024 // 100 MiB
)

func getInMemoryDSN(name string, maxBytes int64) string {
	if maxBytes <= 0 {
		maxBytes = defaultInMemoryMaxBytes
	}
	params := url.Values{}
	params.Add("mode", "memory")
	params.Add("cache", "shared")
	addSharedParams(params, maxBytes)
	u := url.URL{
		Scheme:   "file",
		OmitHost: true,
		Path:     name,
		RawQuery: params.Encode(),
	}
	return u.String()
}

// sqliteInMemoryQueue is a fully in-memory SQLite database. No filesystem
// access needed.
type sqliteInMemoryQueue struct {
	inner       *sqliteQueue
	id          string
	keepAliveDB *sql.DB
	keepAlive   *sql.Conn
}

// Ensure that we implement the interface Queue at compile time.
var _ Queue = (*sqliteInMemoryQueue)(nil)

func newSQLiteInMemoryQueue(cfg Config) (*sqliteInMemoryQueue, error) {
	id := uuid.NewString()
	dsn := getInMemoryDSN(id, cfg.MaxBytes)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}
	if err := recordTeleportVersion(db, teleport.Version); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}

	// The shared in-memory database is destroyed when its last connection
	// closes, so keep one connection pinned for the queue's lifetime.
	keepAliveDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}
	keepAliveDB.SetMaxOpenConns(1)
	keepAlive, err := keepAliveDB.Conn(context.Background())
	if err != nil {
		keepAliveDB.Close()
		db.Close()
		return nil, trace.Wrap(err)
	}

	inner, err := newBaseQueue(db, cfg)
	if err != nil {
		_ = keepAlive.Close()
		keepAliveDB.Close()
		db.Close()
		return nil, trace.Wrap(err)
	}

	return &sqliteInMemoryQueue{
		inner:       inner,
		id:          id,
		keepAliveDB: keepAliveDB,
		keepAlive:   keepAlive,
	}, nil
}

func (m *sqliteInMemoryQueue) Enqueue(event apievents.AuditEvent) error {
	return m.inner.Enqueue(event)
}

// Run drains the in-memory queue.
func (m *sqliteInMemoryQueue) Run(ctx context.Context, handler Handler) error {
	if !m.inner.runMu.TryLock() {
		return trace.Wrap(ErrAlreadyRunning)
	}
	defer m.inner.runMu.Unlock()

	var wg sync.WaitGroup
	wg.Go(func() { m.inner.deadLetterSweepLoop(ctx, handler) })
	defer wg.Wait()

	return m.inner.runPollLoop(ctx, handler)
}

// Close shuts down the in-memory queue.
func (m *sqliteInMemoryQueue) Close() error {
	var err error
	m.inner.closeOnce.Do(func() {
		m.inner.cancel()
		m.inner.wg.Wait()
		err = errors.Join(m.inner.db.Close(), m.keepAlive.Close(), m.keepAliveDB.Close())
	})
	return trace.Wrap(err)
}
