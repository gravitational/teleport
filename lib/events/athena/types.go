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
	EventType string `parquet:"name=event_type, type=BYTE_ARRAY, convertedtype=UTF8"`
	// TODO(tobiaszheller): what precision of timestamp we want. AWS supports micros, maybe we can use it instead of mili?
	EventTime int64  `parquet:"name=event_time, type=INT64, convertedtype=TIMESTAMP_MILLIS"`
	UID       string `parquet:"name=uid, type=BYTE_ARRAY, convertedtype=UTF8"`
	SessionID string `parquet:"name=session_id, type=BYTE_ARRAY, convertedtype=UTF8"`
	User      string `parquet:"name=user, type=BYTE_ARRAY, convertedtype=UTF8"`
	EventData string `parquet:"name=event_data, type=BYTE_ARRAY, convertedtype=UTF8"`
}

func (e eventParquet) GetDate() string {
	return time.UnixMilli(e.EventTime).Format(time.DateOnly)
}

func auditEventToParquet(event apievents.AuditEvent) (*eventParquet, error) {
	jsonBlob, err := utils.FastMarshal(event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &eventParquet{
		EventType: event.GetType(),
		EventTime: event.GetTime().UnixMilli(),
		UID:       event.GetID(),
		SessionID: events.GetSessionID(event),
		User:      events.GetTeleportUser(event),
		EventData: string(jsonBlob),
	}, nil
}
