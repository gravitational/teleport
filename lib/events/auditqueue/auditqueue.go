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

// Package auditqueue provides a queue for audit log events.
package auditqueue

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

var (
	// ErrQueueFull is returned by Enqueue when the queue has no room for more
	// events.
	ErrQueueFull = errors.New("audit queue is full")

	// ErrAlreadyRunning is returned by Run when another goroutine is already
	// consuming the queue. Run is single consumer.
	ErrAlreadyRunning = errors.New("audit queue is already being consumed")

	// ErrClosed is returned by Enqueue after Close has been called.
	ErrClosed = errors.New("audit queue is closed")
)

// Kind identifies a Queue implementation.
type Kind string

const (
	// KindSQLiteDisk selects the SQLite-backed on-disk implementation.
	KindSQLiteDisk Kind = "sqlite_disk"
	// KindSQLiteMemory selects the SQLite-backed in-memory implementation.
	KindSQLiteMemory Kind = "sqlite_memory"
)

// Config configures a Queue.
type Config struct {
	// Path is the path to the directory used by the audit log queue.
	Path string
	// OrphanScanInterval is how often the queue scans for orphaned audit log
	// queues.
	OrphanScanInterval time.Duration
	// MaxBytes sets the maximum database file size
	MaxBytes int64
	// SoftLimit is the size of the audit log queue at which we start logging
	// warning messages.
	SoftLimit int64
	// MaxAttempts is the number of delivery failures before an event is moved
	// to the dead-letter queue.
	MaxAttempts int
	// DeadLetterSweepInterval is how often the dead-letter sweeper re-attempts
	// delivery of failed events.
	DeadLetterSweepInterval time.Duration
	// DeadLetterTTL is the maximum age of a dead-letter event before it is
	// permanently deleted.
	DeadLetterTTL time.Duration
}

// Item is an event yielded to a Handler.
type Item struct {
	ID    int64
	Event apievents.AuditEvent
}

// Handler is the function type that the caller of the auditqueue implements.
// It will take a batch of items to forward to the inner EmitAuditEvent.
// It will return the slice of items that were successfully delivered.
//
// Handler may be invoked concurrently from multiple goroutines.
// Implementations are responsible for their own goroutine safety.
type Handler func(ctx context.Context, items []Item) []Item

// Queue is a queue for audit log events.
type Queue interface {
	// Enqueue "enqueues" an audit log event to the queue. A `nil` error return
	// indicates that the event has been durably written.
	Enqueue(ctx context.Context, event apievents.AuditEvent) error
	// Run is the single consumer that drains the queue and forwards the audit
	// log events to `handler`.
	Run(ctx context.Context, handler Handler) error
	// Close releases resources held by the queue.
	Close() error
}

// New constructs a Queue of the given kind.
func New(kind Kind, cfg Config) (Queue, error) {
	switch kind {
	case KindSQLiteDisk:
		return newSQLiteQueue(cfg)
	case KindSQLiteMemory:
		return newSQLiteInMemoryQueue(cfg)
	default:
		return nil, trace.BadParameter("unknown audit queue kind: '%s'", kind)
	}
}
