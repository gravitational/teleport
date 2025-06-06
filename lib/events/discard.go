/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events

import (
	"github.com/gravitational/teleport/lib/eventsclient"
)

// DiscardAuditLog is do-nothing, discard-everything implementation
// of IAuditLog interface used for cases when audit is turned off
type DiscardAuditLog = eventsclient.DiscardAuditLog

// NewDiscardAuditLog returns a no-op audit log instance
func NewDiscardAuditLog() *DiscardAuditLog {
	return eventsclient.NewDiscardAuditLog()
}

// NewDiscardRecorder returns a [SessionRecorderChecker] that discards events.
func NewDiscardRecorder() *DiscardRecorder {
	return eventsclient.NewDiscardRecorder()
}

// DiscardRecorder returns a stream that discards all events
type DiscardRecorder = eventsclient.DiscardRecorder

// NewDiscardEmitter returns a no-op discard emitter
func NewDiscardEmitter() *DiscardEmitter {
	return eventsclient.NewDiscardEmitter()
}

// DiscardEmitter discards all events
type DiscardEmitter = eventsclient.DiscardEmitter

// NewDiscardStreamer returns a streamer that creates streams that
// discard events
func NewDiscardStreamer() *DiscardStreamer {
	return eventsclient.NewDiscardStreamer()
}

// DiscardStreamer creates DiscardRecorders
type DiscardStreamer = eventsclient.DiscardStreamer

// NoOpPreparer is a SessionEventPreparer that doesn't change events
type NoOpPreparer = eventsclient.NoOpPreparer

// WithNoOpPreparer wraps rec with a SessionEventPreparer that will leave
// events unchanged
func WithNoOpPreparer(rec SessionRecorder) SessionPreparerRecorder {
	return eventsclient.NewSessionPreparerRecorder(&NoOpPreparer{}, rec)
}
