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

func renderSSMRunsText(w io.Writer, output ssmRunsOutput, instanceIDFilter string, showAllRuns bool, baseCommand string) error {
	style := newTextStyle(w)
	now := time.Now().UTC()
	runsInWindow := fmt.Sprintf("%d total (%d failed, %d success)", output.TotalRuns, output.FailedRuns, output.SuccessRuns)
	if output.LimitReached {
		if output.SuggestedLimit > 0 {
			runsInWindow += fmt.Sprintf(" [limit: %d, use --limit=%d to cover full window]", output.FetchLimit, output.SuggestedLimit)
		} else {
			runsInWindow += fmt.Sprintf(" [limit: %d, use --limit to increase]", output.FetchLimit)
		}
	}
	summaryRows := [][]string{
		{"Query window", output.Window},
		{"SSM runs in window", runsInWindow},
		{"VM status snapshot", fmt.Sprintf("%d total VMs, %d currently failing", output.TotalVMs, output.FailingVMs)},
		{"VM rows shown", fmt.Sprintf("%d (range %d-%d)", output.VMPage.End-output.VMPage.Start, output.VMPage.Start, output.VMPage.End)},
	}
	if output.CacheSummary != "" {
		summaryRows = append(summaryRows, []string{"Cache", output.CacheSummary})
	}
	if instanceIDFilter != "" {
		summaryRows = append(summaryRows, []string{"Filtered instance", instanceIDFilter})
	}
	if err := renderTable(w, []string{"Summary Item", "Details"}, summaryRows, style.tableWidth); err != nil {
		return trace.Wrap(err)
	}

	// Collect VM groups to render. When --group-by-account is used, VMs is nil
	// and VMsByAccount is populated instead; render each account as a section.
	type vmSection struct {
		account string // empty when not grouped by account
		vms     []ssmVMGroup
	}
	var sections []vmSection
	if len(output.VMsByAccount) > 0 {
		accounts := mapKeys(output.VMsByAccount)
		slices.Sort(accounts)
		for _, acct := range accounts {
			sections = append(sections, vmSection{account: acct, vms: output.VMsByAccount[acct]})
		}
	} else {
		sections = []vmSection{{vms: output.VMs}}
	}

	allVMs := output.VMs
	if allVMs == nil {
		for _, s := range sections {
			allVMs = append(allVMs, s.vms...)
		}
	}

	if len(allVMs) == 0 {
		if instanceIDFilter != "" {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("No SSM runs found for instance %s in the selected window.", instanceIDFilter)))
		} else {
			fmt.Fprintf(w, "\n%s\n", style.warning("No VMs found for the selected window."))
		}
	} else {
		globalIndex := 0
		for _, section := range sections {
			sectionTitle := "VMs (sorted by most recent run):"
			if instanceIDFilter != "" {
				sectionTitle = "VM:"
			}
			if section.account != "" {
				sectionTitle = fmt.Sprintf("Account %s (%d VMs):", section.account, len(section.vms))
			}
			fmt.Fprintf(w, "\n%s\n", style.section(sectionTitle))
			for i, vm := range section.vms {
				if i > 0 {
					fmt.Fprintln(w, "")
				}
				details := []keyValue{
					{Key: "INSTANCE", Value: vm.InstanceID},
					{Key: "MOST RECENT", Value: formatRelativeOrTimestamp(vm.MostRecent.parsedEventTime, vm.MostRecent.EventTime, now)},
					{Key: "RESULT", Value: cmp.Or(vm.MostRecent.Status, vm.MostRecent.Code)},
					{Key: "RUNS", Value: fmt.Sprintf("%d", vm.TotalRuns)},
					{Key: "FAILED", Value: fmt.Sprintf("%d", vm.FailedRuns)},
					{Key: "REGION", Value: cmp.Or(strings.TrimSpace(vm.MostRecent.Region), placeholderNA)},
				}
				if section.account == "" {
					details = append(details, keyValue{Key: "ACCOUNT", Value: cmp.Or(vm.MostRecent.AccountID, placeholderNA)})
				}
				if vm.ErrorGroupID > 0 {
					details = append(details, keyValue{Key: "GROUP", Value: fmt.Sprintf("%d", vm.ErrorGroupID)})
				}
				if err := style.numberedBlock(w, globalIndex, details); err != nil {
					return trace.Wrap(err)
				}
				globalIndex++
			}
		}

		// In single-instance view, always show history. In multi-VM ls view,
		// show history when --show-all-runs is set or when any VM has output
		// (stdout/stderr from the install script).
		showHistory := showAllRuns || instanceIDFilter != ""
		if !showHistory {
			for _, vm := range allVMs {
				for _, run := range vm.Runs {
					if run.Stdout != "" || run.Stderr != "" {
						showHistory = true
						break
					}
				}
				if showHistory {
					break
				}
			}
		}
		if showHistory {
			fmt.Fprintf(w, "\n%s\n", style.section("Run history:"))
			for vmIndex, vm := range allVMs {
				prefix := fmt.Sprintf("[%d] ", vmIndex+1)
				fmt.Fprintf(w, "\n%s\n", style.info(fmt.Sprintf("%sVM: %s", prefix, vm.InstanceID)))
				rows := buildVMHistoryRows(vm, showAllRuns)
				sub := style.nested(vmIndex)
				if err := renderSSMRunHistoryRows(w, sub, rows, now); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	if len(output.ErrorGroups) > 0 {
		renderSSMRunGroups(w, style, "Error", output.ErrorGroups)
	}
	if len(output.SuccessGroups) > 0 {
		renderSSMRunGroups(w, style, "Success", output.SuccessGroups)
	}

	if output.VMPage.HasNext {
		fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("More VMs available: %d remaining.", output.VMPage.Remaining)))
		pageSize := output.VMPage.End - output.VMPage.Start
		nextEnd := output.VMPage.End + pageSize
		if nextEnd > output.VMPage.Total {
			nextEnd = output.VMPage.Total
		}
		fmt.Fprintf(w, "%s %s\n", style.info("Next page:"), withRangeFlag(baseCommand, output.VMPage.End, nextEnd))
	}

	actions := ssmRunsNextActions(output, instanceIDFilter, showAllRuns, baseCommand)

	return trace.Wrap(renderNextActions(w, style, actions))
}

