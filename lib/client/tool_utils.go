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

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types/events"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// DefaultFormats is the default set of formats to use for commands that have the --format flag.
var DefaultFormats = []string{teleport.Text, teleport.JSON, teleport.YAML}

// FormatFlagDescription creates the description for the --format flag.
func FormatFlagDescription(formats ...string) string {
	return fmt.Sprintf("Format output (%s)", strings.Join(formats, ", "))
}

func DefaultSearchSessionRange(fromUTC, toUTC string) (from time.Time, to time.Time, err error) {
	from = time.Now().Add(time.Hour * -24)
	to = time.Now()
	if fromUTC != "" {
		from, err = time.Parse(time.RFC3339, fromUTC)
		if err != nil {
			return time.Time{}, time.Time{},
				trace.BadParameter("failed to parse session listing start time: expected format %s, got %s.", time.RFC3339, fromUTC)
		}
	}
	if toUTC != "" {
		to, err = time.Parse(time.RFC3339, toUTC)
		if err != nil {
			return time.Time{}, time.Time{},
				trace.BadParameter("failed to parse session listing end time: expected format %s, got %s.", time.RFC3339, toUTC)
		}
	}
	return from, to, nil
}

// GetPaginatedSessions wraps the given sessionGetter function with the required logic to use it's pagination. Returns up to 'max' sessions.
func GetPaginatedSessions(max int, sessionGetter func(startKey string) ([]apievents.AuditEvent, string, error)) ([]apievents.AuditEvent, error) {
	prevEventKey := ""
	sessions := []apievents.AuditEvent{}
	for {
		nextEvents, eventKey, err := sessionGetter(prevEventKey)
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
func ShowSessions(events []apievents.AuditEvent, format string, w io.Writer) error {
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
