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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

func renderIntegrationListText(w io.Writer, items []integrationListItem) error {
	style := newTextStyle(w)

	fmt.Fprintf(w, "%s\n", style.section(fmt.Sprintf("Integrations [%d]", len(items))))
	if len(items) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No integrations found."))
		return trace.Wrap(renderNextActions(w, style, integrationListNextActions(items)))
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			item.Type,
			style.discoveredCount(item.Found),
			style.discoveredCount(item.Enrolled),
			style.failedCount(item.Failed),
			style.pendingCount(item.AwaitingJoin),
			fmt.Sprintf("%d", item.OpenTasks),
		})
	}
	if err := renderTable(w, []string{"Name", "Type", "Found", "Enrolled", "Failed", "Awaiting Join", "Open Tasks"}, rows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(renderNextActions(w, style, integrationListNextActions(items)))
}

func integrationListNextActions(items []integrationListItem) []nextAction {
	if len(items) == 0 {
		return []nextAction{
			{
				Comment:  "Check discovery status",
				Commands: []string{"tctl discovery status"},
			},
		}
	}

	actions := make([]nextAction, 0, 4)
	actions = append(actions, nextAction{
		Comment: "Inspect an integration",
		Commands: []string{
			fmt.Sprintf("tctl discovery integration show %s", items[0].Name),
			"tctl discovery integration show <name>",
		},
	})
	actions = append(actions, nextAction{
		Comment: "List discovery tasks",
		Commands: []string{
			"tctl discovery tasks ls",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "Check discovery status",
		Commands: []string{"tctl discovery status"},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			"tctl discovery integration ls --format=json",
			"tctl discovery integration ls --format=yaml",
		},
	})
	return actions
}

func renderIntegrationShowText(w io.Writer, detail integrationDetail) error {
	style := newTextStyle(w)
	now := time.Now().UTC()

	headerKVs := []keyValue{
		{Key: "NAME", Value: detail.Name},
		{Key: "TYPE", Value: detail.Type},
	}
	credKeys := mapKeys(detail.Credentials)
	slices.Sort(credKeys)
	for _, k := range credKeys {
		headerKVs = append(headerKVs, keyValue{Key: strings.ToUpper(k), Value: detail.Credentials[k]})
	}
	if err := style.keyValues(w, headerKVs); err != nil {
		return trace.Wrap(err)
	}

	if len(detail.ResourceTypeStats) > 0 {
		fmt.Fprintf(w, "\n%s\n", style.section("Resource Stats by Type"))
		rows := make([][]string, 0, len(detail.ResourceTypeStats))
		for _, rs := range detail.ResourceTypeStats {
			rows = append(rows, []string{
				rs.ResourceType,
				style.discoveredCount(rs.Found),
				style.discoveredCount(rs.Enrolled),
				style.failedCount(rs.Failed),
			})
		}
		if err := renderTable(w, []string{"Resource Type", "Found", "Enrolled", "Failed"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(detail.DiscoveryConfigs) > 0 {
		fmt.Fprintf(w, "\n%s\n", style.section("Discovery Configs"))
		rows := make([][]string, 0, len(detail.DiscoveryConfigs))
		for _, cfg := range detail.DiscoveryConfigs {
			rows = append(rows, []string{
				cfg.Name,
				style.statusValue(humanizeEnumValue(cfg.State)),
				cfg.Matchers,
				formatRelativeTime(cfg.LastSync, now),
			})
		}
		if err := renderTable(w, []string{"Name", "State", "Matchers", "Last Sync"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Open Tasks"))
	if len(detail.OpenTasks) == 0 {
		fmt.Fprintf(w, "%s\n", style.info("No open tasks for this integration."))
	} else {
		rows := make([][]string, 0, len(detail.OpenTasks))
		for _, task := range detail.OpenTasks {
			rows = append(rows, []string{
				task.Name,
				task.IssueType,
				fmt.Sprintf("%d", task.Affected),
			})
		}
		if err := renderTable(w, []string{"Task Name", "Issue Type", "Affected"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(renderNextActions(w, style, integrationShowNextActions(detail)))
}

func integrationShowNextActions(detail integrationDetail) []nextAction {
	actions := make([]nextAction, 0, 6)
	if len(detail.OpenTasks) > 0 {
		actions = append(actions, nextAction{
			Comment: "Inspect a task",
			Commands: []string{
				fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(detail.OpenTasks[0].Name)),
				"tctl discovery tasks show <task-id-prefix>",
			},
		})
	}
	actions = append(actions, nextAction{
		Comment: "List tasks for this integration",
		Commands: []string{
			fmt.Sprintf("tctl discovery tasks ls --integration=%s", detail.Name),
			fmt.Sprintf("tctl discovery tasks ls --integration=%s --state=resolved", detail.Name),
		},
	})
	actions = append(actions, nextAction{
		Comment: "Use machine-readable output",
		Commands: []string{
			fmt.Sprintf("tctl discovery integration show %s --format=json", detail.Name),
			fmt.Sprintf("tctl discovery integration show %s --format=yaml", detail.Name),
		},
	})
	actions = append(actions, nextAction{
		Comment: "Check SSM runs",
		Commands: []string{
			"tctl discovery ssm-runs ls",
			"tctl discovery ssm-runs ls --last=1h",
			"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
		},
	})
	actions = append(actions, nextAction{
		Comment: "Check instance joins",
		Commands: []string{
			"tctl discovery joins ls",
			"tctl discovery joins ls --last=1h",
		},
	})
	actions = append(actions, nextAction{
		Comment:  "Return to integration list",
		Commands: []string{"tctl discovery integration ls"},
	})
	actions = append(actions, nextAction{
		Comment:  "Check discovery status",
		Commands: []string{"tctl discovery status"},
	})
	return actions
}
