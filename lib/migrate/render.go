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

package migrate

import (
	"cmp"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
)

// showUnmatched is the --show value for the target agents no host claims.
const showUnmatched = "unmatched"

// ShowValues returns the accepted --show values: every host state, then
// "unmatched" for the target agents no host claims.
func ShowValues() []string {
	return append(states(), showUnmatched)
}

// Render writes the plan report in format, one of teleport.Text, teleport.JSON
// and teleport.CSV. subset, empty or one of ShowValues, selects what is listed:
// the hosts in one state, or the target agents no host claims. Text prints the
// summary and the selected table; JSON prints the summary and the selected
// list, or every list when subset is empty; CSV prints only the selected list,
// or every host.
func (m *Migrator) Render(w io.Writer, format, subset string, summary *Summary) error {
	return trace.Wrap(render(w, format, subset, m.stateFile.state, summary))
}

func render(w io.Writer, format, subset string, current *migrationState, summary *Summary) error {
	if subset != "" && !slices.Contains(ShowValues(), subset) {
		return trace.BadParameter("unknown show %q; use one of %s", subset, strings.Join(ShowValues(), ", "))
	}
	switch format {
	case teleport.Text:
		if err := renderSummary(w, summary); err != nil {
			return trace.Wrap(err)
		}
		switch subset {
		case "":
		case showUnmatched:
			return trace.Wrap(renderUnmatched(w, current.UnmatchedTargets))
		default:
			return trace.Wrap(renderTable(w, selectedHosts(current, subset)))
		}
		return nil
	case teleport.JSON:
		// A nil pointer leaves out the list subset did not select; a selected
		// empty list still prints as [].
		out := struct {
			Summary   *Summary       `json:"summary"`
			Hosts     *[]*host       `json:"hosts,omitempty"`
			Unmatched *[]targetAgent `json:"unmatched_targets,omitempty"`
		}{Summary: summary}
		if subset != showUnmatched {
			hosts := selectedHosts(current, subset)
			out.Hosts = &hosts
		}
		if subset == "" || subset == showUnmatched {
			unmatched := make([]targetAgent, 0, len(current.UnmatchedTargets))
			unmatched = append(unmatched, current.UnmatchedTargets...)
			out.Unmatched = &unmatched
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return trace.Wrap(enc.Encode(out))
	case teleport.CSV:
		if subset == showUnmatched {
			return trace.Wrap(renderUnmatchedCSV(w, current.UnmatchedTargets))
		}
		return trace.Wrap(renderCSV(w, selectedHosts(current, subset)))
	default:
		return trace.BadParameter("unknown format %q; use one of %s, %s, %s", format, teleport.Text, teleport.JSON, teleport.CSV)
	}
}

// selectedHosts returns the hosts in ID order, only those in state subset when
// subset names a state.
func selectedHosts(current *migrationState, subset string) []*host {
	out := make([]*host, 0, len(current.Hosts))
	for _, id := range slices.Sorted(maps.Keys(current.Hosts)) {
		if h := current.Hosts[id]; subset == "" || h.State == subset {
			out = append(out, h)
		}
	}
	return out
}

func renderSummary(w io.Writer, summary *Summary) error {
	var parts []string
	for _, s := range states() {
		if n := summary.ByState[s]; n > 0 {
			part := fmt.Sprintf("%d %s", n, s)
			if s == stateBlocked {
				part += unreachableNote(summary)
			}
			parts = append(parts, part)
		}
	}
	if _, err := fmt.Fprintf(w, "state file: %s\n%d hosts: %s\n", summary.StateFile, summary.Hosts, cmp.Or(strings.Join(parts, ", "), "none")); err != nil {
		return trace.Wrap(err)
	}
	if summary.UnmatchedTargets > 0 {
		if _, err := fmt.Fprintf(w, "target agents matching no host: %d (--show unmatched)\n", summary.UnmatchedTargets); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(renderVerdict(w, summary))
}

// renderVerdict says what is left, from counts the reader would otherwise add
// up across the tables.
func renderVerdict(w io.Writer, summary *Summary) error {
	total := summary.Hosts
	if total == 0 {
		return nil
	}
	var parts []string
	if n := summary.ByState[stateUnstarted]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d ready to migrate", n))
	}
	if n := summary.ByState[stateBlocked] + summary.ByState[stateFailed]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d need an operator", n)+unreachableNote(summary))
	}
	if len(parts) > 0 {
		_, err := fmt.Fprintf(w, "verdict: %s\n", strings.Join(parts, ", "))
		return trace.Wrap(err)
	}
	migrated, retired, promoted := summary.ByState[stateMigrated], summary.ByState[stateRetired], summary.ByState[statePromoted]
	if retired == 0 && promoted == 0 {
		_, err := fmt.Fprintf(w, "verdict: all %d hosts migrated\n", total)
		return trace.Wrap(err)
	}
	_, err := fmt.Fprintf(w, "verdict: all %d hosts resolved (%d migrated, %d retired, %d promoted)\n", total, migrated, retired, promoted)
	return trace.Wrap(err)
}

