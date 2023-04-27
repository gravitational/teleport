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
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
)

// ExitCodeError wraps an exit code as an error.
type ExitCodeError struct {
	// Code is the exit code
	Code int
}

// Error implements the error interface.
func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// SessionsCollection is a collection of session end events.
type SessionsCollection struct {
	SessionEvents []events.AuditEvent
}

// WriteText writes the session collection as text to the provided io.Writer.
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

// WriteJSON writes the session collection as JSON to the provided io.Writer.
func (e *SessionsCollection) WriteJSON(w io.Writer) error {
	data, err := json.MarshalIndent(e.SessionEvents, "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

// WriteYAML writes the session collection as YAML to the provided io.Writer.
func (e *SessionsCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(w, e.SessionEvents)
}

// ShowSessions is a helper function for displaying listed sessions via tsh or tctl.
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

// ClusterAlertGetter manages getting cluster alerts.
type ClusterAlertGetter interface {
	GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error)
}

// ShowClusterAlerts shows cluster alerts with the given labels and severity.
func ShowClusterAlerts(ctx context.Context, client ClusterAlertGetter, w io.Writer, labels map[string]string, minSeverity, maxSeverity types.AlertSeverity) error {
	// get any "on login" alerts
	alerts, err := client.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels:   labels,
		Severity: minSeverity,
	})
	if err != nil && !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}

	types.SortClusterAlerts(alerts)
	var errs []error
	for _, alert := range alerts {
		if err := alert.CheckMessage(); err != nil {
			errs = append(errs, trace.Errorf("invalid alert %q: %w", alert.Metadata.Name, err))
			continue
		}
		if alert.Spec.Severity <= maxSeverity {
			fmt.Fprintf(w, "%s\n\n", utils.FormatAlert(alert))
		}
	}
	return trace.NewAggregate(errs...)
}
