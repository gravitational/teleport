/*
Copyright 2015 Gravitational, Inc.

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

package events

import (
	"github.com/gravitational/teleport/lib/session"
	"io"
	"time"
)

const (
	// Common event fields:
	EventType  = "e"           // event type/kind
	EventTime  = "t"           // event time
	EventLogin = "login"       // OS login
	EventUser  = "user"        // teleport user
	LocalAddr  = "addr.local"  // address on the host
	RemoteAddr = "addr.remote" // client (user's) address

	// SessionPrintEvent event happens every time a write occurs to
	// temirnal I/O during a session
	SessionPrintEvent = "p"

	// SessionEventTimestamp is an offset (in seconds) since the beginning of the
	// session, when terminal IO event happened
	SessionEventTimestamp = "s"

	// SessionEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionStartEvent = "session.start"

	// SessionEndEvent indicates taht a session has ended
	SessionEndEvent = "session.end"
	SessionEventID  = "sid"

	// SessionEventBytes is the number of bytes written to session stream since
	// the beginning
	SessionEventBytes = "b"

	// Join & Leave events indicate when someone joins/leaves a session
	SessionJoinEvent  = "session.join"
	SessionLeaveEvent = "session.leave"

	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent        = "exec"
	ExecEventCommand = "command"
	ExecEventCode    = "exitCode"
	ExecEventError   = "exitError"

	// Port forwarding event
	PortForwardEvent = "port"
	PortForwardAddr  = "addr"

	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent   = "auth"
	AuthAttemptSuccess = "success"
	AuthAttemptErr     = "error"

	// SCPEvent means data transfer that occured on the server
	SCPEvent  = "scp"
	SCPPath   = "path"
	SCPLengh  = "len"
	SCPAction = "action"

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent = "resize"
	ResizeSize  = "size" // expressed as 'W:H'
)

// AuditLogI is the primary (and the only external-facing) interface for AUditLogger.
// If you wish to implement a different kind of logger (not filesystem-based), you
// have to implement this interface
type AuditLogI interface {
	EmitAuditEvent(eventType string, fields EventFields) error

	// GetSessionWriter returns a writer which SSH nodes use to submit
	// their live sessions into the session log
	GetSessionWriter(sid session.ID) (io.WriteCloser, error)

	// GetSessionReader returns a reader which can be used to read a byte stream
	// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
	// beginning)
	GetSessionReader(sid session.ID, offsetBytes int) (io.ReadCloser, error)

	// Returns all events that happen during a session sorted by time
	// (oldest first). Some events are "compressed" (like resize events or "session write"
	// events): if more than one of those happen within a second, only the last one
	// will be returned.
	//
	// This function is usually used in conjunction with GetSessionReader to
	// replay recorded session streams.
	GetSessionEvents(sid session.ID) ([]EventFields, error)
}

// EventFields instance is attached to every logged event
type EventFields map[string]interface{}

// GetString returns a string representation of a logged field
func (f EventFields) GetString(key string) string {
	val, found := f[key]
	if !found {
		return ""
	}
	v, _ := val.(string)
	return v
}

// GetString returns an int representation of a logged field
func (f EventFields) GetInt(key string) int {
	val, found := f[key]
	if !found {
		return 0
	}
	v, _ := val.(int)
	return v
}

// GetString returns an int representation of a logged field
func (f EventFields) GetTime(key string) time.Time {
	val, found := f[key]
	if !found {
		return time.Time{}
	}
	v, _ := val.(time.Time)
	return v
}
