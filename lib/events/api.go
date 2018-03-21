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
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/teleport/lib/session"
)

const (
	// Common event fields:
	EventType   = "event"       // event type/kind
	EventTime   = "time"        // event time
	EventLogin  = "login"       // OS login
	EventUser   = "user"        // teleport user
	LocalAddr   = "addr.local"  // address on the host
	RemoteAddr  = "addr.remote" // client (user's) address
	EventCursor = "id"          // event ID (used as cursor value for enumeration, not stored)

	// EventIndex is an event index as received from the logging server
	EventIndex = "ei"

	// EventNamespace is a namespace of the session event
	EventNamespace = "namespace"

	// SessionPrintEvent event happens every time a write occurs to
	// temirnal I/O during a session
	SessionPrintEvent = "print"

	// SessionPrintEventBytes says how many bytes have been written into the session
	// during "print" event
	SessionPrintEventBytes = "bytes"

	// SessionEventTimestamp is an offset (in milliseconds) since the beginning of the
	// session when the terminal IO event happened
	SessionEventTimestamp = "ms"

	// SessionEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionStartEvent = "session.start"

	// SessionEndEvent indicates that a session has ended
	SessionEndEvent = "session.end"
	SessionEventID  = "sid"
	SessionServerID = "server_id"

	// SessionByteOffset is the number of bytes written to session stream since
	// the beginning
	SessionByteOffset = "offset"

	// SessionJoinEvent indicates that someone joined a session
	SessionJoinEvent = "session.join"
	// SessionLeaveEvent indicates that someone left a session
	SessionLeaveEvent = "session.leave"

	// UserLoginEvent indicates that a user logged into web UI or via tsh
	UserLoginEvent = "user.login"
	// LoginMethod is the event field indicating how the login was performed
	LoginMethod = "method"
	// LoginMethodLocal represents login with username/password
	LoginMethodLocal = "local"
	// LoginMethodOIDC represents login with OIDC
	LoginMethodOIDC = "oidc"
	// LoginMethodSAML represents login with SAML
	LoginMethodSAML = "saml"
	// LoginMethodGithub represents login with Github
	LoginMethodGithub = "github"

	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent        = "exec"
	ExecEventCommand = "command"
	ExecEventCode    = "exitCode"
	ExecEventError   = "exitError"

	// SubsystemEvent is the result of the execution of a subsystem.
	SubsystemEvent = "subsystem"
	SubsystemName  = "name"
	SubsystemError = "exitError"

	// Port forwarding event
	PortForwardEvent   = "port"
	PortForwardAddr    = "addr"
	PortForwardSuccess = "success"
	PortForwardErr     = "error"

	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent   = "auth"
	AuthAttemptSuccess = "success"
	AuthAttemptErr     = "error"
	AuthAttemptMessage = "message"

	// SCPEvent means data transfer that occurred on the server
	SCPEvent  = "scp"
	SCPPath   = "path"
	SCPLengh  = "len"
	SCPAction = "action"

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent  = "resize"
	TerminalSize = "size" // expressed as 'W:H'
)

const (
	// MaxChunkBytes defines the maximum size of a session stream chunk that
	// can be requested via AuditLog.GetSessionChunk(). Set to 5MB
	MaxChunkBytes = 1024 * 1024 * 5
)

const (
	// V1 is the V1 version of slice chunks API,
	// it is 0 because it was not defined before
	V1 = 0
	// V2 is the V2 version of slice chunks  API
	V2 = 2
)

// IAuditLog is the primary (and the only external-facing) interface for AuditLogger.
// If you wish to implement a different kind of logger (not filesystem-based), you
// have to implement this interface
type IAuditLog interface {
	// Closer releases connection and resources associated with log if any
	io.Closer

	// EmitAuditEvent emits audit event
	EmitAuditEvent(eventType string, fields EventFields) error

	// PostSessionSlice sends chunks of recorded session to the event log
	PostSessionSlice(SessionSlice) error

	// PostSessionChunk returns a writer which SSH nodes use to submit
	// their live sessions into the session log
	PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error

	// GetSessionChunk returns a reader which can be used to read a byte stream
	// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
	// beginning) up to maxBytes bytes.
	//
	// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
	GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error)

	// Returns all events that happen during a session sorted by time
	// (oldest first).
	//
	// after tells to use only return events after a specified cursor Id
	//
	// This function is usually used in conjunction with GetSessionReader to
	// replay recorded session streams.
	GetSessionEvents(namespace string, sid session.ID, after int) ([]EventFields, error)

	// SearchEvents is a flexible way to find events. The format of a query string
	// depends on the implementing backend. A recommended format is urlencoded
	// (good enough for Lucene/Solr)
	//
	// Pagination is also defined via backend-specific query format.
	//
	// The only mandatory requirement is a date range (UTC). Results must always
	// show up sorted by date (newest first)
	SearchEvents(fromUTC, toUTC time.Time, query string, limit int) ([]EventFields, error)

	// SearchSessionEvents returns session related events only. This is used to
	// find completed session.
	SearchSessionEvents(fromUTC time.Time, toUTC time.Time, limit int) ([]EventFields, error)

	// WaitForDelivery waits for resources to be released and outstanding requests to
	// complete after calling Close method
	WaitForDelivery(context.Context) error
}

// EventFields instance is attached to every logged event
type EventFields map[string]interface{}

// String returns a string representation of an event structure
func (f EventFields) AsString() string {
	return fmt.Sprintf("%s: login=%s, id=%v, bytes=%v",
		f.GetString(EventType),
		f.GetString(EventLogin),
		f.GetInt(EventCursor),
		f.GetInt(SessionPrintEventBytes))
}

// GetType returns the type (string) of the event
func (f EventFields) GetType() string {
	return f.GetString(EventType)
}

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
	v, ok := val.(int)
	if !ok {
		f, ok := val.(float64)
		if ok {
			v = int(f)
		}
	}
	return v
}

// GetString returns an int representation of a logged field
func (f EventFields) GetTime(key string) time.Time {
	val, found := f[key]
	if !found {
		return time.Time{}
	}
	v, ok := val.(time.Time)
	if !ok {
		s := f.GetString(key)
		v, _ = time.Parse(time.RFC3339, s)
	}
	return v
}
