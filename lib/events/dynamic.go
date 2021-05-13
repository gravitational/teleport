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

package events

import (
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"encoding/json"
)

// FromEventFields converts from the typed dynamic representation
// to the new typed interface-style representation.
//
// This is mainly used to convert from the backend format used by
// our various event backends.
func FromEventFields(fields EventFields) (AuditEvent, error) {
	panic("unimplemented")
}

// GetSessionID pulls the session ID from the events that have a
// SessionMetadata. For other events an empty string is returned.
func GetSessionID(event AuditEvent) (string, error) {
	fields, err := ToEventFields(event)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fields.GetString(SessionEventID), nil
}

// ToEventFields converts from the typed interface-style event representation
// to the old dynamic map style representation in order to provide outer compatability
// with existing public API routes when the backend is updated with the typed events.
func ToEventFields(event AuditEvent) (EventFields, error) {
	encoded, err := utils.FastMarshal(event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var fields EventFields
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, trace.BadParameter("failed to unmarshal event %v", err)
	}

	return fields, nil
}
