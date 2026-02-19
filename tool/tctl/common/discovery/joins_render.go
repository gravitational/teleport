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
	"strings"
	"time"

	"github.com/gravitational/trace"
)

func renderJoinsText(w io.Writer, output joinsOutput, hostIDFilter string, showAllJoins bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	joinsInWindow := fmt.Sprintf("%d total (%d failed, %d success)", output.TotalJoins, output.FailedJoins, output.SuccessJoins)
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			joinsInWindow += fmt.Sprintf(" [limit: %d, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			joinsInWindow += fmt.Sprintf(" [limit: %d, use --limit to increase]", output.FetchLimit)
		}
	}
	summaryRows := [][]string{
		{"Query window", output.Window},
		{"Joins in window", joinsInWindow},
		{"Host snapshot", fmt.Sprintf("%d total hosts, %d with failed joins", output.TotalHosts, output.FailingHosts)},
		{"Host rows shown", fmt.Sprintf("%d (range %d-%d)", output.HostPage.End-output.HostPage.Start, output.HostPage.Start, output.HostPage.End)},
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if hostIDFilter != "" {
		summaryRows = append(summaryRows, []string{"Filtered host", hostIDFilter})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	// Collect host groups to render. When --group-by-account is used, Hosts is nil
	// and HostsByAccount is populated instead; render each account as a section.
	type hostSection struct {
		account string
		hosts   []joinGroup
	}
	var sections []hostSection
	if len(output.HostsByAccount) > 0 {
		accounts := mapKeys(output.HostsByAccount)
		slices.Sort(accounts)
		for _, acct := range accounts {
			sections = append(sections, hostSection{account: acct, hosts: output.HostsByAccount[acct]})
		}
	} else {
		sections = []hostSection{{hosts: output.Hosts}}
	}

	allHosts := output.Hosts
	if allHosts == nil {
		for _, s := range sections {
			allHosts = append(allHosts, s.hosts...)
		}
	}

	if len(allHosts) == 0 {
		if hostIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No join events found for host %s in the selected window.", hostIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No hosts found for the selected window."))
		}
	} else {
		globalIndex := 0
		for _, section := range sections {
			sectionTitle := "Hosts (sorted by most recent join):"
			if hostIDFilter != "" {
				sectionTitle = "Host:"
			}
			if section.account != "" {
				sectionTitle = fmt.Sprintf("Account %s (%d hosts):", section.account, len(section.hosts))
			}
			fmt.Fprintf(w, "\n%s\n", style.section(sectionTitle))
			for i, host := range section.hosts {
				if i > 0 {
					fmt.Fprintln(w, "")
				}
				result := "success"
				if host.MostRecentFailed {
					result = cmp.Or(host.MostRecent.Error, "failed")
				}
				details := []keyValue{
					{Key: "HOST", Value: host.HostID},
					{Key: "NODE NAME", Value: cmp.Or(strings.TrimSpace(host.NodeName), placeholderNA)},
					{Key: "INSTANCE ID", Value: cmp.Or(strings.TrimSpace(host.MostRecent.InstanceID), placeholderNA)},
					{Key: "TOKEN", Value: cmp.Or(strings.TrimSpace(host.MostRecent.TokenName), placeholderNone)},
					{Key: "MOST RECENT", Value: formatRelativeOrTimestamp(host.MostRecent.parsedEventTime, host.MostRecent.EventTime, now)},
					{Key: "RESULT", Value: result},
					{Key: "METHOD", Value: cmp.Or(strings.TrimSpace(host.MostRecent.Method), placeholderNA)},
					{Key: "ROLE", Value: cmp.Or(strings.TrimSpace(host.MostRecent.Role), placeholderNA)},
					{Key: "JOINS", Value: fmt.Sprintf("%d", host.TotalJoins)},
					{Key: "FAILED", Value: fmt.Sprintf("%d", host.FailedJoins)},
				}
				if section.account == "" {
					details = append(details, keyValue{Key: "ACCOUNT ID", Value: cmp.Or(strings.TrimSpace(host.MostRecent.AccountID), placeholderNA)})
				}
				if err := style.numberedBlock(w, globalIndex, details); err != nil {
					return trace.Wrap(err)
				}
				globalIndex++
			}
		}

		// Show history in single-host detail view or when --show-all-joins is set.
		if hostIDFilter != "" || showAllJoins {
			fmt.Fprintf(w, "\n%s\n", style.section("Join history:"))
			for hostIndex, host := range allHosts {
				prefix := fmt.Sprintf("[%d] ", hostIndex+1)
				fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("%sHOST: %s", prefix, host.HostID)))
				rows := buildJoinHistoryRows(host, showAllJoins)
				sub := style.nested(hostIndex)
				if err := renderJoinHistoryRows(w, sub, rows, now); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	if output.HostPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("More hosts available: %d remaining.", output.HostPage.Remaining)))
		pageSize := output.HostPage.End - output.HostPage.Start
		nextEnd := min(output.HostPage.End+pageSize, output.HostPage.Total)
		fmt.Fprintf(w, "%s %s\n", style.info("Next page:"), withRangeFlag(baseCommand, output.HostPage.End, nextEnd))
	}

	actions := joinsNextActions(output, hostIDFilter, showAllJoins, baseCommand)
	return trace.Wrap(renderNextActions(w, style, actions))
}

