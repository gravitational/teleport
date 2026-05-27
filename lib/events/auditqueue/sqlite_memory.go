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
	inner     *sqliteQueue
	id        string
	keepAlive *sql.Conn
}

// Ensure that we implement the interface Queue at compile time.
var _ Queue = (*sqliteInMemoryQueue)(nil)

func newSQLiteInMemoryQueue(cfg Config) (*sqliteInMemoryQueue, error) {
	id := uuid.NewString()
	db, err := sql.Open("sqlite", getInMemoryDSN(id, cfg.MaxBytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}
	if err := recordTeleportVersion(db, teleport.Version); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}

	keepAlive, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}

	inner, err := newBaseQueue(db, cfg)
	if err != nil {
		_ = keepAlive.Close()
		db.Close()
		return nil, trace.Wrap(err)
	}

	return &sqliteInMemoryQueue{
		inner:     inner,
		id:        id,
		keepAlive: keepAlive,
	}, nil
}

func (m *sqliteInMemoryQueue) Enqueue(ctx context.Context, event apievents.AuditEvent) error {
	return m.inner.Enqueue(ctx, event)
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

	m.inner.runPollLoop(ctx, handler)
	return nil
}

// Close shuts down the in-memory queue.
func (m *sqliteInMemoryQueue) Close() error {
	var err error
	m.inner.closeOnce.Do(func() {
		m.inner.cancel()
		m.inner.wg.Wait()
		err = errors.Join(m.keepAlive.Close(), m.inner.db.Close())
	})
	return trace.Wrap(err)
}
