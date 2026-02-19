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
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

func (c *Command) runInventoryList(ctx context.Context, clt discoveryClient) error {
	start, end, err := parseRange(c.inventoryRange)
	if err != nil {
		return trace.Wrap(err)
	}

	allHosts, meta, err := c.fetchInventoryData(ctx, clt, "")
	if err != nil {
		return trace.Wrap(err)
	}

	filteredHosts := filterInventoryHosts(allHosts, c.inventoryStateFilter, c.inventoryMethodFilter)
	displayedHosts, hostPage := paginateSlice(filteredHosts, start, end)

	output := c.buildInventoryOutput(allHosts, displayedHosts, hostPage, meta)
	baseCommand := buildInventoryCommand(c, "ls", "")
	return trace.Wrap(c.writeInventoryOutput(output, "", baseCommand))
}

func (c *Command) runInventoryShow(ctx context.Context, clt discoveryClient) error {
	hostID := c.inventoryShowHostID

	allHosts, meta, err := c.fetchInventoryData(ctx, clt, hostID)
	if err != nil {
		return trace.Wrap(err)
	}

	var showHosts []inventoryHost
	for _, h := range allHosts {
		if h.DisplayID == hostID || h.HostID == hostID || h.InstanceID == hostID {
			showHosts = []inventoryHost{h}
			break
		}
	}
	hostPage := fullPageInfo(len(showHosts))

	output := c.buildInventoryOutput(allHosts, showHosts, hostPage, meta)
	baseCommand := buildInventoryCommand(c, "show", hostID)
	return trace.Wrap(c.writeInventoryOutput(output, hostID, baseCommand))
}

func (c *Command) fetchInventoryData(ctx context.Context, clt discoveryClient, hostIDFilter string) ([]inventoryHost, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := c.inventoryLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	// Fetch all three data sources. Nodes are always current (not windowed).
	nodes, err := client.GetAllResources[types.Server](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}

	ssmResult, err := fetchAuditEventsInRange(ctx, clt, c.cache, from, to, libevents.SSMRunEvent, fetchLimit)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	ssmRecords := parseSSMRunEvents(ssmResult.Events, ssmRunEventFilters{InstanceID: hostIDFilter})

	joinResult, err := fetchAuditEventsInRange(ctx, clt, c.cache, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	joinRecords := parseInstanceJoinEvents(joinResult.Events, joinEventFilters{HostID: hostIDFilter})

	meta := fetchMeta{
		FetchLimit:   fetchLimit,
		LimitReached: ssmResult.LimitReached || joinResult.LimitReached,
		CacheHits:    ssmResult.CacheHits + joinResult.CacheHits,
		CacheMisses:  ssmResult.CacheMisses + joinResult.CacheMisses,
		CacheFiles:   ssmResult.CacheFiles + joinResult.CacheFiles,
	}

	if meta.LimitReached {
		// Use the worst case: whichever event type needs the larger limit.
		now := time.Now().UTC()
		var suggested int
		if ssmResult.LimitReached && len(ssmResult.Events) > 0 {
			oldest := ssmResult.Events[len(ssmResult.Events)-1].GetTime()
			if s := estimateRequiredLimit(len(ssmResult.Events), oldest, now, from); s > suggested {
				suggested = s
			}
		}
		if joinResult.LimitReached && len(joinResult.Events) > 0 {
			oldest := joinResult.Events[len(joinResult.Events)-1].GetTime()
			if s := estimateRequiredLimit(len(joinResult.Events), oldest, now, from); s > suggested {
				suggested = s
			}
		}
		meta.SuggestedLimit = suggested
	}

	return buildInventoryHosts(nodes, ssmRecords, joinRecords), meta, nil
}

func filterInventoryHosts(hosts []inventoryHost, stateFilter, methodFilter string) []inventoryHost {
	if stateFilter == "" && methodFilter == "" {
		return hosts
	}
	stateFilter = strings.ToLower(strings.TrimSpace(stateFilter))
	methodFilter = strings.ToLower(strings.TrimSpace(methodFilter))

	filtered := make([]inventoryHost, 0, len(hosts))
	for _, h := range hosts {
		if stateFilter != "" && !matchesStateFilter(h.State, stateFilter) {
			continue
		}
		if methodFilter != "" && !strings.EqualFold(h.Method, methodFilter) {
			continue
		}
		filtered = append(filtered, h)
	}
	return filtered
}

func matchesStateFilter(state inventoryHostState, filter string) bool {
	switch filter {
	case "online":
		return state == inventoryStateOnline || state == inventoryStateJoinedOnly
	case "offline":
		return state == inventoryStateOffline
	case "failed":
		return state == inventoryStateJoinFailed || state == inventoryStateSSMFailed
	case "attempted":
		return state == inventoryStateSSMAttempted
	default:
		return strings.EqualFold(string(state), filter)
	}
}

func (c *Command) buildInventoryOutput(allHosts, displayedHosts []inventoryHost, hostPage pageInfo, meta fetchMeta) inventoryOutput {
	var online, offline, failed int
	for _, h := range allHosts {
		switch h.State {
		case inventoryStateOnline, inventoryStateJoinedOnly:
			online++
		case inventoryStateOffline:
			offline++
		case inventoryStateJoinFailed, inventoryStateSSMFailed:
			failed++
		}
	}
	from, to, _ := resolveTimeRangeFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC)
	return inventoryOutput{
		Window:         timeRangeDescriptionFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC),
		From:           from,
		To:             to,
		CacheSummary:   c.cache.cacheSummary(),
		TotalHosts:     len(allHosts),
		OnlineHosts:    online,
		OfflineHosts:   offline,
		FailedHosts:    failed,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		HostPage:       hostPage,
		Hosts:          displayedHosts,
	}
}

func (c *Command) writeInventoryOutput(output inventoryOutput, hostIDFilter, baseCommand string) error {
	if c.groupByAccount {
		output.groupByAccount()
	}
	if c.inventoryFormat == formatCSV {
		return trace.Wrap(writeInventoryCSV(c.output(), c.csvDir, output, time.Now().UTC()))
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.inventoryFormat, output, func(w io.Writer) error {
		return renderInventoryText(w, output, hostIDFilter, c.inventoryShowAll, baseCommand)
	}))
}
