/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package events contains event related types and logic required by the Teleport API.
package events

import (
	"context"
	"time"
)

// ProtoMarshaler implements marshaler interface
type ProtoMarshaler interface {
	// Size returns size of the object when marshaled
	Size() (n int)

	// MarshalTo marshals the object to sized buffer
	MarshalTo(dAtA []byte) (int, error)
}

// AuditEvent represents an audit event.
type AuditEvent interface {
	// ProtoMarshaler implements efficient
	// protobuf marshaling methods
	ProtoMarshaler

	// GetID returns unique event ID
	GetID() string
	// SetID sets unique event ID
	SetID(id string)

	// GetCode returns event short diagnostic code
	GetCode() string
	// SetCode sets unique event diagnostic code
	SetCode(string)

	// GetType returns event type
	GetType() string
	// SetType sets unique type
	SetType(string)

	// GetTime returns event time
	GetTime() time.Time
	// SetTime sets event time
	SetTime(time.Time)

	// GetIndex gets event index - a non-unique
	// monotonically incremented number
	// in the event sequence
	GetIndex() int64
	// SetIndex sets event index
	SetIndex(idx int64)

	// GetClusterName returns the name of the teleport cluster
	// as set on the event.
	GetClusterName() string
	// SetClusterName sets the name of the teleport cluster on the event.
	SetClusterName(string)

	// TrimToMaxSize returns a copy of the event trimmed to a smaller
	// size. The desired size may not be achievable so callers
	// should always check the size of the returned event before
	// using it. Generally fields that are unique to the event
	// will be trimmed, other fields are not currently modified
	// (ie *Metadata messages).
	TrimToMaxSize(maxSizeBytes int) AuditEvent
}

// Emitter emits audit events.
type Emitter interface {
	// EmitAuditEvent emits a single audit event.
	//
	// NOTE: when implementing this interface, the context should have
	// its cancel stripped via `context.WithoutCancel`
	EmitAuditEvent(context.Context, AuditEvent) error
}

// PreparedSessionEvent is an event that has been prepared by
// a [github.com/gravitational/teleport/lib/events.SessionEventPreparer].
// More specifically, it is a wrapper around an AuditEvent that signifies
// the event has been prepared and is ready to be recorded or emitted.
type PreparedSessionEvent interface {
	GetAuditEvent() AuditEvent
}

// Stream is used to create continuous ordered sequence of events
// associated with a session.
type Stream interface {
	// RecordEvent records a single session event if session recording is enabled.
	RecordEvent(ctx context.Context, event PreparedSessionEvent) error
	// Status returns channel broadcasting updates about the stream state:
	// last event index that was uploaded and the upload ID
	Status() <-chan StreamStatus
	// Done returns channel closed when streamer is closed
	// should be used to detect sending errors
	Done() <-chan struct{}
	// Complete closes the stream and marks it finalized,
	// releases associated resources, in case of failure,
	// closes this stream on the client side
	Complete(ctx context.Context) error
	// Close flushes non-uploaded flight stream data without marking
	// the stream completed and closes the stream instance
	Close(ctx context.Context) error
}
