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
)

const summaryTextLabelWidth = 16

func renderSummaryText(w io.Writer, configs []configSummary, now time.Time) error {
	if len(configs) == 0 {
		_, err := fmt.Fprintln(w, "No AWS or Azure discovery_config resources are configured.")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = fmt.Fprintln(w, "Static discovery_service matchers from teleport.yaml do not report discovery config status.")
		return trace.Wrap(err)
	}

	var b strings.Builder
	for i, config := range configs {
		if i > 0 {
			b.WriteString("\n")
		}
		writeSummaryConfig(&b, config, now)
		for _, resource := range config.Resources {
			b.WriteString("\n")
			writeSummaryResource(&b, resource, now)
		}
	}

	_, err := io.WriteString(w, b.String())
	return trace.Wrap(err)
}

func writeSummaryConfig(b *strings.Builder, config configSummary, now time.Time) {
	b.WriteString("Discovery config status\n")
	if config.Name != "" {
		writeSummaryField(b, "Config:", config.Name)
	}
	if config.DiscoveryGroup != "" {
		writeSummaryField(b, "Discovery group:", config.DiscoveryGroup)
	}
	writeSummaryField(b, "Status:", config.Status.label)
	writeSummaryField(b, "Last run:", formatSummaryLastRun(config.Status.lastRun, now))
	if config.Status.errorMessage != "" {
		writeSummaryField(b, "Error:", config.Status.errorMessage)
	}
}

func writeSummaryResource(b *strings.Builder, resource resourceSummary, now time.Time) {
	fmt.Fprintf(b, "%s %s discovery\n", resource.Cloud, resource.ResourceType)
	writeSummaryField(b, "Source:", formatSummarySource(resource.Integration))
	writeSummaryScope(b, resource)
	if resource.LastSync != nil {
		writeSummaryField(b, "Last resource sync:", formatSummaryLastRun(resource.LastSync, now))
	}
	writeSummaryField(b, "Result:", formatSummaryResult(resource.Result))
}

func writeSummaryField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "  %-*s %s\n", summaryTextLabelWidth, label, value)
}

func writeSummaryScope(b *strings.Builder, resource resourceSummary) {
	scopes := resource.Scopes

	if len(scopes) == 1 {
		writeSummaryScopeFields(b, scopes[0])
		return
	}

	b.WriteString("  Matcher scopes:\n")
	for _, scope := range scopes {
		fmt.Fprintf(b, "  - %s\n", formatSummaryScope(scope))
	}
}

func writeSummaryScopeFields(b *strings.Builder, scope resourceScope) {
	writeSummaryField(b, "Regions:", formatSummaryList(scope.Regions))
	if len(scope.Subscriptions) > 0 {
		writeSummaryField(b, "Subscriptions:", formatSummaryList(scope.Subscriptions))
	}
	if len(scope.ResourceGroups) > 0 {
		writeSummaryField(b, "Resource groups:", formatSummaryList(scope.ResourceGroups))
	}
	writeSummaryField(b, "Match tags:", formatSummaryList(scope.MatchTags))
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

func formatSummarySource(integration string) string {
	if integration == "" {
		return "ambient credentials"
	}
	return "integration " + integration
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

func formatSummaryResult(result summaryResult) string {
	switch result.Kind {
	case resultNotReporting:
		return "no status reported by a Discovery Service"
	case resultUnsupported:
		return "detailed counts are not available for this resource type"
	case resultNoResourceStatus:
		return "no resource status reported for this discovery target"
	case resultCounts:
		return strings.Join([]string{
			strconv.FormatUint(result.Found, 10) + " found",
			strconv.FormatUint(result.Enrolled, 10) + " enrolled",
			strconv.FormatUint(result.Failed, 10) + " failed",
		}, ", ")
	default:
		return ""
	}
}
