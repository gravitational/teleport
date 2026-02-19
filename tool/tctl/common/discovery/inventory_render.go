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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

func renderInventoryText(w io.Writer, output inventoryOutput, hostIDFilter string, showAll bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()

	// Summary
	windowDesc := output.Window + " (audit events); nodes: current"
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			windowDesc += fmt.Sprintf(" [limit: %d/type, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			windowDesc += fmt.Sprintf(" [limit: %d/type, use --limit to increase]", output.FetchLimit)
		}
	}
	var summaryRows [][]string
	if hostIDFilter != "" {
		summaryRows = [][]string{
			{"Query window", windowDesc},
		}
	} else {
		summaryRows = [][]string{
			{"Query window", windowDesc},
			{"Hosts", fmt.Sprintf("%d total, %s online, %s offline, %s failed",
				output.TotalHosts,
				style.good(strconv.Itoa(output.OnlineHosts)),
				style.pendingCount(uint64(output.OfflineHosts)),
				style.failedCount(uint64(output.FailedHosts)))},
			{"Displayed", fmt.Sprintf("%d (range %d-%d)", output.HostPage.End-output.HostPage.Start, output.HostPage.Start, output.HostPage.End)},
		}
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	// Collect host sections. When --group-by-account is used, Hosts is nil
	// and HostsByAccount is populated instead; render each account as a section.
	type invSection struct {
		account string
		hosts   []inventoryHost
	}
	var sections []invSection
	if len(output.HostsByAccount) > 0 {
		accounts := mapKeys(output.HostsByAccount)
		slices.Sort(accounts)
		for _, acct := range accounts {
			sections = append(sections, invSection{account: acct, hosts: output.HostsByAccount[acct]})
		}
	} else {
		sections = []invSection{{hosts: output.Hosts}}
	}

	allHosts := output.Hosts
	if allHosts == nil {
		for _, s := range sections {
			allHosts = append(allHosts, s.hosts...)
		}
	}

	if len(allHosts) == 0 {
		if hostIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No data found for host %s in the selected window.", hostIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No hosts found for the selected window."))
		}
		return nil
	}

	globalIndex := output.HostPage.Start
	for _, section := range sections {
		if hostIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.section("Host:"))
		} else if section.account != "" {
			fmt.Fprintf(w, "\n%s\n", style.section(fmt.Sprintf("Account %s (%d hosts):", section.account, len(section.hosts))))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.section("Hosts (sorted by most recent activity):"))
		}

		for i, host := range section.hosts {
			if i > 0 {
				fmt.Fprintln(w, "")
			}
			details := []keyValue{
				{Key: "HOST", Value: host.DisplayID},
				{Key: "STATE", Value: style.inventoryStateValue(host.State)},
				{Key: "NODE NAME", Value: cmp.Or(host.NodeName, placeholderNone)},
				{Key: "NODE UUID", Value: host.HostID},
				{Key: "INSTANCE ID", Value: cmp.Or(host.InstanceID, placeholderNone)},
				{Key: "REGION", Value: cmp.Or(host.Region, placeholderNone)},
				{Key: "METHOD", Value: cmp.Or(host.Method, placeholderNone)},
				{Key: "LAST SSM RUN", Value: inventoryTimeValue(host.LastSSMRun, now)},
				{Key: "LAST JOIN", Value: inventoryTimeValue(host.LastJoin, now)},
				{Key: "LAST SEEN", Value: inventoryLastSeenValue(host, now)},
				{Key: "SSM RUNS", Value: fmt.Sprintf("%d (%d success, %d failed)", host.SSMRuns, host.SSMSuccess, host.SSMFailed)},
				{Key: "JOINS", Value: fmt.Sprintf("%d (%d success, %d failed)", host.Joins, host.JoinSuccess, host.JoinFailed)},
			}
			if section.account == "" {
				details = append(details, keyValue{Key: "ACCOUNT ID", Value: cmp.Or(host.AccountID, placeholderNone)})
			}
			if err := style.numberedBlock(w, globalIndex, details); err != nil {
				return trace.Wrap(err)
			}
			globalIndex++

			// In show view, render the timeline
			if hostIDFilter != "" {
				if err := renderInventoryTimeline(w, style, host, now, showAll); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}

	// Pagination
	if output.HostPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("  ... %d more hosts available", output.HostPage.Remaining)))
	}

	// Next actions
	actions := inventoryNextActions(output, hostIDFilter, showAll, baseCommand)
	return trace.Wrap(renderNextActions(w, style, actions))
}

func inventoryTimeValue(t time.Time, now time.Time) string {
	if t.IsZero() {
		return placeholderNone
	}
	return formatRelativeTime(t, now)
}

func inventoryLastSeenValue(host inventoryHost, now time.Time) string {
	if host.IsOnline {
		return "now (online)"
	}
	if host.LastSeen.IsZero() {
		return "never"
	}
	return formatRelativeTime(host.LastSeen, now)
}