func renderJoinHistoryRows(w io.Writer, style textStyle, rows []joinHistoryRow, now time.Time) error {
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		details := []keyValue{
			{Key: "TIMESTAMP", Value: formatHistoryTimestamp(row.Timestamp, now)},
			{Key: "RESULT", Value: row.Result},
			{Key: "METHOD", Value: cmp.Or(strings.TrimSpace(row.Method), placeholderNA)},
			{Key: "ROLE", Value: cmp.Or(strings.TrimSpace(row.Role), placeholderNA)},
			{Key: "INSTANCE ID", Value: cmp.Or(strings.TrimSpace(row.InstanceID), placeholderNA)},
			{Key: "ACCOUNT ID", Value: cmp.Or(strings.TrimSpace(row.AccountID), placeholderNA)},
			{Key: "TOKEN", Value: cmp.Or(strings.TrimSpace(row.TokenName), placeholderNone)},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func joinsNextActions(output joinsOutput, hostIDFilter string, showAllJoins bool, baseCommand string) []nextAction {
	if hostIDFilter != "" {
		actions := []nextAction{
			{
				Comment: "Return to joins overview",
				Commands: []string{
					"tctl discovery joins ls",
					"tctl discovery joins ls --last=1h",
					"tctl discovery joins ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
		}
		if !showAllJoins {
			actions = append(actions, nextAction{
				Comment:  "Show full join history for this host",
				Commands: []string{fmt.Sprintf("tctl discovery joins show %s --show-all-joins", shellQuoteArg(hostIDFilter))},
			})
		}
		actions = append(actions,
			nextAction{
				Comment: "Use machine-readable output",
				Commands: []string{
					fmt.Sprintf("tctl discovery joins show %s --format=json", shellQuoteArg(hostIDFilter)),
				},
			},
			nextAction{
				Comment: "Check SSM runs",
				Commands: []string{
					"tctl discovery ssm-runs ls",
					"tctl discovery ssm-runs ls --last=1h",
					"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
			nextAction{
				Comment:  "Inspect the discovery tasks",
				Commands: []string{"tctl discovery tasks ls"},
			},
			nextAction{
				Comment:  "List integrations",
				Commands: []string{"tctl discovery integration ls"},
			},
		)
		return actions
	}

	// List view.
	hostCommand := "tctl discovery joins show <host-id> --show-all-joins"
	if len(output.Hosts) > 0 {
		hostCommand = fmt.Sprintf("tctl discovery joins show %s --show-all-joins", shellQuoteArg(output.Hosts[0].HostID))
	}

	var actions []nextAction
	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  "Fetch more events to cover full search window",
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}
	actions = append(actions,
		nextAction{
			Comment: "Adjust joins time window",
			Commands: []string{
				"tctl discovery joins ls --last=1h",
				"tctl discovery joins ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
		nextAction{
			Comment:  "View all joins for a specific host",
			Commands: []string{hostCommand},
		},
	)
	actions = append(actions,
		nextAction{
			Comment:  "Hide hosts with unknown/empty host ID",
			Commands: []string{"tctl discovery joins ls --hide-unknown"},
		},
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery joins ls --last=1h --format=json",
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
			Comment:  "Inspect the discovery tasks",
			Commands: []string{"tctl discovery tasks ls"},
		},
		nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
		},
		nextAction{
			Comment:  "View unified host inventory",
			Commands: []string{"tctl discovery inventory ls"},
		},
	)
	return actions
}
