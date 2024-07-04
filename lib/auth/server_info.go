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
	"maps"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
)

// ReconcileServerInfos periodically reconciles the labels of ServerInfo
// resources with their corresponding Teleport SSH servers.
func (a *Server) ReconcileServerInfos(ctx context.Context) error {
	const batchSize = 100
	const timeBetweenBatches = 10 * time.Second
	const timeBetweenReconciliationLoops = 10 * time.Minute
	clock := a.GetClock()

	for {
		var failedUpdates int
		// Iterate over nodes in batches.
		nodeStream := a.GetNodeStream(ctx, defaults.Namespace)
		var nodes []types.Server

		for moreNodes := true; moreNodes; {
			nodes, moreNodes = stream.Take(nodeStream, batchSize)
			updates, err := a.setLabelsOnNodes(ctx, nodes)
			if err != nil {
				return trace.Wrap(err)
			}
			failedUpdates += updates

			select {
			case <-clock.After(timeBetweenBatches):
			case <-ctx.Done():
				return nil
			}
		}

		// Log number of nodes that we couldn't find a control stream for.
		if failedUpdates > 0 {
			log.Debugf("unable to update labels on %v node(s) due to missing control stream", failedUpdates)
		}

		select {
		case <-clock.After(timeBetweenReconciliationLoops):
		case <-ctx.Done():
			return nil
		}
	}
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

func (a *Server) setLabelsOnNodes(ctx context.Context, nodes []types.Server) (failedUpdates int, err error) {
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
			si, err := a.GetServerInfo(ctx, name)
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
		if err := a.updateLabelsOnNode(ctx, node, serverInfos); trace.IsNotFound(err) {
			failedUpdates++
		} else if err != nil {
			return failedUpdates, trace.Wrap(err)
		}
	}
	return failedUpdates, nil
}

func (a *Server) updateLabelsOnNode(ctx context.Context, node types.Server, serverInfos []types.ServerInfo) error {
	// Merge labels from server infos. Later label sets should override earlier
	// ones if they conflict.
	newLabels := make(map[string]string)
	for _, si := range serverInfos {
		maps.Copy(newLabels, si.GetNewLabels())
	}
	err := a.UpdateLabels(ctx, proto.InventoryUpdateLabelsRequest{
		ServerID: node.GetName(),
		Kind:     proto.LabelUpdateKind_SSHServerCloudLabels,
		Labels:   newLabels,
	})
	return trace.Wrap(err)
}
