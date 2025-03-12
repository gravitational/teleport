/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/asciitable"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	t := asciitable.MakeTable([]string{"ID", "Type", "Participants", "Target", "Timestamp"})
	for _, event := range e.SessionEvents {
		var id, typ, participants, target, timestamp string

		switch session := event.(type) {
		case *events.SessionEnd:
			id = session.GetSessionID()
			typ = session.Protocol
			participants = strings.Join(session.Participants, ", ")
			timestamp = session.GetTime().Format(constants.HumanDateFormatSeconds)

			target = session.ServerHostname
			if typ == libevents.EventProtocolKube {
				target = session.KubernetesCluster
			}

		case *events.WindowsDesktopSessionEnd:
			id = session.GetSessionID()
			typ = "windows"
			participants = strings.Join(session.Participants, ", ")
			target = session.DesktopName
			timestamp = session.GetTime().Format(constants.HumanDateFormatSeconds)
		case *events.DatabaseSessionEnd:
			id = session.GetSessionID()
			typ = session.DatabaseProtocol
			participants = session.GetUser()
			target = session.DatabaseName
			timestamp = session.GetTime().Format(constants.HumanDateFormatSeconds)
		default:
			slog.WarnContext(context.Background(), "unsupported event type: expected SessionEnd, WindowsDesktopSessionEnd or DatabaseSessionEnd", "event_type", logutils.TypeAttr(event))
			continue
		}

		t.AddRow([]string{id, typ, participants, target, timestamp})
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

// ShowClusterAlerts shows cluster alerts matching the given labels and minimum severity.
func ShowClusterAlerts(ctx context.Context, client ClusterAlertGetter, w io.Writer, labels map[string]string, severity types.AlertSeverity) error {
	// get any "on login" alerts
	alerts, err := client.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels:   labels,
		Severity: severity,
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
		fmt.Fprintf(w, "%s\n\n", utils.FormatAlert(alert))
	}
	return trace.NewAggregate(errs...)
}

// FormatLabels filters out Teleport namespaced (teleport.[dev|hidden|internal])
// labels in non-verbose mode, groups the labels by namespace, sorts each
// group, then re-combines the groups and returns the result as a comma
// separated string.
func FormatLabels(labels map[string]string, verbose bool) string {
	var (
		teleportNamespaced []string
		namespaced         []string
		result             []string
	)
	for key, val := range labels {
		if strings.HasPrefix(key, types.TeleportNamespace+"/") ||
			strings.HasPrefix(key, types.TeleportHiddenLabelPrefix) ||
			strings.HasPrefix(key, types.TeleportInternalLabelPrefix) {
			// remove teleport.[dev|hidden|internal] labels in non-verbose mode.
			if verbose {
				teleportNamespaced = append(teleportNamespaced, fmt.Sprintf("%s=%s", key, val))
			}
			continue
		}
		if strings.Contains(key, "/") {
			namespaced = append(namespaced, fmt.Sprintf("%s=%s", key, val))
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", key, val))
	}
	sort.Strings(result)
	sort.Strings(namespaced)
	sort.Strings(teleportNamespaced)
	namespaced = append(namespaced, teleportNamespaced...)
	return strings.Join(append(result, namespaced...), ",")
}

// FormatResourceName returns the resource's name or its name as originally
// discovered in the cloud by the Teleport Discovery Service.
// In verbose mode, it always returns the resource name.
// In non-verbose mode, if the resource came from discovery and has the
// discovered name label, it returns the discovered name.
func FormatResourceName(r types.ResourceWithLabels, verbose bool) string {
	if !verbose {
		// return the (shorter) discovered name in non-verbose mode.
		discoveredName, ok := r.GetAllLabels()[types.DiscoveredNameLabel]
		if ok && discoveredName != "" {
			return discoveredName
		}
	}
	return r.GetName()
}
