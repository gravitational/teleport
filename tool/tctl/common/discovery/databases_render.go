// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/discoverydoctor"
	"github.com/gravitational/teleport/lib/utils"
)

func renderDatabases(w io.Writer, diagnostics discoverydoctor.DatabaseDiagnostics, format string) error {
	switch format {
	case teleport.Text:
		return trace.Wrap(renderDatabasesText(w, diagnostics))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(w, diagnostics))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, diagnostics))
	default:
		return trace.BadParameter("unknown format %q", format)
	}
}

// renderDatabasesText writes discovered databases, availability, and health.
func renderDatabasesText(w io.Writer, diagnostics discoverydoctor.DatabaseDiagnostics) error {
	if len(diagnostics) == 0 {
		_, err := fmt.Fprintln(w, "No databases found.")
		return trace.Wrap(err)
	}

	columns := []string{"Cloud", "Account", "Region", "Database", "Time", "Status", "Details"}
	table := asciitable.MakeTable(columns)
	for _, diagnostic := range diagnostics {
		var timestamp string
		if !diagnostic.Time.IsZero() {
			timestamp = diagnostic.Time.UTC().Format(time.RFC3339)
		}
		table.AddRow([]string{
			diagnostic.Cloud,
			diagnostic.AccountID,
			diagnostic.Region,
			diagnostic.Name,
			timestamp,
			diagnostic.Status,
			databaseTextDetails(diagnostic),
		})
	}

	if _, err := table.AsBuffer().WriteTo(w); err != nil {
		return trace.Wrap(err)
	}
	recommendations := diagnostics.Recommendations()
	if len(recommendations) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nNext steps"); err != nil {
		return trace.Wrap(err)
	}
	for _, recommendation := range recommendations {
		detail := shortDatabaseDetail(recommendation.Detail)
		desc := formatRecommendationDescription(recommendation.Description)
		if _, err := fmt.Fprintf(w, "\n%s\n%s\n", detail, desc); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// formatRecommendationDescription formats a recommendation description for text output.
func formatRecommendationDescription(description string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			lines[i] = strings.Repeat(" ", 2) + line
		}
	}
	return strings.Join(lines, "\n")
}

// databaseTextDetails uses short issue labels in the table. The next steps section
// provides the troubleshooting guidance for these issues.
func databaseTextDetails(diagnostic discoverydoctor.DatabaseDiagnostic) string {
	details := make([]string, len(diagnostic.Details))
	for i, detail := range diagnostic.Details {
		details[i] = shortDatabaseDetail(detail)
	}
	return strings.Join(details, "; ")
}

// shortDatabaseDetail maps full details to the labels used in the text table
// and next-step headings. Details without a short label are preserved.
func shortDatabaseDetail(detail string) string {
	switch detail {
	case discoverydoctor.DatabaseDetailNoService:
		return "No Database Service"
	case discoverydoctor.DatabaseDetailUnhealthy:
		return "Unhealthy connection"
	default:
		return detail
	}
}
