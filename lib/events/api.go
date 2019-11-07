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
	"math"
	"time"

	"github.com/gravitational/teleport/lib/session"
)

const (
	// EventType is event type/kind
	EventType = "event"
	// EventID is a unique event identifier
	EventID = "uid"
	// EventCode is a code that uniquely identifies a particular event type
	EventCode = "code"
	// EventTime is event time
	EventTime = "time"
	// EventLogin is OS login
	EventLogin = "login"
	// EventUser is teleport user name
	EventUser = "user"
	// EventProtocol specifies protocol that was captured
	EventProtocol = "proto"
	// EventProtocolsSSH specifies SSH as a type of captured protocol
	EventProtocolSSH = "ssh"
	// EventProtocolKube specifies kubernetes as a type of captured protocol
	EventProtocolKube = "kube"
	// LocalAddr is a target address on the host
	LocalAddr = "addr.local"
	// RemoteAddr is a client (user's) address
	RemoteAddr = "addr.remote"
	// EventCursor is an event ID (used as cursor value for enumeration, not stored)
	EventCursor = "id"

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
	// SessionUploadEvent indicates that session has been uploaded to the external storage
	SessionUploadEvent = "session.upload"
	// URL is used for a session upload URL
	URL = "url"

	SessionEventID  = "sid"
	SessionServerID = "server_id"

	// SessionByteOffset is the number of bytes written to session stream since
	// the beginning
	SessionByteOffset = "offset"

	// SessionJoinEvent indicates that someone joined a session
	SessionJoinEvent = "session.join"
	// SessionLeaveEvent indicates that someone left a session
	SessionLeaveEvent = "session.leave"

	// Data transfer events.
	SessionDataEvent = "session.data"
	DataTransmitted  = "tx"
	DataReceived     = "rx"

	// ClientDisconnectEvent is emitted when client is disconnected
	// by the server due to inactivity or any other reason
	ClientDisconnectEvent = "client.disconnect"

	// Reason is a field that specifies reason for event, e.g. in disconnect
	// event it explains why server disconnected the client
	Reason = "reason"

	// UserLoginEvent indicates that a user logged into web UI or via tsh
	UserLoginEvent = "user.login"
	// LoginMethod is the event field indicating how the login was performed
	LoginMethod = "method"
	// LoginMethodLocal represents login with username/password
	LoginMethodLocal = "local"
	// LoginMethodClientCert represents login with client certificate
	LoginMethodClientCert = "client.cert"
	// LoginMethodOIDC represents login with OIDC
	LoginMethodOIDC = "oidc"
	// LoginMethodSAML represents login with SAML
	LoginMethodSAML = "saml"
	// LoginMethodGithub represents login with Github
	LoginMethodGithub = "github"

	// UserUpdatedEvent is emitted when the user is created or updated (upsert).
	UserUpdatedEvent = "user.update"

	// UserDeleteEvent is emitted when the user is deleted.
	UserDeleteEvent = "user.delete"

	// UserExpires is when the user will expire.
	UserExpires = "expires"

	// UserRoles is a list of roles for the user.
	UserRoles = "roles"
	// IdentityAttributes is a map of user attributes
	// received from identity provider
	IdentityAttributes = "attributes"

	// UserConnector is the connector used to create the user.
	UserConnector = "connector"

	// AccessRequestCreateEvent is emitted when a new access request is created.
	AccessRequestCreateEvent = "access_request.create"
	// AccessRequestUpdateEvent is emitted when a request's state is updated.
	AccessRequestUpdateEvent = "access_request.update"
	// AccessRequestUpdateBy indicates the user that updated the request state.
	AccessRequestUpdateBy = "updated_by"
	// AccessRequestState is the state of a request.
	AccessRequestState = "state"
	// AccessRequestID is the ID of an access request.
	AccessRequestID = "id"

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
	SCPEvent          = "scp"
	SCPPath           = "path"
	SCPLengh          = "len"
	SCPAction         = "action"
	SCPActionUpload   = "upload"
	SCPActionDownload = "download"

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent  = "resize"
	TerminalSize = "size" // expressed as 'W:H'

	// SessionUploadIndex is a very large number of the event index
	// to indicate that this is the last event in the chain
	// used for the last event of the sesion - session upload
	SessionUploadIndex = math.MaxInt32
	// SessionDataIndex is a very large number of the event index
	// to indicate one of the last session events, used to report
	// data transfer
	SessionDataIndex = math.MaxInt32 - 1
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
	// V3 is almost like V2, but it assumes
	// that session recordings are being uploaded
	// at the end of the session, so it skips writing session event index
	// on the fly
	V3 = 3
)

// IAuditLog is the primary (and the only external-facing) interface for AuditLogger.
// If you wish to implement a different kind of logger (not filesystem-based), you
// have to implement this interface
type IAuditLog interface {
	// Closer releases connection and resources associated with log if any
	io.Closer

	// EmitAuditEvent emits audit event
	EmitAuditEvent(Event, EventFields) error

	// DELETE IN: 2.7.0
	// This method is no longer necessary as nodes and proxies >= 2.7.0
	// use UploadSessionRecording method.
	// PostSessionSlice sends chunks of recorded session to the event log
	PostSessionSlice(SessionSlice) error

	// UploadSessionRecording uploads session recording to the audit server
	UploadSessionRecording(r SessionRecording) error

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
	GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]EventFields, error)

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

// GetID returns the unique event ID
func (f EventFields) GetID() string {
	return f.GetString(EventID)
}

// GetCode returns the event code
func (f EventFields) GetCode() string {
	return f.GetString(EventCode)
}

// GetTimestamp returns the event timestamp (when it was emitted)
func (f EventFields) GetTimestamp() time.Time {
	return f.GetTime(EventTime)
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

// HasField returns true if the field exists in the event.
func (f EventFields) HasField(key string) bool {
	_, ok := f[key]
	return ok
}
