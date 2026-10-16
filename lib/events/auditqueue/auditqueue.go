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

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
)

const (
	// DefaultDeliveryTimeout is the default bound on a single invocation of the
	// delivery handler over one batch of events.
	DefaultDeliveryTimeout = apidefaults.DefaultIOTimeout

	// MaxPayloadBytes is used to enforce that events stay within the limit for
	// gRPC messages. It is the largest audit event that Enqueue will accept.
	// The minus 256k is to account for overhead such as age headers, protobuf
	// overhead, etc.
	MaxPayloadBytes = (4 * 1024 * 1024) - (256 * 1024)
)

var (
	// ErrQueueFull is returned by Enqueue when the queue has no room for more
	// events.
	ErrQueueFull = errors.New("audit queue is full")

	// ErrEventTooLarge is returned by Enqueue when an event's encoding exceeds
	// MaxPayloadBytes. The event is not stored if the caller gets this error.
	ErrEventTooLarge = errors.New("audit event is too large to be delivered")

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

// SynchronousMode controls the SQLite synchronous pragma.
type SynchronousMode string

const (
	// SynchronousNormal commits are durable against OS crashes but not against
	// power loss mid-write. This is the default for SQLite and is safe for most
	// use cases in WAL mode.
	SynchronousNormal SynchronousMode = "NORMAL"
	// SynchronousFull syncs to disk before each commit, providing full
	// durability at the cost of higher write latency.
	SynchronousFull SynchronousMode = "FULL"
)

// Config configures a Queue.
type Config struct {
	// Path is the path to the directory used by the audit log queue.
	Path string
	// OrphanScanInterval is how often the queue scans for orphaned audit log
	// queues.
	OrphanScanInterval time.Duration
	// MaxBytes sets the maximum database size
	MaxBytes int64
	// SoftLimit is the size of the audit log queue at which we start logging
	// warning messages.
	SoftLimit int64
	// MaxAttempts is the number of delivery failures before an event is moved
	// to the dead-letter queue. Defaults to 3 if unset.
	MaxAttempts int
	// DeliveryTimeout bounds a single invocation of the delivery handler over
	// one batch of events. Defaults to DefaultDeliveryTimeout if unset.
	DeliveryTimeout time.Duration
	// DeadLetterSweepInterval is how often the dead-letter sweeper re-attempts
	// delivery of failed events.
	DeadLetterSweepInterval time.Duration
	// DeadLetterTTL is the maximum age of a dead-letter event before it is
	// permanently deleted.
	DeadLetterTTL time.Duration
	// Synchronous controls the SQLite synchronous pragma.
	Synchronous SynchronousMode
	// WriteLinger bounds how long the writer waits for additional events before
	// committing a batch. Defaults to defaultWriteLinger if unset. A negative
	// value disables lingering.
	WriteLinger time.Duration
	// Sealer, when set, encrypts each batch payload before it is written to
	// disk. The queue borrows the Sealer. It is closed by the emitter that
	// owns it, never by the queue.
	Sealer         Sealer
	StatsInterval  time.Duration
	OnStatsUpdated func()
}

const (
	// FormatPlaintext marks a row whose payload is an AuditQueueBatch
	// message.
	FormatPlaintext = 0
	// FormatAgeV1 marks a row whose payload is an age encrypted
	// AuditQueueBatch message. Such a row can be opened by the process that
	// wrote it by using the identity that exists only in that process's memory,
	// or by the auth server, which holds the cluster private keys.
	FormatAgeV1 = 1
)

// Item is a batch of events yielded to a Handler.
type Item struct {
	id int64
	// Events is the decoded batch. It is populated only for plaintext items.
	Events []apievents.AuditEvent
	// Payload is the raw sealed batch. It is populated only for sealed items.
	// This process will decode only the payloads that were encrypted with this
	// process's identity. I.e., orphans will not be decoded by the agent and
	// will be forwarded to the auth server for decoding.
	Payload []byte
	// Format identifies the payload encoding (FormatPlaintext or FormatAgeV1).
	Format int
	// EventCount is the number of events in the batch, valid for both
	// plaintext and sealed items.
	EventCount int
}

// Sealed reports whether the item carries an encrypted payload instead of
// decoded events.
func (i Item) Sealed() bool { return i.Format != FormatPlaintext }

// Stats reports the current depth of a Queue.
type Stats struct {
	// PendingCount is the number of events waiting in the main queue.
	PendingCount int64
	// DeadLetterCount is the number of events in the dead-letter queue.
	DeadLetterCount int64
	// CorruptCount is the number of events quarantined because their payloads
	// failed to deserialize.
	CorruptCount int64
	// OldestPendingTime is when the oldest event in the main queue was
	// enqueued. Zero when the main queue is empty.
	OldestPendingTime time.Time
	// OldestDeadLetterTime is when the oldest event in the dead-letter queue
	// failed. Zero when the dead-letter queue is empty.
	OldestDeadLetterTime time.Time
}

// Sealer encrypts audit event payloads before they are written to disk.
type Sealer interface {
	// Seal encrypts the plaintext and returns cyphertext if and only if
	// encryption is enabled, typically by setting
	// auth_service.session_recording_config.encryption.enabled to true.
	// If encryption is enabled, then the sealed return value will be true.
	// It will be false otherwise.
	Seal(ctx context.Context, plaintext []byte) (payload []byte, sealed bool, err error)
	// Close closes the Sealer
	Close() error
}

// Handler is the function type that the caller of the auditqueue implements.
// It will take a batch of items to forward to the inner EmitAuditEvent.
// It will return the slice of items that were successfully delivered.
//
// Handler may be invoked concurrently from multiple goroutines.
// Implementations are responsible for their own goroutine safety.
type Handler func(ctx context.Context, items []Item) ([]Item, error)

// Queue is a queue for audit log events.
type Queue interface {
	// Enqueue "enqueues" an audit log event to the queue. A `nil` error return
	// indicates that the event has been durably written.
	Enqueue(event apievents.AuditEvent) error
	// Run is the single consumer that drains the queue and forwards the audit
	// log events to `handler`.
	Run(ctx context.Context, handler Handler) error
	// Drain blocks until the queue is drained. It makes a best-effort attempt
	// to forward all pending events to the consumer before shutdown. It stops
	// adopting new orphans and waits for both the main queue and the
	// dead-letter queue to empty or for ctx to be done, whichever comes first.
	// Callers must still call Close.
	Drain(ctx context.Context) error
	// Stats reports the current depth of the queue (pending and dead-letter
	// counts).
	Stats(ctx context.Context) (Stats, error)
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
