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

func renderSummaryText(w io.Writer, blocks []summaryBlock, now time.Time) error {
	if len(blocks) == 0 {
		_, err := fmt.Fprintln(w, "No AWS or Azure discovery is configured.")
		return trace.Wrap(err)
	}

	var b strings.Builder
	for i, block := range blocks {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s %s discovery\n", block.Cloud, block.ResourceType)
		writeSummaryField(&b, "Source:", formatSummarySource(block.Integration))
		writeSummaryField(&b, "Regions:", formatSummaryList(block.Regions))
		if len(block.Subscriptions) > 0 {
			writeSummaryField(&b, "Subscriptions:", formatSummaryList(block.Subscriptions))
		}
		if len(block.ResourceGroups) > 0 {
			writeSummaryField(&b, "Resource groups:", formatSummaryList(block.ResourceGroups))
		}
		writeSummaryField(&b, "Match tags:", formatSummaryList(block.MatchTags))
		writeSummaryField(&b, "Status:", block.Status)
		writeSummaryField(&b, "Last run:", formatSummaryLastRun(block.LastRun, now))
		writeSummaryField(&b, "Result:", formatSummaryResult(block.Result))
	}

	_, err := io.WriteString(w, b.String())
	return trace.Wrap(err)
}

func writeSummaryField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "  %-*s %s\n", summaryTextLabelWidth, label, value)
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
	case resultError:
		if result.Message != "" {
			return result.Message
		}
		return "discovery reported an error"
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
