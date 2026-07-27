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

package servicecfg

import (
	"time"

	"github.com/gravitational/teleport/lib/events/auditqueue"
)

// AuditQueueConfig is used to control the audit log event queue.
type AuditQueueConfig struct {
	// SoftLimit is the database file size when we start warning.
	SoftLimit int64
	// HardLimit is the database file size when we start dropping events.
	MaxBytes int64
	// MaxAttempts is the maximum number of times we retry sending an event
	// before moving it to the dead-letter queue.
	MaxAttempts int
	// DeadLetterTTL is the time to live for an event in the dead-letter queue
	// before we delete it.
	DeadLetterTTL time.Duration
	// DeadLetterSweepInterval is how often the dead-letter sweeper attempts to
	// redeliver failed events.
	DeadLetterSweepInterval time.Duration
	// OrphanScanInterval is how often the process scans for orphaned queues.
	OrphanScanInterval time.Duration
	// Backend is the ordered list of preferred audit queue backends.
	Backends []string
	// Synchronous controls the SQLite synchronous pragma.
	Synchronous auditqueue.SynchronousMode
}
