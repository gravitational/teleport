// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package athena

import (
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

type eventParquet struct {
	EventType string    `parquet:"event_type"`
	EventTime time.Time `parquet:"event_time,timestamp(millisecond)"`
	UID       string    `parquet:"uid"`
	SessionID string    `parquet:"session_id"`
	User      string    `parquet:"user"`
	EventData string    `parquet:"event_data"`
}

func auditEventToParquet(event apievents.AuditEvent) (*eventParquet, error) {
	jsonBlob, err := utils.FastMarshal(event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &eventParquet{
		EventType: event.GetType(),
		EventTime: event.GetTime().UTC(),
		UID:       event.GetID(),
		SessionID: events.GetSessionID(event),
		User:      events.GetTeleportUser(event),
		EventData: string(jsonBlob),
	}, nil
}
