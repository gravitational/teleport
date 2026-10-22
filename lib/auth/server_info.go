/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"iter"
	"log/slog"
	"maps"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
)

const serverInfoBatchSize = 100
const timeBetweenServerInfoBatches = 10 * time.Second
const timeBetweenServerInfoLoops = 10 * time.Minute

// ServerInfoAccessPoint is the subset of the auth server interface needed to
// reconcile server info resources.
type ServerInfoAccessPoint interface {
	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	// GetServerInfo returns a ServerInfo by name.
	GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error)
	// UpdateLabels updates the labels on an instance over the inventory control
	// stream.
	UpdateLabels(ctx context.Context, req *proto.InventoryUpdateLabelsRequest) error
	// GetClock returns the server clock.
	GetClock() clockwork.Clock
}

// ReconcileServerInfos periodically reconciles the labels of ServerInfo
// resources with their corresponding Teleport SSH servers.
func ReconcileServerInfos(ctx context.Context, ap ServerInfoAccessPoint) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(defaults.MaxWatcherBackoff / 10),
		Step:   defaults.MaxWatcherBackoff / 5,
		Max:    defaults.MaxWatcherBackoff,
		Jitter: retryutils.HalfJitter,
		Clock:  ap.GetClock(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		err := retry.For(ctx, func() error { return trace.Wrap(reconcileServerInfos(ctx, ap)) })
		if err != nil {
			return trace.Wrap(err)
		}
		retry.Reset()
		select {
		case <-ap.GetClock().After(timeBetweenServerInfoLoops):
		case <-ctx.Done():
			return nil
		}
	}
}

func reconcileServerInfos(ctx context.Context, ap ServerInfoAccessPoint) error {
	var failedUpdates int
	var startKey string

	for {
		resp, err := ap.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    apidefaults.Namespace,
			Limit:        serverInfoBatchSize,
			StartKey:     startKey,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, r := range resp.Resources {
			node, ok := r.(types.Server)
			if !ok {
				return trace.BadParameter("expected types.Server, got %T", r)
			}

			failedUpdate, err := setLabelsOnNode(ctx, ap, node)
			if err != nil {
				return trace.Wrap(err)
			}

			if failedUpdate {
				failedUpdates++
			}
		}

		if resp.NextKey == "" {
			break
		}
		startKey = resp.NextKey

		select {
		case <-ap.GetClock().After(timeBetweenServerInfoBatches):
		case <-ctx.Done():
			return nil
		}
	}

	// Log number of nodes that we couldn't find a control stream for.
	if failedUpdates > 0 {
		slog.DebugContext(ctx, "unable to update labels on nodes due to missing control stream", "failed_updates", failedUpdates)
	}
	return nil
}

// getServerInfoNames gets the names of ServerInfos that could exist for a
// node. The list of names returned are ordered such that later ServerInfos
// override earlier ones on conflicting labels.
func getServerInfoNames(node types.Server) iter.Seq[string] {
	return func(yield func(string) bool) {
		if meta := node.GetCloudMetadata(); meta != nil && meta.AWS != nil {
			if !yield(types.ServerInfoNameFromAWS(meta.AWS.AccountID, meta.AWS.InstanceID)) {
				return
			}
		}

		// ServerInfos matched by node name should override any ServerInfos created
		// by the discovery service.
		yield(types.ServerInfoNameFromNodeName(node.GetName()))
	}
}

func setLabelsOnNode(ctx context.Context, ap ServerInfoAccessPoint, node types.Server) (failedUpdate bool, err error) {
	// Agentless Node labels can't be updated using the Inventory Control
	// Stream because there's teleport instance connected via the ICS.
	if node.IsOpenSSHNode() {
		return false, nil
	}

	// Get the server infos that match this node.
	newLabels := make(map[string]string)
	var found bool
	for name := range getServerInfoNames(node) {
		si, err := ap.GetServerInfo(ctx, name)
		switch {
		case trace.IsNotFound(err):
			continue
		case err != nil:
			return false, trace.Wrap(err)
		default:
			// Merge labels from server infos. Later label sets should override earlier
			// ones if they conflict.
			maps.Copy(newLabels, si.GetNewLabels())
			found = true
		}
	}

	if !found {
		return false, nil
	}

	// Didn't find control stream for node, save count for logging.
	if err := updateLabelsOnNode(ctx, ap, node, newLabels); trace.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, trace.Wrap(err)
	}
	return false, nil
}

func updateLabelsOnNode(ctx context.Context, ap ServerInfoAccessPoint, node types.Server, newLabels map[string]string) error {
	err := ap.UpdateLabels(ctx, proto.InventoryUpdateLabelsRequest_builder{
		ServerID: node.GetName(),
		Kind:     proto.LabelUpdateKind_SSHServerCloudLabels,
		Labels:   newLabels,
	}.Build())
	return trace.Wrap(err)
}
