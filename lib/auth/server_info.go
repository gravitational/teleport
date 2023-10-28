/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
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
// node.
func getServerInfoNames(node types.Server) []string {
	var names []string
	if meta := node.GetCloudMetadata(); meta != nil && meta.AWS != nil {
		names = append(names, types.ServerInfoNameFromAWS(meta.AWS.AccountID, meta.AWS.InstanceID))
	}
	// Manually added ServerInfos should override any other ServerInfos.
	names = append(names, types.ServerInfoNameFromNodeName(node.GetName()))
	return names
}

func (a *Server) setLabelsOnNodes(ctx context.Context, nodes []types.Server) (failedUpdates int, err error) {
	for _, node := range nodes {
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

		err := a.updateLabelsOnNode(ctx, node, serverInfos)
		// Didn't find control stream for node, save count for logging.
		if trace.IsNotFound(err) {
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
		for k, v := range si.GetNewLabels() {
			newLabels[k] = v
		}
	}
	err := a.UpdateLabels(ctx, proto.InventoryUpdateLabelsRequest{
		ServerID: node.GetName(),
		Kind:     proto.LabelUpdateKind_SSHServerCloudLabels,
		Labels:   newLabels,
	})
	return trace.Wrap(err)
}
