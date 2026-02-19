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
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/gravitational/teleport/api/utils/clientutils"

	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/trace"
)

func (c *Command) runStatus(ctx context.Context, client discoveryClient) error {
	slog.DebugContext(ctx, "Fetching user tasks", "integration_filter", c.statusIntegration)
	tasks, err := listUserTasks(ctx, client, c.statusIntegration, "")
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched user tasks", "count", len(tasks))

	slog.DebugContext(ctx, "Fetching discovery configs")
	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched discovery configs", "count", len(discoveryConfigs))

	slog.DebugContext(ctx, "Fetching integrations")
	integrations, err := listIntegrations(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched integrations", "count", len(integrations))

	summary := makeStatusSummary(tasks, discoveryConfigs, integrations, c.statusIntegration)

	slog.DebugContext(ctx, "Fetching SSM run events", "window", c.statusLast, "limit", c.statusSSMLimit)
	ssmStats, err := fetchSSMRunStats(ctx, client, c.cache, c.statusLast, c.statusFromUTC, c.statusToUTC, c.statusSSMLimit)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched SSM run events", "total", ssmStats.Total, "limit_reached", ssmStats.LimitReached)
	summary.SSMRunStats = ssmStats

	slog.DebugContext(ctx, "Fetching instance join events", "window", c.statusLast, "limit", c.statusJoinLimit)
	joinStats, err := fetchJoinStats(ctx, client, c.cache, c.statusLast, c.statusFromUTC, c.statusToUTC, c.statusJoinLimit)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched instance join events", "total", joinStats.Total, "limit_reached", joinStats.LimitReached)
	summary.JoinStats = joinStats
	summary.CacheSummary = c.cache.cacheSummary()

	return trace.Wrap(writeOutputByFormat(c.output(), c.statusFormat, summary, func(w io.Writer) error {
		return renderStatusText(w, summary)
	}))
}

func fetchSSMRunStats(ctx context.Context, client discoveryClient, cache *eventCache, last, fromUTC, toUTC string, limit int) (*auditEventStats, error) {
	events, limitReached, err := fetchAuditEventStats(ctx, client, cache, last, fromUTC, toUTC, libevents.SSMRunEvent, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	records := parseSSMRunEvents(events, ssmRunEventFilters{})
	analysis := analyzeSSMRuns(records)
	from, to, _ := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	stats := &auditEventStats{
		Window:        timeRangeDescriptionFromFlags(last, fromUTC, toUTC),
		From:          from,
		To:            to,
		Total:         analysis.Total,
		Success:       analysis.Success,
		Failed:        analysis.Failed,
		DistinctHosts: len(analysis.ByInstance),
		FailingHosts:  len(analysis.FailedByInstance),
		LimitReached:  limitReached,
	}
	if len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		stats.OldestEvent = oldest
		if limitReached {
			stats.EffectiveWindow = formatRelativeDelta(oldest, now, false)
			stats.SuggestedLimit = estimateRequiredLimit(stats.Total, oldest, now, from)
		}
	}
	return stats, nil
}

func fetchJoinStats(ctx context.Context, client discoveryClient, cache *eventCache, last, fromUTC, toUTC string, limit int) (*auditEventStats, error) {
	events, limitReached, err := fetchAuditEventStats(ctx, client, cache, last, fromUTC, toUTC, libevents.InstanceJoinEvent, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	records := parseInstanceJoinEvents(events, joinEventFilters{})
	analysis := analyzeInstanceJoins(records)
	from, to, _ := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	stats := &auditEventStats{
		Window:        timeRangeDescriptionFromFlags(last, fromUTC, toUTC),
		From:          from,
		To:            to,
		Total:         analysis.Total,
		Success:       analysis.Success,
		Failed:        analysis.Failed,
		DistinctHosts: len(analysis.ByHost),
		FailingHosts:  len(analysis.FailedByHost),
		LimitReached:  limitReached,
	}
	if len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		stats.OldestEvent = oldest
		if limitReached {
			stats.EffectiveWindow = formatRelativeDelta(oldest, now, false)
			stats.SuggestedLimit = estimateRequiredLimit(stats.Total, oldest, now, from)
		}
	}
	return stats, nil
}
