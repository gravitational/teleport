/*
Copyright 2017 Gravitational, Inc.

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

package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/reporting"

	"github.com/gravitational/configure/jsonschema"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// Event defines an interface all event types should implement
type Event interface {
	// GetName returns the event name
	GetName() string
	// GetMetadata returns the event metadata
	GetMetadata() Metadata
	// SetAccountID sets the event account ID
	SetAccountID(string)
}

// Metadata represents event resource metadata
type Metadata struct {
	// Name is the event name
	Name string `json:"name"`
	// Created is the event creation timestamp
	Created time.Time `json:"created"`
}

// ServerEvent represents server-related event, such as "logged into server"
type ServerEvent struct {
	// Kind is resource kind, for events it is "event"
	Kind string `json:"kind"`
	// Version is the event resource version
	Version string `json:"version"`
	// Metadata is the event metadata
	Metadata Metadata `json:"metadata"`
	// Spec is the event spec
	Spec ServerEventSpec `json:"spec"`
}

// ServerEventSpec is server event specification
type ServerEventSpec struct {
	// ID is event ID, may be used for de-duplication
	ID string `json:"id"`
	// Action is event action, such as "login"
	Action string `json:"action"`
	// AccountID is ID of account that triggered the event
	AccountID string `json:"accountID"`
	// ServerID is anonymized ID of server that triggered the event
	ServerID string `json:"serverID"`
}

// NewServerLoginEvent creates an instance of "server login" event
func NewServerLoginEvent(serverID string) *ServerEvent {
	return &ServerEvent{
		Kind:    KindEvent,
		Version: ResourceVersion,
		Metadata: Metadata{
			Name:    EventTypeServer,
			Created: time.Now().UTC(),
		},
		Spec: ServerEventSpec{
			ID:       uuid.New(),
			Action:   EventActionLogin,
			ServerID: serverID,
		},
	}
}

// GetName returns the event name
func (e *ServerEvent) GetName() string { return e.Metadata.Name }

// GetMetadata returns the event metadata
func (e *ServerEvent) GetMetadata() Metadata { return e.Metadata }

// SetAccountID sets the event account ID
func (e *ServerEvent) SetAccountID(id string) {
	e.Spec.AccountID = id
}

// UserEvent represents user-related event, such as "user logged in"
type UserEvent struct {
	// Kind is resource kind, for events it is "event"
	Kind string `json:"kind"`
	// Version is the event resource version
	Version string `json:"version"`
	// Metadata is the event metadata
	Metadata Metadata `json:"metadata"`
	// Spec is the event spec
	Spec UserEventSpec `json:"spec"`
}

// UserEventSpec is user event specification
type UserEventSpec struct {
	// ID is event ID, may be used for de-duplication
	ID string `json:"id"`
	// Action is event action, such as "login"
	Action string `json:"action"`
	// AccountID is ID of account that triggered the event
	AccountID string `json:"accountID"`
	// UserID is anonymized ID of user that triggered the event
	UserID string `json:"userID"`
}

// NewUserLoginEvent creates an instance of "user login" event
func NewUserLoginEvent(userID string) *UserEvent {
	return &UserEvent{
		Kind:    KindEvent,
		Version: ResourceVersion,
		Metadata: Metadata{
			Name:    EventTypeUser,
			Created: time.Now().UTC(),
		},
		Spec: UserEventSpec{
			ID:     uuid.New(),
			Action: EventActionLogin,
			UserID: userID,
		},
	}
}

// GetName returns the event name
func (e *UserEvent) GetName() string { return e.Metadata.Name }

// GetMetadata returns the event metadata
func (e *UserEvent) GetMetadata() Metadata { return e.Metadata }

// SetAccountID sets the event account id
func (e *UserEvent) SetAccountID(id string) {
	e.Spec.AccountID = id
}

// ToGRPCEvent converts provided event to the format used by gRPC server/client
func ToGRPCEvent(event Event) (*reporting.GRPCEvent, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &reporting.GRPCEvent{
		Data: payload,
	}, nil
}

// FromGRPCEvent converts event from the format used by gRPC server/client
func FromGRPCEvent(grpcEvent reporting.GRPCEvent) (Event, error) {
	var header resourceHeader
	if err := json.Unmarshal(grpcEvent.Data, &header); err != nil {
		return nil, trace.Wrap(err)
	}
	if header.Kind != KindEvent {
		return nil, trace.BadParameter("expected kind %q, got %q",
			KindEvent, header.Kind)
	}
	if header.Version != ResourceVersion {
		return nil, trace.BadParameter("expected resource version %q, got %q",
			ResourceVersion, header.Version)
	}
	switch header.Metadata.Name {
	case EventTypeServer:
		var event ServerEvent
		err := unmarshalWithSchema(
			getServerEventSchema(), grpcEvent.Data, &event)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &event, nil
	case EventTypeUser:
		var event UserEvent
		err := unmarshalWithSchema(
			getUserEventSchema(), grpcEvent.Data, &event)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &event, nil
	default:
		return nil, trace.BadParameter("unknown event type %q", header.Metadata.Name)
	}
}

// FromGRPCEvents converts a series of events from the format used by gRPC server/client
func FromGRPCEvents(grpcEvents reporting.GRPCEvents) ([]Event, error) {
	var events []Event
	for _, grpcEvent := range grpcEvents.Events {
		event, err := FromGRPCEvent(*grpcEvent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, event)
	}
	return events, nil
}

// resourceHeader is used when unmarhsaling resources
type resourceHeader struct {
	// Kind the the resource kind
	Kind string `json:"kind"`
	// Version is the resource version
	Version string `json:"version"`
	// Metadata is the resource metadata
	Metadata Metadata `json:"metadata"`
}

// schemaTemplate is the event resource schema template
const schemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "version", "metadata", "spec"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v2"},
    "metadata": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "created"],
      "properties": {
        "name": {"type": "string"},
        "created": {"type": "string"}
      }
    },
    "spec": %v
  }
}`

// getServerEventSchema returns full server event JSON schema
func getServerEventSchema() string {
	return fmt.Sprintf(schemaTemplate, serverEventSchema)
}

// serverEventSchema is the server event spec schema
const serverEventSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["id", "action", "accountID", "serverID"],
  "properties": {
    "id": {"type": "string"},
    "action": {"type": "string"},
    "accountID": {"type": "string"},
    "serverID": {"type": "string"}
  }
}`

// getUserEventSchema returns full user event JSON schema
func getUserEventSchema() string {
	return fmt.Sprintf(schemaTemplate, userEventSchema)
}

// userEventSchema is the user event spec schema
const userEventSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["id", "action", "accountID", "userID"],
  "properties": {
    "id": {"type": "string"},
    "action": {"type": "string"},
    "accountID": {"type": "string"},
    "userID": {"type": "string"}
  }
}`

// unmarshalWithSchema unmarshals the provided data into the provided object
// using specified JSON schema
func unmarshalWithSchema(objectSchema string, data []byte, object interface{}) error {
	schema, err := jsonschema.New([]byte(objectSchema))
	if err != nil {
		return trace.Wrap(err)
	}
	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return trace.Wrap(err)
	}
	processed, err := schema.ProcessObject(raw)
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := json.Marshal(processed)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(bytes, object); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// marshalWithSchema marshals the provided objects while checking the specified schema
func marshalWithSchema(objectSchema string, object interface{}) ([]byte, error) {
	schema, err := jsonschema.New([]byte(objectSchema))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	processed, err := schema.ProcessObject(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := json.Marshal(processed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}
