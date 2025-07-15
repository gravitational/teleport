// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mcp

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
)

// IdleConnectionsChecker is a helper used by database MCP servers to close idle
// connections. Servers must use it to initialize database connections and, once
// a connection becomes idle, it will automatically get closed.
//
// Note: The idle checker doesn't ensure connection single usage across
// goroutines. Meaning that retrieving connections for the same database
// identifier concurrently will return the same underlaying database connection.
type IdleConnectionsChecker[T any, C CloseableConn[T]] struct {
	mu sync.Mutex

	cancel            context.CancelFunc
	closed            atomic.Bool
	cfg               *IdleConnectionsCheckerConfig
	checkerInterval   time.Duration
	activeConnections map[string]*activeConn[T, C]
	newConnFunc       NewConnFunc[T, C]
}

// NewConnFunc defines a function to establish a new database connection.
type NewConnFunc[T any, C CloseableConn[T]] func(ctx context.Context, id string) (*T, error)

// CloseableConn represents an active database connection that defines a close
// function.
type CloseableConn[T any] interface {
	*T
	// Close closes the connection.
	Close(context.Context) error
}

// activeConn contains information about an active connection.
type activeConn[T any, C CloseableConn[T]] struct {
	dbConn       C
	lastActivity time.Time
}

// IdleConnectionsCheckerConfig are the idle checker configuration options.
type IdleConnectionsCheckerConfig struct {
	// MaxIdleTime represents the maximum idle time before the connection is
	// closed.
	MaxIdleTime time.Duration
	// Logger is the slog.Logger used by the idle checker.
	Logger *slog.Logger
}

// CheckAndSetDefaults checks and set default values for the config.
func (c *IdleConnectionsCheckerConfig) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// NewIdleConnectionsChecker initializes and starts a new connection idle
// checker.
func NewIdleConnectionsChecker[T any, C CloseableConn[T]](ctx context.Context, cfg *IdleConnectionsCheckerConfig, newConnFunc NewConnFunc[T, C]) (*IdleConnectionsChecker[T, C], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	i := &IdleConnectionsChecker[T, C]{
		cfg:               cfg,
		cancel:            cancel,
		activeConnections: make(map[string]*activeConn[T, C]),
		checkerInterval:   cfg.MaxIdleTime / 2,
		newConnFunc:       newConnFunc,
	}
	go i.idleChecker(ctx)
	return i, nil
}

// Get retrieves a connection for the database identifier. If the database has
// no active connection, it will establish a new one.
func (i *IdleConnectionsChecker[T, C]) Get(ctx context.Context, id string) (*T, error) {
	if i.closed.Load() {
		return nil, trace.AccessDenied("idle checker closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	conn, ok := i.activeConnections[id]
	if ok {
		conn.lastActivity = time.Now()
		return conn.dbConn, nil
	}

	dbConn, err := i.newConnFunc(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.activeConnections[id] = &activeConn[T, C]{dbConn: dbConn, lastActivity: time.Now()}
	return dbConn, nil
}

// Close closes the idle checker and all active connections.
func (i *IdleConnectionsChecker[T, C]) Close(ctx context.Context) error {
	if i.closed.Load() {
		return nil
	}

	i.cancel()
	i.closed.Store(true)

	i.mu.Lock()
	defer i.mu.Unlock()
	var errs []error
	for _, conn := range i.activeConnections {
		errs = append(errs, conn.dbConn.Close(ctx))
	}
	return trace.NewAggregate(errs...)
}

// idleChecker runs continuously checking if database connections are
// idle and closes them.
func (i *IdleConnectionsChecker[T, C]) idleChecker(ctx context.Context) {
	timer := time.NewTicker(i.checkerInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			i.cfg.Logger.DebugContext(ctx, "checking idle database connections")
			i.mu.Lock()
			maps.DeleteFunc(i.activeConnections, func(id string, conn *activeConn[T, C]) bool {
				if time.Now().Sub(conn.lastActivity) < i.cfg.MaxIdleTime {
					return false
				}

				err := conn.dbConn.Close(ctx)
				i.cfg.Logger.DebugContext(ctx, "closed idle database connection", "database", id, "error", err)
				return true
			})
			i.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
