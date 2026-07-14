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
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
)

const discoveryServiceSetupDocsURL = "https://goteleport.com/docs/reference/deployment/config/#discovery-service"

func (s discoverySummary) renderText(w io.Writer, now time.Time) error {
	var out strings.Builder
	if len(s) == 0 {
		writeSummaryLine(&out, "No discovery_config resources are configured.")
	} else {
		for i, config := range s {
			if i > 0 {
				out.WriteByte('\n')
			}
			config.writeText(&out, now)
		}
	}

	_, err := io.WriteString(w, out.String())
	return trace.Wrap(err)
}

func (c configSummary) writeText(w *strings.Builder, now time.Time) {
	writeSummaryLine(w, "Discovery config %s:", c.Name)
	writeSummaryLine(w, "  Discovery group: %s", c.DiscoveryGroup)
	writeSummaryLine(w, "  Status: %s", formatSummaryState(c.State))
	if c.LastSyncTime != nil {
		writeSummaryLine(w, "  Last run: %s", formatSummaryLastRun(c.LastSyncTime, now))
	}
	if c.ErrorMessage != "" {
		writeSummaryLine(w, "  Error: %s", c.ErrorMessage)
	}
	if len(c.Servers) == 0 {
		writeSummaryLine(w, "  No Discovery Services running for %s. See %s.", c.DiscoveryGroup, discoveryServiceSetupDocsURL)
		return
	}
	w.WriteByte('\n')
	for _, server := range c.Servers {
		server.writeText(w, now)
	}
}

func (s serverSummary) writeText(w *strings.Builder, now time.Time) {
	writeSummaryLine(w, "  Service (%s):", s.ServerID)
	if s.PollInterval != "" {
		writeSummaryLine(w, "    Poll interval: %s", formatPollInterval(s.PollInterval))
	}
	if s.LastUpdate != nil {
		writeSummaryLine(w, "    Last update: %s", formatSummaryLastRun(s.LastUpdate, now))
	}
	for _, integration := range s.Integrations {
		integration.writeText(w, now)
	}
}

func (i integrationSummary) writeText(w *strings.Builder, now time.Time) {
	writeSummaryLine(w, "    %s:", formatIntegrationName(i.Integration))
	for _, resource := range i.Resources {
		resource.writeText(w, now)
	}
}

func (r resourceResult) writeText(w *strings.Builder, now time.Time) {
	writeSummaryLine(w, "      %s:", r.Kind)
	writeSummaryLine(w, "        Previous sync: %s%s", formatSummaryLastRun(r.SyncEnd, now), formatSyncDuration(r.SyncStart, r.SyncEnd))
	writeSummaryLine(w, "        Result: %s", formatResourceResult(r))
}

func writeSummaryLine(w *strings.Builder, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	w.WriteString(strings.TrimRight(line, " "))
	w.WriteByte('\n')
}

func formatSummaryState(state string) string {
	switch state {
	case "", discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_UNSPECIFIED.String():
		return summaryStatusNotReporting
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String():
		return "error"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String():
		return "syncing"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String():
		return "healthy"
	default:
		return strings.ToLower(strings.TrimPrefix(state, "DISCOVERY_CONFIG_STATE_"))
	}
}

func formatIntegrationName(integration string) string {
	if integration == "" {
		return "ambient credentials"
	}
	return integration
}

func formatPollInterval(pollInterval string) string {
	duration, err := time.ParseDuration(pollInterval)
	if err != nil {
		return pollInterval
	}
	return strings.TrimSpace(humanize.RelTime(time.Time{}, time.Time{}.Add(duration), "", ""))
}

func formatSyncDuration(start, end *time.Time) string {
	if start == nil || end == nil {
		return ""
	}
	duration := end.Sub(*start).Round(time.Second)
	return " (took " + duration.String() + ")"
}

func formatResourceResult(result resourceResult) string {
	return fmt.Sprintf(
		"%d found, %d enrolled, %d failed",
		result.Found,
		result.Enrolled,
		result.Failed,
	)
}

func formatSummaryLastRun(t *time.Time, now time.Time) string {
	if t == nil || t.IsZero() {
		return "never"
	}
	return humanize.RelTime(*t, now, "ago", "from now")
}
