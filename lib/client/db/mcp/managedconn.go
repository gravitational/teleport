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
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

var ErrConnClosed = errors.New("connection closed")

// ManagedConn manages a database connection, providing a way to keep a single
// active connection per database, and also closing the connection once it
// becomes inactive.
//
// Managed connections are always used with Exec function.
//
// We require two definitions of the same type (one regular and another as
// pointer) due to the necessity of having the `conn` interface definition on
// the types constraint. For callers that want to store the managed connections
// we can expect a definition like `ManagedConn[MyDbConn, *MyDbConn]`.
type ManagedConn[T any, C conn[T]] struct {
	mu   sync.Mutex
	cond *sync.Cond

	cfg      *ManagedConnConfig
	newFunc  func(context.Context) (C, error)
	active   C
	closed   bool
	watchdog *time.Timer

	// cancelExec will cancel currently executed query when invoked. It will
	// only be set when there is a running execution, so it can be also used to
	// indicate the connection is in use.
	cancelExec context.CancelFunc
}

// ManagedConnConfig represents a managed connection config.
type ManagedConnConfig struct {
	// MaxIdleTime is the max connection idle time before it gets closed
	MaxIdleTime time.Duration
	// Logger is the logger used by the managed connection.
	Logger *slog.Logger
}

// CheckAndSetDefaults checks and sets the defaults
func (c *ManagedConnConfig) CheckAndSetDefaults() error {
	if c.MaxIdleTime == 0 {
		return trace.BadParameter("max idle time must be greater than zero")
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// conn represents the database connection.
type conn[T any] interface {
	*T
	// Close closes the connection.
	Close(context.Context) error
	// IsClosed returns true if the connection is closed.
	IsClosed() bool
}

// NewManagedConn creates a new managed connection.
func NewManagedConn[T any, C conn[T]](cfg *ManagedConnConfig, newFunc func(context.Context) (C, error)) (*ManagedConn[T, C], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	c := &ManagedConn[T, C]{
		newFunc: newFunc,
		cfg:     cfg,
	}
	c.cond = sync.NewCond(&c.mu)
	c.watchdog = time.AfterFunc(cfg.MaxIdleTime, func() {
		c.closeActive(context.Background())
	})
	return c, nil
}

// Exec executes the provided function with an active connection.
//
// Execution result (`R`) can be anytype and is not managed or used by the
// managed connection.
//
// Note: There is no guarantee the connection still open. Callers should handle
// the scenarios where the connection was closed outside the managed connection.
// For example, if there is network interruption.
func Exec[T any, C conn[T], R any](ctx context.Context, conn *ManagedConn[T, C], fn func(context.Context, *T) (R, error)) (R, error) {
	var empty R
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	active, err := conn.acquire(ctx, cancel)
	if err != nil {
		return empty, trace.Wrap(err)
	}
	defer conn.release()

	return fn(ctx, active)
}

func (m *ManagedConn[T, C]) acquire(ctx context.Context, cancel context.CancelFunc) (*T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for m.cancelExec != nil && !m.closed {
		m.cond.Wait()
	}

	if m.closed {
		return nil, trace.Wrap(ErrConnClosed)
	}

	if m.active == nil || m.active.IsClosed() {
		var err error
		m.active, err = m.newFunc(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	m.cancelExec = cancel
	m.watchdog.Stop()
	return m.active, nil
}

func (m *ManagedConn[T, C]) release() {
	m.mu.Lock()
	m.cancelExec = nil
	m.watchdog.Reset(m.cfg.MaxIdleTime)
	m.mu.Unlock()
	m.cond.Broadcast()
}

func (m *ManagedConn[T, C]) closeActive(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return nil
	}

	m.cfg.Logger.DebugContext(ctx, "closing idle connection")
	if err := m.active.Close(ctx); err != nil {
		m.cfg.Logger.WarnContext(ctx, "error while closing connection", "error", err)
	}
	m.active = nil
	return nil
}

// Close closes the managed connection.
func (m *ManagedConn[T, C]) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.cancelExec != nil {
		m.cfg.Logger.DebugContext(ctx, "canceling active execution")
		m.cancelExec()
	}

	for m.cancelExec != nil {
		m.cond.Wait()
	}

	if m.active == nil {
		return nil
	}

	return m.active.Close(ctx)
}