func (s textStyle) inventoryStateValue(state inventoryHostState) string {
	switch state {
	case inventoryStateOnline, inventoryStateJoinedOnly:
		return s.good(string(state))
	case inventoryStateJoinFailed, inventoryStateSSMFailed:
		return s.bad(string(state))
	case inventoryStateOffline, inventoryStateSSMAttempted:
		return s.warning(string(state))
	default:
		return string(state)
	}
}

type timelineEntry struct {
	Time   time.Time
	Kind   string // "JOIN" or "SSM RUN"
	Status string
	Detail string
	Error  string // join error message (displayed in red)
	Output string // SSM run stdout/stderr (displayed with > quotes)
}

func buildTimeline(host inventoryHost) []timelineEntry {
	entries := make([]timelineEntry, 0, len(host.SSMRecords)+len(host.JoinRecords))

	for _, r := range host.JoinRecords {
		errMsg := ""
		if isJoinFailure(r) {
			errMsg = r.Error
		}
		detail := strings.TrimSpace(r.Method)
		if r.Role != "" {
			detail += "  " + r.Role + " role"
		}
		entries = append(entries, timelineEntry{
			Time:   r.parsedEventTime,
			Kind:   "JOIN",
			Status: r.Code,
			Detail: detail,
			Error:  errMsg,
		})
	}

	for _, r := range host.SSMRecords {
		output := ""
		if isSSMRunFailure(r) {
			output = combineOutput(r.Stdout, r.Stderr)
		}
		var detailParts []string
		if r.ExitCode != "" {
			detailParts = append(detailParts, "exit="+r.ExitCode)
		}
		if r.CommandID != "" {
			detailParts = append(detailParts, r.CommandID)
		}
		detail := strings.Join(detailParts, "  ")
		entries = append(entries, timelineEntry{
			Time:   r.parsedEventTime,
			Kind:   "SSM RUN",
			Status: cmp.Or(r.Status, r.Code),
			Detail: detail,
			Output: output,
		})
	}

	slices.SortFunc(entries, func(a, b timelineEntry) int {
		return compareTimeDesc(a.Time, b.Time)
	})

	return entries
}

func renderInventoryTimeline(w io.Writer, style textStyle, host inventoryHost, now time.Time, showAll bool) error {
	entries := buildTimeline(host)
	if len(entries) == 0 {
		return nil
	}

	displayEntries := entries
	if !showAll && len(displayEntries) > 10 {
		displayEntries = displayEntries[:10]
	}

	fmt.Fprintf(w, "\n%s\n", style.section("Timeline:"))
	for i, e := range displayEntries {
		relative := formatRelativeTime(e.Time, now)
		status := e.Status
		fmt.Fprintf(w, "  #%-3d %-12s %-7s  %s", i+1, relative, e.Kind, status)
		if e.Detail != "" {
			fmt.Fprintf(w, "  %s", e.Detail)
		}
		fmt.Fprintln(w)
		if e.Error != "" {
			for _, line := range strings.Split(e.Error, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(w, "       %s\n", style.bad(line))
				}
			}
		}
		if e.Output != "" {
			for _, line := range strings.Split(e.Output, "\n") {
				line = strings.ReplaceAll(line, "\r", "")
				fmt.Fprintf(w, "       > %s\n", line)
			}
		}
	}

	if !showAll && len(entries) > 10 {
		fmt.Fprintf(w, "\n  %s\n", style.info(fmt.Sprintf("... %d more events, use --show-all-events to see all", len(entries)-10)))
	}

	return nil
}

func inventoryNextActions(output inventoryOutput, hostIDFilter string, showAll bool, baseCommand string) []nextAction {
	var actions []nextAction

	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  "Fetch more events to cover full search window",
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}

	if hostIDFilter == "" {
		// ls view
		if output.HostPage.HasNext {
			actions = append(actions, nextAction{
				Comment:  "Next page",
				Commands: []string{withRangeFlag(baseCommand, output.HostPage.End, output.HostPage.End+25)},
			})
		}
		if output.FailedHosts > 0 {
			actions = append(actions, nextAction{
				Comment:  "Show only failing hosts",
				Commands: []string{"tctl discovery inventory ls --state=failed"},
			})
		}
		if len(output.Hosts) > 0 {
			actions = append(actions, nextAction{
				Comment:  "Drill into a specific host",
				Commands: []string{fmt.Sprintf("tctl discovery inventory show %s", output.Hosts[0].DisplayID)},
			})
		}
	} else {
		// show view
		if !showAll {
			actions = append(actions, nextAction{
				Comment:  "Show full event timeline",
				Commands: []string{baseCommand + " --show-all-events"},
			})
		}
		actions = append(actions, nextAction{
			Comment:  "Return to host list",
			Commands: []string{"tctl discovery inventory ls"},
		})
		actions = append(actions, nextAction{
			Comment:  "Check SSM runs for this host",
			Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s", hostIDFilter)},
		})
		actions = append(actions, nextAction{
			Comment:  "Check join events for this host",
			Commands: []string{fmt.Sprintf("tctl discovery joins show %s", hostIDFilter)},
		})
	}

	return actions
}
