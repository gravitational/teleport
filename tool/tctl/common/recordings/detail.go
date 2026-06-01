/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package recordings

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

const fieldLabelWidth = 20

// sanitize removes terminal control sequences from s before it is written to
// an operator's terminal. Call this on every string that originates from
// external data (session metadata, resource names, command summaries, etc.) to
// prevent terminal injection.
func sanitize(s string) string {
	return toolcommon.StripTerminalControlSequences(s)
}

// renderDetail builds the scrollable content string for the right-side detail
// pane. It is regenerated whenever the selected item or palette changes.
func renderDetail(s *sessionsearchv1pb.SessionSummary, p palette) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(p.section)
	faintStyle := lipgloss.NewStyle().Faint(true)

	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %s %s\n",
			faintStyle.Render(fmt.Sprintf("%-*s", fieldLabelWidth+1, label+":")),
			sanitize(value),
		)
	}

	section := func(title string) {
		fmt.Fprintf(&b, "%s\n", sectionStyle.Render("● "+title))
	}

	section("Session")
	field("ID", s.GetSessionId())
	field("Kind", s.GetKind())
	if ts := s.GetSessionStart(); ts != nil {
		field("Start", ts.AsTime().UTC().Format(time.RFC3339))
	}
	if end := s.GetSessionEnd(); end != nil {
		field("End", end.AsTime().UTC().Format(time.RFC3339))
		if start := s.GetSessionStart(); start != nil && !end.AsTime().IsZero() {
			field("Duration", end.AsTime().Sub(start.AsTime()).Round(time.Second).String())
		}
	}
	field("Severity", formatSeverityColored(s.GetSeverity()))
	b.WriteString("\n")

	section("User")
	field("Username", s.GetUsername())
	if roles := s.GetUserRoles(); len(roles) > 0 {
		field("Roles", strings.Join(roles, ", "))
	}
	if ids := s.GetAccessRequestIds(); len(ids) > 0 {
		field("Access Requests", strings.Join(ids, ", "))
	}
	if parts := s.GetParticipants(); len(parts) > 0 {
		field("Participants", strings.Join(parts, ", "))
	}
	b.WriteString("\n")

	section("Resource")
	field("Kind", s.GetResourceKind())
	field("Name", s.GetResourceName())
	field("ID", s.GetResourceId())
	field("Host ID", s.GetHostId())
	if labels := s.GetResourceLabels(); len(labels) > 0 {
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		indent := strings.Repeat(" ", fieldLabelWidth+4) // align with value column
		for i, k := range keys {
			if i == 0 {
				field("Labels", sanitize(k)+"="+sanitize(labels[k]))
			} else {
				fmt.Fprintf(&b, "%s%s\n", indent, sanitize(k)+"="+sanitize(labels[k]))
			}
		}
	}

	if props := s.GetResourceProperties(); props != nil {
		b.WriteString("\n")
		switch t := props.Type.(type) {
		case *sessionsearchv1pb.ResourceProperties_Ssh:
			section("SSH Properties")
			field("Hostname", t.Ssh.GetServerHostname())
			field("Address", t.Ssh.GetServerAddr())
		case *sessionsearchv1pb.ResourceProperties_Kubernetes:
			section("Kubernetes Properties")
			field("Namespace", t.Kubernetes.GetPodNamespace())
			field("Pod", t.Kubernetes.GetPodName())
		case *sessionsearchv1pb.ResourceProperties_Database:
			section("Database Properties")
			field("Database", t.Database.GetDatabaseName())
		}
	}

	return b.String()
}

// severityColor returns a terminal color for the given risk level.
func severityColor(level summarizerv1pb.RiskLevel) lipgloss.TerminalColor {
	switch level {
	case summarizerv1pb.RiskLevel_RISK_LEVEL_LOW:
		return lipgloss.Color("10") // bright green
	case summarizerv1pb.RiskLevel_RISK_LEVEL_MEDIUM:
		return lipgloss.Color("11") // bright yellow
	case summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH:
		return lipgloss.Color("208") // orange
	case summarizerv1pb.RiskLevel_RISK_LEVEL_CRITICAL:
		return lipgloss.Color("196") // bright red
	default:
		return lipgloss.NoColor{}
	}
}

// formatSeverity converts a RiskLevel to a short uppercase label.
func formatSeverity(level summarizerv1pb.RiskLevel) string {
	return strings.TrimPrefix(level.String(), "RISK_LEVEL_")
}

// formatSeverityColored returns the severity label with ANSI color applied.
func formatSeverityColored(level summarizerv1pb.RiskLevel) string {
	label := formatSeverity(level)
	c := severityColor(level)
	if _, ok := c.(lipgloss.NoColor); ok {
		return label
	}
	style := lipgloss.NewStyle().Foreground(c)
	switch level {
	case summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH,
		summarizerv1pb.RiskLevel_RISK_LEVEL_CRITICAL:
		style = style.Bold(true)
	}
	return style.Render(label)
}
