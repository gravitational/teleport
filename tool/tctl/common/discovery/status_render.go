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
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
)

func (s discoverySummary) renderText(w io.Writer, now time.Time) error {
	if len(s) == 0 {
		_, err := fmt.Fprintln(w, "No AWS or Azure discovery_config resources are configured.")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = fmt.Fprintln(w, "Static discovery_service matchers from teleport.yaml do not report discovery config status.")
		return trace.Wrap(err)
	}

	for i, config := range s {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return trace.Wrap(err)
			}
		}
		if err := config.writeText(w, now); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range config.Resources {
			if _, err := fmt.Fprintln(w); err != nil {
				return trace.Wrap(err)
			}
			if err := resource.writeText(w, now); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func (c configSummary) writeText(w io.Writer, now time.Time) error {
	if _, err := fmt.Fprintln(w, "Discovery config status"); err != nil {
		return trace.Wrap(err)
	}

	table := asciitable.MakeHeadlessTable(2)
	if c.Name != "" {
		table.AddRow([]string{"Config:", c.Name})
	}
	if c.DiscoveryGroup != "" {
		table.AddRow([]string{"Discovery group:", c.DiscoveryGroup})
	}
	table.AddRow([]string{"Status:", c.Status.State})
	table.AddRow([]string{"Last run:", formatSummaryLastRun(c.Status.LastRun, now)})
	if c.Status.ErrorMessage != "" {
		table.AddRow([]string{"Error:", c.Status.ErrorMessage})
	}
	return trace.Wrap(writeSummaryTable(w, table))
}

func (r resourceSummary) writeText(w io.Writer, now time.Time) error {
	if _, err := fmt.Fprintf(w, "%s %s discovery\n", r.Cloud, r.ResourceType); err != nil {
		return trace.Wrap(err)
	}

	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Source:", formatSummarySource(r)})
	r.addScopeRows(&table)
	if r.LastSync != nil {
		table.AddRow([]string{"Last resource sync:", formatSummaryLastRun(r.LastSync, now)})
	}
	table.AddRow([]string{"Result:", formatSummaryResult(r.Result)})
	return trace.Wrap(writeSummaryTable(w, table))
}

func writeSummaryTable(w io.Writer, table asciitable.Table) error {
	var b strings.Builder
	if err := table.WriteTo(&b); err != nil {
		return trace.Wrap(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n") {
		if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r resourceSummary) addScopeRows(table *asciitable.Table) {
	if len(r.Scopes) == 1 {
		addSingleScopeRows(table, r.Scopes[0])
		return
	}

	for i, scope := range r.Scopes {
		label := ""
		if i == 0 {
			label = "Matcher scopes:"
		}
		table.AddRow([]string{label, "- " + formatSummaryScope(scope)})
	}
}

func addSingleScopeRows(table *asciitable.Table, scope resourceScope) {
	table.AddRow([]string{"Regions:", formatSummaryList(scope.Regions)})
	if len(scope.Subscriptions) > 0 {
		table.AddRow([]string{"Subscriptions:", formatSummaryList(scope.Subscriptions)})
	}
	if len(scope.ResourceGroups) > 0 {
		table.AddRow([]string{"Resource groups:", formatSummaryList(scope.ResourceGroups)})
	}
	table.AddRow([]string{"Match tags:", formatSummaryList(scope.MatchTags)})
}

func formatSummaryScope(scope resourceScope) string {
	parts := []string{"Regions: " + formatSummaryList(scope.Regions)}
	if len(scope.Subscriptions) > 0 {
		parts = append(parts, "Subscriptions: "+formatSummaryList(scope.Subscriptions))
	}
	if len(scope.ResourceGroups) > 0 {
		parts = append(parts, "Resource groups: "+formatSummaryList(scope.ResourceGroups))
	}
	parts = append(parts, "Match tags: "+formatSummaryList(scope.MatchTags))
	return strings.Join(parts, "; ")
}

func formatSummarySource(resource resourceSummary) string {
	if resource.Source == "ambient_credentials" || resource.Integration == "" {
		return "ambient credentials"
	}
	return "integration " + resource.Integration
}

func formatSummaryResult(result resultSummary) string {
	if result.Counts == nil {
		return result.Message
	}
	return strings.Join([]string{
		strconv.FormatUint(result.Counts.Found, 10) + " found",
		strconv.FormatUint(result.Counts.Enrolled, 10) + " enrolled",
		strconv.FormatUint(result.Counts.Failed, 10) + " failed",
	}, ", ")
}

func formatSummaryList(values []string) string {
	if len(values) == 0 {
		return "not specified"
	}
	if len(values) == 1 && values[0] == "*" {
		return "all"
	}
	return strings.Join(values, ", ")
}

func formatSummaryLastRun(t *time.Time, now time.Time) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return humanize.RelTime(*t, now, "ago", "from now")
}