func renderSSMRunHistoryRows(w io.Writer, style textStyle, rows []ssmRunHistoryRow, now time.Time) error {
	for i, row := range rows {
		if i > 0 {
			fmt.Fprintln(w, "")
		}

		details := []keyValue{
			{Key: "TIMESTAMP", Value: formatHistoryTimestamp(row.Timestamp, now)},
			{Key: "RESULT", Value: row.Result},
			{Key: "COMMAND", Value: cmp.Or(strings.TrimSpace(row.CommandID), placeholderNA)},
			{Key: "EXIT", Value: cmp.Or(strings.TrimSpace(row.ExitCode), placeholderNone)},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
		renderQuotedOutput(w, row.Output, style.nested(i).indent, details)
	}
	return nil
}

func renderQuotedOutput(w io.Writer, output string, indent string, details []keyValue) {
	output = strings.TrimSpace(output)
	if output == "" {
		return
	}
	maxKeyWidth := len("OUTPUT")
	for _, kv := range details {
		maxKeyWidth = max(maxKeyWidth, len(kv.Key))
	}
	fmt.Fprintf(w, "%s%-*s:\n", indent, maxKeyWidth, "OUTPUT")
	for _, line := range strings.Split(output, "\n") {
		line = strings.ReplaceAll(line, "\r", "")
		fmt.Fprintf(w, "%s> %s\n", indent, line)
	}
}

func renderSSMRunGroups(w io.Writer, style textStyle, kind string, groups []ssmRunGroup) {
	fmt.Fprintf(w, "\n%s\n", style.section(fmt.Sprintf("%s Groups (%d):", kind, len(groups))))
	for _, c := range groups {
		totalRuns := 0
		for _, inst := range c.Instances {
			totalRuns += inst.RunCount
		}
		fmt.Fprintf(w, "\n  %s %d runs across %d instances\n",
			style.info(fmt.Sprintf("Group %d:", c.ID+1)),
			totalRuns, len(c.Instances))

		fmt.Fprintf(w, "  %s\n", style.warning("Pattern:"))
		for _, line := range strings.Split(c.Template, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Fprintf(w, "    > %s\n", line)
			}
		}
	}
}

func ssmRunsNextActions(output ssmRunsOutput, instanceIDFilter string, showAllRuns bool, baseCommand string) []nextAction {
	if instanceIDFilter != "" {
		// Single-instance view: suggest going back to list view and checking tasks.
		actions := []nextAction{
			{
				Comment: "Return to SSM overview (use --group to group similar runs)",
				Commands: []string{
					"tctl discovery ssm-runs ls",
					"tctl discovery ssm-runs ls --group",
					"tctl discovery ssm-runs ls --last=1h",
					"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
				},
			},
		}
		if !showAllRuns {
			actions = append(actions, nextAction{
				Comment:  "Show full run history for this instance",
				Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --last=1h --show-all-runs", instanceIDFilter)},
			})
		}
		actions = append(actions, nextAction{
			Comment:  "Use machine-readable output",
			Commands: []string{fmt.Sprintf("tctl discovery ssm-runs show %s --last=1h --format=json", instanceIDFilter)},
		})
		actions = append(actions, nextAction{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		})
		actions = append(actions, nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
		})
		return actions
	}

	// List view: suggest drilling into a specific instance.
	instanceCommand := "tctl discovery ssm-runs show <instance-id> --show-all-runs"
	if len(output.VMs) > 0 && output.VMs[0].InstanceID != "unknown" {
		instanceCommand = fmt.Sprintf("tctl discovery ssm-runs show %s --show-all-runs", output.VMs[0].InstanceID)
	}

	actions := []nextAction{}
	if output.LimitReached {
		limit := suggestedOrFallbackLimit(output.SuggestedLimit, output.FetchLimit)
		actions = append(actions, nextAction{
			Comment:  fmt.Sprintf("Rerun with --limit=%d to cover full window", limit),
			Commands: []string{fmt.Sprintf("%s --limit=%d", baseCommand, limit)},
		})
	}
	if output.ErrorGroups == nil && output.SuccessGroups == nil {
		actions = append(actions, nextAction{
			Comment: "Group similar runs",
			Commands: []string{
				"tctl discovery ssm-runs ls --group",
				"tctl discovery ssm-runs ls --group --format=json",
			},
		})
	}
	actions = append(actions,
		nextAction{
			Comment:  "View all runs for a specific failing instance",
			Commands: []string{instanceCommand},
		},
		nextAction{
			Comment: "Adjust SSM time window",
			Commands: []string{
				"tctl discovery ssm-runs ls --last=1h",
				"tctl discovery ssm-runs ls --from-utc=2026-02-15T08:00 --to-utc=2026-02-15T20:00",
			},
		},
	)
	actions = append(actions,
		nextAction{
			Comment: "Use machine-readable output",
			Commands: []string{
				"tctl discovery ssm-runs ls --last=1h --format=json",
			},
		},
		nextAction{
			Comment:  "Inspect the discovery tasks themselves",
			Commands: []string{"tctl discovery tasks ls --task-type=discover-ec2 --state=open"},
		},
		nextAction{
			Comment:  "List integrations",
			Commands: []string{"tctl discovery integration ls"},
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
	)
	return actions
}
