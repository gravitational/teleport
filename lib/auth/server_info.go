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
	"log/slog"
	"maps"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
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
	// GetNodeStream returns a stream of nodes.
	GetNodeStream(ctx context.Context, namespace string) stream.Stream[types.Server]
	// GetServerInfo returns a ServerInfo by name.
	GetServerInfo(ctx context.Context, name string) (types.ServerInfo, error)
	// UpdateLabels updates the labels on an instance over the inventory control
	// stream.
	UpdateLabels(ctx context.Context, req proto.InventoryUpdateLabelsRequest) error
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
	// Iterate over nodes in batches.
	nodeStream := ap.GetNodeStream(ctx, apidefaults.Namespace)

	for {
		nodes, moreNodes := stream.Take(nodeStream, serverInfoBatchSize)
		updates, err := setLabelsOnNodes(ctx, ap, nodes)
		if err != nil {
			return trace.Wrap(err)
		}
		failedUpdates += updates
		if !moreNodes {
			break
		}

		select {
		case <-ap.GetClock().After(timeBetweenServerInfoBatches):
		case <-ctx.Done():
			return nil
		}
	}
	if err := nodeStream.Done(); err != nil {
		return trace.Wrap(err)
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
func getServerInfoNames(node types.Server) []string {
	var names []string
	if meta := node.GetCloudMetadata(); meta != nil && meta.AWS != nil {
		names = append(names, types.ServerInfoNameFromAWS(meta.AWS.AccountID, meta.AWS.InstanceID))
	}
	// ServerInfos matched by node name should override any ServerInfos created
	// by the discovery service.
	return append(names, types.ServerInfoNameFromNodeName(node.GetName()))
}

func setLabelsOnNodes(ctx context.Context, ap ServerInfoAccessPoint, nodes []types.Server) (failedUpdates int, err error) {
	for _, node := range nodes {
		// EICE Node labels can't be updated using the Inventory Control Stream because there's no reverse tunnel.
		// Labels are updated by the DiscoveryService during 'Server.handleEC2Instances'.
		// The same is valid for OpenSSH Nodes.
		if node.IsOpenSSHNode() {
			continue
		}

		// Get the server infos that match this node.
		serverInfoNames := getServerInfoNames(node)
		serverInfos := make([]types.ServerInfo, 0, len(serverInfoNames))
		for _, name := range serverInfoNames {
			si, err := ap.GetServerInfo(ctx, name)
			if err == nil {
				serverInfos = append(serverInfos, si)
			} else if !trace.IsNotFound(err) {
				return failedUpdates, trace.Wrap(err)
			}
		}
		if len(serverInfos) == 0 {
			continue
		}

		// Didn't find control stream for node, save count for logging.
		if err := updateLabelsOnNode(ctx, ap, node, serverInfos); trace.IsNotFound(err) {
			failedUpdates++
		} else if err != nil {
			return failedUpdates, trace.Wrap(err)
		}
	}
	return failedUpdates, nil
}

func updateLabelsOnNode(ctx context.Context, ap ServerInfoAccessPoint, node types.Server, serverInfos []types.ServerInfo) error {
	// Merge labels from server infos. Later label sets should override earlier
	// ones if they conflict.
	newLabels := make(map[string]string)
	for _, si := range serverInfos {
		maps.Copy(newLabels, si.GetNewLabels())
	}
	err := ap.UpdateLabels(ctx, proto.InventoryUpdateLabelsRequest{
		ServerID: node.GetName(),
		Kind:     proto.LabelUpdateKind_SSHServerCloudLabels,
		Labels:   newLabels,
	})
	return trace.Wrap(err)
}
