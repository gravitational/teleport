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

// ConnectionsManager is a helper used by database MCP servers to manage
// connections. Servers can use it to initialize database connections and, once
// a connection becomes idle, it will automatically get closed.
type ConnectionsManager[T any, C CloseableDBConn[T]] struct {
	mu sync.Mutex

	cancel            context.CancelFunc
	closed            atomic.Bool
	cfg               *ConnectionsManagerConfig
	checkerInterval   time.Duration
	activeConnections map[string]*activeConn[T, C]
	newConnFunc       NewConnFunc[T, C]
}

// NewConnFunc defines a function to establish a new database connection.
type NewConnFunc[T any, C CloseableDBConn[T]] func(ctx context.Context, id string) (*T, error)

// CloseableDBConn represents an active database connection that defines a close
// function.
type CloseableDBConn[T any] interface {
	*T
	// Close closes the connection.
	Close(context.Context) error
}

// ManagedConn is a wrapper returned to the caller around an active connection.
type ManagedConn[T any, C CloseableDBConn[T]] struct {
	activeConn *activeConn[T, C]
	released   atomic.Bool
}

// Conn returns the database connection.
func (c *ManagedConn[T, C]) Conn() *T {
	if c.released.Load() {
		return nil
	}

	return c.activeConn.dbConn
}

// Release releases the managed connection, so it can be reused.
func (c *ManagedConn[T, C]) Release() {
	c.activeConn.mu.Lock()
	defer c.activeConn.mu.Unlock()
	c.activeConn.inUse = false
	c.activeConn.lastActivity = time.Now()
	c.activeConn.cond.Broadcast()
}

func makeManagedConn[T any, C CloseableDBConn[T]](c *activeConn[T, C]) *ManagedConn[T, C] {
	return &ManagedConn[T, C]{activeConn: c}
}

// activeConn contains information about an active connection.
type activeConn[T any, C CloseableDBConn[T]] struct {
	mu   sync.Mutex
	cond sync.Cond

	inUse        bool
	dbConn       C
	lastActivity time.Time
}

func (c *activeConn[T, C]) close(ctx context.Context) error {
	c.acquire()
	return c.dbConn.Close(ctx)
}

func (c *activeConn[T, C]) acquire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.inUse {
		c.cond.Wait()
	}

	c.inUse = true
}

func (c *activeConn[T, C]) isInUse() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inUse
}

func (c *activeConn[T, C]) getLastActivity() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastActivity
}

// ConnectionsManagerConfig are the idle checker configuration options.
type ConnectionsManagerConfig struct {
	// MaxIdleTime represents the maximum idle time before the connection is
	// closed.
	MaxIdleTime time.Duration
	// Logger is the slog.Logger used by the idle checker.
	Logger *slog.Logger
}

// CheckAndSetDefaults checks and set default values for the config.
func (c *ConnectionsManagerConfig) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// NewConnectionsManager initializes and starts a new connection idle
// checker.
func NewConnectionsManager[T any, C CloseableDBConn[T]](ctx context.Context, cfg *ConnectionsManagerConfig, newConnFunc NewConnFunc[T, C]) (*ConnectionsManager[T, C], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	i := &ConnectionsManager[T, C]{
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
func (i *ConnectionsManager[T, C]) Get(ctx context.Context, id string) (*ManagedConn[T, C], error) {
	if i.closed.Load() {
		return nil, trace.AccessDenied("manager is closed")
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	conn, ok := i.activeConnections[id]
	if ok {
		conn.acquire()
		return makeManagedConn(conn), nil
	}

	dbConn, err := i.newConnFunc(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn = &activeConn[T, C]{
		dbConn:       dbConn,
		inUse:        true,
		lastActivity: time.Now(),
	}
	conn.cond = *sync.NewCond(&conn.mu)
	i.activeConnections[id] = conn
	return makeManagedConn(conn), nil
}

// Close closes the idle checker and all active connections.
func (i *ConnectionsManager[T, C]) Close(ctx context.Context) error {
	if i.closed.Load() {
		return nil
	}

	i.cancel()
	i.closed.Store(true)

	i.mu.Lock()
	defer i.mu.Unlock()
	var errs []error
	for _, conn := range i.activeConnections {
		errs = append(errs, conn.close(ctx))
	}
	return trace.NewAggregate(errs...)
}

// idleChecker runs continuously checking if database connections are
// idle and closes them.
func (i *ConnectionsManager[T, C]) idleChecker(ctx context.Context) {
	timer := time.NewTicker(i.checkerInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			i.cfg.Logger.DebugContext(ctx, "checking idle database connections")
			i.mu.Lock()
			maps.DeleteFunc(i.activeConnections, func(id string, conn *activeConn[T, C]) bool {
				if conn.isInUse() || time.Since(conn.getLastActivity()) < i.cfg.MaxIdleTime {
					return false
				}

				err := conn.close(ctx)
				i.cfg.Logger.DebugContext(ctx, "closed idle database connection", "database", id, "error", err)
				return true
			})
			i.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
