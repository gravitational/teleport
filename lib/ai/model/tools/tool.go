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

package tools

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// proxyLookupClusterMaxSize is max the number of nodes in the cluster to attempt an opportunistic node lookup
	// in the proxy cache. We always do embedding lookups if the cluster is larger than this number.
	proxyLookupClusterMaxSize = 100
	maxEmbeddingsPerLookup    = 10

	// TODO(joel): remove/change when migrating to embeddings
	maxShownRequestableItems = 50
)

// *ToolContext contains various "data" which is commonly needed by various tools.
type ToolContext struct {
	assist.AssistEmbeddingServiceClient
	AccessRequestClient
	AccessPoint
	services.AccessChecker
	NodeWatcher NodeWatcher
	User        string
	ClusterName string
}

// NodeWatcher abstracts away services.NodeWatcher for testing purposes.
type NodeWatcher interface {
	// GetNodes returns a list of nodes that match the given filter.
	GetNodes(ctx context.Context, fn func(n services.Node) bool) []types.Server

	// NodeCount returns the number of nodes in the cluster.
	NodeCount() int
}

// AccessPoint allows reading resources from proxy cache.
type AccessPoint interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// AccessRequestClient abstracts away the access request client for testing purposes.
type AccessRequestClient interface {
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

// Tool is an interface that allows the agent to interact with the outside world.
// It is used to implement things such as vector document retrieval and command execution.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error)
}
