/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"math/rand/v2"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/lib/inventory"
)

// SampleAgentsFromAutoUpdateGroup iterates over every handle in the inventory to
// build a random sample of agents belonging to a given group.
// The main use-case for this function is to pick canaries that can be updated.
func (a *Server) SampleAgentsFromAutoUpdateGroup(ctx context.Context, groupName string, sampleSize int, groups []string) ([]*autoupdatev1pb.Canary, error) {
	if len(groups) == 0 {
		return nil, trace.BadParameter("no groups specified")
	}
	isCatchAll := groupName == groups[len(groups)-1]
	var groupSet map[string]struct{}

	// Small optimization, we only need to build the groupSet if we are sampling the catch-all group
	if isCatchAll {
		groupSet = make(map[string]struct{})
		for _, group := range groups {
			groupSet[group] = struct{}{}
		}
	}

	filter := func(handle inventory.UpstreamHandle) bool {
		if result := filterHandler(handle, a.clock.Now()); result != autoUpdateFilterResultMatching {
			return false
		}

		// If the agent group matches, we keep it.
		if handle.Hello().UpdaterInfo.UpdateGroup == groupName {
			// No need to check for UpdaterInfo being nil, it would have been filtered
			// out by filterHandler().
			return true
		}

		// If this is not the catch-all group, we filter the agent out
		if !isCatchAll {
			return false
		}

		// This is the catch-call group, it matches agents from every group not in groups.
		_, ok := groupSet[handle.Hello().UpdaterInfo.UpdateGroup]
		// If the agent group is not in the group list, it falls into the catch-all.
		return !ok
	}

	sampler := newHandlerSampler(sampleSize, filter)

	a.inventory.UniqueHandles(sampler.visit)

	sampled := sampler.Sampled()
	canaries := make([]*autoupdatev1pb.Canary, 0, len(sampled))
	for _, h := range sampled {
		hello := h.Hello()
		updaterID, err := uuid.FromBytes(hello.UpdaterInfo.UpdateUUID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		canaries = append(canaries, &autoupdatev1pb.Canary{
			UpdaterId: updaterID.String(),
			HostId:    hello.ServerID,
			Hostname:  hello.Hostname,
			Success:   false,
		})
	}
	return canaries, nil
}

// handleSampler randomly samples handles from the inventory.
// It implements Alan Waterman's Reservoir Sampling Algorithm R
// (The Art of Computer Programming Volume 2).
// See https://en.wikipedia.org/wiki/Reservoir_sampling for more details.
type handleSampler struct {
	sampleSize int
	seenCount  int
	filter     func(handle inventory.UpstreamHandle) bool
	sample     []inventory.UpstreamHandle
}

func newHandlerSampler(sampleSize int, filter func(handle inventory.UpstreamHandle) bool) *handleSampler {
	return &handleSampler{
		sampleSize: sampleSize,
		seenCount:  sampleSize,
		filter:     filter,
		sample:     make([]inventory.UpstreamHandle, 0, sampleSize),
	}
}

// pass this function to inventory.AllHandles()
func (h *handleSampler) visit(handle inventory.UpstreamHandle) {
	// filter out everything we don't want
	if !h.filter(handle) {
		return
	}

	// Fill the reservoir
	if len(h.sample) < h.sampleSize {
		h.sample = append(h.sample, handle)
		h.seenCount++
		return
	}

	// Reservoir is already filled, replace existing elements.
	if j := rand.N(h.seenCount); j < h.sampleSize {
		h.sample[j] = handle
	}
	h.seenCount++
}

func (h *handleSampler) Sampled() []inventory.UpstreamHandle {
	return h.sample
}

// LookupAgentInInventory looks for a specific hostID in the local inventory
// and returns all the Hello messages associated to those agents.
// Agents that joined less than a minute ago or that are terminating/restarting
// are ignored.
func (a *Server) LookupAgentInInventory(ctx context.Context, hostID string) ([]*proto.UpstreamInventoryHello, error) {
	handles := a.inventory.Handles(hostID)
	now := a.clock.Now()

	var qualifiedHellos []*proto.UpstreamInventoryHello

	for handle := range handles {
		// If the instance is being soft-reloaded or shut down, we ignore it.
		if goodbye := handle.Goodbye(); goodbye.GetSoftReload() || goodbye.GetDeleteResources() {
			continue
		}

		// We skip servers that joined less than a minute ago as they might have been
		// connected to another auth instance a few seconds ago, which would lead to double-counting.
		if now.Sub(handle.RegistrationTime()) < constants.AutoUpdateAgentReportPeriod {
			continue
		}
		// Do don't apply other filtering logic like filterHandle() does because the instance already
		// got selected with strict constraints earlier during sampling. We don't want a filtering rule change, or an instance change, to make the lookup fail and block the rollout.
		qualifiedHellos = append(qualifiedHellos, handle.Hello())
	}

	if len(qualifiedHellos) == 0 {
		return nil, trace.NotFound("no control streams meet requirements for host %v", hostID)
	}

	return qualifiedHellos, nil

}
