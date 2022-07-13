/*
Copyright 2016-2022 Gravitational, Inc.

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

package common

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// GetPaginatedSessions grabs up to 'max' sessions.
func GetPaginatedSessions(ctx context.Context, fromUTC, toUTC time.Time, pageSize int, order types.EventOrder, max int, authClient auth.ClientI) ([]events.AuditEvent, error) {
	prevEventKey := ""
	sessions := []events.AuditEvent{}
	for {
		nextEvents, eventKey, err := authClient.SearchSessionEvents(fromUTC, toUTC,
			pageSize, order, prevEventKey, nil /* where condition */, "" /* session ID */)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sessions = append(sessions, nextEvents...)
		if eventKey == "" || len(sessions) > max {
			break
		}
		prevEventKey = eventKey
	}
	if max < len(sessions) {
		return sessions[:max], nil
	}
	return sessions, nil
}

type SessionsCollection struct {
	SessionEvents []events.AuditEvent
}

func (e *SessionsCollection) WriteText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"ID", "Type", "Participants", "Hostname", "Timestamp"})
	for _, event := range e.SessionEvents {
		session, ok := event.(*events.SessionEnd)
		if !ok {
			log.Warn(trace.BadParameter("unsupported event type: expected SessionEnd: got: %T", event))
			continue
		}
		t.AddRow([]string{
			session.GetSessionID(),
			session.Protocol,
			strings.Join(session.Participants, ", "),
			session.ServerHostname,
			session.GetTime().Format(constants.HumanDateFormatSeconds),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (e *SessionsCollection) WriteJSON(w io.Writer) error {
	data, err := json.MarshalIndent(e.SessionEvents, "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (e *SessionsCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(w, e.SessionEvents)
}

// ShowSessions is s helper function for displaying listed sessions via tsh or tctl
func ShowSessions(events []events.AuditEvent, format string, w io.Writer) error {
	sessions := &SessionsCollection{SessionEvents: events}
	switch format {
	case teleport.Text, "":
		return trace.Wrap(sessions.WriteText(w))
	case teleport.YAML:
		return trace.Wrap(sessions.WriteYAML(w))
	case teleport.JSON:
		return trace.Wrap(sessions.WriteJSON(w))
	default:
		return trace.BadParameter("unknown format %q", format)
	}
}
