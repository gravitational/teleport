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
	"time"

	"github.com/gravitational/trace"
)

func renderStatusText(w io.Writer, summary statusSummary) error {
	style := newTextStyle(w)
	now := summary.GeneratedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	fmt.Fprintf(w, "  %s\n", style.section(fmt.Sprintf("User Tasks [%d total, %d open, %d resolved]", summary.TotalTasks, summary.OpenTasks, summary.ResolvedTasks)))
	if summary.FilteredIntegration != "" {
		fmt.Fprintf(w, "  %s\n", style.info(fmt.Sprintf("Filtered by integration=%s", summary.FilteredIntegration)))
	}
	if len(summary.UserTasks) == 0 {
		fmt.Fprintf(w, "%s\n", style.warning("No user tasks for the selected filters."))
	} else {
		rows := make([][]string, 0, len(summary.UserTasks))
		for _, task := range summary.UserTasks {
			rows = append(rows, []string{
				task.Name,
				style.statusValue(task.State),
				friendlyTaskType(task.TaskType),
				task.IssueType,
				fmt.Sprintf("%d", task.Affected),
				displayIntegrationName(task.Integration),
				formatRelativeTime(task.LastStateChange, now),
			})
		}
		if err := renderTable(w, []string{"Name", "State", "TaskType", "IssueType", "Affected", "Integration", "Last State Change"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(summary.DiscoveryConfigs) > 0 {
		fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("Discovery Configs [%d total, %s]", summary.DiscoveryConfigCount, formatCountLabel(summary.DiscoveryGroupCount, "group", "groups"))))
		rows := make([][]string, 0, len(summary.DiscoveryConfigs))
		for _, cfg := range summary.DiscoveryConfigs {
			rows = append(rows, []string{
				cfg.Name,
				cfg.Group,
				style.statusValue(humanizeEnumValue(cfg.State)),
				cfg.Matchers,
				style.discoveredCount(cfg.Discovered),
				formatRelativeTime(cfg.LastSync, summary.GeneratedAt),
			})
		}
		if err := renderTable(w, []string{"Name", "Group", "State", "Matchers", "Discovered", "Last Sync"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(summary.Integrations) > 0 {
		activeCount := 0
		for _, item := range summary.Integrations {
			if item.Found > 0 || item.Enrolled > 0 || item.Failed > 0 {
				activeCount++
			}
		}
		fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("Integrations [%d total, %d active]",
			len(summary.Integrations), activeCount)))
		rows := make([][]string, 0, len(summary.Integrations))
		for _, item := range summary.Integrations {
			rows = append(rows, []string{
				item.Name,
				item.Type,
				fmt.Sprintf("%d", item.OpenTasks),
				style.discoveredCount(item.Found),
				style.pendingCount(item.AwaitingJoin),
				style.discoveredCount(item.Enrolled),
				style.failedCount(item.Failed),
			})
		}
		if err := renderTable(w, []string{"Name", "Type", "Open Tasks", "Found", "Awaiting Join", "Enrolled", "Failed"}, rows, style.tableWidth); err != nil {
			return trace.Wrap(err)
		}
	}

	if summary.SSMRunStats != nil || summary.JoinStats != nil {
		renderAuditEventStatsSection(w, style, "SSM Runs", summary.SSMRunStats, "--ssm-limit")
		renderAuditEventStatsSection(w, style, "Instance Joins", summary.JoinStats, "--join-limit")
	}

	if summary.CacheSummary != "" {
		fmt.Fprintf(w, "\n  %s %s\n", style.info("Cache:"), summary.CacheSummary)
	}

	return trace.Wrap(renderNextActions(w, style, statusNextActions(summary)))
}

func renderAuditEventStatsSection(w io.Writer, style textStyle, title string, stats *auditEventStats, limitFlag string) {
	if stats == nil {
		return
	}
	windowLabel := stats.Window
	if stats.LimitReached && stats.EffectiveWindow != "" {
		if stats.SuggestedLimit > 0 {
			windowLabel = fmt.Sprintf("requested %s, oldest returned %s; use %s=%d", stats.Window, stats.EffectiveWindow, limitFlag, stats.SuggestedLimit)
		} else {
			windowLabel = fmt.Sprintf("requested %s, oldest returned %s; increase %s", stats.Window, stats.EffectiveWindow, limitFlag)
		}
	} else if stats.LimitReached {
		windowLabel += fmt.Sprintf("; increase %s", limitFlag)
	}
	fmt.Fprintf(w, "\n  %s\n", style.section(fmt.Sprintf("%s (%s)", title, windowLabel)))

	details := []keyValue{
		{Key: "TOTAL EVENTS", Value: fmt.Sprintf("%d", stats.Total)},
		{Key: "SUCCESSFUL", Value: fmt.Sprintf("%d", stats.Success)},
		{Key: "FAILED", Value: style.failedCount(uint64(stats.Failed))},
		{Key: "DISTINCT HOSTS", Value: fmt.Sprintf("%d", stats.DistinctHosts)},
		{Key: "HOSTS WITH FAILURES", Value: style.failedCount(uint64(stats.FailingHosts))},
	}
	_ = style.indented().keyValues(w, details)
}

func statusNextActions(summary statusSummary) []nextAction {
	var actions []nextAction

	// Limit-reached warnings first — incomplete data is critical to surface.
	if summary.SSMRunStats != nil && summary.SSMRunStats.LimitReached {
		limit := suggestedOrFallbackLimit(summary.SSMRunStats.SuggestedLimit, 0)
		if limit > 0 {
			actions = append(actions, nextAction{
				Comment:  fmt.Sprintf("SSM run limit reached — rerun with --ssm-limit=%d to cover full window", limit),
				Commands: []string{fmt.Sprintf("tctl discovery status --ssm-limit=%d", limit)},
			})
		}
	}
	if summary.JoinStats != nil && summary.JoinStats.LimitReached {
		limit := suggestedOrFallbackLimit(summary.JoinStats.SuggestedLimit, 0)
		if limit > 0 {
			actions = append(actions, nextAction{
				Comment:  fmt.Sprintf("Join limit reached — rerun with --join-limit=%d to cover full window", limit),
				Commands: []string{fmt.Sprintf("tctl discovery status --join-limit=%d", limit)},
			})
		}
	}

	inspectCommands := []string{"tctl discovery tasks show <task-id-prefix>"}
	if len(summary.UserTasks) > 0 {
		inspectCommands = []string{
			fmt.Sprintf("tctl discovery tasks show %s", taskNamePrefix(summary.UserTasks[0].Name)),
			"tctl discovery tasks show <task-id-prefix>",
		}
	}
	actions = append(actions,
		nextAction{
			Comment: "List discovery tasks",
			Commands: []string{
				"tctl discovery tasks ls",
				"tctl discovery tasks ls --state=resolved",
			},
		},
		nextAction{
			Comment:  "Investigate particular open task",
			Commands: inspectCommands,
		},
		nextAction{
			Comment: "List integrations",
			Commands: []string{
				"tctl discovery integration ls",
			},
		},
		nextAction{
			Comment: "Check SSM runs (use --group to group similar runs)",
			Commands: []string{
				"tctl discovery ssm-runs ls",
				"tctl discovery ssm-runs ls --group",
				"tctl discovery ssm-runs ls --last=1h",
				"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
		nextAction{
			Comment: "Check instance joins",
			Commands: []string{
				"tctl discovery joins ls",
				"tctl discovery joins ls --last=1h",
			},
		},
		nextAction{
			Comment:  "View unified host inventory",
			Commands: []string{"tctl discovery inventory ls"},
		},
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery status --format=json",
				"tctl discovery status --format=yaml",
			},
		},
	)
	return actions
}
