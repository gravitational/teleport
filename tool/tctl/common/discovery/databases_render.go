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
	"cmp"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/usertasks"
	"github.com/gravitational/teleport/lib/utils"
)

func renderDatabases(w io.Writer, databases []databaseInfo, format string) error {
	switch format {
	case teleport.Text:
		return trace.Wrap(renderDatabasesText(w, databases))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(w, databases))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, databases))
	default:
		return trace.BadParameter("unknown format %q", format)
	}
}

// renderDatabasesText writes discovered databases, availability, and health.
func renderDatabasesText(w io.Writer, databaseInfos []databaseInfo) error {
	if len(databaseInfos) == 0 {
		_, err := fmt.Fprintln(w, "No databases found.")
		return trace.Wrap(err)
	}

	var (
		hasNoService bool
		hasUnhealthy bool
	)
	issueTypes := map[string]struct{}{}
	columns := []string{"Cloud", "Account", "Region", "Database", "Time", "Status", "Details"}
	table := asciitable.MakeTable(columns)
	for _, dbInfo := range databaseInfos {
		hasNoService = hasNoService || dbInfo.noService
		hasUnhealthy = hasUnhealthy || dbInfo.unhealthy
		for _, issueType := range dbInfo.userTaskIssueTypes {
			issueTypes[issueType] = struct{}{}
		}
		var timestamp string
		if !dbInfo.Time.IsZero() {
			timestamp = dbInfo.Time.UTC().Format(time.RFC3339)
		}
		table.AddRow([]string{
			dbInfo.Cloud,
			dbInfo.AccountID,
			dbInfo.Region,
			dbInfo.Name,
			timestamp,
			dbInfo.Status,
			dbInfo.textDetails(),
		})
	}

	if _, err := table.AsBuffer().WriteTo(w); err != nil {
		return trace.Wrap(err)
	}
	if !hasNoService && !hasUnhealthy && len(issueTypes) == 0 {
		return nil
	}
	// TODO(gavin): add docs links for each step
	if _, err := fmt.Fprintln(w, "\nNext steps"); err != nil {
		return trace.Wrap(err)
	}
	actionItems := []actionItem{
		{
			render: hasNoService,
			detail: databaseDetailNoService,
			description: `
Deploy a Teleport Database Service and ensure its db_service.resources label
selectors match the database's labels.`,
		},
		{
			render: hasUnhealthy,
			detail: databaseDetailUnhealthy,
			description: `
Inspect the health check failure in db_server status.target_health and the
Database Service logs. Ensure the Database Service can reach the database
endpoint over the network, including VPC routes and security group rules.`,
		},
	}

	for issueType := range issueTypes {
		title, description := usertasks.DescriptionForDiscoverRDSIssue(issueType)
		actionItems = append(actionItems, actionItem{
			render: true,
			detail: cmp.Or(title, issueType),
			description: cmp.Or(description, `
No troubleshooting guidance for this issue is available in this version of tctl.
`),
		})
	}

	for _, nextStep := range actionItems {
		if !nextStep.render {
			continue
		}
		detail := shortDatabaseDetail(nextStep.detail)
		desc := nextStep.formatText()
		if _, err := fmt.Fprintf(w, "\n%s\n%s\n", detail, desc); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type actionItem struct {
	// render controls whether the actionable is included in rendered output.
	render bool
	// detail is the detail title.
	detail string
	// description is the action that must be taken to resolve the issue.
	description string
}

// formatText formats the actionItem description for text output.
func (a actionItem) formatText() string {
	lines := strings.Split(strings.TrimSpace(a.description), "\n")
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			lines[i] = strings.Repeat(" ", 2) + line
		}
	}
	return strings.Join(lines, "\n")
}

// textDetails uses short issue labels in the table. The next steps section
// provides the troubleshooting guidance for these issues.
func (db databaseInfo) textDetails() string {
	details := make([]string, len(db.Details))
	for i, detail := range db.Details {
		details[i] = shortDatabaseDetail(detail)
	}
	return strings.Join(details, "; ")
}

// shortDatabaseDetail maps full details to the labels used in the text table
// and next-step headings. Details without a short label are preserved.
func shortDatabaseDetail(detail string) string {
	switch detail {
	case databaseDetailNoService:
		return "No Database Service"
	case databaseDetailUnhealthy:
		return "Unhealthy connection"
	default:
		return detail
	}
}
