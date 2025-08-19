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
type ManagedConn[C conn] struct {
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
type conn interface {
	comparable
	// Close closes the connection.
	Close(context.Context) error
	// IsClosed returns true if the connection is closed.
	IsClosed() bool
}

// NewManagedConn creates a new managed connection.
func NewManagedConn[C conn](cfg *ManagedConnConfig, newFunc func(context.Context) (C, error)) (*ManagedConn[C], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	c := &ManagedConn[C]{
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
func Exec[C conn, R any](ctx context.Context, conn *ManagedConn[C], fn func(context.Context, C) (R, error)) (R, error) {
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

func (m *ManagedConn[C]) acquire(ctx context.Context, cancel context.CancelFunc) (C, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for m.cancelExec != nil && !m.closed {
		m.cond.Wait()
	}

	var zero C
	if m.closed {
		return zero, trace.Wrap(ErrConnClosed)
	}

	if m.active == zero || m.active.IsClosed() {
		var err error
		m.active, err = m.newFunc(ctx)
		if err != nil {
			return zero, trace.Wrap(err)
		}
	}

	m.cancelExec = cancel
	m.watchdog.Stop()
	return m.active, nil
}

func (m *ManagedConn[C]) release() {
	m.mu.Lock()
	m.cancelExec = nil
	m.watchdog.Reset(m.cfg.MaxIdleTime)
	m.mu.Unlock()
	m.cond.Broadcast()
}

func (m *ManagedConn[C]) closeActive(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var zero C
	if m.active == zero {
		return nil
	}

	m.cfg.Logger.DebugContext(ctx, "closing idle connection")
	if err := m.active.Close(ctx); err != nil {
		m.cfg.Logger.WarnContext(ctx, "error while closing connection", "error", err)
	}
	m.active = zero
	return nil
}

// Close closes the managed connection.
func (m *ManagedConn[C]) Close(ctx context.Context) error {
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

	var zero C
	if m.active == zero {
		return nil
	}

	return m.active.Close(ctx)
}
