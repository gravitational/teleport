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
	"encoding/json"
	"io"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"

	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

func (c *Command) runJoinsList(ctx context.Context, client discoveryClient) error {
	if c.joinsRaw {
		return trace.Wrap(c.runJoinsRaw(ctx, client, ""))
	}

	start, end, err := parseRange(c.joinsRange)
	if err != nil {
		return trace.Wrap(err)
	}

	analysis, hostGroups, meta, err := c.fetchJoinData(ctx, client, "")
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingGroups := selectFailingJoinGroups(hostGroups, 0)
	displayGroups := hostGroups
	if c.joinsHideUnknown {
		displayGroups = filterOutUnknownJoinGroups(hostGroups)
	}
	displayedGroups, hostPage := paginateSlice(displayGroups, start, end)

	output := c.buildJoinsOutput(analysis, hostGroups, allFailingGroups, displayedGroups, hostPage, meta)
	baseCommand := buildJoinsCommand(c, "ls", "")
	return trace.Wrap(c.writeJoinsOutput(output, "", baseCommand))
}

func (c *Command) runJoinsShow(ctx context.Context, client discoveryClient) error {
	hostID := c.joinsShowHostID
	if c.joinsRaw {
		return trace.Wrap(c.runJoinsRaw(ctx, client, hostID))
	}
	analysis, hostGroups, meta, err := c.fetchJoinData(ctx, client, hostID)
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingGroups := selectFailingJoinGroups(hostGroups, 0)
	var showGroups []joinGroup
	for _, group := range hostGroups {
		if group.HostID == hostID {
			showGroups = []joinGroup{group}
			break
		}
	}
	hostPage := fullPageInfo(len(showGroups))

	output := c.buildJoinsOutput(analysis, hostGroups, allFailingGroups, showGroups, hostPage, meta)
	baseCommand := buildJoinsCommand(c, "show", hostID)
	return trace.Wrap(c.writeJoinsOutput(output, hostID, baseCommand))
}

// runJoinsRaw fetches raw instance.join audit events and dumps them as JSON.
// When hostIDFilter is non-empty, only events matching that HostID are included.
func (c *Command) runJoinsRaw(ctx context.Context, client discoveryClient, hostIDFilter string) error {
	from, to, err := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	if err != nil {
		return trace.Wrap(err)
	}
	fetchLimit := effectiveFetchLimit(c.joinsLimit)

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return trace.Wrap(err)
	}

	// Filter to matching host ID if specified, and collect raw events.
	var rawEvents []any
	for _, ev := range result.Events {
		join, ok := ev.(*apievents.InstanceJoin)
		if !ok {
			continue
		}
		if hostIDFilter != "" && join.HostID != hostIDFilter {
			continue
		}
		rawEvents = append(rawEvents, join)
	}

	enc := json.NewEncoder(c.output())
	enc.SetIndent("", "  ")
	return trace.Wrap(enc.Encode(rawEvents))
}

func (c *Command) fetchJoinData(ctx context.Context, client discoveryClient, hostIDFilter string) (joinAnalysis, []joinGroup, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	if err != nil {
		return joinAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := effectiveFetchLimit(c.joinsLimit)

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return joinAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}

	meta := buildFetchMeta(fetchLimit, result)
	records := parseInstanceJoinEvents(result.Events, joinEventFilters{
		HostID: hostIDFilter,
	})
	if meta.LimitReached && len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		meta.SuggestedLimit = estimateRequiredLimit(len(records), oldest, now, from)
	}
	analysis := analyzeInstanceJoins(records)
	hostGroups := groupJoinsByHost(records)
	return analysis, hostGroups, meta, nil
}

func (c *Command) buildJoinsOutput(analysis joinAnalysis, hostGroups, allFailingGroups, displayedGroups []joinGroup, hostPage pageInfo, meta fetchMeta) joinsOutput {
	from, to, _ := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	return joinsOutput{
		Window:         timeRangeDescriptionFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC),
		From:           from,
		To:             to,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		CacheSummary: c.cache.cacheSummary(),
		TotalJoins:   analysis.Total,
		SuccessJoins: analysis.Success,
		FailedJoins:  analysis.Failed,
		TotalHosts:   len(hostGroups),
		FailingHosts: len(allFailingGroups),
		HostPage:     hostPage,
		Hosts:        displayedGroups,
	}
}

func (c *Command) writeJoinsOutput(output joinsOutput, hostIDFilter, baseCommand string) error {
	if c.groupByAccount {
		output.groupByAccount()
	}
	if c.joinsFormat == formatCSV {
		return trace.Wrap(writeJoinsCSV(c.output(), c.csvDir, output, time.Now().UTC()))
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.joinsFormat, output, func(w io.Writer) error {
		return renderJoinsText(w, output, hostIDFilter, c.joinsShowAll, baseCommand)
	}))
}
