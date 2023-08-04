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
func (s *Server) ReconcileServerInfos(ctx context.Context) error {
	const batchSize = 100

	for {
		var failedUpdates int
		// Iterate over nodes in batches.
		nodeStream := s.StreamNodes(ctx, defaults.Namespace)
		var nodes []types.Server
		moreNodes := true
		for moreNodes {
			nodes, moreNodes = stream.Take(nodeStream, batchSize)
			updates, err := s.setCloudLabelsOnNodes(ctx, nodes)
			if err != nil {
				return trace.Wrap(err)
			}
			failedUpdates += updates

			select {
			case <-s.GetClock().After(time.Second):
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}

		// Log number of nodes that we couldn't find a control stream for.
		if failedUpdates > 0 {
			log.Debugf("unable to update labels on %v node(s) due to missing control stream", failedUpdates)
		}
	}
}

func (s *Server) setCloudLabelsOnNodes(ctx context.Context, nodes []types.Server) (failedUpdates int, err error) {
	for _, node := range nodes {
		meta := node.GetCloudMetadata()
		if meta != nil && meta.AWS != nil {
			si, err := s.GetServerInfo(ctx, meta.AWS.GetServerInfoName())
			if err == nil {
				err := s.updateLabelsOnNode(ctx, node, si)
				// Didn't find control stream for node, save count for logging.
				if trace.IsNotFound(err) {
					failedUpdates++
				} else if err != nil {
					return failedUpdates, trace.Wrap(err)
				}
			} else if !trace.IsNotFound(err) {
				return failedUpdates, trace.Wrap(err)
			}
		}
	}
	return failedUpdates, nil
}

func (s *Server) updateLabelsOnNode(ctx context.Context, node types.Server, si types.ServerInfo) error {
	err := s.UpdateLabels(ctx, proto.InventoryUpdateLabelsRequest{
		ServerID: node.GetName(),
		Kind:     proto.LabelUpdateKind_SSHServerCloudLabels,
		Labels:   si.GetStaticLabels(),
	})
	return trace.Wrap(err)
}
