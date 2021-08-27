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

	"github.com/gravitational/trace"
)

// Heartbeat represents a heartbeat that is sent from control plane to teleport
type Heartbeat struct {
	// Kind is resource kind, for heartbeat it is "heartbeat"
	Kind string `json:"kind"`
	// Version is the heartbeat resource version
	Version string `json:"version"`
	// Metadata is the heartbeat metadata
	Metadata Metadata `json:"metadata"`
	// Spec is the heartbeat spec
	Spec HeartbeatSpec `json:"spec"`
}

// HeartbeatSpec is the heartbeat resource spec
type HeartbeatSpec struct {
	// Notifications is a list of notifications sent with the heartbeat
	Notifications []Notification `json:"notifications,omitempty"`
}

// Notification represents a user notification message
type Notification struct {
	// Type is the notification type
	Type string `json:"type"`
	// Severity is the notification severity: info, warning or error
	Severity string `json:"severity"`
	// Text is the notification plain text
	Text string `json:"text"`
	// HTML is the notification HTML
	HTML string `json:"html"`
}

// NewHeartbeat returns a new heartbeat
func NewHeartbeat(notifications ...Notification) *Heartbeat {
	return &Heartbeat{
		Kind:    KindHeartbeat,
		Version: ResourceVersion,
		Metadata: Metadata{
			Name:    "heartbeat",
			Created: time.Now().UTC(),
		},
		Spec: HeartbeatSpec{
			Notifications: notifications,
		},
	}
}

// GetName returns the resource name
func (h *Heartbeat) GetName() string { return h.Metadata.Name }

// GetMetadata returns the heartbeat metadata
func (h *Heartbeat) GetMetadata() Metadata { return h.Metadata }

// UnmarshalHeartbeat unmarshals heartbeat with schema validation
func UnmarshalHeartbeat(bytes []byte) (*Heartbeat, error) {
	var header resourceHeader
	if err := json.Unmarshal(bytes, &header); err != nil {
		return nil, trace.Wrap(err)
	}
	if header.Kind != KindHeartbeat {
		return nil, trace.BadParameter("expected kind %q, got %q",
			KindHeartbeat, header.Kind)
	}
	if header.Version != ResourceVersion {
		return nil, trace.BadParameter("expected resource version %q, got %q",
			ResourceVersion, header.Version)
	}
	var heartbeat Heartbeat
	err := unmarshalWithSchema(
		getHeartbeatSchema(), bytes, &heartbeat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &heartbeat, nil
}

// MarshalHeartbeat marshals heartbeat with schema validation
func MarshalHeartbeat(h Heartbeat) ([]byte, error) {
	bytes, err := marshalWithSchema(getHeartbeatSchema(), h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// heartbeatSchema is the heartbeat spec schema
const heartbeatSchema = `{
  "type": "object",
  "properties": {
    "notifications": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "severity", "text", "html"],
        "additionalProperties": false,
        "properties": {
          "type": {"type": "string"},
          "severity": {"type": "string"},
          "text": {"type": "string"},
          "html": {"type": "string"}
        }
      }
    }
  }
}`

// getHeartbeatSchema returns the full heartbeat resource schema
func getHeartbeatSchema() string {
	return fmt.Sprintf(schemaTemplate, heartbeatSchema)
}
