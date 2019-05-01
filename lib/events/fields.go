/*
Copyright 2019 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
)

// UpdateEventFields updates passed event fields with additional information
// common for all event types such as unique IDs, timestamps, codes, etc.
//
// This method is a "final stop" for various audit log implementations for
// updating event fields before it gets persisted in the backend.
func UpdateEventFields(event Event, fields EventFields, clock clockwork.Clock, uid utils.UID) (err error) {
	additionalFields := make(map[string]interface{})
	if fields.GetType() == "" {
		additionalFields[EventType] = event.Name
	}
	if fields.GetID() == "" {
		additionalFields[EventID] = uid.New()
	}
	if fields.GetTimestamp().IsZero() {
		additionalFields[EventTime] = clock.Now().UTC().Round(time.Second)
	}
	if event.Code != "" {
		additionalFields[EventCode] = event.Code
	}
	for k, v := range additionalFields {
		fields[k] = v
	}
	return nil
}
