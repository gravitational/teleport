/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"encoding/json"
	"time"

	"github.com/gravitational/trace"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types/events"
)

const (
	// sessionEndType represents type name for session end event
	sessionEndType = "session.upload"
	// loginType represents type name for user login event
	loginType = "user.login"
)

// TeleportEvent represents helper struct around main audit log event
type TeleportEvent struct {
	// event is the event
	Event []byte
	// cursor is the event ID (real/generated when empty)
	ID string
	// cursor is the current cursor value
	Cursor string
	// Type is an event type
	Type string
	// Time is an event timestamp
	Time time.Time
	// Index is an event index within session
	Index int64
	// IsSessionEnd is true when this event is session.end
	IsSessionEnd bool
	// SessionID is the session ID this event belongs to
	SessionID string
	// IsFailedLogin is true when this event is the failed login event
	IsFailedLogin bool
	// FailedLoginData represents failed login user data
	FailedLoginData struct {
		// Login represents user login
		Login string
		// Login represents user name
		User string
		// Login represents cluster name
		ClusterName string
	}
}

// NewTeleportEvent creates TeleportEvent using AuditEvent as a source
func NewTeleportEvent(e *auditlogpb.EventUnstructured, cursor string) (*TeleportEvent, error) {
	payload, err := e.Unstructured.MarshalJSON()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	evt := &TeleportEvent{
		Cursor: cursor,
		Type:   e.GetType(),
		Time:   e.Time.AsTime(),
		Index:  e.GetIndex(),
		ID:     e.Id,
		Event:  payload,
	}

	switch e.GetType() {
	case sessionEndType:
		err = evt.setSessionID(e)
	case loginType:
		err = evt.setLoginData(e)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return evt, nil
}

// setSessionID sets session id for session end event
func (e *TeleportEvent) setSessionID(evt *auditlogpb.EventUnstructured) error {
	sessionUploadEvt := &events.SessionUpload{}
	if err := json.Unmarshal(e.Event, sessionUploadEvt); err != nil {
		return trace.Wrap(err)
	}

	sid := sessionUploadEvt.SessionID

	e.IsSessionEnd = true
	e.SessionID = sid
	return nil
}

// setLoginValues sets values related to login event
func (e *TeleportEvent) setLoginData(evt *auditlogpb.EventUnstructured) error {
	loginEvent := &events.UserLogin{}
	if err := json.Unmarshal(e.Event, loginEvent); err != nil {
		return trace.Wrap(err)
	}

	if loginEvent.Success {
		return nil
	}

	e.IsFailedLogin = true
	e.FailedLoginData.Login = loginEvent.Login
	e.FailedLoginData.User = loginEvent.User
	e.FailedLoginData.ClusterName = loginEvent.ClusterName
	return nil
}