// unreachableNote is " (N unreachable)" when preflight could not reach N
// blocked hosts, or "".
func unreachableNote(summary *Summary) string {
	if summary.Unreachable == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d unreachable)", summary.Unreachable)
}

// renderTable writes hosts as a table. Names, targets and reasons originate on
// machines the operator does not control, so they are escaped to keep an agent
// from driving the admin's terminal.
func renderTable(w io.Writer, hosts []*host) error {
	table := asciitable.MakeTable([]string{"Name", "Scope", "State", "Matched By", "Target", "Reason"})
	for _, h := range hosts {
		table.AddRow([]string{utils.EscapeControl(h.Hostname), h.TargetScope, h.State, h.MatchedBy, utils.EscapeControl(h.TargetID), utils.EscapeControl(h.Reason)})
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(table.WriteTo(w))
}

func renderUnmatched(w io.Writer, targets []targetAgent) error {
	table := asciitable.MakeTable([]string{"Target", "Name", "Scope"})
	for _, t := range targets {
		table.AddRow([]string{utils.EscapeControl(t.ID), utils.EscapeControl(t.Hostname), utils.EscapeControl(t.Scope)})
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(table.WriteTo(w))
}

// renderUnmatchedCSV writes the unmatched targets, the same rows
// renderUnmatched prints.
func renderUnmatchedCSV(w io.Writer, targets []targetAgent) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"target_id", "hostname", "scope"}); err != nil {
		return trace.Wrap(err)
	}
	for _, t := range targets {
		if err := writer.Write([]string{csvSafe(t.ID), csvSafe(t.Hostname), csvSafe(t.Scope)}); err != nil {
			return trace.Wrap(err)
		}
	}
	writer.Flush()
	return trace.Wrap(writer.Error())
}

func renderCSV(w io.Writer, hosts []*host) error {
	writer := csv.NewWriter(w)
	header := []string{"id", "hostname", "labels", "target_scope", "state", "reason", "matched_by", "target_id", "join_method", "services", "preflight_at"}
	if err := writer.Write(header); err != nil {
		return trace.Wrap(err)
	}
	for _, h := range hosts {
		var joinMethod, services, preflightAt string
		if preflight := h.Preflight; preflight != nil {
			joinMethod = preflight.JoinMethod
			services = strings.Join(preflight.Services, " ")
			preflightAt = preflight.At.Format(time.RFC3339)
		}
		labels := make([]string, 0, len(h.Labels))
		for _, k := range slices.Sorted(maps.Keys(h.Labels)) {
			labels = append(labels, k+"="+h.Labels[k])
		}
		row := []string{h.ID, h.Hostname, strings.Join(labels, " "), h.TargetScope, h.State, h.Reason, h.MatchedBy, h.TargetID, joinMethod, services, preflightAt}
		for i := range row {
			row[i] = csvSafe(row[i])
		}
		if err := writer.Write(row); err != nil {
			return trace.Wrap(err)
		}
	}
	writer.Flush()
	return trace.Wrap(writer.Error())
}

// csvSafe keeps a spreadsheet from evaluating a cell as a formula. Hostnames,
// labels and error text come from source agents, which choose their own;
// csv.Writer quotes syntax but not formula prefixes.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// RenderReport writes how many hosts the command finished and failed on,
// then each failed host with its reason.
func RenderReport(w io.Writer, report *Report) {
	fmt.Fprintf(w, "%d %s, %d failed\n", report.Done, report.Verb, len(report.Failures))
	for _, failure := range report.Failures {
		fmt.Fprintf(w, "  %s (node/%s): %s\n", utils.EscapeControl(failure.Hostname), utils.EscapeControl(failure.ID), utils.EscapeControl(failure.Reason))
	}
}
