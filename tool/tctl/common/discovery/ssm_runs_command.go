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


	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

func (c *Command) runSSMRunsList(ctx context.Context, client discoveryClient) error {
	start, end, err := parseRange(c.ssmRunsRange)
	if err != nil {
		return trace.Wrap(err)
	}

	analysis, vmGroups, meta, err := c.fetchSSMRunData(ctx, client, "")
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "SSM run data fetched", "total_runs", analysis.Total, "failed", analysis.Failed, "vm_groups", len(vmGroups))

	allFailingVMGroups := selectFailingVMGroups(vmGroups, 0)
	displayedVMs, vmPage := paginateSlice(vmGroups, start, end)

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, displayedVMs, vmPage, meta)
	if c.ssmRunsGroup {
		slog.DebugContext(ctx, "Starting run grouping", "similarity", c.ssmRunsSimilarity)
		opts := groupingDefaults()
		opts.drainSimThreshold = c.ssmRunsSimilarity
		var errorStats, successStats groupingStats
		output.ErrorGroups, output.SuccessGroups, errorStats, successStats = groupSSMRuns(vmGroups, opts, c.ssmRunsGroupDebug)
		if c.ssmRunsGroupDebug {
			output.ErrorGroupStats = &errorStats
			output.SuccessGroupStats = &successStats
		}
		// Assign group IDs back to VMs.
		assignGroupIDs(output.VMs, output.ErrorGroups)

		slog.DebugContext(ctx, "Run grouping complete",
			"error_groups", len(output.ErrorGroups),
			"success_groups", len(output.SuccessGroups))
	}
	baseCommand := buildSSMRunsCommand(c, "ls", "")
	return trace.Wrap(c.writeSSMRunsOutput(output, "", baseCommand))
}

func (c *Command) runSSMRunsShow(ctx context.Context, client discoveryClient) error {
	instanceID := c.ssmRunsShowInstanceID
	analysis, vmGroups, meta, err := c.fetchSSMRunData(ctx, client, instanceID)
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingVMGroups := selectFailingVMGroups(vmGroups, 0)
	var showGroups []ssmVMGroup
	for _, group := range vmGroups {
		if group.InstanceID == instanceID {
			showGroups = []ssmVMGroup{group}
			break
		}
	}
	vmPage := fullPageInfo(len(showGroups))

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, showGroups, vmPage, meta)
	baseCommand := buildSSMRunsCommand(c, "show", instanceID)
	return trace.Wrap(c.writeSSMRunsOutput(output, instanceID, baseCommand))
}

func (c *Command) fetchSSMRunData(ctx context.Context, client discoveryClient, instanceIDFilter string) (ssmRunAnalysis, []ssmVMGroup, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC)
	if err != nil {
		return ssmRunAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := effectiveFetchLimit(c.ssmRunsLimit)

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.SSMRunEvent, fetchLimit)
	if err != nil {
		return ssmRunAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}

	meta := buildFetchMeta(fetchLimit, result)
	slog.DebugContext(ctx, "Parsing SSM run events", "events", len(result.Events))
	parseStart := time.Now()
	records := parseSSMRunEvents(result.Events, ssmRunEventFilters{
		InstanceID: instanceIDFilter,
	})
	slog.DebugContext(ctx, "Parsed SSM run events", "records", len(records), "elapsed", time.Since(parseStart).Round(time.Millisecond))
	if meta.LimitReached && len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		meta.SuggestedLimit = estimateRequiredLimit(len(records), oldest, now, from)
	}
	analysis := analyzeSSMRuns(records)
	vmGroups := groupSSMRunsByVM(records)
	return analysis, vmGroups, meta, nil
}

func (c *Command) buildSSMRunsOutput(analysis ssmRunAnalysis, vmGroups, allFailingVMGroups, displayedVMGroups []ssmVMGroup, vmPage pageInfo, meta fetchMeta) ssmRunsOutput {
	from, to, _ := resolveTimeRangeFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC)
	return ssmRunsOutput{
		Window:         timeRangeDescriptionFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC),
		From:           from,
		To:             to,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		CacheSummary: c.cache.cacheSummary(),
		TotalRuns:    analysis.Total,
		SuccessRuns:  analysis.Success,
		FailedRuns:   analysis.Failed,
		TotalVMs:     len(vmGroups),
		FailingVMs:   len(allFailingVMGroups),
		VMPage:       vmPage,
		VMs:          displayedVMGroups,
	}
}

func (c *Command) writeSSMRunsOutput(output ssmRunsOutput, instanceIDFilter, baseCommand string) error {
	if c.groupByAccount {
		output.groupByAccount()
	}
	if c.ssmRunsFormat == formatCSV {
		return trace.Wrap(writeSSMRunsCSV(c.output(), c.csvDir, output, time.Now().UTC()))
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.ssmRunsFormat, output, func(w io.Writer) error {
		return renderSSMRunsText(w, output, instanceIDFilter, c.ssmRunsShowAll, baseCommand)
	}))
}
